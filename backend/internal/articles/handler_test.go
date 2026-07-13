package articles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/auth"
	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
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

func TestSearchPublishedArticlesRejectsQueriesOverOneHundredUnicodeCharacters(t *testing.T) {
	store := &articleStoreStub{}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)

	valid := httptest.NewRequest(http.MethodGet, "/api/v1/articles/search?q="+strings.Repeat("中", 100), nil)
	validRecorder := httptest.NewRecorder()
	router.ServeHTTP(validRecorder, valid)
	if validRecorder.Code != http.StatusOK {
		t.Fatalf("100-rune query status = %d, want %d", validRecorder.Code, http.StatusOK)
	}
	if store.searchCalls != 1 {
		t.Fatalf("search calls after valid query = %d, want 1", store.searchCalls)
	}

	tooLong := httptest.NewRequest(http.MethodGet, "/api/v1/articles/search?q="+strings.Repeat("中", 101), nil)
	tooLongRecorder := httptest.NewRecorder()
	router.ServeHTTP(tooLongRecorder, tooLong)
	if tooLongRecorder.Code != http.StatusBadRequest {
		t.Fatalf("101-rune query status = %d, want %d", tooLongRecorder.Code, http.StatusBadRequest)
	}
	if store.searchCalls != 1 {
		t.Fatalf("overlong query must not access store; calls = %d", store.searchCalls)
	}
}

func TestListPublishedArticlesReturnsHomeSectionFields(t *testing.T) {
	publishedAt := pgtype.Timestamptz{Time: time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC), Valid: true}
	featuredAt := pgtype.Timestamptz{Time: time.Date(2026, 6, 12, 8, 0, 0, 0, time.UTC), Valid: true}
	store := &articleStoreStub{
		publishedRows: []dbgen.ListPublishedArticlesRow{
			{
				ArticleID:    uuidForTest("11111111-1111-1111-1111-111111111111"),
				Slug:         "aB3dE9",
				ArticleName:  "redis-sentinel-with-docker",
				Title:        "短链文章",
				Summary:      "摘要",
				BodyMdx:      "正文",
				Status:       dbgen.ArticleStatusPublished,
				Tags:         []string{"go"},
				Featured:     true,
				FeaturedRank: 10,
				FeaturedAt:   featuredAt,
				PublishedAt:  publishedAt,
				AuthorID:     uuidForTest("22222222-2222-2222-2222-222222222222"),
				Version:      1,
			},
		},
	}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body []map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("article count = %d, want 1", len(body))
	}
	got := body[0]
	if got["slug"] != "aB3dE9" || got["canonicalSlug"] != "aB3dE9" {
		t.Fatalf("unexpected slug fields: %#v", got)
	}
	if got["isFeatured"] != true {
		t.Fatalf("isFeatured = %#v, want true", got["isFeatured"])
	}
	if got["featuredRank"] != float64(10) {
		t.Fatalf("featuredRank = %#v, want 10", got["featuredRank"])
	}
	if got["featuredAt"] == nil {
		t.Fatalf("featuredAt should be present: %#v", got)
	}
	for _, field := range []string{
		"slug", "canonicalSlug", "articleName", "title", "summary", "tags",
		"publishedAt", "updatedAt", "isFeatured", "featuredRank", "featuredAt", "coverImageUrl",
	} {
		if _, exists := got[field]; !exists {
			t.Errorf("public article summary is missing %q: %#v", field, got)
		}
	}
	for _, field := range []string{"bodyMdx", "authorId", "version", "articleId"} {
		if _, exists := got[field]; exists {
			t.Errorf("public article summary must not expose %q: %#v", field, got)
		}
	}
}

func TestPublicArticleListSupportsETagConditionalRequests(t *testing.T) {
	store := &articleStoreStub{publishedRows: []dbgen.ListPublishedArticlesRow{{
		ArticleID: uuidForTest("11111111-1111-1111-1111-111111111111"),
		Slug:      "aB3dE9", ArticleName: "cached-article", Title: "Cached", BodyMdx: "Body",
		Status: dbgen.ArticleStatusPublished, AuthorID: uuidForTest("22222222-2222-2222-2222-222222222222"),
	}}}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", first.Code)
	}
	if got := first.Header().Get("Cache-Control"); got != "public, max-age=60, stale-while-revalidate=300" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := first.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Fatalf("Vary = %q", got)
	}
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag is empty")
	}

	conditionalRequest := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	conditionalRequest.Header.Set("If-None-Match", etag)
	conditional := httptest.NewRecorder()
	router.ServeHTTP(conditional, conditionalRequest)
	if conditional.Code != http.StatusNotModified {
		t.Fatalf("conditional status = %d, want 304", conditional.Code)
	}
	if conditional.Body.Len() != 0 {
		t.Fatalf("304 body = %q, want empty", conditional.Body.String())
	}
}

