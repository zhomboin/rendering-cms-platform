package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

func TestRecordArticleViewFallsBackFromArticleNameToShortSlugMapping(t *testing.T) {
	articleID := uuidForTest("11111111-1111-1111-1111-111111111111")
	store := &analyticsStoreStub{
		byArticleNameArticle: dbgen.GetArticleByArticleNameRow{
			ArticleID:   articleID,
			Slug:        "aB3dE9",
			ArticleName: "redis-sentinel-with-docker",
			Status:      dbgen.ArticleStatusPublished,
		},
	}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/articles/redis-sentinel-with-docker/views", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status code = %d, want %d, body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if store.lastArticleNameLookup != "redis-sentinel-with-docker" {
		t.Fatalf("articleName lookup = %q", store.lastArticleNameLookup)
	}
	if store.lastArticleViewID != articleID {
		t.Fatalf("article view id = %v, want mapped article id", store.lastArticleViewID)
	}
}

func TestTrendReturnsSiteAndArticleTrend(t *testing.T) {
	store := &analyticsStoreStub{
		siteTrendRows: []dbgen.ListSiteViewTrendRow{
			{ViewDate: pgtype.Date{Time: time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC), Valid: true}, Views: 42},
		},
		articleTrendRows: []dbgen.ListArticleViewTrendRow{
			{
				ViewDate: pgtype.Date{Time: time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC), Valid: true},
				Slug:     "postgres-search",
				Title:    "PostgreSQL 搜索增强",
				Views:    12,
			},
		},
	}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterAdminRoutes(router)
	req := httptest.NewRequest(http.MethodGet, "/analytics/trend?days=30", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.lastSiteTrendDays != 30 || store.lastArticleTrendDays != 30 {
		t.Fatalf("trend days = site:%d article:%d, want 30", store.lastSiteTrendDays, store.lastArticleTrendDays)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["days"] != float64(30) {
		t.Fatalf("days = %#v, want 30", body["days"])
	}
	if len(body["site"].([]interface{})) != 1 {
		t.Fatalf("site trend = %#v, want one row", body["site"])
	}
	if len(body["articles"].([]interface{})) != 1 {
		t.Fatalf("article trend = %#v, want one row", body["articles"])
	}
}

type analyticsStoreStub struct {
	siteTrendRows         []dbgen.ListSiteViewTrendRow
	articleTrendRows      []dbgen.ListArticleViewTrendRow
	byArticleNameArticle  dbgen.GetArticleByArticleNameRow
	lastArticleNameLookup string
	lastArticleViewID     pgtype.UUID
	lastSiteTrendDays     int32
	lastArticleTrendDays  int32
}

func (s *analyticsStoreStub) UpsertArticleViewDaily(ctx context.Context, arg dbgen.UpsertArticleViewDailyParams) error {
	s.lastArticleViewID = arg.ArticleID
	return nil
}

func (s *analyticsStoreStub) UpsertSiteViewDaily(ctx context.Context, arg dbgen.UpsertSiteViewDailyParams) error {
	return nil
}

func (s *analyticsStoreStub) GetArticleBySlug(ctx context.Context, slug string) (dbgen.GetArticleBySlugRow, error) {
	return dbgen.GetArticleBySlugRow{}, pgx.ErrNoRows
}

func (s *analyticsStoreStub) GetArticleByArticleName(ctx context.Context, articleName string) (dbgen.GetArticleByArticleNameRow, error) {
	s.lastArticleNameLookup = articleName
	if s.byArticleNameArticle.ArticleID.Valid {
		return s.byArticleNameArticle, nil
	}
	return dbgen.GetArticleByArticleNameRow{}, pgx.ErrNoRows
}

func (s *analyticsStoreStub) GetTodaySiteViews(ctx context.Context) (int32, error) {
	return 0, nil
}

func (s *analyticsStoreStub) ListSiteViewsLast7Days(ctx context.Context) ([]dbgen.ListSiteViewsLast7DaysRow, error) {
	return nil, nil
}

func (s *analyticsStoreStub) ListHotArticles(ctx context.Context, limit int32) ([]dbgen.ListHotArticlesRow, error) {
	return nil, nil
}

func (s *analyticsStoreStub) ListArticleAnalyticsRows(ctx context.Context, days int32) ([]dbgen.ListArticleAnalyticsRowsRow, error) {
	return nil, nil
}

func (s *analyticsStoreStub) ListSiteViewTrend(ctx context.Context, days int32) ([]dbgen.ListSiteViewTrendRow, error) {
	s.lastSiteTrendDays = days
	return s.siteTrendRows, nil
}

func (s *analyticsStoreStub) ListArticleViewTrend(ctx context.Context, days int32) ([]dbgen.ListArticleViewTrendRow, error) {
	s.lastArticleTrendDays = days
	return s.articleTrendRows, nil
}

func uuidForTest(value string) pgtype.UUID {
	var uuid pgtype.UUID
	if err := uuid.Scan(value); err != nil {
		panic(err)
	}
	return uuid
}
