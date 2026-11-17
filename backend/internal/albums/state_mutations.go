package albums

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

var (
	errAlbumPublished = errors.New("相册已发布")
	errAlbumEmpty     = errors.New("相册至少需要一张照片才能发布")
)

func (h Handler) publishAlbumRecord(ctx context.Context, id pgtype.UUID) error {
	return h.queries.WithinAlbumWrite(ctx, id, func(ctx context.Context, q albumStore) error {
		album, err := q.GetPhotoAlbumByID(ctx, id)
		if err != nil {
			return err
		}
		if album.Status == dbgen.PhotoAlbumStatusPublished {
			return errAlbumPublished
		}
		count, err := q.CountAlbumPhotosByID(ctx, id)
		if err != nil {
			return err
		}
		if count == 0 {
			return errAlbumEmpty
		}
		_, err = q.PublishPhotoAlbum(ctx, id)
		return err
	})
}

func (h Handler) setCoverPhoto(ctx context.Context, id, photoID pgtype.UUID) error {
	return h.queries.WithinAlbumWrite(ctx, id, func(ctx context.Context, q albumStore) error {
		if _, err := q.GetPhotoAlbumByID(ctx, id); err != nil {
			return err
		}
		if photoID.Valid {
			if _, err := q.GetAlbumPhotoByID(ctx, dbgen.GetAlbumPhotoByIDParams{
				AlbumID: id,
				PhotoID: photoID,
			}); err != nil {
				return err
			}
		}
		_, err := q.SetPhotoAlbumCover(ctx, dbgen.SetPhotoAlbumCoverParams{
			AlbumID:      id,
			CoverPhotoID: photoID,
		})
		return err
	})
}
