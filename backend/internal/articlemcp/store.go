package articlemcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"rendering-cms-platform/backend/internal/articles"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

var (
	errDenied  = errors.New("连接无权执行此操作")
	errMissing = errors.New("文章不可访问")
	errVersion = errors.New("草稿状态或版本已变更，请重新读取")
	errKey     = errors.New("请求键已用于不同内容")
	errInput   = errors.New("请检查文章英文名、请求键、标题和正文")
	errName    = errors.New("无法使用此文章英文名")
)

type principal struct {
	id           pgtype.UUID
	publishScope bool
	albumScope   bool
}
type store struct {
	pool                    *pgxpool.Pool
	adminOrigin, blogOrigin string
}
type Content struct {
	Title   string   `json:"title"`
	Summary string   `json:"summary,omitempty"`
	Body    string   `json:"bodyMdx"`
	Tags    []string `json:"tags,omitempty"`
	Presentation
}
type CreateInput struct {
	RequestKey  string `json:"requestKey" jsonschema:"建稿及其重试使用相同请求键"`
	ArticleName string `json:"articleName"`
	Content
}
type ArticleInput struct {
	ArticleID string `json:"articleId"`
}
type VersionInput struct {
	ArticleID       string `json:"articleId"`
	ExpectedVersion int32  `json:"expectedVersion"`
}
type EditInput struct {
	VersionInput
	Title   *string  `json:"title,omitempty" jsonschema:"与 bodyMdx 一起提交以替换正文内容"`
	Body    *string  `json:"bodyMdx,omitempty" jsonschema:"与 title 一起提交；仅调整封面或精选时可省略两者"`
	Summary *string  `json:"summary,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Presentation
}
type Article struct {
	ArticleID                     string     `json:"articleId"`
	ArticleName                   string     `json:"articleName"`
	Slug                          string     `json:"slug"`
	Status                        string     `json:"status"`
	Version                       int32      `json:"version"`
	PreviewURL                    string     `json:"previewUrl"`
	PublicURL                     string     `json:"publicUrl,omitempty"`
	FeaturedAt                    *time.Time `json:"featuredAt,omitempty"`
	searchResult, searchCheckedAt string
	Content
}

func (c *Content) normalize() error {
	c.Title = strings.TrimSpace(c.Title)
	if c.Tags == nil {
		c.Tags = []string{}
	}
	if c.Title == "" || strings.TrimSpace(c.Body) == "" {
		return errInput
	}
	return nil
}

type action func(*dbgen.Queries, dbgen.LockMCPConnectionRow) (pgtype.UUID, error)

func (s store) run(ctx context.Context, p principal, tool string, op action) (Article, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Article{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	conn, err := q.LockMCPConnection(ctx, p.id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Article{}, errDenied
	}
	if err != nil {
		return Article{}, err
	}
	if tool == publishTool {
		if !p.publishScope || !conn.AllowPublish {
			return Article{}, errDenied
		}
	} else if !slices.Contains(conn.DraftPermissions, tool) {
		return Article{}, errDenied
	}
	id, err := op(q, conn)
	if err != nil {
		return Article{}, err
	}
	readAll := conn.ReadAll && (tool == readTool || tool == statusTool)
	row, err := accessibleArticle(ctx, q, p, id, readAll)
	if err != nil {
		return Article{}, err
	}
	a := Article{
		ArticleID:   id.String(),
		ArticleName: row.ArticleName,
		Slug:        row.Slug,
		Status:      string(row.Status),
		Version:     row.Version,
		PreviewURL:  s.adminOrigin + "/admin/articles/" + id.String() + "/read",
	}
	// 无读取授权时，建稿重试和写入回执只返回文章标识、版本和链接。
	if slices.Contains(conn.DraftPermissions, readTool) {
		a.Content = Content{Title: row.Title, Summary: row.Summary, Body: row.BodyMdx, Tags: row.Tags}
		a.Presentation = Presentation{CoverImageURL: &row.CoverImageUrl.String, Featured: &row.Featured, FeaturedRank: &row.FeaturedRank}
		if row.FeaturedAt.Valid {
			a.FeaturedAt = &row.FeaturedAt.Time
		}
	}
	if a.Status == "published" {
		a.PublicURL = s.blogOrigin + "/blog/" + row.Slug
		if row.SearchVersion.Valid && row.SearchVersion.Int32 == row.Version && row.SearchCheckedAt.Valid {
			a.searchResult = row.SearchResult.String
			a.searchCheckedAt = row.SearchCheckedAt.Time.UTC().Format(time.RFC3339)
		}
	}
	return a, tx.Commit(ctx)
}
func accessibleArticle(ctx context.Context, q *dbgen.Queries, p principal, id pgtype.UUID, readAll bool) (dbgen.GetMCPArticleRow, error) {
	a, err := q.GetMCPArticle(ctx, dbgen.GetMCPArticleParams{ConnectionID: p.id, ArticleID: id, ReadAll: readAll})
	if errors.Is(err, pgx.ErrNoRows) {
		err = errMissing
	}
	return a, err
}
func parseID(raw string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(raw); err != nil || !id.Valid {
		return id, errMissing
	}
	return id, nil
}
func (s store) create(ctx context.Context, p principal, in CreateInput) (Article, error) {
	in.ArticleName = strings.TrimSpace(in.ArticleName)
	if in.normalize() != nil || !articles.ValidArticleName(in.ArticleName) || len(in.ArticleName) > 128 || strings.TrimSpace(in.RequestKey) == "" || len(in.RequestKey) > 128 {
		return Article{}, errInput
	}
	if err := in.Presentation.validate(); err != nil {
		return Article{}, err
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return Article{}, err
	}
	hash := sha256.Sum256(raw)
	// 短链冲突会中止当前事务，使用新事务重试。
	for range 8 {
		a, err := s.run(ctx, p, createTool, func(q *dbgen.Queries, c dbgen.LockMCPConnectionRow) (pgtype.UUID, error) {
			prev, err := q.FindMCPRequest(ctx, dbgen.FindMCPRequestParams{ConnectionID: p.id, RequestKey: in.RequestKey})
			if err == nil {
				if !bytes.Equal(prev.PayloadHash, hash[:]) {
					return pgtype.UUID{}, errKey
				}
				if !prev.ArticleID.Valid {
					return pgtype.UUID{}, errMissing
				}
				return prev.ArticleID, nil
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return pgtype.UUID{}, err
			}
			slug, err := articles.GenerateShortSlug()
			if err != nil {
				return pgtype.UUID{}, err
			}
			display := storedPresentation{rank: 100}
			display.apply(in.Presentation)
			row, err := q.CreateDraftArticle(ctx, dbgen.CreateDraftArticleParams{
				AuthorID: c.OwnerID, Slug: slug, ArticleName: in.ArticleName,
				Title: in.Title, Summary: in.Summary, BodyMdx: in.Body, Tags: in.Tags,
				CoverImageUrl: display.cover, Featured: display.featured, FeaturedRank: display.rank, FeaturedAt: display.since,
			})
			if err != nil {
				return pgtype.UUID{}, err
			}
			return row.ArticleID, q.BindMCPDraft(ctx, dbgen.BindMCPDraftParams{ArticleID: row.ArticleID, ConnectionID: p.id, RequestKey: in.RequestKey, PayloadHash: hash[:]})
		})
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			if pe.ConstraintName == "articles_slug_key" {
				continue
			}
			return Article{}, errName
		}
		return a, err
	}
	return Article{}, errors.New("短链分配失败")
}
func (s store) read(ctx context.Context, p principal, raw string, tool string) (Article, error) {
	return s.run(ctx, p, tool, func(*dbgen.Queries, dbgen.LockMCPConnectionRow) (pgtype.UUID, error) { return parseID(raw) })
}
func (s store) change(ctx context.Context, p principal, in VersionInput, edit *EditInput) (Article, error) {
	id, err := parseID(in.ArticleID)
	if err != nil {
		return Article{}, err
	}
	tool := publishTool
	if edit != nil {
		tool = editTool
		if err := edit.validate(); err != nil {
			return Article{}, err
		}
	}
	return s.run(ctx, p, tool, func(q *dbgen.Queries, _ dbgen.LockMCPConnectionRow) (pgtype.UUID, error) {
		current, err := accessibleArticle(ctx, q, p, id, false)
		if err != nil {
			return id, err
		}
		if current.Status != dbgen.ArticleStatusDraft || current.Version != in.ExpectedVersion {
			return id, errVersion
		}
		if edit == nil {
			_, err = q.PublishArticle(ctx, dbgen.PublishArticleParams{ArticleID: id, ExpectedVersion: pgtype.Int4{Int32: in.ExpectedVersion, Valid: true}})
		} else {
			display := storedPresentation{cover: current.CoverImageUrl, featured: current.Featured, rank: current.FeaturedRank, since: current.FeaturedAt}
			display.apply(edit.Presentation)
			args := dbgen.EditMCPDraftParams{ArticleID: id, Version: in.ExpectedVersion,
				Title: current.Title, Summary: current.Summary, BodyMdx: current.BodyMdx, Tags: current.Tags,
				CoverImageUrl: display.cover, Featured: display.featured, FeaturedRank: display.rank, FeaturedAt: display.since}
			if edit.Title != nil {
				args.Title, args.BodyMdx = strings.TrimSpace(*edit.Title), *edit.Body
				args.Tags = append([]string{}, edit.Tags...)
				args.Summary = ""
				if edit.Summary != nil {
					args.Summary = *edit.Summary
				}
			}
			_, err = q.EditMCPDraft(ctx, args)
		}
		return id, err
	})
}

func (in EditInput) validate() error {
	if in.Title == nil && in.Body == nil {
		if in.Presentation == (Presentation{}) || in.Summary != nil || in.Tags != nil {
			return errEditFields
		}
	} else if in.Title == nil || in.Body == nil || strings.TrimSpace(*in.Title) == "" || strings.TrimSpace(*in.Body) == "" {
		return errEditFields
	}
	return in.Presentation.validate()
}
