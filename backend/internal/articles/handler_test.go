package articles

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

func TestSearchPublishedArticlesRequiresQuery(t *testing.T) {
	handler := NewHandler(&articleStoreStub{})
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/search", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "搜索关键词不能为空" {
		t.Fatalf("error = %q, want 搜索关键词不能为空", body["error"])
	}
}

func TestSearchPublishedArticlesReturnsPublishedMatches(t *testing.T) {
	publishedAt := pgtype.Timestamptz{Time: time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC), Valid: true}
	store := &articleStoreStub{
		searchRows: []dbgen.SearchPublishedArticlesRow{
			{
				ArticleID:   uuidForTest("11111111-1111-1111-1111-111111111111"),
				Slug:        "postgres-search",
				Title:       "PostgreSQL 搜索增强",
				Summary:     "基于 full text search 的文章检索",
				PublishedAt: publishedAt,
			},
		},
	}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/search?q=postgres", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if store.lastSearchQuery != "postgres" {
		t.Fatalf("search query = %q, want postgres", store.lastSearchQuery)
	}
	var body []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("result count = %d, want 1", len(body))
	}
	got := body[0]
	if got["slug"] != "postgres-search" || got["title"] != "PostgreSQL 搜索增强" {
		t.Fatalf("unexpected result: %#v", got)
	}
	if _, exists := got["bodyMdx"]; exists {
		t.Fatalf("search result should not expose bodyMdx: %#v", got)
	}
}

type articleStoreStub struct {
	searchRows      []dbgen.SearchPublishedArticlesRow
	lastSearchQuery string
}

func (s *articleStoreStub) SearchPublishedArticles(ctx context.Context, query string) ([]dbgen.SearchPublishedArticlesRow, error) {
	s.lastSearchQuery = strings.TrimSpace(query)
	return s.searchRows, nil
}

func (s *articleStoreStub) ListPublishedArticles(ctx context.Context) ([]dbgen.Article, error) {
	return nil, nil
}

func (s *articleStoreStub) GetArticleBySlug(ctx context.Context, slug string) (dbgen.Article, error) {
	return dbgen.Article{}, nil
}

func (s *articleStoreStub) ListAdminArticles(ctx context.Context) ([]dbgen.Article, error) {
	return nil, nil
}

func (s *articleStoreStub) CreateDraftArticle(ctx context.Context, arg dbgen.CreateDraftArticleParams) (dbgen.Article, error) {
	return dbgen.Article{}, nil
}

func (s *articleStoreStub) UpdateDraftArticle(ctx context.Context, arg dbgen.UpdateDraftArticleParams) (dbgen.Article, error) {
	return dbgen.Article{}, nil
}

func (s *articleStoreStub) PublishArticle(ctx context.Context, articleID pgtype.UUID) (dbgen.Article, error) {
	return dbgen.Article{}, nil
}

func uuidForTest(value string) pgtype.UUID {
	var uuid pgtype.UUID
	if err := uuid.Scan(value); err != nil {
		panic(err)
	}
	return uuid
}
