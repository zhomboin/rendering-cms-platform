package albums

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

var errPhotoOrder = errors.New("排序数组必须包含相册内全部照片且不重复")

func (h Handler) removeAlbum(ctx context.Context, albumID pgtype.UUID) ([]string, error) {
	var keys []string
	err := h.queries.WithinAlbumWrite(ctx, albumID, func(ctx context.Context, q albumStore) error {
		rows, err := q.ListAlbumAssetKeys(ctx, albumID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err := q.EnqueueAlbumObjectCleanup(ctx, dbgen.EnqueueAlbumObjectCleanupParams{
				StorageKey:    row.StorageKey,
				NextAttemptAt: pgtype.Timestamptz{Time: h.options.Now(), Valid: true},
			}); err != nil {
				return err
			}
			keys = append(keys, row.StorageKey)
		}
		if _, err := q.SoftDeleteAlbumAssets(ctx, albumID); err != nil {
			return err
		}
		return q.DeletePhotoAlbum(ctx, albumID)
	})
	return keys, err
}

func (h Handler) removePhoto(ctx context.Context, albumID, photoID pgtype.UUID) (string, error) {
	var key string
	err := h.queries.WithinAlbumWrite(ctx, albumID, func(ctx context.Context, q albumStore) error {
		photo, err := q.GetAlbumPhotoByID(ctx, dbgen.GetAlbumPhotoByIDParams{
			AlbumID: albumID,
			PhotoID: photoID,
		})
		if err != nil {
			return err
		}
		key = photo.StorageKey
		if err := q.ClearPhotoAlbumCoverIfRemoved(ctx, dbgen.ClearPhotoAlbumCoverIfRemovedParams{
			AlbumID:      albumID,
			CoverPhotoID: photoID,
		}); err != nil {
			return err
		}
		deleted, err := q.DeleteAlbumPhoto(ctx, dbgen.DeleteAlbumPhotoParams{
			AlbumID: albumID,
			PhotoID: photoID,
		})
		if err != nil {
			return err
		}
		if deleted == 0 {
			return pgx.ErrNoRows
		}
		if _, err := q.UpdateAssetStatus(ctx, dbgen.UpdateAssetStatusParams{
			AssetID:   photo.AssetID,
			Status:    dbgen.AssetStatusDeleted,
			DeletedAt: pgtype.Timestamptz{Time: h.options.Now(), Valid: true},
		}); err != nil {
			return err
		}
		return q.EnqueueAlbumObjectCleanup(ctx, dbgen.EnqueueAlbumObjectCleanupParams{
			StorageKey:    key,
			NextAttemptAt: pgtype.Timestamptz{Time: h.options.Now(), Valid: true},
		})
	})
	return key, err
}

func (h Handler) reorderPhotos(ctx context.Context, albumID pgtype.UUID, ordered []string) ([]dbgen.ListAlbumPhotosRow, error) {
	var result []dbgen.ListAlbumPhotosRow
	err := h.queries.WithinAlbumWrite(ctx, albumID, func(ctx context.Context, q albumStore) error {
		rows, err := q.ListAlbumPhotos(ctx, albumID)
		if err != nil {
			return err
		}
		if len(rows) != len(ordered) {
			return errPhotoOrder
		}
		known := make(map[pgtype.UUID]bool, len(rows))
		for _, row := range rows {
			known[row.PhotoID] = true
		}
		ids := make([]pgtype.UUID, 0, len(rows))
		positions := make([]int32, 0, len(rows))
		for index, raw := range ordered {
			id, err := uuidFromString(raw)
			if err != nil || !known[id] {
				return errPhotoOrder
			}
			delete(known, id)
			ids = append(ids, id)
			positions = append(positions, int32(index+1))
		}
		if err := q.ReorderAlbumPhotos(ctx, dbgen.ReorderAlbumPhotosParams{
			AlbumID:   albumID,
			PhotoIds:  ids,
			Positions: positions,
		}); err != nil {
			return err
		}
		result, err = q.ListAlbumPhotos(ctx, albumID)
		return err
	})
	return result, err
}
