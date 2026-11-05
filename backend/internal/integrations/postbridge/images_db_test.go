package postbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"testing"
	"time"
)

func TestImageDraftRechecksPublishedPhotoAndQueuesRevocation(t *testing.T) {
	store, pool, sourceID, articleID := postBridgeDatabaseFixture(t)
	ctx := context.Background()
	const photo = "https://images.example/album/Sample.png"
	body := "Paragraph.\n\n![Sample](" + photo + ")"
	var version int64
	if err := pool.QueryRow(ctx, "update articles set body_mdx = $2 where article_id::text = $1 returning version", articleID, body).Scan(&version); err != nil {
		t.Fatal(err)
	}

	var prep Preparation
	var events []SourceEvent
	imagesEnabled := false
	prepared := 0
	client := testClient(t)
	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method + " " + req.URL.Path {
		case "GET /api/integrations/v1/targets":
			return workflowResponse(http.StatusOK, map[string][]Target{"items": {{
				Platform: "csdn", AccountID: "target-test", Images: imagesEnabled,
				Operations: []string{"save_draft"}, BodyFormats: []string{"markdown"},
			}}}), nil
		case "POST /api/integrations/v1/preparations":
			prepared++
			if err := json.NewDecoder(req.Body).Decode(&prep.PreparationInput); err != nil {
				return nil, err
			}
			prep.ID, prep.Status = "image-preparation", "prepared"
			prep.ExpiresAt = time.Now().Add(time.Hour)
			return workflowResponse(http.StatusCreated, prep), nil
		case "GET /api/integrations/v1/preparations/image-preparation":
			prep.Confirmed, prep.Status = true, "confirmed"
			prep.DraftRevisionIDs = []string{"image-revision"}
			prep.ConfirmationFingerprint = "test-confirmation"
			return workflowResponse(http.StatusOK, prep), nil
		case "POST /api/integrations/v1/source-events":
			var event SourceEvent
			if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
				return nil, err
			}
			events = append(events, event)
			return response(http.StatusOK, `{"received":true,"applied":true}`), nil
		default:
			t.Errorf("unexpected PostBridge call: %s %s", req.Method, req.URL.Path)
			return nil, fmt.Errorf("unexpected PostBridge call")
		}
	})
	service := NewService(store, client, sourceID, "test")
	input := CreateAttemptInput{RequestID: "create-image", ExpectedVersion: version, TargetAccountID: "target-test"}
	if _, err := service.Create(ctx, articleID, input); !errors.Is(err, ErrUnsupportedContent) {
		t.Fatalf("target without images: %v", err)
	}
	imagesEnabled = true

	var albumID, assetID string
	if err := pool.QueryRow(ctx, `
		with album as (
			insert into photo_albums (slug, title, description, created_by)
			select article_name, 'Album', '', author_id from articles where article_id::text = $1
			returning album_id
		), asset as (
			insert into assets (filename, content_type, byte_size, storage_key, public_url, created_by, width, height)
			select 'Sample.png', 'image/png', 100, article_name || '/Sample.png', $2, author_id, 1, 1
			from articles where article_id::text = $1 returning asset_id
		)
		insert into album_photos (album_id, asset_id, alt, width, height)
		select album_id, asset_id, 'Sample', 1, 1 from album, asset
		returning album_id::text, asset_id::text
	`, articleID, photo).Scan(&albumID, &assetID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "delete from photo_albums where album_id::text = $1", albumID)
		_, _ = pool.Exec(ctx, "delete from assets where asset_id::text = $1", assetID)
	})
	changeAlbumStatus := func(status string) {
		t.Helper()
		if _, err := pool.Exec(ctx, "update photo_albums set status = $2 where album_id::text = $1", albumID, status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.Create(ctx, articleID, input); !errors.Is(err, ErrUnapprovedImage) {
		t.Fatalf("draft album: %v", err)
	}
	changeAlbumStatus("published")
	if err := checkPublishedImages(ctx, store.queries, []string{"https://images.example/album/sample.png"}); !errors.Is(err, ErrUnapprovedImage) {
		t.Fatalf("different URL case: %v", err)
	}

	attempt, err := service.Create(ctx, articleID, input)
	if err != nil {
		t.Fatal(err)
	}
	if row := workflowStep(t, service, attempt.ID); row.Status != "prepared" || prepared != 1 {
		t.Fatalf("expected exactly one preparation, got %s and %d calls", row.Status, prepared)
	}
	if prep.Snapshot.Body != body || prep.Snapshot.CoverImageURL != "" || prep.Snapshot.Assets == nil || len(prep.Snapshot.Assets) != 0 {
		t.Fatalf("unexpected preparation snapshot: %+v", prep.Snapshot)
	}
	if intent, err := service.Submit(ctx, attempt.ID); err != nil || intent.Status != "submitting" {
		t.Fatalf("submit intent: %+v, %v", intent, err)
	}
	changeAlbumStatus("draft")
	if row := workflowStep(t, service, attempt.ID); row.RevokedAt == nil || !row.CancelPending {
		t.Fatalf("expected revocation and cancellation: %+v", row)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := service.deliverEvent(ctx, logger); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "authorization_revoked" || events[0].AuthorizationID != prep.AuthorizationID {
		t.Fatalf("unexpected album revocation events: %+v", events)
	}

	changeAlbumStatus("published")
	input.RequestID = "second-image"
	nextAttempt, err := service.Create(ctx, articleID, input)
	if err != nil {
		t.Fatal(err)
	}
	newBody := body + "\n\nUpdated."
	if _, err := pool.Exec(ctx, "update articles set body_mdx = $2 where article_id::text = $1", articleID, newBody); err != nil {
		t.Fatal(err)
	}
	if current, err := service.Get(ctx, nextAttempt.ID); err != nil || !current.Revoked {
		t.Fatalf("revision did not revoke the previous image attempt: %+v, %v", current, err)
	}
	for range 2 {
		if err := service.deliverEvent(ctx, logger); err != nil {
			t.Fatal(err)
		}
	}
	_, fingerprint, _, err := validateSnapshot(Snapshot{Title: "Example title", Body: newBody, BodyFormat: "markdown"}, true)
	hasEvent := func(kind string, match func(SourceEvent) bool) bool {
		return slices.ContainsFunc(events, func(event SourceEvent) bool { return event.Type == kind && match(event) })
	}
	if err != nil || len(events) != 3 ||
		!hasEvent("authorization_revoked", func(event SourceEvent) bool { return event.AuthorizationID == "grant:"+nextAttempt.ID }) ||
		!hasEvent("updated", func(event SourceEvent) bool { return event.ContentFingerprint == fingerprint }) {
		t.Fatalf("unexpected article revision events: %+v, %v", events, err)
	}
}
