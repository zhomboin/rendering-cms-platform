package analytics

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

type Handler struct {
	queries *dbgen.Queries
}

func NewHandler(queries *dbgen.Queries) Handler {
	return Handler{queries: queries}
}

func (h Handler) RegisterPublicRoutes(router chi.Router) {
	router.Post("/api/v1/articles/{slug}/views", h.recordArticleView)
}

func (h Handler) RegisterAdminRoutes(router chi.Router) {
	router.Get("/analytics/summary", h.summary)
}

func (h Handler) recordArticleView(w http.ResponseWriter, r *http.Request) {
	article, err := h.queries.GetArticleBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "文章不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "文章读取失败")
		return
	}

	today := pgtype.Date{Time: time.Now(), Valid: true}
	if err := h.queries.UpsertArticleViewDaily(r.Context(), dbgen.UpsertArticleViewDailyParams{
		ArticleID: article.ArticleID,
		ViewDate:  today,
		Views:     1,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "文章访问统计写入失败")
		return
	}
	if err := h.queries.UpsertSiteViewDaily(r.Context(), dbgen.UpsertSiteViewDailyParams{
		ViewDate: today,
		Views:    1,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "站点访问统计写入失败")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) summary(w http.ResponseWriter, r *http.Request) {
	todayViews, err := h.queries.GetTodaySiteViews(r.Context())
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "今日访问量读取失败")
		return
	}
	days, err := h.queries.ListSiteViewsLast7Days(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "近 7 天访问量读取失败")
		return
	}
	hotArticles, err := h.queries.ListHotArticles(r.Context(), 5)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "热门文章读取失败")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"todayViews": todayViews,
		"last7Days":  mapDailyViews(days),
		"hotArticles": func() []map[string]interface{} {
			result := make([]map[string]interface{}, 0, len(hotArticles))
			for index, article := range hotArticles {
				result = append(result, map[string]interface{}{
					"rank":  index + 1,
					"slug":  article.Slug,
					"title": article.Title,
					"views": article.Views,
				})
			}
			return result
		}(),
	})
}

func mapDailyViews(days []dbgen.SiteViewDaily) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(days))
	for _, day := range days {
		result = append(result, map[string]interface{}{
			"date":  day.ViewDate.Time.Format("2006-01-02"),
			"views": day.Views,
		})
	}
	return result
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
