package postbridge

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const revocationPhotoURL = "https://images.example/album/revocation.png"

func insertRevocationPhoto(t *testing.T, pool *pgxpool.Pool, articleID, suffix string) (albumID, photoID, assetID string) {
	t.Helper()
	ctx := context.Background()
	if err := pool.QueryRow(ctx, `
		with album as (
			insert into photo_albums (slug, title, description, status, created_by)
			select article_name || $2, 'Album', '', 'published', author_id from articles where article_id::text = $1
			returning album_id
		), asset as (
			insert into assets (filename, content_type, byte_size, storage_key, public_url, created_by, width, height)
			select 'revocation.png', 'image/png', 100, article_name || $2 || '/revocation.png', $3, author_id, 1, 1
			from articles where article_id::text = $1 returning asset_id
		)
		insert into album_photos (album_id, asset_id, alt, width, height)
		select album_id, asset_id, 'Sample', 1, 1 from album, asset
		returning album_id::text, photo_id::text, asset_id::text
	`, articleID, suffix, revocationPhotoURL).Scan(&albumID, &photoID, &assetID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "delete from photo_albums where album_id::text = $1", albumID)
		_, _ = pool.Exec(ctx, "delete from assets where asset_id::text = $1", assetID)
	})
	return
}

func createImageRevocationAttempt(ctx context.Context, store *Store, sourceID, articleID, id, status string, verified func()) (AttemptRecord, error) {
	return store.CreateAttempt(ctx, sourceID, articleID, id, func(_ ArticleSource, _ int64, approve func([]string) error) (AttemptRecord, error) {
		urls := []string{revocationPhotoURL}
		if err := approve(urls); err != nil {
			return AttemptRecord{}, err
		}
		if verified != nil {
			verified()
		}
		data, err := json.Marshal(attemptData{ImageURLs: urls})
		return AttemptRecord{ID: id + "-" + articleID, TargetAccountID: "target-test", AuthorizationID: "grant:" + id + "-" + articleID,
			AuthorizationExpiresAt: time.Now().Add(time.Hour), Status: status, Data: data}, err
	})
}

func TestImageProvenanceChangeRevokesCompletedGrantAtomically(t *testing.T) {
	for _, tc := range []struct{ name, mutation string }{
		{"album", "update photo_albums set status = 'draft' where album_id::text = $1"},
		{"album_deleted", "delete from photo_albums where album_id::text = $1"},
		{"photo", "delete from album_photos where photo_id::text = $1"},
		{"asset", "update assets set status = 'archived' where asset_id::text = $1"},
		{"address", "update assets set public_url = 'https://images.example/album/changed.png' where asset_id::text = $1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, pool, sourceID, articleID := postBridgeDatabaseFixture(t)
			ctx := context.Background()
			albumID, photoID, assetID := insertRevocationPhoto(t, pool, articleID, "-photo")
			key := map[string]string{"album": albumID, "album_deleted": albumID, "photo": photoID, "asset": assetID, "address": assetID}[tc.name]
			attempt, err := createImageRevocationAttempt(ctx, store, sourceID, articleID, "image-completed", "completed", nil)
			if err != nil {
				t.Fatal(err)
			}
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, tc.mutation, key); err != nil {
				t.Fatal(err)
			}
			var eventCount int
			if err := tx.QueryRow(ctx, "select count(*) from postbridge_events where source_id=$1 and event_type='authorization_revoked'", sourceID).Scan(&eventCount); err != nil || eventCount != 1 {
				t.Fatalf("expected one event inside transaction, got %d, %v", eventCount, err)
			}
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			if current, err := store.GetAttempt(ctx, attempt.ID); err != nil || current.RevokedAt != nil {
				t.Fatalf("rolled back mutation revoked grant: %+v %v", current, err)
			}
			if _, err := pool.Exec(ctx, tc.mutation, key); err != nil {
				t.Fatal(err)
			}
			current, err := store.GetAttempt(ctx, attempt.ID)
			if err != nil || current.RevokedAt == nil || current.Status != "completed" || current.CancelPending {
				t.Fatalf("expected completed grant revoked without cancelling saved draft: %+v %v", current, err)
			}
			event, err := store.ClaimEvent(ctx, sourceID)
			var body SourceEvent
			if err != nil || json.Unmarshal(event.Body, &body) != nil || body.Type != "authorization_revoked" ||
				body.AuthorizationID != attempt.AuthorizationID || body.AuthorizationValid {
				t.Fatalf("unexpected scoped event: %+v %v", body, err)
			}
		})
	}
}

