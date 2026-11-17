package albums

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

type DatabaseStore struct {
	*dbgen.Queries
	pool *pgxpool.Pool
}

func NewDatabaseStore(pool *pgxpool.Pool) *DatabaseStore {
	return &DatabaseStore{
		Queries: dbgen.New(pool),
		pool:    pool,
	}
}

func (s *DatabaseStore) WithinAlbumWrite(ctx context.Context, id pgtype.UUID, write func(context.Context, albumStore) error) error {
	if s.pool == nil {
		return write(ctx, s)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer func() {
		rollbackCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		_ = tx.Rollback(rollbackCtx)
	}()
	queries := s.Queries.WithTx(tx)
	if err := queries.LockAlbumImageWrites(ctx); err != nil {
		return err
	}
	if _, err := queries.LockPhotoAlbum(ctx, id); err != nil {
		return err
	}
	if err := write(ctx, &DatabaseStore{Queries: queries}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
