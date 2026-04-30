package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"rendering-cms-platform/backend/internal/auth"
)

func TestAdminAuthMiddlewareRequiresBearerToken(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/articles", nil)
	rec := httptest.NewRecorder()

	AdminAuthMiddleware("secret-32-characters-minimum-value")(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAdminAuthMiddlewareAcceptsAdminToken(t *testing.T) {
	token, err := auth.IssueToken("secret-32-characters-minimum-value", "user-1", "admin")
	if err != nil {
		t.Fatalf("IssueToken() returned error: %v", err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			t.Fatal("expected user in context")
		}
		if user.UserID != "user-1" || user.Role != "admin" {
			t.Fatalf("unexpected user: %#v", user)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/articles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	AdminAuthMiddleware("secret-32-characters-minimum-value")(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
