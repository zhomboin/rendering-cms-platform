package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	articleutil "rendering-cms-platform/backend/internal/articles"
	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
)

type Handler struct {
	queries analyticsStore
}

type analyticsStore interface {
	GetArticleBySlug(ctx context.Context, slug string) (dbgen.GetArticleBySlugRow, error)
	GetArticleByArticleName(ctx context.Context, articleName string) (dbgen.GetArticleByArticleNameRow, error)
	GetPublishedPhotoAlbumBySlug(ctx context.Context, slug string) (dbgen.GetPublishedPhotoAlbumBySlugRow, error)
	RecordAnalyticsView(context.Context, dbgen.RecordAnalyticsViewParams) (bool, error)
	GetTodaySiteViews(ctx context.Context) (int32, error)
	ListSiteViewsLast7Days(ctx context.Context) ([]dbgen.ListSiteViewsLast7DaysRow, error)
	ListHotArticles(ctx context.Context, limit int32) ([]dbgen.ListHotArticlesRow, error)
	ListArticleAnalyticsRows(ctx context.Context, days int32) ([]dbgen.ListArticleAnalyticsRowsRow, error)
	ListSiteViewTrend(ctx context.Context, days int32) ([]dbgen.ListSiteViewTrendRow, error)
	ListArticleViewTrend(ctx context.Context, days int32) ([]dbgen.ListArticleViewTrendRow, error)
}

const (
	analyticsEventArticleView = "article_view"
	analyticsEventSiteView    = "site_view"
	analyticsEventAlbumView   = "album_view"
)

func NewHandler(queries analyticsStore) Handler {
	return Handler{queries: queries}
}

func (h Handler) RegisterPublicRoutes(router chi.Router) {
	router.Post("/api/v1/articles/{slug}/views", h.recordArticleView)
	router.Post("/api/v1/photos/{slug}/views", h.recordAlbumView)
	router.Post("/api/v1/analytics/site-views", h.recordSiteView)
}

func (h Handler) RegisterAdminRoutes(router chi.Router) {
	router.Get("/analytics/summary", h.summary)
	router.Get("/analytics/articles", h.articleAnalytics)
	router.Get("/analytics/trend", h.trend)
}

func (h Handler) recordArticleView(w http.ResponseWriter, r *http.Request) {
	articleID, err := h.resolveArticleID(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "文章不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "文章读取失败")
		return
	}
	counted, err := h.recordAnalyticsEvent(r, analyticsEventArticleView, articleID, pgtype.UUID{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "访问事件记录失败")
		return
	}
	if !counted {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) resolveArticleID(ctx context.Context, identifier string) (pgtype.UUID, error) {
	if !articleutil.ValidPublishedIdentifier(identifier) {
		return pgtype.UUID{}, pgx.ErrNoRows
	}
	article, _, err := articleutil.ResolvePublishedArticle(ctx, h.queries, identifier)
	return article.ArticleID, err
}

// recordAlbumView 与文章浏览上报对称：仅对已发布相册计数，
// 计入相册日聚合与站点日聚合，返回 204。
func (h Handler) recordAlbumView(w http.ResponseWriter, r *http.Request) {
	album, err := h.queries.GetPublishedPhotoAlbumBySlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "相册不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "相册读取失败")
		return
	}
	counted, err := h.recordAnalyticsEvent(r, analyticsEventAlbumView, pgtype.UUID{}, album.AlbumID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "访问事件记录失败")
		return
	}
	if !counted {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) recordSiteView(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Path     string `json:"path"`
		Referrer string `json:"referrer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		// Rendering 静态站上报失败不应影响访问；扩展字段无效时仍记录一次站点 PV。
	}
	counted, err := h.recordAnalyticsEvent(r, analyticsEventSiteView, pgtype.UUID{}, pgtype.UUID{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "访问事件记录失败")
		return
	}
	if !counted {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) recordAnalyticsEvent(r *http.Request, eventType string, articleID pgtype.UUID, albumID pgtype.UUID) (bool, error) {
	return h.queries.RecordAnalyticsView(r.Context(), dbgen.RecordAnalyticsViewParams{
		ArticleID: articleID,
		AlbumID:   albumID,
		EventType: eventType,
		IpHash:    httpapi.ClientIPHash(r),
		UserAgent: nullableText(r.UserAgent()),
	})
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

func (h Handler) articleAnalytics(w http.ResponseWriter, r *http.Request) {
	days := normalizeArticleAnalyticsDays(r.URL.Query().Get("days"))
	articles, err := h.queries.ListArticleAnalyticsRows(r.Context(), days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "文章访问量读取失败")
		return
	}

	writeJSON(w, http.StatusOK, mapArticleAnalyticsRows(days, articles))
}

func (h Handler) trend(w http.ResponseWriter, r *http.Request) {
	days := normalizeAnalyticsTrendDays(r.URL.Query().Get("days"))
	site, err := h.queries.ListSiteViewTrend(r.Context(), days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "站点访问趋势读取失败")
		return
	}
	articles, err := h.queries.ListArticleViewTrend(r.Context(), days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "文章访问趋势读取失败")
		return
	}

	writeJSON(w, http.StatusOK, mapAnalyticsTrend(days, site, articles))
}

func mapDailyViews(days []dbgen.ListSiteViewsLast7DaysRow) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(days))
	for _, day := range days {
		result = append(result, map[string]interface{}{
			"date":  day.ViewDate.Time.Format("2006-01-02"),
			"views": day.Views,
		})
	}
	return result
}

func nullableText(value string) pgtype.Text {
	return pgtype.Text{
		String: value,
		Valid:  value != "",
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
