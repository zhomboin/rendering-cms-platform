package albums

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
	"rendering-cms-platform/backend/internal/testutil"
)

func TestAlbumConsistencyMigrationRepairsDuplicatePositionsAndRoundTrips(t *testing.T) {
	f := albumDatabaseFixture(t)
	photo := f.singlePhoto(t)
	migration := func(suffix string) {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000017_album_write_consistency."+suffix+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		testutil.Exec(t, f.pool, string(raw))
	}
	migration("down")
	testutil.Exec(t, f.pool, "update album_photos set position=0")
	testutil.Exec(t, f.pool, "insert into album_photos(album_id,asset_id,alt,width,height,position) values($1,$2,'duplicate',2,2,0)", f.id, photo.AssetID)
	migration("up")
	rows, err := f.queries.ListAlbumPhotos(context.Background(), f.id)
	if err != nil || len(rows) != 2 || rows[0].Position != 1 || rows[1].Position != 2 {
		t.Fatal("migration did not repair duplicate positions", err)
	}
	tx, err := f.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(context.Background(), "insert into album_photos(album_id,asset_id,alt,width,height,position) values($1,$2,'duplicate',2,2,1)", f.id, photo.AssetID); err != nil {
		t.Fatal("constraint should be deferred until commit", err)
	}
	if err := tx.Commit(context.Background()); err == nil {
		t.Fatal("duplicate position was committed")
	}
	migration("down")
	migration("up")
}

func TestAlbumMigrationDownPreservesPendingCleanup(t *testing.T) {
	f := albumDatabaseFixture(t)
	if err := f.queries.EnqueueAlbumObjectCleanup(context.Background(), dbgen.EnqueueAlbumObjectCleanupParams{
		StorageKey:    "synthetic-pending-object",
		NextAttemptAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile(filepath.Join("..", "..", "migrations", "000017_album_write_consistency.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(context.Background(), string(down)); err == nil {
		t.Fatal("rollback discarded durable cleanup work")
	}
	if f.count(t, "album_object_cleanup") != 1 {
		t.Fatal("failed rollback changed pending cleanup")
	}
}
