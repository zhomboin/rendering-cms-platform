package albums

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

func (h Handler) cleanupObject(ctx context.Context, key string) error {
	if h.store == nil {
		return nil
	}
	job, err := h.queries.ClaimAlbumObjectCleanup(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := h.store.DeleteObject(ctx, job.StorageKey); err != nil {
		delay := min(time.Hour, 5*time.Second<<min(job.Attempts, 10))
		retryErr := h.queries.RetryAlbumObjectCleanup(ctx, dbgen.RetryAlbumObjectCleanupParams{
			StorageKey:    job.StorageKey,
			LeaseToken:    job.LeaseToken,
			NextAttemptAt: pgtype.Timestamptz{Time: h.options.Now().Add(delay), Valid: true},
		})
		return errors.Join(err, retryErr)
	}
	return h.queries.CompleteAlbumObjectCleanup(ctx, dbgen.CompleteAlbumObjectCleanupParams{
		StorageKey: job.StorageKey,
		LeaseToken: job.LeaseToken,
	})
}

func (h Handler) cleanupObjectAfterWrite(ctx context.Context, key string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := h.cleanupObject(cleanupCtx, key); err != nil {
		slog.Error("album object cleanup pending", "storage_key", key, "error", err)
	}
}

// StartObjectCleanup 重试持久化清理任务，调用方取消后关闭返回通道。
func (h Handler) StartObjectCleanup(ctx context.Context, logger *slog.Logger) <-chan struct{} {
	done := make(chan struct{})
	if logger == nil {
		logger = slog.Default()
	}
	if h.store == nil {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := h.cleanupObject(jobCtx, "")
			cancel()
			if err != nil && ctx.Err() == nil {
				logger.Error("album object cleanup retry failed", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return done
}
