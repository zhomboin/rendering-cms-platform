package albums

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
	"rendering-cms-platform/backend/internal/testutil"
)

func TestCleanupSkipsLinkedImagesAndStopsWithContext(t *testing.T) {
	f := albumDatabaseFixture(t)
	photo := f.singlePhoto(t)
	if err := f.queries.EnqueueAlbumObjectCleanup(context.Background(), dbgen.EnqueueAlbumObjectCleanupParams{
		StorageKey:    photo.StorageKey,
		NextAttemptAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.handler.cleanupObject(context.Background(), photo.StorageKey); err != nil {
		t.Fatal(err)
	}
	if f.objects.deleted != 0 || len(f.objects.objects) != 1 {
		t.Fatal("cleanup deleted a still-linked photo")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := f.handler.StartObjectCleanup(ctx, nil)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup worker leaked after cancellation")
	}
}

func TestCleanupLeaseExpiryAndStaleCompletions(t *testing.T) {
	f := albumDatabaseFixture(t)
	ctx := context.Background()
	if err := f.queries.EnqueueAlbumObjectCleanup(ctx, dbgen.EnqueueAlbumObjectCleanupParams{
		StorageKey:    "synthetic-orphan",
		NextAttemptAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.queries.ClaimAlbumObjectCleanup(ctx, ""); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("worker claimed an in-flight upload intent", err)
	}
	first, err := f.queries.ClaimAlbumObjectCleanup(ctx, "synthetic-orphan")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.queries.ClaimAlbumObjectCleanup(ctx, "synthetic-orphan"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal("live cleanup lease was stolen", err)
	}
	testutil.Exec(t, f.pool, "update album_object_cleanup set lease_until = now() - interval '1 second'")
	second, err := f.queries.ClaimAlbumObjectCleanup(ctx, "synthetic-orphan")
	if err != nil || first.LeaseToken == second.LeaseToken || second.Attempts != first.Attempts+1 {
		t.Fatal("expired lease was not recovered", err)
	}
	if err := f.queries.CompleteAlbumObjectCleanup(ctx, dbgen.CompleteAlbumObjectCleanupParams{
		StorageKey: first.StorageKey, LeaseToken: first.LeaseToken,
	}); err != nil {
		t.Fatal(err)
	}
	if f.count(t, "album_object_cleanup") != 1 {
		t.Fatal("stale worker removed a newer lease")
	}
	if err := f.queries.CompleteAlbumObjectCleanup(ctx, dbgen.CompleteAlbumObjectCleanupParams{
		StorageKey: second.StorageKey, LeaseToken: second.LeaseToken,
	}); err != nil {
		t.Fatal(err)
	}
	if f.count(t, "album_object_cleanup") != 0 {
		t.Fatal("lease owner could not finish cleanup")
	}
}
