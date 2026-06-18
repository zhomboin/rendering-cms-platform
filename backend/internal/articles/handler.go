package articles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
)

type Handler struct {
	queries      articleStore
	generateSlug func() (string, error)
}

type ArticlePayload struct {
	Slug          string   `json:"slug"`
	ArticleName   string   `json:"articleName"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	BodyMdx       string   `json:"bodyMdx"`
	Tags          []string `json:"tags"`
	Featured      bool     `json:"featured"`
	FeaturedRank  *int32   `json:"featuredRank"`
	FeaturedAt    *string  `json:"featuredAt"`
	CoverImageURL string   `json:"coverImageUrl"`
}

type articleStore interface {
	ListPublishedArticles(ctx context.Context) ([]dbgen.ListPublishedArticlesRow, error)
	GetArticleBySlug(ctx context.Context, slug string) (dbgen.GetArticleBySlugRow, error)
	GetArticleByArticleName(ctx context.Context, articleName string) (dbgen.GetArticleByArticleNameRow, error)
	GetArticleByID(ctx context.Context, articleID pgtype.UUID) (dbgen.GetArticleByIDRow, error)
	ListAdminArticles(ctx context.Context) ([]dbgen.ListAdminArticlesRow, error)
	CreateDraftArticle(ctx context.Context, arg dbgen.CreateDraftArticleParams) (dbgen.CreateDraftArticleRow, error)
	UpdateDraftArticle(ctx context.Context, arg dbgen.UpdateDraftArticleParams) (dbgen.UpdateDraftArticleRow, error)
	PublishArticle(ctx context.Context, articleID pgtype.UUID) (dbgen.PublishArticleRow, error)
	SearchPublishedArticles(ctx context.Context, query string) ([]dbgen.SearchPublishedArticlesRow, error)
}

func NewHandler(queries articleStore) Handler {
	return NewHandlerWithSlugGenerator(queries, GenerateShortSlug)
}

func NewHandlerWithSlugGenerator(queries articleStore, generateSlug func() (string, error)) Handler {
	return Handler{queries: queries, generateSlug: generateSlug}
}

func (h Handler) RegisterPublicRoutes(router chi.Router) {
	router.Get("/api/v1/articles", h.listPublishedArticles)
	router.Get("/api/v1/articles/search", h.searchPublishedArticles)
	router.Get("/api/v1/articles/{slug}", h.getPublishedArticle)
}

func (h Handler) RegisterAdminRoutes(router chi.Router) {
	router.Get("/articles", h.listAdminArticles)
	router.Post("/articles", h.createDraftArticle)
	router.Patch("/articles/{id}", h.updateDraftArticle)
	router.Post("/articles/{id}/publish", h.publishArticle)
}

func (h Handler) listPublishedArticles(w http.ResponseWriter, r *http.Request) {
	articles, err := h.queries.ListPublishedArticles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "文章列表读取失败")
		return
	}
	writeJSON(w, http.StatusOK, mapArticles(articles))
}

func (h Handler) getPublishedArticle(w http.ResponseWriter, r *http.Request) {
	article, resolvedBy, err := h.resolvePublishedArticle(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "文章不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "文章读取失败")
		return
	}
	response := mapArticle(article)
	response["resolvedBy"] = resolvedBy
	response["canonicalSlug"] = article.Slug
	writeJSON(w, http.StatusOK, response)
}

func (h Handler) resolvePublishedArticle(ctx context.Context, identifier string) (articleView, string, error) {
	if ValidSlug(identifier) {
		article, err := h.queries.GetArticleBySlug(ctx, identifier)
		if err != nil {
			return articleView{}, "", err
		}
		return articleViewFromRow(article), "slug", nil
	}
	article, err := h.queries.GetArticleByArticleName(ctx, identifier)
	if err != nil {
		return articleView{}, "", err
	}
	return articleViewFromRow(article), "articleName", nil
}

func (h Handler) searchPublishedArticles(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, "搜索关键词不能为空")
		return
	}
	articles, err := h.queries.SearchPublishedArticles(r.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "文章搜索失败")
		return
	}
	writeJSON(w, http.StatusOK, mapSearchResults(articles))
}

func (h Handler) listAdminArticles(w http.ResponseWriter, r *http.Request) {
	articles, err := h.queries.ListAdminArticles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "后台文章列表读取失败")
		return
	}
	writeJSON(w, http.StatusOK, mapArticles(articles))
}

func (h Handler) createDraftArticle(w http.ResponseWriter, r *http.Request) {
	user, ok := httpapi.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "未登录")
		return
	}
	payload, ok := decodeArticlePayload(w, r)
	if !ok {
		return
	}
	authorID, err := uuidFromString(user.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "用户 ID 无效")
		return
	}

	for attempt := 0; attempt < 8; attempt++ {
		slug, err := h.generateSlug()
		if err != nil || !ValidSlug(slug) {
			writeError(w, http.StatusInternalServerError, "短链生成失败")
			return
		}

		article, err := h.queries.CreateDraftArticle(r.Context(), dbgen.CreateDraftArticleParams{
			Slug:          slug,
			ArticleName:   payload.ArticleName,
			Title:         payload.Title,
			Summary:       payload.Summary,
			BodyMdx:       payload.BodyMdx,
			Tags:          payload.Tags,
			Featured:      payload.Featured,
			FeaturedRank:  int32WithDefault(payload.FeaturedRank, 100),
			FeaturedAt:    nullableTimestamptzFromString(payload.FeaturedAt),
			CoverImageUrl: nullableText(payload.CoverImageURL),
			AuthorID:      authorID,
		})
		if err == nil {
			writeJSON(w, http.StatusCreated, mapArticle(article))
			return
		}
		if isArticleNameUniqueViolation(err) {
			writeError(w, http.StatusBadRequest, "文章英文名已存在")
			return
		}
		if !isSlugUniqueViolation(err) {
			writeError(w, http.StatusInternalServerError, "草稿创建失败")
			return
		}
	}
	writeError(w, http.StatusInternalServerError, "短链生成冲突过多")
}

func (h Handler) updateDraftArticle(w http.ResponseWriter, r *http.Request) {
	user, ok := httpapi.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "未登录")
		return
	}
	payload, ok := decodeArticlePayload(w, r)
	if !ok {
		return
	}
	articleID, err := uuidFromString(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "文章 ID 无效")
		return
	}
	if _, err := uuidFromString(user.UserID); err != nil {
		writeError(w, http.StatusUnauthorized, "用户 ID 无效")
		return
	}
	current, err := h.queries.GetArticleByID(r.Context(), articleID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "文章不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "文章读取失败")
		return
	}

	article, err := h.queries.UpdateDraftArticle(r.Context(), dbgen.UpdateDraftArticleParams{
		ArticleID:     articleID,
		Slug:          current.Slug,
		ArticleName:   payload.ArticleName,
		Title:         payload.Title,
		Summary:       payload.Summary,
		BodyMdx:       payload.BodyMdx,
		Tags:          payload.Tags,
		Featured:      payload.Featured,
		FeaturedRank:  int32WithDefault(payload.FeaturedRank, 100),
		FeaturedAt:    nullableTimestamptzFromString(payload.FeaturedAt),
		CoverImageUrl: nullableText(payload.CoverImageURL),
	})
	if err != nil {
		if isArticleNameUniqueViolation(err) {
			writeError(w, http.StatusBadRequest, "文章英文名已存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "草稿更新失败")
		return
	}
	writeJSON(w, http.StatusOK, mapArticle(article))
}

func (h Handler) publishArticle(w http.ResponseWriter, r *http.Request) {
	user, ok := httpapi.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "未登录")
		return
	}
	articleID, err := uuidFromString(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "文章 ID 无效")
		return
	}
	if _, err := uuidFromString(user.UserID); err != nil {
		writeError(w, http.StatusUnauthorized, "用户 ID 无效")
		return
	}

	article, err := h.queries.PublishArticle(r.Context(), articleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "文章发布失败")
		return
	}
	writeJSON(w, http.StatusOK, mapArticle(article))
}

func decodeArticlePayload(w http.ResponseWriter, r *http.Request) (ArticlePayload, bool) {
	var payload ArticlePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式不正确")
		return ArticlePayload{}, false
	}
	if payload.ArticleName == "" || payload.Title == "" || payload.BodyMdx == "" {
		writeError(w, http.StatusBadRequest, "英文名、标题和正文不能为空")
		return ArticlePayload{}, false
	}
	if !ValidArticleName(payload.ArticleName) {
		writeError(w, http.StatusBadRequest, "英文名只能使用小写字母、数字和中划线")
		return ArticlePayload{}, false
	}
	return payload, true
}

func isSlugUniqueViolation(err error) bool {
	return isUniqueViolation(err, "articles_slug")
}

func isArticleNameUniqueViolation(err error) bool {
	return isUniqueViolation(err, "articles_article_name")
}

func isUniqueViolation(err error, constraintPrefix string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, constraintPrefix)
}

func uuidFromString(value string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	if err := uuid.Scan(value); err != nil {
		return pgtype.UUID{}, err
	}
	return uuid, nil
}

func nullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func int32WithDefault(value *int32, defaultValue int32) int32 {
	if value == nil {
		return defaultValue
	}
	return *value
}

func nullableTimestamptzFromString(value *string) pgtype.Timestamptz {
	if value == nil || strings.TrimSpace(*value) == "" {
		return pgtype.Timestamptz{}
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	if err != nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: parsed, Valid: true}
}

func mapArticles[T articleRow](articles []T) []map[string]interface{} {
	response := make([]map[string]interface{}, 0, len(articles))
	for _, article := range articles {
		response = append(response, mapArticle(article))
	}
	return response
}

type articleRow interface {
	dbgen.CreateDraftArticleRow |
		dbgen.GetArticleByIDRow |
		dbgen.GetArticleByArticleNameRow |
		dbgen.GetArticleBySlugRow |
		dbgen.ListAdminArticlesRow |
		dbgen.ListPublishedArticlesRow |
		dbgen.PublishArticleRow |
		dbgen.UpdateDraftArticleRow |
		articleView
}

type articleView struct {
	ArticleID     pgtype.UUID
	Slug          string
	ArticleName   string
	Title         string
	Summary       string
	BodyMdx       string
	Status        dbgen.ArticleStatus
	Tags          []string
	Featured      bool
	FeaturedRank  int32
	FeaturedAt    pgtype.Timestamptz
	CoverImageUrl pgtype.Text
	PublishedAt   pgtype.Timestamptz
	AuthorID      pgtype.UUID
	CreatedAt     pgtype.Timestamptz
	UpdatedAt     pgtype.Timestamptz
	Version       int32
}

func mapArticle[T articleRow](row T) map[string]interface{} {
	article := articleViewFromRow(row)
	return map[string]interface{}{
		"articleId":     article.ArticleID.String(),
		"slug":          article.Slug,
		"canonicalSlug": article.Slug,
		"articleName":   article.ArticleName,
		"title":         article.Title,
		"summary":       article.Summary,
		"bodyMdx":       article.BodyMdx,
		"status":        string(article.Status),
		"version":       article.Version,
		"tags":          article.Tags,
		"featured":      article.Featured,
		"isFeatured":    article.Featured,
		"featuredRank":  article.FeaturedRank,
		"featuredAt":    timestamptzValue(article.FeaturedAt),
		"coverImageUrl": textValue(article.CoverImageUrl),
		"publishedAt":   timestamptzValue(article.PublishedAt),
		"authorId":      article.AuthorID.String(),
		"createdAt":     timestamptzValue(article.CreatedAt),
		"updatedAt":     timestamptzValue(article.UpdatedAt),
	}
}

func articleViewFromRow[T articleRow](row T) articleView {
	switch article := any(row).(type) {
	case dbgen.CreateDraftArticleRow:
		return articleView{
			ArticleID: article.ArticleID, Slug: article.Slug, ArticleName: article.ArticleName,
			Title: article.Title, Summary: article.Summary, BodyMdx: article.BodyMdx,
			Status: article.Status, Tags: article.Tags, Featured: article.Featured,
			FeaturedRank: article.FeaturedRank, FeaturedAt: article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl, PublishedAt: article.PublishedAt,
			AuthorID: article.AuthorID, CreatedAt: article.CreatedAt, UpdatedAt: article.UpdatedAt,
			Version: article.Version,
		}
	case dbgen.GetArticleByIDRow:
		return articleView{
			ArticleID: article.ArticleID, Slug: article.Slug, ArticleName: article.ArticleName,
			Title: article.Title, Summary: article.Summary, BodyMdx: article.BodyMdx,
			Status: article.Status, Tags: article.Tags, Featured: article.Featured,
			FeaturedRank: article.FeaturedRank, FeaturedAt: article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl, PublishedAt: article.PublishedAt,
			AuthorID: article.AuthorID, CreatedAt: article.CreatedAt, UpdatedAt: article.UpdatedAt,
			Version: article.Version,
		}
	case dbgen.GetArticleByArticleNameRow:
		return articleView{
			ArticleID: article.ArticleID, Slug: article.Slug, ArticleName: article.ArticleName,
			Title: article.Title, Summary: article.Summary, BodyMdx: article.BodyMdx,
			Status: article.Status, Tags: article.Tags, Featured: article.Featured,
			FeaturedRank: article.FeaturedRank, FeaturedAt: article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl, PublishedAt: article.PublishedAt,
			AuthorID: article.AuthorID, CreatedAt: article.CreatedAt, UpdatedAt: article.UpdatedAt,
			Version: article.Version,
		}
	case dbgen.GetArticleBySlugRow:
		return articleView{
			ArticleID: article.ArticleID, Slug: article.Slug, ArticleName: article.ArticleName,
			Title: article.Title, Summary: article.Summary, BodyMdx: article.BodyMdx,
			Status: article.Status, Tags: article.Tags, Featured: article.Featured,
			FeaturedRank: article.FeaturedRank, FeaturedAt: article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl, PublishedAt: article.PublishedAt,
			AuthorID: article.AuthorID, CreatedAt: article.CreatedAt, UpdatedAt: article.UpdatedAt,
			Version: article.Version,
		}
	case dbgen.ListAdminArticlesRow:
		return articleView{
			ArticleID: article.ArticleID, Slug: article.Slug, ArticleName: article.ArticleName,
			Title: article.Title, Summary: article.Summary, BodyMdx: article.BodyMdx,
			Status: article.Status, Tags: article.Tags, Featured: article.Featured,
			FeaturedRank: article.FeaturedRank, FeaturedAt: article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl, PublishedAt: article.PublishedAt,
			AuthorID: article.AuthorID, CreatedAt: article.CreatedAt, UpdatedAt: article.UpdatedAt,
			Version: article.Version,
		}
	case dbgen.ListPublishedArticlesRow:
		return articleView{
			ArticleID: article.ArticleID, Slug: article.Slug, ArticleName: article.ArticleName,
			Title: article.Title, Summary: article.Summary, BodyMdx: article.BodyMdx,
			Status: article.Status, Tags: article.Tags, Featured: article.Featured,
			FeaturedRank: article.FeaturedRank, FeaturedAt: article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl, PublishedAt: article.PublishedAt,
			AuthorID: article.AuthorID, CreatedAt: article.CreatedAt, UpdatedAt: article.UpdatedAt,
			Version: article.Version,
		}
	case dbgen.PublishArticleRow:
		return articleView{
			ArticleID: article.ArticleID, Slug: article.Slug, ArticleName: article.ArticleName,
			Title: article.Title, Summary: article.Summary, BodyMdx: article.BodyMdx,
			Status: article.Status, Tags: article.Tags, Featured: article.Featured,
			FeaturedRank: article.FeaturedRank, FeaturedAt: article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl, PublishedAt: article.PublishedAt,
			AuthorID: article.AuthorID, CreatedAt: article.CreatedAt, UpdatedAt: article.UpdatedAt,
			Version: article.Version,
		}
	case dbgen.UpdateDraftArticleRow:
		return articleView{
			ArticleID: article.ArticleID, Slug: article.Slug, ArticleName: article.ArticleName,
			Title: article.Title, Summary: article.Summary, BodyMdx: article.BodyMdx,
			Status: article.Status, Tags: article.Tags, Featured: article.Featured,
			FeaturedRank: article.FeaturedRank, FeaturedAt: article.FeaturedAt,
			CoverImageUrl: article.CoverImageUrl, PublishedAt: article.PublishedAt,
			AuthorID: article.AuthorID, CreatedAt: article.CreatedAt, UpdatedAt: article.UpdatedAt,
			Version: article.Version,
		}
	case articleView:
		return article
	default:
		panic("unsupported article row type")
	}
}

func textValue(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func timestamptzValue(value pgtype.Timestamptz) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
