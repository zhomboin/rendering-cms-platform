package albums

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

func TestFormerlyPublishedAlbumKeepsSlug(t *testing.T) {
	store := &albumStoreStub{albumByID: dbgen.GetPhotoAlbumByIDRow{
		AlbumID:     uuidForTest(testAlbumID),
		Slug:        "original",
		Status:      dbgen.PhotoAlbumStatusDraft,
		PublishedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}}
	router := authenticatedAlbumRouter(t, NewHandler(store))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, authRequest(t, http.MethodPatch, "/albums/"+testAlbumID,
		map[string]any{"slug": "changed", "title": "测试", "description": "描述"}))
	if response.Code != http.StatusConflict || store.lastUpdateAlbum.AlbumID.Valid {
		t.Fatalf("status=%d, immutable slug reached update", response.Code)
	}
}

func TestFailedUploadCleansObject(t *testing.T) {
	for _, stage := range []string{"asset", "photo"} {
		t.Run(stage, func(t *testing.T) {
			store := &albumStoreStub{albumByID: dbgen.GetPhotoAlbumByIDRow{
				AlbumID: uuidForTest(testAlbumID),
				Status:  dbgen.PhotoAlbumStatusDraft,
			}}
			if stage == "asset" {
				store.createAssetErr = errors.New("injected asset failure")
			} else {
				store.createPhotoErr = errors.New("injected photo failure")
			}
			handler, objects := newUploadHandler(t, store)
			router := authenticatedAlbumRouter(t, handler)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, multipartPhotoRequest(t, "/albums/"+testAlbumID+"/photos",
				map[string]string{"alt": "test"}, "test.png", encodeTestPNG(t, 2, 2), "image/png"))
			if response.Code != 500 || len(objects.putCalls) != 1 || len(objects.deleteCalls) != 1 {
				t.Fatalf("status=%d put=%d delete=%d", response.Code, len(objects.putCalls), len(objects.deleteCalls))
			}
			if len(store.cleanupJobs) != 0 {
				t.Fatal("successful compensation left a cleanup job")
			}
		})
	}
}
