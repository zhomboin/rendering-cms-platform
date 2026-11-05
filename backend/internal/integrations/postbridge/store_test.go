package postbridge

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TEST_DATABASE_URL 指向允许创建隔离 schema 的测试库。
func postBridgeTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	if dsn := os.Getenv("POSTBRIDGE_TEST_DATABASE_URL"); dsn != "" {
		db, err := pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatal("open migrated PostBridge test database")
		}
		t.Cleanup(db.Close)
		return db
	}

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal("open isolated PostBridge test database")
	}
	schema := pgx.Identifier{"postbridge_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")}.Sanitize()
	t.Cleanup(func() {
		defer admin.Close(ctx)
		if _, err := admin.Exec(ctx, "drop schema if exists "+schema+" cascade"); err != nil {
			t.Errorf("drop PostBridge test schema: %v", err)
		}
	})
	if _, err := admin.Exec(ctx, "create schema "+schema); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("parse PostBridge test database settings")
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema + ", public"
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal("open PostBridge test schema")
	}
	t.Cleanup(db.Close)

	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "migrations", "*.up.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("locate migrations: %v", err)
	}
	for _, path := range paths {
		sql, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		// PostgreSQL 16 has gen_random_uuid built in. Keep parallel test
		// packages from racing to install a database-wide extension.
		migration := strings.TrimPrefix(string(sql), "create extension if not exists pgcrypto;")
		if _, err := db.Exec(ctx, migration); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(path), err)
		}
	}
	return db
}

func postBridgeDatabaseFixture(t *testing.T) (*Store, *pgxpool.Pool, string, string) {
	t.Helper()
	ctx := context.Background()
	db := postBridgeTestDatabase(t)

	key := uuid.NewString()
	var articleID string
	err := db.QueryRow(ctx, `
		with author as (
			insert into users (email, name, password_hash)
			values ($1, 'Fixture', 'synthetic')
			returning user_id
		)
		insert into articles (slug, article_name, title, summary, body_mdx, status, author_id)
		select $2, $3, 'Example title', '', 'Example body', 'published', user_id from author
		returning article_id::text
	`, key+"@example.invalid", key[:6], "fixture-"+key).Scan(&articleID)
	if err != nil {
		t.Fatal(err)
	}

	sourceID := "store-fixture-" + key
	t.Cleanup(func() {
		_, _ = db.Exec(ctx, "delete from articles where article_id::text = $1", articleID)
		_, _ = db.Exec(ctx, "delete from postbridge_events where source_id = $1", sourceID)
		_, _ = db.Exec(ctx, "delete from postbridge_attempts where source_id = $1", sourceID)
		_, _ = db.Exec(ctx, "delete from postbridge_article_states where source_id = $1", sourceID)
		_, _ = db.Exec(ctx, "delete from postbridge_connections where source_id = $1", sourceID)
		_, _ = db.Exec(ctx, "delete from users where email = $1", key+"@example.invalid")
	})
	return NewStore(db), db, sourceID, articleID
}

