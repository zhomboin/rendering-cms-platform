package albums

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
	"rendering-cms-platform/backend/internal/testutil"
)

func TestUploadDatabaseFailuresRollbackAndCompensate(t *testing.T) {
	for _, table := range []string{"assets", "album_photos"} {
		for _, deletionFails := range []bool{false, true} {
			t.Run(table+map[bool]string{true: "-retry", false: "-cleanup"}[deletionFails], func(t *testing.T) {
				f := albumDatabaseFixture(t)
				f.injectFailure(t, table, "insert")
				if deletionFails {
					f.objects.deleteErr = errTestObjectDelete
				}
				result := f.upload(t)
				if result.Code != 500 || f.count(t, "assets") != 0 || f.count(t, "album_photos") != 0 {
					t.Fatalf("status=%d, partial database write survived rollback", result.Code)
				}
				if deletionFails {
					if f.count(t, "album_object_cleanup") != 1 || len(f.objects.objects) != 1 {
						t.Fatal("failed cleanup was not persisted")
					}
					f.objects.deleteErr = nil
					testutil.Exec(t, f.pool, "update album_object_cleanup set next_attempt_at = now()")
					if err := f.handler.cleanupObject(context.Background(), ""); err != nil {
						t.Fatal(err)
					}
				}
				if f.count(t, "album_object_cleanup") != 0 || len(f.objects.objects) != 0 {
					t.Fatal("object or cleanup job survived successful compensation")
				}
			})
		}
	}
}

func TestUploadRegistrationFailureNeverWritesObject(t *testing.T) {
	f := albumDatabaseFixture(t)
	f.injectFailure(t, "album_object_cleanup", "insert")
	result := f.upload(t)
	if result.Code != 500 || len(f.objects.objects) != 0 || f.count(t, "assets") != 0 {
		t.Fatalf("status=%d: upload was not rejected before object write", result.Code)
	}
}

func TestUploadRechecksAlbumAndHandlesClientCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{true: "cancelled", false: "deleted"}[cancelled], func(t *testing.T) {
			f := albumDatabaseFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f.objects.afterPut = func() {
				if cancelled {
					cancel()
				} else if err := f.queries.DeletePhotoAlbum(context.Background(), f.id); err != nil {
					t.Error(err)
				}
			}
			request := multipartPhotoRequest(t, "/albums/"+f.id.String()+"/photos",
				map[string]string{"alt": "test"}, "test.png", encodeTestPNG(t, 2, 2), "image/png").WithContext(ctx)
			response := httptest.NewRecorder()
			authenticatedAlbumRouter(t, f.handler).ServeHTTP(response, request)
			if response.Code < 400 || f.count(t, "assets") != 0 || len(f.objects.objects) != 0 {
				t.Fatalf("status=%d: orphan survived failed upload", response.Code)
			}
		})
	}
}

type unknownCommitStore struct{ *DatabaseStore }

func (s unknownCommitStore) WithinAlbumWrite(ctx context.Context, id pgtype.UUID, write func(context.Context, albumStore) error) error {
	if err := s.DatabaseStore.WithinAlbumWrite(ctx, id, write); err != nil {
		return err
	}
	return errors.New("commit response lost")
}

func TestUnknownCommitResultDoesNotDeleteLinkedObject(t *testing.T) {
	f := albumDatabaseFixture(t)
	f.handler.queries = unknownCommitStore{NewDatabaseStore(f.pool)}
	response := f.upload(t)
	if response.Code != 500 || f.count(t, "album_photos") != 1 || len(f.objects.objects) != 1 || f.objects.deleted != 0 {
		t.Fatal("an ambiguous successful commit must not delete the linked image")
	}
}

func TestPhotoAndAlbumDeleteFailuresRollbackAllDatabaseWrites(t *testing.T) {
	for _, action := range []string{"photo", "album"} {
		for _, failure := range []struct{ table, operation string }{
			{"album_photos", "delete"}, {"assets", "update"}, {"album_object_cleanup", "insert"},
		} {
			t.Run(action+"-"+failure.table, func(t *testing.T) {
				f := albumDatabaseFixture(t)
				photo := f.singlePhoto(t)
				_, err := f.queries.SetPhotoAlbumCover(context.Background(), dbgen.SetPhotoAlbumCoverParams{
					AlbumID: f.id, CoverPhotoID: photo.PhotoID,
				})
				if err != nil {
					t.Fatal(err)
				}
				f.injectFailure(t, failure.table, failure.operation)
				path := "/albums/" + f.id.String()
				if action == "photo" {
					path += "/photos/" + photo.PhotoID.String()
				}
				response := httptest.NewRecorder()
				authenticatedAlbumRouter(t, f.handler).ServeHTTP(response, authRequest(t, http.MethodDelete, path, nil))
				album, err := f.queries.GetPhotoAlbumByID(context.Background(), f.id)
				asset, assetErr := f.queries.GetAssetByID(context.Background(), photo.AssetID)
				if response.Code != 500 || err != nil || assetErr != nil ||
					album.CoverPhotoID != photo.PhotoID || asset.Status != dbgen.AssetStatusActive ||
					f.count(t, "album_photos") != 1 || f.count(t, "album_object_cleanup") != 0 ||
					f.objects.deleted != 0 {
					t.Fatalf("delete failed without full rollback: status=%d error=%v", response.Code, err)
				}
			})
		}
	}
}