func TestPublicArticleCacheHeadersPreserveCORSVaryOrigin(t *testing.T) {
	store := &articleStoreStub{}
	handler := NewHandler(store)
	router := httpapi.NewRouter(
		httpapi.WithFrontendOrigin("https://rendering.me"),
		httpapi.WithPublicRoutes(handler.RegisterPublicRoutes),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	req.Header.Set("Origin", "https://rendering.me")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	vary := strings.Join(recorder.Header().Values("Vary"), ",")
	if !strings.Contains(vary, "Origin") || !strings.Contains(vary, "Accept-Encoding") {
		t.Fatalf("Vary = %q, want Origin and Accept-Encoding", vary)
	}
}

func TestPublicArticleErrorsUseBoundedCachePolicies(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		handler := NewHandler(&articleStoreStub{})
		router := chi.NewRouter()
		handler.RegisterPublicRoutes(router)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/articles/missing-article", nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", recorder.Code)
		}
		if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=15" {
			t.Fatalf("Cache-Control = %q", got)
		}
	})

	t.Run("server error", func(t *testing.T) {
		handler := NewHandler(&articleStoreStub{listError: errors.New("database unavailable")})
		router := chi.NewRouter()
		handler.RegisterPublicRoutes(router)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil))
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}
		if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("Cache-Control = %q", got)
		}
	})
}

