package postbridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

func TestWorkflowRecoversAcceptedSubmissionAndCancelsWithoutReplaying(t *testing.T) {
	store, pool, sourceID, articleID := postBridgeDatabaseFixture(t)
	ctx := context.Background()
	var version int64
	if err := pool.QueryRow(ctx, "select version from articles where article_id::text = $1", articleID).Scan(&version); err != nil {
		t.Fatal(err)
	}

	client := testClient(t)
	service := NewService(store, client, sourceID, "test")
	var prep Preparation
	var distribution Distribution
	var submitted, cancelled, events int
	throttlePreparation := true
	rejectPreparation := false
	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method + " " + req.URL.Path {
		case "GET /api/integrations/v1/targets":
			return response(http.StatusOK, `{"items":[{"platform":"csdn","account_id":"target-test","operations":["save_draft"],"body_formats":["markdown"]}]}`), nil
		case "POST /api/integrations/v1/preparations":
			if err := json.NewDecoder(req.Body).Decode(&prep.PreparationInput); err != nil {
				return nil, err
			}
			prep.ID, prep.Status = "preparation-test", "prepared"
			prep.ExpiresAt = time.Now().Add(time.Hour)
			return workflowResponse(http.StatusCreated, prep), nil
		case "GET /api/integrations/v1/preparations/preparation-test":
			if rejectPreparation {
				return response(http.StatusForbidden, "{}"), nil
			}
			if throttlePreparation {
				throttlePreparation = false
				reply := response(http.StatusTooManyRequests, "{}")
				reply.Header.Set("Retry-After", "30")
				return reply, nil
			}
			prep.Confirmed, prep.Status = true, "confirmed"
			prep.DraftRevisionIDs = []string{"draft-test"}
			prep.ConfirmationFingerprint = "confirmation-test"
			return workflowResponse(http.StatusOK, prep), nil
		case "POST /api/integrations/v1/distributions":
			submitted++
			var in SubmitInput
			if err := json.NewDecoder(req.Body).Decode(&in); err != nil {
				return nil, err
			}
			distribution = Distribution{
				ID: "distribution-test", RequestID: in.RequestID, ArticleID: in.ArticleID, SourceID: in.SourceID,
				SourceVersion: in.SourceVersion, StateSeq: in.StateSeq, PreparationID: in.PreparationID, Targets: in.Targets,
				ContentFingerprint: in.ContentFingerprint, ConfirmationFingerprint: in.ConfirmationFingerprint,
				Status: "in_progress", Children: []DistributionChild{{
					Platform: "csdn", AccountID: "target-test", Operation: "save_draft", Status: "queued", ExternalStatus: "not_submitted",
				}},
			}
			return nil, errors.New("synthetic lost response")
		case "GET /api/integrations/v1/distributions":
			if req.URL.Query().Get("request_id") != distribution.RequestID {
				return nil, errors.New("recovery did not use the original submission ID")
			}
			return workflowResponse(http.StatusOK, distribution), nil
		case "POST /api/integrations/v1/distributions/distribution-test/cancel":
			cancelled++
			distribution.Status, distribution.Children[0].Status = "completed", "cancelled"
			return workflowResponse(http.StatusOK, distribution), nil
		case "POST /api/integrations/v1/source-events":
			events++
			var event SourceEvent
			if err := json.NewDecoder(req.Body).Decode(&event); err != nil {
				return nil, err
			}
			if event.Type != "authorization_revoked" || event.AuthorizationID != prep.AuthorizationID {
				return nil, errors.New("unexpected source event")
			}
			return response(http.StatusOK, `{"received":true,"applied":true}`), nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	input := CreateAttemptInput{RequestID: "create-test", ExpectedVersion: version, TargetAccountID: "target-test"}
	attempt, err := service.Create(ctx, articleID, input)
	if err != nil || attempt.Status != "preparing" {
		t.Fatalf("create: %+v, %v", attempt, err)
	}
	repeat, err := service.Create(ctx, articleID, input)
	if err != nil || repeat.ID != attempt.ID {
		t.Fatalf("idempotent create: %+v, %v", repeat, err)
	}
	if row := workflowStep(t, service, attempt.ID); row.Status != "prepared" {
		t.Fatalf("preparation not saved: %s", row.Status)
	}
	checkExpirySchedule := func() {
		var nextSync *time.Time
		if err := pool.QueryRow(ctx, "select next_sync_at from postbridge_attempts where id = $1", attempt.ID).Scan(&nextSync); err != nil {
			t.Fatal(err)
		}
		if nextSync == nil || nextSync.Sub(prep.ExpiresAt).Abs() > time.Millisecond {
			t.Fatalf("expected automatic expiry at preparation deadline, got %v", nextSync)
		}
	}
	checkExpirySchedule()
	rejectPreparation = true
	workflowStep(t, service, attempt.ID)
	checkExpirySchedule()
	rejectPreparation = false
	for range 2 {
		if _, err := service.Submit(ctx, attempt.ID); err == nil {
			t.Fatal("submit returned success without saving an intent")
		}
	}
	claimed, err := store.ClaimAttempt(ctx, attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := loadAttemptData(claimed)
	if err != nil || len(blocked.SubmitBody) != 0 || submitted != 0 || blocked.RetryNotBefore.IsZero() {
		t.Fatalf("unexpected submit state after throttling: %v, %d writes", err, submitted)
	}
	blocked.RetryNotBefore = time.Time{}
	if _, err := service.save(ctx, claimed, blocked, nil, true); err != nil {
		t.Fatal(err)
	}
	intent, err := service.Submit(ctx, attempt.ID)
	if err != nil || intent.Status != "submitting" {
		t.Fatalf("submit intent: %+v, %v", intent, err)
	}
	if row := workflowStep(t, service, attempt.ID); row.Status != "submitting" || submitted != 1 {
		t.Fatalf("lost submission: %s, upstream writes=%d", row.Status, submitted)
	}
	// Simulate the retry time being reached without sleeping for the backoff.
	row, err := store.ClaimAttempt(ctx, attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := loadAttemptData(row)
	if err != nil {
		t.Fatal(err)
	}
	row, err = service.submitRemote(ctx, row, data)
	if err != nil || row.Status != "in_progress" || submitted != 1 {
		t.Fatalf("recovery: %s, upstream writes=%d, error=%v", row.Status, submitted, err)
	}
	revoked, err := service.Cancel(ctx, attempt.ID)
	if err != nil || !revoked.Revoked || !revoked.CancelPending {
		t.Fatalf("revoke: %+v, %v", revoked, err)
	}
	if err := service.deliverEvent(ctx, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	row = workflowStep(t, service, attempt.ID)
	if row.Status != "cancelled" || row.CancelPending || cancelled != 1 || events != 1 || submitted != 1 {
		t.Fatalf("cancelled status=%s, pending=%v, cancel=%d, events=%d, submissions=%d", row.Status, row.CancelPending, cancelled, events, submitted)
	}
}

func workflowStep(t *testing.T, service *Service, attemptID string) AttemptRecord {
	t.Helper()
	ctx := context.Background()
	record, err := service.store.ClaimAttempt(ctx, attemptID)
	if err != nil {
		t.Fatal(err)
	}
	record, err = service.process(ctx, record)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func workflowResponse(status int, body any) *http.Response {
	encoded, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	return response(status, string(encoded))
}
