package analytics

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

func TestAlbumViewLookupAndStorageFailures(t *testing.T) {
	for _, test := range []struct {
		name  string
		store analyticsStoreStub
		want  int
	}{
		{"missing", analyticsStoreStub{}, 404},
		{"lookup failure", analyticsStoreStub{albumLookupErr: errors.New("injected")}, 500},
		{"event failure", analyticsStoreStub{
			publishedAlbum: dbgen.GetPublishedPhotoAlbumBySlugRow{AlbumID: uuidForTest("11111111-1111-1111-1111-111111111111")},
			createEventErr: errors.New("injected"),
		}, 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := chi.NewRouter()
			NewHandler(&test.store).RegisterPublicRoutes(router)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/photos/example/views", nil))
			if response.Code != test.want || test.store.albumViewCalls != 0 || test.store.siteViewCalls != 0 {
				t.Fatalf("status=%d want=%d, invalid view counted", response.Code, test.want)
			}
		})
	}
}