func createDatabaseAttempt(t *testing.T, store *Store, sourceID, articleID, requestID string) AttemptRecord {
	t.Helper()
	attempt, err := store.CreateAttempt(context.Background(), sourceID, articleID, requestID, func(article ArticleSource, seq int64, _ func([]string) error) (AttemptRecord, error) {
		if article.ID != articleID || article.Status != "published" || article.Version < 1 || seq < 1 {
			t.Fatalf("unexpected source: %+v, sequence %d", article, seq)
		}
		return AttemptRecord{
			ID: uuid.NewString(), TargetAccountID: "account-fixture", AuthorizationID: uuid.NewString(),
			AuthorizationExpiresAt: time.Now().Add(time.Hour), Status: "preparing", Data: []byte("{}"),
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return attempt
}

func TestPostBridgeArticleWriteAtomicallyInvalidatesGrants(t *testing.T) {
	store, db, sourceID, articleID := postBridgeDatabaseFixture(t)
	ctx := context.Background()
	first := createDatabaseAttempt(t, store, sourceID, articleID, "first")
	other := createDatabaseAttempt(t, store, sourceID, articleID, "other")
	if first.StateSeq != other.StateSeq {
		t.Fatal("one article revision must reuse its source sequence")
	}
	replayed, err := store.CreateAttempt(ctx, sourceID, articleID, first.RequestID, func(ArticleSource, int64, func([]string) error) (AttemptRecord, error) {
		t.Fatal("idempotent request rebuilt")
		return AttemptRecord{}, nil
	})
	if err != nil || replayed.ID != first.ID {
		t.Fatalf("replay = %v, %v", replayed.ID, err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "update articles set body_mdx = 'Rolled back' where article_id::text = $1", articleID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	before, err := store.GetAttempt(ctx, first.ID)
	if err != nil || before.RevokedAt != nil {
		t.Fatalf("rollback exposed grant revocation: %+v, %v", before, err)
	}
	if _, err := store.ClaimEvent(ctx, sourceID); !errors.Is(err, ErrStoreNotFound) {
		t.Fatalf("rollback exposed event: %v", err)
	}

	if _, err := db.Exec(ctx, "update articles set body_mdx = 'Updated public body' where article_id::text = $1", articleID); err != nil {
		t.Fatal(err)
	}
	for _, prior := range []AttemptRecord{first, other} {
		revoked, err := store.GetAttempt(ctx, prior.ID)
		if err != nil || revoked.RevokedAt == nil || !revoked.CancelPending {
			t.Fatalf("edit left authorization active: %+v, %v", revoked, err)
		}
	}
	replacement := createDatabaseAttempt(t, store, sourceID, articleID, "replacement")
	if replacement.SourceVersion <= first.SourceVersion || replacement.StateSeq <= first.StateSeq {
		t.Fatal("new revision did not get a newer sequence")
	}
	pending, err := store.HasPendingArticleEvents(ctx, sourceID, articleID, replacement.StateSeq)
	if err != nil || !pending {
		t.Fatalf("expected revision barrier, got %v, %v", pending, err)
	}
	seen := map[string]int{}
	for range 3 {
		event, err := store.ClaimEvent(ctx, sourceID)
		if err != nil {
			t.Fatal(err)
		}
		var payload struct {
			Type     string `json:"type"`
			StateSeq int64  `json:"state_seq"`
		}
		if err := json.Unmarshal(event.Body, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.StateSeq <= first.StateSeq || payload.StateSeq >= replacement.StateSeq {
			t.Fatalf("event sequence escaped source revision boundary: %+v", payload)
		}
		seen[payload.Type]++
		if payload.Type == "updated" {
			var snapshot ArticleSource
			if err := json.Unmarshal(event.Snapshot, &snapshot); err != nil || snapshot.Body != "Updated public body" {
				t.Fatalf("unexpected frozen article snapshot: %+v, %v", snapshot, err)
			}
		}
		if err := store.FinishEvent(ctx, event, nil, ""); err != nil {
			t.Fatal(err)
		}
	}
	pending, err = store.HasPendingArticleEvents(ctx, sourceID, articleID, replacement.StateSeq)
	if err != nil || pending {
		t.Fatalf("delivered event blocked replacement: %v, %v", pending, err)
	}
	if seen["updated"] != 1 || seen["authorization_revoked"] != 2 {
		t.Fatalf("unexpected event types: %v", seen)
	}
}

func TestPostBridgeLeasesPreserveConcurrentRevocation(t *testing.T) {
	store, db, sourceID, articleID := postBridgeDatabaseFixture(t)
	ctx := context.Background()
	attempt := createDatabaseAttempt(t, store, sourceID, articleID, "first")
	other := createDatabaseAttempt(t, store, sourceID, articleID, "other")
	claim, err := store.ClaimAttempt(ctx, attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimAttempt(ctx, attempt.ID); !errors.Is(err, ErrStoreBusy) {
		t.Fatalf("concurrent claim = %v", err)
	}
	if _, err := store.RevokeAttempt(ctx, attempt.ID); err != nil {
		t.Fatal(err)
	}
	saved, err := store.SaveAttempt(ctx, claim, nil, true)
	if err != nil || !saved.CancelPending || saved.RevokedAt == nil {
		t.Fatalf("stale save lost cancel: %+v, %v", saved, err)
	}
	// Make the unrelated preparation ineligible for the next due claim.
	if _, err := db.Exec(ctx, "update postbridge_attempts set next_sync_at = null where id = $1", other.ID); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := store.ClaimNextAttempt(ctx, sourceID)
	if err != nil || reclaimed.ID != attempt.ID || reclaimed.LeaseToken == claim.LeaseToken {
		t.Fatalf("revocation wasn't requeued: %+v, %v", reclaimed, err)
	}
	if _, err := store.SaveAttempt(ctx, claim, nil, true); !errors.Is(err, ErrStoreBusy) {
		t.Fatalf("stale lease update returned %v", err)
	}
	stillActive, err := store.GetAttempt(ctx, other.ID)
	if err != nil || stillActive.RevokedAt != nil {
		t.Fatalf("scoped revoke affected another authorization: %+v, %v", stillActive, err)
	}
	event, err := store.ClaimEvent(ctx, sourceID)
	if err != nil || len(event.Body) == 0 {
		t.Fatalf("revocation event: %+v, %v", event, err)
	}
	if _, err := store.ClaimEvent(ctx, sourceID); !errors.Is(err, ErrStoreNotFound) {
		t.Fatalf("leased event claimed again: %v", err)
	}
	if err := store.FinishEvent(ctx, event, nil, ""); err != nil {
		t.Fatal(err)
	}
}

func TestPostBridgeConnectionBindingRejectsDestinationChanges(t *testing.T) {
	store, _, sourceID, _ := postBridgeDatabaseFixture(t)
	ctx := context.Background()
	const origin = "https://bridge.example"
	if err := store.BindConnection(ctx, sourceID, "test", origin, "fixture-client"); err != nil {
		t.Fatal(err)
	}
	if err := store.BindConnection(ctx, sourceID, "test", origin, "fixture-client"); err != nil {
		t.Fatalf("same client and origin rejected: %v", err)
	}
	for _, values := range [][3]string{
		{"other", origin, "fixture-client"},
		{"test", "https://changed.example", "fixture-client"},
		{"test", origin, "changed-client"},
	} {
		if err := store.BindConnection(ctx, sourceID, values[0], values[1], values[2]); !errors.Is(err, ErrStoreConnectionMismatch) {
			t.Fatalf("unexpected rebind result: %v", err)
		}
	}
	if err := store.BindConnection(ctx, sourceID+"-other", "other", "https://changed.example", "changed-client"); !errors.Is(err, ErrStoreConnectionMismatch) {
		t.Fatalf("unexpected rebind with a different source: %v", err)
	}
	if err := store.BindConnection(ctx, sourceID, "test", origin, "fixture-client"); err != nil {
		t.Fatalf("original binding changed: %v", err)
	}
}
