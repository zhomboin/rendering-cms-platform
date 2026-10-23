package albums

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

func authRequest(t *testing.T, method string, target string, body interface{}) *http.Request {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken(t))
	return req
}

func jsonUnmarshal(data []byte, target *map[string]interface{}) error {
	return json.Unmarshal(data, target)
}

type albumStoreStub struct {
	publishedTotal    int32
	publishedRows     []dbgen.ListPublishedPhotoAlbumsRow
	publishedAlbum    dbgen.GetPublishedPhotoAlbumBySlugRow
	publishedAlbumErr error
	photoTotal        int32
	photoRows         []dbgen.ListPublishedAlbumPhotosBySlugRow

	adminRows      []dbgen.ListAdminPhotoAlbumsRow
	albumByID      dbgen.GetPhotoAlbumByIDRow
	albumByIDErr   error
	photoCountByID int32

	albumPhotos  []dbgen.ListAlbumPhotosRow
	photoByID    dbgen.GetAlbumPhotoByIDRow
	photoByIDErr error
	deleteRows   int64
	assetKeys    []dbgen.ListAlbumAssetKeysRow

	createAlbumErr error
	updateAlbumErr error
	publishErr     error
	unpublishErr   error
	setCoverErr    error
	deleteAlbumErr error
	createAssetErr error
	createPhotoErr error
	updatePhotoErr error
	reorderErr     error
	softDeleteErr  error

	lastListLimit     int32
	lastListOffset    int32
	lastPhotoLimit    int32
	lastPhotoOffset   int32
	lastCreateAlbum   dbgen.CreatePhotoAlbumParams
	lastUpdateAlbum   dbgen.UpdatePhotoAlbumParams
	lastAssetCreate   dbgen.CreateAlbumPhotoAssetParams
	lastPhotoCreate   dbgen.CreateAlbumPhotoParams
	lastPhotoUpdate   dbgen.UpdateAlbumPhotoParams
	lastReorder       dbgen.ReorderAlbumPhotosParams
	lastCover         dbgen.SetPhotoAlbumCoverParams
	lastDeleteAlbumID pgtype.UUID
}

func (s *albumStoreStub) CountPublishedPhotoAlbums(ctx context.Context) (int32, error) {
	return s.publishedTotal, nil
}

func (s *albumStoreStub) ListPublishedPhotoAlbums(ctx context.Context, arg dbgen.ListPublishedPhotoAlbumsParams) ([]dbgen.ListPublishedPhotoAlbumsRow, error) {
	s.lastListLimit = arg.Limit
	s.lastListOffset = arg.Offset
	return s.publishedRows, nil
}

func (s *albumStoreStub) GetPublishedPhotoAlbumBySlug(ctx context.Context, slug string) (dbgen.GetPublishedPhotoAlbumBySlugRow, error) {
	if s.publishedAlbumErr != nil {
		return dbgen.GetPublishedPhotoAlbumBySlugRow{}, s.publishedAlbumErr
	}
	if s.publishedAlbum.Slug == "" {
		return dbgen.GetPublishedPhotoAlbumBySlugRow{}, pgx.ErrNoRows
	}
	return s.publishedAlbum, nil
}

func (s *albumStoreStub) CountPublishedAlbumPhotosBySlug(ctx context.Context, slug string) (int32, error) {
	return s.photoTotal, nil
}

func (s *albumStoreStub) ListPublishedAlbumPhotosBySlug(ctx context.Context, arg dbgen.ListPublishedAlbumPhotosBySlugParams) ([]dbgen.ListPublishedAlbumPhotosBySlugRow, error) {
	s.lastPhotoLimit = arg.Limit
	s.lastPhotoOffset = arg.Offset
	return s.photoRows, nil
}

func (s *albumStoreStub) ListAdminPhotoAlbums(ctx context.Context) ([]dbgen.ListAdminPhotoAlbumsRow, error) {
	return s.adminRows, nil
}

