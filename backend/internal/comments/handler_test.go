package comments

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

func TestCreateCommentReturnsTooManyRequestsWhenIPHashIsOverLimit(t *testing.T) {
	now := time.Now()
	store := &commentStoreStub{
		recentCommentTimes: []pgtype.Timestamptz{
			{Time: now.Add(-10 * time.Second), Valid: true},
			{Time: now.Add(-20 * time.Second), Valid: true},
			{Time: now.Add(-30 * time.Second), Valid: true},
		},
	}
	handler := NewHandler(store)
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/articles/example/comments", strings.NewReader(`{
		"authorName": "Alice",
		"body": "hello"
	}`))
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if store.createCalled {
		t.Fatal("CreateComment should not be called when request is over limit")
	}
}

type commentStoreStub struct {
	recentCommentTimes []pgtype.Timestamptz
	createCalled       bool
}

func (s *commentStoreStub) ListRecentCommentTimesByIPHash(ctx context.Context, arg dbgen.ListRecentCommentTimesByIPHashParams) ([]pgtype.Timestamptz, error) {
	return s.recentCommentTimes, nil
}

func (s *commentStoreStub) CreateComment(ctx context.Context, arg dbgen.CreateCommentParams) (dbgen.Comment, error) {
	s.createCalled = true
	return dbgen.Comment{}, nil
}

func (s *commentStoreStub) ListApprovedCommentsByArticleSlug(ctx context.Context, slug string) ([]dbgen.ListApprovedCommentsByArticleSlugRow, error) {
	return nil, nil
}

func (s *commentStoreStub) ListAdminComments(ctx context.Context) ([]dbgen.ListAdminCommentsRow, error) {
	return nil, nil
}

func (s *commentStoreStub) ReviewComment(ctx context.Context, arg dbgen.ReviewCommentParams) (dbgen.Comment, error) {
	return dbgen.Comment{}, nil
}