func TestImageRevocationDoesNotAdoptNewWorkerLease(t *testing.T) {
	store, pool, sourceID, articleID := postBridgeDatabaseFixture(t)
	ctx := context.Background()
	insertRevocationPhoto(t, pool, articleID, "-photo")
	attempt, err := createImageRevocationAttempt(ctx, store, sourceID, articleID, "stale-lease", "preparing", nil)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := store.ClaimAttempt(ctx, attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "update postbridge_attempts set lease_expires_at = clock_timestamp() - interval '1 second' where id = $1", attempt.ID); err != nil {
		t.Fatal(err)
	}
	newClaim, err := store.ClaimAttempt(ctx, attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil, sourceID, "test")
	if _, err := service.revokeImageAttempt(ctx, stale, attemptData{}); !errors.Is(err, ErrStoreBusy) {
		t.Fatalf("stale worker adopted current lease: %v", err)
	}
	current, err := store.GetAttempt(ctx, attempt.ID)
	if err != nil || current.RevokedAt != nil || current.LeaseToken != newClaim.LeaseToken {
		t.Fatalf("stale worker changed current record: %+v %v", current, err)
	}
}

func waitForImageInvalidationLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, pid uint32, result <-chan error) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-result:
			t.Fatalf("invalidation finished before the image transaction committed: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
			var blocked bool
			if err := pool.QueryRow(ctx, "select cardinality(pg_blocking_pids($1::int)) > 0", pid).Scan(&blocked); err != nil {
				t.Fatal(err)
			}
			if blocked {
				return
			}
		}
	}
}

