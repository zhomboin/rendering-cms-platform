package postbridge

import (
	"errors"
	"testing"
	"time"
)

func TestPreparationConfirmationBoundToFrozenIntent(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	record := AttemptRecord{
		ID:                     "attempt-test",
		ArticleID:              "article-test",
		SourceID:               "cms-test",
		SourceVersion:          2,
		StateSeq:               3,
		AuthorizationID:        "grant-test",
		TargetAccountID:        "target-test",
		AuthorizationExpiresAt: now.Add(time.Hour),
	}
	data := attemptData{ContentFingerprint: "fingerprint"}
	preparation := Preparation{
		PreparationInput: PreparationInput{
			ArticleID:              record.ArticleID,
			SourceID:               record.SourceID,
			SourceVersion:          record.SourceVersion,
			StateSeq:               record.StateSeq,
			AuthorizationID:        record.AuthorizationID,
			ContentFingerprint:     data.ContentFingerprint,
			Targets:                attemptTargets(record),
			RequestID:              prepareRequestID(record),
			AuthorizationExpiresAt: record.AuthorizationExpiresAt,
		},
		ID:                      "prepare-test",
		Confirmed:               true,
		Status:                  "confirmed",
		ExpiresAt:               now.Add(time.Hour),
		DraftRevisionIDs:        []string{"draft-test"},
		ConfirmationFingerprint: "confirmed-test",
	}
	if err := data.acceptPreparation(record, preparation, now); err != nil {
		t.Fatal(err)
	}
	if !data.readyToSubmit(now) {
		t.Fatal("matching confirmed preparation should be eligible")
	}
	stale := preparation
	stale.StateSeq--
	if !errors.Is(data.acceptPreparation(record, stale, now), ErrRemoteMismatch) {
		t.Fatal("stale preparation was accepted")
	}
	preparation.Confirmed = false
	if err := data.acceptPreparation(record, preparation, now); err != nil || data.readyToSubmit(now) {
		t.Fatal("withdrawn confirmation was accepted")
	}
}

func TestAttemptViewDistinguishesUnknownFromSavedDraft(t *testing.T) {
	now := time.Now().UTC()
	record := AttemptRecord{
		Status:                 "completed",
		AuthorizationExpiresAt: now.Add(time.Hour),
	}
	data := attemptData{Distribution: &distributionState{Child: DistributionChild{
		ExternalStatus: "unknown",
		Status:         "succeeded",
		RemoteURL:      "https://example.test/item",
	}}}
	view := attemptView(record, data, now)
	if view.ExternalStatus != "unknown" || view.RemoteURL != "" || view.CanSubmit {
		t.Fatalf("unknown result: %+v", view)
	}
	data.Distribution.Child.ExternalStatus = "draft_saved"
	view = attemptView(record, data, now)
	if view.ExternalStatus != "draft_saved" || view.RemoteURL == "" {
		t.Fatalf("saved result: %+v", view)
	}
}

func TestTerminalKnownFailureDoesNotPoll(t *testing.T) {
	data := attemptData{Distribution: &distributionState{
		Status: "needs_attention",
		Child: DistributionChild{
			Status:         "failed",
			ExternalStatus: "not_submitted",
		},
	}}
	if data.needsPolling() {
		t.Fatal("known failure should be shown without continued automatic polling")
	}
	data.Distribution.Child.ExternalStatus = "unknown"
	if !data.needsPolling() {
		t.Fatal("unknown external result requires reconciliation polling")
	}
}
