package articles

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"

	"rendering-cms-platform/backend/internal/auth"
)

func TestDeleteArticle(t *testing.T) {
	const id = "11111111-1111-1111-1111-111111111111"
	const validPath = "/articles/" + id + "?expectedVersion=3"
	admin := adminToken(t)
	editor, err := auth.IssueToken("test-secret", id, "editor")
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("store failed")
	for _, tc := range []struct {
		name, token, path string
		store             articleStoreStub
		status            int
		queried           bool
	}{
		{"anonymous", "", validPath, articleStoreStub{}, 401, false},
		{"editor", editor, validPath, articleStoreStub{}, 403, false},
		{"invalid id", admin, "/articles/invalid?expectedVersion=3", articleStoreStub{}, 400, false},
		{"missing version", admin, "/articles/" + id, articleStoreStub{}, 400, false},
		{"invalid version", admin, "/articles/" + id + "?expectedVersion=0", articleStoreStub{}, 400, false},
		{"duplicate version", admin, validPath + "&expectedVersion=4", articleStoreStub{}, 400, false},
		{"success", admin, validPath, articleStoreStub{deleteRows: 1}, 204, true},
		{"missing", admin, validPath, articleStoreStub{getByIDErr: pgx.ErrNoRows}, 404, true},
		{"stale", admin, validPath, articleStoreStub{}, 409, true},
		{"delete failed", admin, validPath, articleStoreStub{deleteErr: failure}, 500, true},
		{"read failed", admin, validPath, articleStoreStub{getByIDErr: failure}, 500, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := authenticatedArticleRouter(t, NewHandler(&tc.store))
			req := httptest.NewRequest(http.MethodDelete, tc.path, nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.status || (rec.Code == http.StatusNoContent && rec.Body.Len() != 0) {
				t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
			}
			args := tc.store.deleteArgs
			if !tc.queried && len(args) != 0 {
				t.Fatalf("unexpected delete: %v", args)
			}
			if tc.queried && (len(args) != 1 || args[0].ArticleID.String() != id || args[0].ExpectedVersion != 3) {
				t.Fatalf("delete arguments: %v", args)
			}
		})
	}
}