func TestImageProvenanceSerializesConcurrentPreparationAndInvalidation(t *testing.T) {
	store, pool, sourceID, articleID := postBridgeDatabaseFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	albumID, _, _ := insertRevocationPhoto(t, pool, articleID, "-photo")
	verified := make(chan struct{})
	release := make(chan struct{})
	type result struct {
		attempt AttemptRecord
		err     error
	}
	created := make(chan result, 1)
	go func() {
		attempt, err := createImageRevocationAttempt(ctx, store, sourceID, articleID, "race", "preparing", func() {
			close(verified)
			select {
			case <-release:
			case <-ctx.Done():
			}
		})
		created <- result{attempt, err}
	}()
	select {
	case <-verified:
	case result := <-created:
		t.Fatalf("attempt failed before its photo was checked: %v", result.err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := conn.Conn().PgConn().PID()
	invalidationDone := make(chan struct{})
	defer func() {
		cancel()
		<-invalidationDone
		conn.Release()
	}()
	invalidated := make(chan error, 1)
	go func() {
		defer close(invalidationDone)
		_, err := conn.Exec(ctx, "update photo_albums set status = 'draft' where album_id::text = $1", albumID)
		invalidated <- err
	}()
	waitForImageInvalidationLock(t, ctx, pool, pid, invalidated)
	close(release)
	resultRow := <-created
	if resultRow.err != nil {
		t.Fatal(resultRow.err)
	}
	if err := <-invalidated; err != nil {
		t.Fatal(err)
	}
	current, err := store.GetAttempt(ctx, resultRow.attempt.ID)
	if err != nil || current.RevokedAt == nil || !current.CancelPending {
		t.Fatalf("concurrent invalidation missed the new grant: %+v %v", current, err)
	}
	if _, err := createImageRevocationAttempt(ctx, store, sourceID, articleID, "after", "preparing", nil); !errors.Is(err, ErrUnapprovedImage) {
		t.Fatalf("new grant approved after invalidation: %v", err)
	}
}

func TestImageProvenanceChecksOtherPublishedCopiesUnderConcurrentInvalidation(t *testing.T) {
	store, pool, sourceID, articleID := postBridgeDatabaseFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	firstAlbum, _, _ := insertRevocationPhoto(t, pool, articleID, "-first")
	secondAlbum, _, _ := insertRevocationPhoto(t, pool, articleID, "-second")
	attempt, err := createImageRevocationAttempt(ctx, store, sourceID, articleID, "duplicate-photo", "completed", nil)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	const unpublish = "update photo_albums set status = 'draft' where album_id::text = $1"
	if _, err := tx.Exec(ctx, unpublish, firstAlbum); err != nil {
		t.Fatal(err)
	}
	var revoked bool
	if err := tx.QueryRow(ctx, "select revoked_at is not null from postbridge_attempts where id=$1", attempt.ID).Scan(&revoked); err != nil || revoked {
		t.Fatalf("still-published duplicate did not preserve the grant: %v %v", revoked, err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pid := conn.Conn().PgConn().PID()
	invalidationDone := make(chan struct{})
	defer func() {
		cancel()
		<-invalidationDone
		conn.Release()
	}()
	invalidated := make(chan error, 1)
	go func() {
		defer close(invalidationDone)
		_, err := conn.Exec(ctx, unpublish, secondAlbum)
		invalidated <- err
	}()
	waitForImageInvalidationLock(t, ctx, pool, pid, invalidated)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-invalidated; err != nil {
		t.Fatal(err)
	}
	current, err := store.GetAttempt(ctx, attempt.ID)
	if err != nil || current.RevokedAt == nil || current.CancelPending {
		t.Fatalf("last published copy did not revoke the completed grant: %+v %v", current, err)
	}
}

func TestImageRevocationMigrationBackfillsAlreadyUnavailableGrants(t *testing.T) {
	if os.Getenv("POSTBRIDGE_TEST_DATABASE_URL") != "" {
		t.Skip("migration replay requires an isolated schema")
	}
	store, pool, sourceID, articleID := postBridgeDatabaseFixture(t)
	ctx := context.Background()
	albumID, _, _ := insertRevocationPhoto(t, pool, articleID, "-photo")
	attempt, err := createImageRevocationAttempt(ctx, store, sourceID, articleID, "backfill", "completed", nil)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(direction string) {
		t.Helper()
		migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "000016_postbridge_image_revocation."+direction+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	apply("down")
	if _, err := pool.Exec(ctx, "update photo_albums set status = 'draft' where album_id::text = $1", albumID); err != nil {
		t.Fatal(err)
	}
	if current, err := store.GetAttempt(ctx, attempt.ID); err != nil || current.RevokedAt != nil {
		t.Fatalf("older schema unexpectedly changed the grant: %+v %v", current, err)
	}
	apply("up")
	current, err := store.GetAttempt(ctx, attempt.ID)
	if err != nil || current.RevokedAt == nil || current.CancelPending {
		t.Fatalf("migration did not revoke the completed grant: %+v %v", current, err)
	}
	if _, err := store.ClaimEvent(ctx, sourceID); err != nil {
		t.Fatalf("migration did not queue the authorization event: %v", err)
	}
}

func TestImageInvalidationWithStaleRepeatableReadSnapshotRetries(t *testing.T) {
	store, pool, sourceID, articleID := postBridgeDatabaseFixture(t)
	ctx := context.Background()
	albumID, _, _ := insertRevocationPhoto(t, pool, articleID, "-photo")
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var status string
	if err := tx.QueryRow(ctx, "select status from photo_albums where album_id::text=$1", albumID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	attempt, err := createImageRevocationAttempt(ctx, store, sourceID, articleID, "newer-grant", "completed", nil)
	if err != nil {
		t.Fatal(err)
	}
	const unpublish = "update photo_albums set status='draft' where album_id::text=$1"
	var dbErr *pgconn.PgError
	if _, err := tx.Exec(ctx, unpublish, albumID); !errors.As(err, &dbErr) || dbErr.Code != "40001" {
		t.Fatalf("invalidation committed using stale snapshot: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, unpublish, albumID); err != nil {
		t.Fatal(err)
	}
	if current, err := store.GetAttempt(ctx, attempt.ID); err != nil || current.RevokedAt == nil {
		t.Fatalf("retried invalidation missed the newer grant: %v", err)
	}
}