func (s *albumStoreStub) GetPhotoAlbumByID(ctx context.Context, albumID pgtype.UUID) (dbgen.GetPhotoAlbumByIDRow, error) {
	if s.albumByIDErr != nil {
		return dbgen.GetPhotoAlbumByIDRow{}, s.albumByIDErr
	}
	if !s.albumByID.AlbumID.Valid {
		return dbgen.GetPhotoAlbumByIDRow{}, pgx.ErrNoRows
	}
	return s.albumByID, nil
}

func (s *albumStoreStub) CreatePhotoAlbum(ctx context.Context, arg dbgen.CreatePhotoAlbumParams) (dbgen.PhotoAlbum, error) {
	s.lastCreateAlbum = arg
	if s.createAlbumErr != nil {
		return dbgen.PhotoAlbum{}, s.createAlbumErr
	}
	return dbgen.PhotoAlbum{
		AlbumID:       uuidForTest(testAlbumID),
		Slug:          arg.Slug,
		Title:         arg.Title,
		TitleEn:       arg.TitleEn,
		Description:   arg.Description,
		DescriptionEn: arg.DescriptionEn,
		Status:        dbgen.PhotoAlbumStatusDraft,
		Tags:          arg.Tags,
		CreatedBy:     arg.CreatedBy,
	}, nil
}

func (s *albumStoreStub) UpdatePhotoAlbum(ctx context.Context, arg dbgen.UpdatePhotoAlbumParams) (dbgen.PhotoAlbum, error) {
	s.lastUpdateAlbum = arg
	if s.updateAlbumErr != nil {
		return dbgen.PhotoAlbum{}, s.updateAlbumErr
	}
	return dbgen.PhotoAlbum{
		AlbumID:       arg.AlbumID,
		Slug:          arg.Slug,
		Title:         arg.Title,
		TitleEn:       arg.TitleEn,
		Description:   arg.Description,
		DescriptionEn: arg.DescriptionEn,
		Status:        dbgen.PhotoAlbumStatusDraft,
		Tags:          arg.Tags,
	}, nil
}

func (s *albumStoreStub) PublishPhotoAlbum(ctx context.Context, albumID pgtype.UUID) (dbgen.PhotoAlbum, error) {
	if s.publishErr != nil {
		return dbgen.PhotoAlbum{}, s.publishErr
	}
	return dbgen.PhotoAlbum{AlbumID: albumID, Status: dbgen.PhotoAlbumStatusPublished}, nil
}

func (s *albumStoreStub) UnpublishPhotoAlbum(ctx context.Context, albumID pgtype.UUID) (dbgen.PhotoAlbum, error) {
	if s.unpublishErr != nil {
		return dbgen.PhotoAlbum{}, s.unpublishErr
	}
	return dbgen.PhotoAlbum{AlbumID: albumID, Status: dbgen.PhotoAlbumStatusDraft}, nil
}

func (s *albumStoreStub) SetPhotoAlbumCover(ctx context.Context, arg dbgen.SetPhotoAlbumCoverParams) (dbgen.PhotoAlbum, error) {
	s.lastCover = arg
	if s.setCoverErr != nil {
		return dbgen.PhotoAlbum{}, s.setCoverErr
	}
	return dbgen.PhotoAlbum{AlbumID: arg.AlbumID, CoverPhotoID: arg.CoverPhotoID}, nil
}

func (s *albumStoreStub) DeletePhotoAlbum(ctx context.Context, albumID pgtype.UUID) error {
	s.lastDeleteAlbumID = albumID
	return s.deleteAlbumErr
}

func (s *albumStoreStub) CountAlbumPhotosByID(ctx context.Context, albumID pgtype.UUID) (int32, error) {
	return s.photoCountByID, nil
}

func (s *albumStoreStub) ListAlbumPhotos(ctx context.Context, albumID pgtype.UUID) ([]dbgen.ListAlbumPhotosRow, error) {
	return s.albumPhotos, nil
}

