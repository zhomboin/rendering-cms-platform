package articles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
)

type Handler struct {
	queries articleStore
}

type ArticlePayload struct {
	Slug          string   `json:"slug"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	BodyMdx       string   `json:"bodyMdx"`
	Tags          []string `json:"tags"`
	Featured      bool     `json:"featured"`
	CoverImageURL string   `json:"coverImageUrl"`
}

type articleStore interface {
	ListPublishedArticles(ctx context.Context) ([]dbgen.Article, error)
	GetArticleBySlug(ctx context.Context, slug string) (dbgen.Article, error)
	ListAdminArticles(ctx context.Context) ([]dbgen.Article, error)
	CreateDraftArticle(ctx context.Context, arg dbgen.CreateDraftArticleParams) (dbgen.Article, error)
	UpdateDraftArticle(ctx context.Context, arg dbgen.UpdateDraftArticleParams) (dbgen.Article, error)
	PublishArticle(ctx context.Context, articleID pgtype.UUID) (dbgen.Article, error)
	SearchPublishedArticles(ctx context.Context, query string) ([]dbgen.SearchPublishedArticlesRow, error)
}

func NewHandler(queries articleStore) Handler {
	return Handler{queries: queries}
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
	article, err := h.queries.GetArticleBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "文章不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "文章读取失败")
		return
	}
	writeJSON(w, http.StatusOK, mapArticle(article))
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

	article, err := h.queries.CreateDraftArticle(r.Context(), dbgen.CreateDraftArticleParams{
		Slug:          payload.Slug,
		Title:         payload.Title,
		Summary:       payload.Summary,
		BodyMdx:       payload.BodyMdx,
		Tags:          payload.Tags,
		Featured:      payload.Featured,
		CoverImageUrl: nullableText(payload.CoverImageURL),
		AuthorID:      authorID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "草稿创建失败")
		return
	}
	writeJSON(w, http.StatusCreated, mapArticle(article))
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

	article, err := h.queries.UpdateDraftArticle(r.Context(), dbgen.UpdateDraftArticleParams{
		ArticleID:     articleID,
		Slug:          payload.Slug,
		Title:         payload.Title,
		Summary:       payload.Summary,
		BodyMdx:       payload.BodyMdx,
		Tags:          payload.Tags,
		Featured:      payload.Featured,
		CoverImageUrl: nullableText(payload.CoverImageURL),
	})
	if err != nil {
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
	if payload.Title == "" || payload.Slug == "" || payload.BodyMdx == "" {
		writeError(w, http.StatusBadRequest, "标题、slug 和正文不能为空")
		return ArticlePayload{}, false
	}
	if !ValidSlug(payload.Slug) {
		writeError(w, http.StatusBadRequest, "slug 格式不正确")
		return ArticlePayload{}, false
	}
	return payload, true
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

func mapArticles(articles []dbgen.Article) []map[string]interface{} {
	response := make([]map[string]interface{}, 0, len(articles))
	for _, article := range articles {
		response = append(response, mapArticle(article))
	}
	return response
}

func mapArticle(article dbgen.Article) map[string]interface{} {
	return map[string]interface{}{
		"articleId":     article.ArticleID.String(),
		"slug":          article.Slug,
		"title":         article.Title,
		"summary":       article.Summary,
		"bodyMdx":       article.BodyMdx,
		"status":        string(article.Status),
		"version":       article.Version,
		"tags":          article.Tags,
		"featured":      article.Featured,
		"coverImageUrl": textValue(article.CoverImageUrl),
		"publishedAt":   timestamptzValue(article.PublishedAt),
		"authorId":      article.AuthorID.String(),
		"createdAt":     timestamptzValue(article.CreatedAt),
		"updatedAt":     timestamptzValue(article.UpdatedAt),
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