func TestDeleteCoverAndAlbumCommitBeforeObjectCleanup(t *testing.T) {
	for _, action := range []string{"photo", "album"} {
		t.Run(action, func(t *testing.T) {
			f := albumDatabaseFixture(t)
			photo := f.singlePhoto(t)
			_, err := f.queries.SetPhotoAlbumCover(context.Background(), dbgen.SetPhotoAlbumCoverParams{
				AlbumID: f.id, CoverPhotoID: photo.PhotoID,
			})
			if err != nil {
				t.Fatal(err)
			}
			f.objects.deleteErr = errTestObjectDelete
			path := "/albums/" + f.id.String()
			if action == "photo" {
				path += "/photos/" + photo.PhotoID.String()
			}
			response := httptest.NewRecorder()
			authenticatedAlbumRouter(t, f.handler).ServeHTTP(response, authRequest(t, http.MethodDelete, path, nil))
			if response.Code != 204 || f.count(t, "album_photos") != 0 || f.count(t, "album_object_cleanup") != 1 {
				t.Fatalf("status=%d: database deletion was not committed atomically", response.Code)
			}
			if action == "photo" {
				album, err := f.queries.GetPhotoAlbumByID(context.Background(), f.id)
				if err != nil || album.CoverPhotoID.Valid {
					t.Fatal("deleted cover was not cleared")
				}
			} else if f.count(t, "photo_albums") != 0 {
				t.Fatal("album was not deleted")
			}
			f.objects.deleteErr = nil
			if err := f.handler.cleanupObject(context.Background(), photo.StorageKey); err != nil {
				t.Fatal(err)
			}
			if f.count(t, "album_object_cleanup") != 0 || len(f.objects.objects) != 0 {
				t.Fatal("durable object cleanup did not recover")
			}
		})
	}
}

func TestConcurrentUploadsAndReordersHaveUniquePositions(t *testing.T) {
	f := albumDatabaseFixture(t)
	first := f.singlePhoto(t)
	router := authenticatedAlbumRouter(t, f.handler)
	var group sync.WaitGroup
	failures := make(chan int, 9)
	for index := 0; index < 8; index++ {
		request := multipartPhotoRequest(t, "/albums/"+f.id.String()+"/photos",
			map[string]string{"alt": "test"}, "test.png", encodeTestPNG(t, 2, 2), "image/png")
		group.Add(1)
		go func() {
			defer group.Done()
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != 201 {
				failures <- response.Code
			}
		}()
	}
	group.Add(1)
	go func() {
		defer group.Done()
		_, err := f.handler.reorderPhotos(context.Background(), f.id, []string{first.PhotoID.String()})
		if err != nil && !errors.Is(err, errPhotoOrder) {
			failures <- 500
		}
	}()
	group.Wait()
	close(failures)
	for status := range failures {
		t.Errorf("concurrent operation failed: %d", status)
	}
	rows, err := f.queries.ListAlbumPhotos(context.Background(), f.id)
	if err != nil || len(rows) != 9 {
		t.Fatalf("photos=%d err=%v", len(rows), err)
	}
	reversed := make([]string, len(rows))
	for index, row := range rows {
		if row.Position != int32(index+1) {
			t.Fatalf("duplicate or missing position: %d", row.Position)
		}
		reversed[len(rows)-1-index] = row.PhotoID.String()
	}
	reordered, err := f.handler.reorderPhotos(context.Background(), f.id, reversed)
	if err != nil {
		t.Fatal(err)
	}
	for index, row := range reordered {
		if row.PhotoID.String() != reversed[index] || row.Position != int32(index+1) {
			t.Fatal("reorder lost photos or failed to apply unique positions")
		}
	}
	if f.count(t, "assets") != 9 || f.count(t, "album_object_cleanup") != 0 {
		t.Fatal("concurrent upload left orphan records")
	}
}

func TestPublishedSlugDatabaseGuardRejectsConcurrentChange(t *testing.T) {
	f := albumDatabaseFixture(t)
	f.singlePhoto(t)
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := f.queries.WithTx(tx).PublishPhotoAlbum(context.Background(), f.id); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := f.queries.UpdatePhotoAlbum(context.Background(), dbgen.UpdatePhotoAlbumParams{
			AlbumID: f.id, Slug: "changed", Title: "测试", Description: "描述", Tags: []string{},
		})
		result <- err
	}()
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("concurrent rename was not rejected: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent update did not finish")
	}
	if _, err := f.queries.UnpublishPhotoAlbum(context.Background(), f.id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.queries.UpdatePhotoAlbum(context.Background(), dbgen.UpdatePhotoAlbumParams{
		AlbumID: f.id, Slug: "changed", Title: "测试", Description: "描述", Tags: []string{},
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("formerly published album accepted a new slug: %v", err)
	}
}

func TestDraftSlugAndPublishedMetadataRemainEditable(t *testing.T) {
	f := albumDatabaseFixture(t)
	ctx := context.Background()
	input := dbgen.UpdatePhotoAlbumParams{
		AlbumID:     f.id,
		Slug:        "draft-renamed",
		Title:       "草稿改名",
		Description: "描述",
		Tags:        []string{},
	}
	if _, err := f.queries.UpdatePhotoAlbum(ctx, input); err != nil {
		t.Fatal("never-published draft must accept a new slug", err)
	}
	f.singlePhoto(t)
	if err := f.handler.publishAlbumRecord(ctx, f.id); err != nil {
		t.Fatal(err)
	}
	input.Title = "发布后更新标题"
	if _, err := f.queries.UpdatePhotoAlbum(ctx, input); err != nil {
		t.Fatal("metadata with unchanged slug must remain editable", err)
	}
}
