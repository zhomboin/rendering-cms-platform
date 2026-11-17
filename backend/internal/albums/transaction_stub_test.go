package albums

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

func (s *albumStoreStub) WithinAlbumWrite(ctx context.Context, id pgtype.UUID, write func(context.Context, albumStore) error) error {
	return write(ctx, s)
}

func (s *albumStoreStub) EnqueueAlbumObjectCleanup(ctx context.Context, arg dbgen.EnqueueAlbumObjectCleanupParams) error {
	if s.cleanupJobs == nil {
		s.cleanupJobs = make(map[string]dbgen.EnqueueAlbumObjectCleanupParams)
	}
	s.cleanupJobs[arg.StorageKey] = arg
	return nil
}

func (s *albumStoreStub) CancelAlbumObjectCleanup(ctx context.Context, key string) (int64, error) {
	delete(s.cleanupJobs, key)
	return 1, nil
}

func (s *albumStoreStub) ClaimAlbumObjectCleanup(ctx context.Context, key string) (dbgen.ClaimAlbumObjectCleanupRow, error) {
	if _, ok := s.cleanupJobs[key]; !ok {
		return dbgen.ClaimAlbumObjectCleanupRow{}, pgx.ErrNoRows
	}
	return dbgen.ClaimAlbumObjectCleanupRow{StorageKey: key, LeaseToken: "synthetic-lease", Attempts: 1}, nil
}

func (s *albumStoreStub) CompleteAlbumObjectCleanup(ctx context.Context, arg dbgen.CompleteAlbumObjectCleanupParams) error {
	delete(s.cleanupJobs, arg.StorageKey)
	return nil
}

func (s *albumStoreStub) RetryAlbumObjectCleanup(ctx context.Context, arg dbgen.RetryAlbumObjectCleanupParams) error {
	return nil
}
