package articlemcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/albums"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

var (
	errAlbumMissing = errors.New("相册或照片不可访问")
	errAlbumInput   = errors.New("相册参数无效，或相册标识已被占用")
	errAlbumState   = errors.New("相册状态不允许此操作，请先核对发布状态")
)

type AlbumRef struct {
	AlbumID string `json:"albumId"`
}
type albumTx struct {
	q         *dbgen.Queries
	owner, id pgtype.UUID
}

func (s *Server) withAlbum(ctx context.Context, p principal, tool, rawID string, fn func(albumTx) error) error {
	if !p.albumScope {
		return errDenied
	}
	tx, err := s.store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	grant, err := q.LockMCPConnection(ctx, p.id)
	if errors.Is(err, pgx.ErrNoRows) {
		return errDenied
	}
	if err != nil {
		return err
	}
	if !slices.Contains(grant.DraftPermissions, tool) {
		return errDenied
	}
	a := albumTx{q: q, owner: grant.OwnerID}
	if tool != createAlbumTool && tool != listAlbumsTool {
		id, err := parseID(rawID)
		if err != nil {
			return errAlbumMissing
		}
		write := tool != getAlbumTool && tool != listPhotosTool
		if write {
			if err := q.LockAlbumImageWrites(ctx); err != nil {
				return err
			}
		}
		current, err := q.GetOwnedMCPAlbum(ctx, dbgen.GetOwnedMCPAlbumParams{AlbumID: id, ConnectionID: p.id})
		if errors.Is(err, pgx.ErrNoRows) {
			return errAlbumMissing
		}
		if err != nil {
			return err
		}
		if write {
			required := dbgen.PhotoAlbumStatusDraft
			if tool == unpublishAlbumTool {
				required = dbgen.PhotoAlbumStatusPublished
			}
			if current.Status != required {
				return errAlbumState
			}
		}
		a.id = id
	}
	if err := fn(a); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errAlbumMissing
		}
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" && strings.HasPrefix(pe.ConstraintName, "photo_albums_slug") {
			return errAlbumInput
		}
		return err
	}
	return tx.Commit(ctx)
}

type AlbumFields struct {
	Slug          string   `json:"slug"`
	Title         string   `json:"title"`
	TitleEn       string   `json:"titleEn,omitempty"`
	Description   string   `json:"description"`
	DescriptionEn string   `json:"descriptionEn,omitempty"`
	Tags          []string `json:"tags,omitempty"`
}

func (in AlbumFields) payload() (albums.AlbumPayload, error) {
	p := albums.AlbumPayload(in)
	if p.Normalize() != nil {
		return p, errAlbumInput
	}
	return p, nil
}

type AlbumCreate struct {
	RequestKey string `json:"requestKey"`
	AlbumFields
}
type AlbumEdit struct {
	AlbumRef
	AlbumFields
}
type AlbumDetails struct {
	AlbumRef
	AlbumFields
	Status       string `json:"status"`
	PhotoCount   int32  `json:"photoCount"`
	CoverPhotoID string `json:"coverPhotoId,omitempty"`
}

func optionalText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
func validKey(key string) bool          { return strings.TrimSpace(key) != "" && len(key) <= 128 }

func (s *Server) createAlbum(ctx context.Context, p principal, in AlbumCreate) (out AlbumRef, err error) {
	fields, err := in.payload()
	if err != nil || !validKey(in.RequestKey) {
		return out, errAlbumInput
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return out, err
	}
	hash := sha256.Sum256(raw)
	err = s.withAlbum(ctx, p, createAlbumTool, "", func(a albumTx) error {
		found, err := a.q.FindMCPAlbumRequest(ctx, dbgen.FindMCPAlbumRequestParams{ConnectionID: p.id, RequestKey: in.RequestKey})
		if err == nil {
			if !bytes.Equal(found.PayloadHash, hash[:]) {
				return errKey
			}
			out.AlbumID = found.AlbumID.String()
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		album, err := a.q.CreatePhotoAlbum(ctx, dbgen.CreatePhotoAlbumParams{Slug: fields.Slug, Title: fields.Title, Description: fields.Description, TitleEn: optionalText(fields.TitleEn), DescriptionEn: optionalText(fields.DescriptionEn), Tags: fields.Tags, CreatedBy: a.owner})
		if err != nil {
			return err
		}
		out.AlbumID = album.AlbumID.String()
		return a.q.BindMCPAlbum(ctx, dbgen.BindMCPAlbumParams{AlbumID: album.AlbumID, ConnectionID: p.id, RequestKey: in.RequestKey, PayloadHash: hash[:]})
	})
	return out, err
}
func (s *Server) getAlbum(ctx context.Context, p principal, in AlbumRef) (out AlbumDetails, err error) {
	err = s.withAlbum(ctx, p, getAlbumTool, in.AlbumID, func(a albumTx) error {
		row, err := a.q.GetPhotoAlbumByID(ctx, a.id)
		if err != nil {
			return err
		}
		out = AlbumDetails{AlbumRef: in, AlbumFields: AlbumFields{Slug: row.Slug, Title: row.Title, Description: row.Description, TitleEn: row.TitleEn.String, DescriptionEn: row.DescriptionEn.String, Tags: row.Tags}, Status: string(row.Status), PhotoCount: row.PhotoCount, CoverPhotoID: row.CoverPhotoID.String()}
		return nil
	})
	return out, err
}
func (s *Server) updateAlbum(ctx context.Context, p principal, in AlbumEdit) (AlbumRef, error) {
	f, err := in.payload()
	if err != nil {
		return AlbumRef{}, err
	}
	err = s.withAlbum(ctx, p, updateAlbumTool, in.AlbumID, func(a albumTx) error {
		_, err := a.q.UpdatePhotoAlbum(ctx, dbgen.UpdatePhotoAlbumParams{AlbumID: a.id, Slug: f.Slug, Title: f.Title, Description: f.Description, TitleEn: optionalText(f.TitleEn), DescriptionEn: optionalText(f.DescriptionEn), Tags: f.Tags})
		if errors.Is(err, pgx.ErrNoRows) {
			return errAlbumInput
		}
		return err
	})
	return in.AlbumRef, err
}
func (s *Server) albumState(tool string) func(context.Context, principal, AlbumRef) (AlbumRef, error) {
	return func(ctx context.Context, p principal, in AlbumRef) (AlbumRef, error) {
		err := s.withAlbum(ctx, p, tool, in.AlbumID, func(a albumTx) error {
			if tool == publishAlbumTool {
				return a.handler().PublishAlbumRecord(ctx, a.id)
			}
			_, err := a.q.UnpublishPhotoAlbum(ctx, a.id)
			return err
		})
		return in, err
	}
}

// 调用方已经持有当前事务的来源锁与相册行锁。
func (a albumTx) handler() albums.Handler {
	return albums.NewHandler(&albums.DatabaseStore{Queries: a.q})
}