func TestCreateDraftArticleGeneratesShortSlugOnServer(t *testing.T) {
	store := &articleStoreStub{}
	handler := NewHandlerWithSlugGenerator(store, func() (string, error) {
		return "aB3dE9", nil
	})
	router := authenticatedArticleRouter(t, handler)
	req := httptest.NewRequest(http.MethodPost, "/articles", strings.NewReader(`{
		"slug": "../user-named-slug",
		"articleName": "redis-sentinel-with-docker",
		"title": "短链文章",
		"summary": "摘要",
		"bodyMdx": "正文",
		"tags": ["go"],
		"featured": false,
		"coverImageUrl": ""
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status code = %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if len(store.createArgs) != 1 {
		t.Fatalf("create calls = %d, want 1", len(store.createArgs))
	}
	if store.createArgs[0].Slug != "aB3dE9" {
		t.Fatalf("created slug = %q, want generated short slug", store.createArgs[0].Slug)
	}
	if store.createArgs[0].ArticleName != "redis-sentinel-with-docker" {
		t.Fatalf("created articleName = %q, want user English name", store.createArgs[0].ArticleName)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["slug"] != "aB3dE9" {
		t.Fatalf("response slug = %q, want generated short slug", body["slug"])
	}
	if body["articleName"] != "redis-sentinel-with-docker" {
		t.Fatalf("response articleName = %q, want user English name", body["articleName"])
	}
}

func TestUpdateDraftArticlePreservesExistingShortSlug(t *testing.T) {
	articleID := uuidForTest("11111111-1111-1111-1111-111111111111")
	store := &articleStoreStub{
		getByIDArticle: articleByIDForTest(articleID, "Z9yX8w", "旧标题"),
	}
	handler := NewHandlerWithSlugGenerator(store, func() (string, error) {
		return "aB3dE9", nil
	})
	router := authenticatedArticleRouter(t, handler)
	req := httptest.NewRequest(http.MethodPatch, "/articles/11111111-1111-1111-1111-111111111111", strings.NewReader(`{
		"slug": "abcdef",
		"articleName": "updated-english-name",
		"title": "更新标题",
		"summary": "更新摘要",
		"bodyMdx": "更新正文",
		"tags": ["go"],
		"featured": false,
		"coverImageUrl": ""
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(store.updateArgs) != 1 {
		t.Fatalf("update calls = %d, want 1", len(store.updateArgs))
	}
	if store.updateArgs[0].Slug != "Z9yX8w" {
		t.Fatalf("updated slug = %q, want existing short slug", store.updateArgs[0].Slug)
	}
	if store.updateArgs[0].ArticleName != "updated-english-name" {
		t.Fatalf("updated articleName = %q, want user English name", store.updateArgs[0].ArticleName)
	}
}

func TestUpdateDraftArticleRejectsPublishedArticle(t *testing.T) {
	articleID := uuidForTest("11111111-1111-1111-1111-111111111111")
	published := articleByIDForTest(articleID, "Z9yX8w", "线上标题")
	published.Status = dbgen.ArticleStatusPublished
	store := &articleStoreStub{
		getByIDArticle: published,
	}
	handler := NewHandler(store)
	router := authenticatedArticleRouter(t, handler)
	req := httptest.NewRequest(http.MethodPatch, "/articles/11111111-1111-1111-1111-111111111111", strings.NewReader(`{
		"articleName": "updated-english-name",
		"title": "更新标题",
		"summary": "更新摘要",
		"bodyMdx": "更新正文",
		"tags": ["go"],
		"featured": false,
		"coverImageUrl": ""
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status code = %d, want %d, body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if len(store.updateArgs) != 0 {
		t.Fatalf("update calls = %d, want 0", len(store.updateArgs))
	}
}

func TestGetPublishedArticleFallsBackFromArticleNameToShortSlug(t *testing.T) {
	articleID := uuidForTest("11111111-1111-1111-1111-111111111111")
	store := &articleStoreStub{
		byArticleNameArticle: publishedArticleByNameForTest(articleID, "aB3dE9", "redis-sentinel-with-docker", "短链文章"),
	}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/redis-sentinel-with-docker", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if rec.Header().Get("ETag") == "" {
		t.Fatal("detail ETag is empty")
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60, stale-while-revalidate=300" {
		t.Fatalf("detail Cache-Control = %q", got)
	}
	if store.lastArticleNameLookup != "redis-sentinel-with-docker" {
		t.Fatalf("articleName lookup = %q", store.lastArticleNameLookup)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["slug"] != "aB3dE9" || body["articleName"] != "redis-sentinel-with-docker" {
		t.Fatalf("unexpected mapped article response: %#v", body)
	}
	if body["bodyMdx"] != "正文" {
		t.Fatalf("bodyMdx = %#v, want full article body", body["bodyMdx"])
	}
	if body["canonicalSlug"] != "aB3dE9" || body["resolvedBy"] != "articleName" {
		t.Fatalf("unexpected resolution metadata: %#v", body)
	}
}

func TestResolvePublishedArticleRejectsInvalidIdentifiersWithoutStoreAccess(t *testing.T) {
	invalidIdentifiers := []string{
		strings.Repeat("a", 129),
		"legacy/route",
		`legacy\route`,
		"legacy name",
		"Legacy-Article-Name",
		"legacy--name",
		"legacy\x00name",
	}

	for _, identifier := range invalidIdentifiers {
		t.Run(identifier, func(t *testing.T) {
			store := &articleStoreStub{}
			handler := NewHandler(store)
			_, _, err := handler.resolvePublishedArticle(context.Background(), identifier)
			if err == nil {
				t.Fatal("resolvePublishedArticle error = nil, want invalid identifier")
			}
			if store.slugLookupCalls != 0 || store.articleNameLookupCalls != 0 {
				t.Fatalf("invalid identifier accessed store: slug=%d articleName=%d", store.slugLookupCalls, store.articleNameLookupCalls)
			}
		})
	}
}

type articleStoreStub struct {
	searchRows             []dbgen.SearchPublishedArticlesRow
	publishedRows          []dbgen.ListPublishedArticlesRow
	lastSearchQuery        string
	lastArticleNameLookup  string
	createArgs             []dbgen.CreateDraftArticleParams
	updateArgs             []dbgen.UpdateDraftArticleParams
	getByIDArticle         dbgen.GetArticleByIDRow
	byArticleNameArticle   dbgen.GetArticleByArticleNameRow
	searchCalls            int
	slugLookupCalls        int
	articleNameLookupCalls int
	listError              error
}

func (s *articleStoreStub) SearchPublishedArticles(ctx context.Context, query string) ([]dbgen.SearchPublishedArticlesRow, error) {
	s.searchCalls++
	s.lastSearchQuery = strings.TrimSpace(query)
	return s.searchRows, nil
}

func (s *articleStoreStub) ListPublishedArticles(ctx context.Context) ([]dbgen.ListPublishedArticlesRow, error) {
	if s.listError != nil {
		return nil, s.listError
	}
	return s.publishedRows, nil
}

func (s *articleStoreStub) GetArticleBySlug(ctx context.Context, slug string) (dbgen.GetArticleBySlugRow, error) {
	s.slugLookupCalls++
	return dbgen.GetArticleBySlugRow{}, pgx.ErrNoRows
}

func (s *articleStoreStub) GetArticleByArticleName(ctx context.Context, articleName string) (dbgen.GetArticleByArticleNameRow, error) {
	s.articleNameLookupCalls++
	s.lastArticleNameLookup = articleName
	if s.byArticleNameArticle.ArticleID.Valid {
		return s.byArticleNameArticle, nil
	}
	return dbgen.GetArticleByArticleNameRow{}, pgx.ErrNoRows
}

func (s *articleStoreStub) GetArticleByID(ctx context.Context, articleID pgtype.UUID) (dbgen.GetArticleByIDRow, error) {
	if s.getByIDArticle.ArticleID.Valid {
		return s.getByIDArticle, nil
	}
	return articleByIDForTest(articleID, "aB3dE9", "短链文章"), nil
}

func (s *articleStoreStub) ListAdminArticles(ctx context.Context) ([]dbgen.ListAdminArticlesRow, error) {
	return nil, nil
}

func (s *articleStoreStub) CreateDraftArticle(ctx context.Context, arg dbgen.CreateDraftArticleParams) (dbgen.CreateDraftArticleRow, error) {
	s.createArgs = append(s.createArgs, arg)
	return createArticleForTest(uuidForTest("11111111-1111-1111-1111-111111111111"), arg.Slug, arg.ArticleName, arg.Title), nil
}

func (s *articleStoreStub) UpdateDraftArticle(ctx context.Context, arg dbgen.UpdateDraftArticleParams) (dbgen.UpdateDraftArticleRow, error) {
	s.updateArgs = append(s.updateArgs, arg)
	return updateArticleForTest(arg.ArticleID, arg.Slug, arg.ArticleName, arg.Title), nil
}

func (s *articleStoreStub) PublishArticle(ctx context.Context, articleID pgtype.UUID) (dbgen.PublishArticleRow, error) {
	return dbgen.PublishArticleRow{}, nil
}

func uuidForTest(value string) pgtype.UUID {
	var uuid pgtype.UUID
	if err := uuid.Scan(value); err != nil {
		panic(err)
	}
	return uuid
}

func articleByIDForTest(articleID pgtype.UUID, slug string, title string) dbgen.GetArticleByIDRow {
	return dbgen.GetArticleByIDRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: "redis-sentinel-with-docker",
		Title:       title,
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusDraft,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func createArticleForTest(articleID pgtype.UUID, slug string, articleName string, title string) dbgen.CreateDraftArticleRow {
	return dbgen.CreateDraftArticleRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: articleName,
		Title:       title,
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusDraft,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func updateArticleForTest(articleID pgtype.UUID, slug string, articleName string, title string) dbgen.UpdateDraftArticleRow {
	return dbgen.UpdateDraftArticleRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: articleName,
		Title:       title,
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusDraft,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func publishedArticleByNameForTest(articleID pgtype.UUID, slug string, articleName string, title string) dbgen.GetArticleByArticleNameRow {
	return dbgen.GetArticleByArticleNameRow{
		ArticleID:   articleID,
		Slug:        slug,
		ArticleName: articleName,
		Title:       title,
		Summary:     "摘要",
		BodyMdx:     "正文",
		Status:      dbgen.ArticleStatusPublished,
		Tags:        []string{"go"},
		AuthorID:    uuidForTest("22222222-2222-2222-2222-222222222222"),
		Version:     1,
	}
}

func authenticatedArticleRouter(t *testing.T, handler Handler) chi.Router {
	t.Helper()
	router := chi.NewRouter()
	router.Use(httpapi.AdminAuthMiddleware("test-secret"))
	handler.RegisterAdminRoutes(router)
	return router
}

func adminToken(t *testing.T) string {
	t.Helper()
	token, err := auth.IssueToken("test-secret", "22222222-2222-2222-2222-222222222222", "admin")
	if err != nil {
		t.Fatal(err)
	}
	return token
}
