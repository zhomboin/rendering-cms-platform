package httpapi

import (
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouterUploadBodyLimits(t *testing.T) {
	for _, tc := range []struct {
		name, path, method string
		limit, size        int64
		unknown            bool
		want               int
	}{
		{"ordinary exact", "/api/v1/ordinary", "POST", 0, 25 << 20, false, 204},
		{"ordinary over", "/api/v1/ordinary", "POST", 0, (25 << 20) + 1, false, 413},
		{"ordinary chunked over", "/api/v1/ordinary", "POST", 0, (25 << 20) + 1, true, 413},
		{"photo exact", "/api/v1/admin/albums/123/photos", "POST", 0, 51 << 20, false, 204},
		{"photo chunked exact", "/api/v1/admin/albums/123/photos", "POST", 0, 51 << 20, true, 204},
		{"photo over", "/api/v1/admin/albums/123/photos", "POST", 0, (51 << 20) + 1, false, 413},
		{"photo chunked over", "/api/v1/admin/albums/123/photos", "POST", 0, (51 << 20) + 1, true, 413},
		{"suffix not exempt", "/api/v1/admin/albums/123/photos/order", "POST", 0, 26 << 20, false, 413},
		{"method not exempt", "/api/v1/admin/albums/123/photos", "PUT", 0, 26 << 20, false, 413},
		{"empty id not exempt", "/api/v1/admin/albums//photos", "POST", 0, 26 << 20, false, 413},
		{"explicit known over", "/api/v1/admin/albums/123/photos", "POST", 32, 33, false, 413},
		{"explicit exact", "/api/v1/admin/albums/123/photos", "POST", 32, 32, true, 204},
		{"explicit over", "/api/v1/admin/albums/123/photos", "POST", 32, 33, true, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					w.WriteHeader(413)
					return
				}
				w.WriteHeader(204)
			}
			router := NewRouter(func(c *RouterConfig) { c.RequestBodyLimit = tc.limit }, WithPublicRoutes(func(r chi.Router) { r.MethodFunc(tc.method, tc.path, handler) }))
			req := httptest.NewRequest(tc.method, tc.path, io.LimitReader(strings.NewReader(strings.Repeat("x", int(tc.size))), tc.size))
			req.ContentLength = tc.size
			if tc.unknown {
				req.ContentLength = -1
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d", rec.Code, tc.want)
			}
		})
	}
}