func (s *albumStoreStub) GetAlbumPhotoByID(ctx context.Context, arg dbgen.GetAlbumPhotoByIDParams) (dbgen.GetAlbumPhotoByIDRow, error) {
	if s.photoByIDErr != nil {
		return dbgen.GetAlbumPhotoByIDRow{}, s.photoByIDErr
	}
	if !s.photoByID.PhotoID.Valid {
		return dbgen.GetAlbumPhotoByIDRow{}, pgx.ErrNoRows
	}
	return s.photoByID, nil
}

func (s *albumStoreStub) CreateAlbumPhotoAsset(ctx context.Context, arg dbgen.CreateAlbumPhotoAssetParams) (dbgen.Asset, error) {
	s.lastAssetCreate = arg
	if s.createAssetErr != nil {
		return dbgen.Asset{}, s.createAssetErr
	}
	return dbgen.Asset{
		AssetID:     uuidForTest("44444444-4444-4444-4444-444444444444"),
		Filename:    arg.Filename,
		ContentType: arg.ContentType,
		ByteSize:    arg.ByteSize,
		StorageKey:  arg.StorageKey,
		PublicUrl:   arg.PublicUrl,
		CreatedBy:   arg.CreatedBy,
		Width:       arg.Width,
		Height:      arg.Height,
	}, nil
}

func (s *albumStoreStub) CreateAlbumPhoto(ctx context.Context, arg dbgen.CreateAlbumPhotoParams) (dbgen.AlbumPhoto, error) {
	s.lastPhotoCreate = arg
	if s.createPhotoErr != nil {
		return dbgen.AlbumPhoto{}, s.createPhotoErr
	}
	return dbgen.AlbumPhoto{
		PhotoID:  uuidForTest(testPhotoID),
		AlbumID:  arg.AlbumID,
		AssetID:  uuidForTest("44444444-4444-4444-4444-444444444444"),
		Alt:      arg.Alt,
		Caption:  arg.Caption,
		Width:    arg.Width,
		Height:   arg.Height,
		TakenAt:  arg.TakenAt,
		Position: 1,
	}, nil
}

func (s *albumStoreStub) UpdateAlbumPhoto(ctx context.Context, arg dbgen.UpdateAlbumPhotoParams) (dbgen.AlbumPhoto, error) {
	s.lastPhotoUpdate = arg
	if s.updatePhotoErr != nil {
		return dbgen.AlbumPhoto{}, s.updatePhotoErr
	}
	return dbgen.AlbumPhoto{PhotoID: arg.PhotoID, Alt: arg.Alt}, nil
}

func (s *albumStoreStub) DeleteAlbumPhoto(ctx context.Context, arg dbgen.DeleteAlbumPhotoParams) (int64, error) {
	if s.deleteRows == 0 {
		return 0, nil
	}
	return s.deleteRows, nil
}

func (s *albumStoreStub) ClearPhotoAlbumCoverIfRemoved(ctx context.Context, arg dbgen.ClearPhotoAlbumCoverIfRemovedParams) error {
	return nil
}

func (s *albumStoreStub) ReorderAlbumPhotos(ctx context.Context, arg dbgen.ReorderAlbumPhotosParams) error {
	s.lastReorder = arg
	return s.reorderErr
}

func (s *albumStoreStub) ListAlbumAssetKeys(ctx context.Context, albumID pgtype.UUID) ([]dbgen.ListAlbumAssetKeysRow, error) {
	return s.assetKeys, nil
}

func (s *albumStoreStub) SoftDeleteAlbumAssets(ctx context.Context, albumID pgtype.UUID) (int64, error) {
	return 0, s.softDeleteErr
}

func (s *albumStoreStub) UpdateAssetStatus(ctx context.Context, arg dbgen.UpdateAssetStatusParams) (dbgen.Asset, error) {
	return dbgen.Asset{AssetID: arg.AssetID, Status: arg.Status}, nil
}
