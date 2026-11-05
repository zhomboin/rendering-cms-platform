package postbridge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const (
	authorizationTTL      = 24 * time.Hour
	manualRefreshInterval = 5 * time.Second
)

type Service struct {
	store       *Store
	client      *Client
	sourceID    string
	environment string
}

func NewService(store *Store, client *Client, sourceID, environment string) *Service {
	return &Service{store: store, client: client, sourceID: sourceID, environment: environment}
}

func (s *Service) BindConnection(ctx context.Context) error {
	if !s.client.SigningConfigured() {
		return nil
	}
	if !identifier.MatchString(s.sourceID) {
		return ErrAttemptInput
	}
	return s.store.BindConnection(ctx, s.sourceID, s.environment, s.client.origin, s.client.clientID)
}

func (s *Service) SaveRegistration(ctx context.Context, result Registration) error {
	return s.store.SaveRegistration(ctx, s.sourceID, s.environment, result)
}

func (s *Service) Registration(ctx context.Context) (Registration, error) {
	return s.store.GetRegistration(ctx, s.sourceID, s.environment)
}

func (s *Service) Create(ctx context.Context, articleID string, in CreateAttemptInput) (Attempt, error) {
	if !s.client.SigningConfigured() {
		return Attempt{}, ErrNotConfigured
	}
	if !identifier.MatchString(s.sourceID) || !requestKey.MatchString(in.RequestID) ||
		!requestKey.MatchString(in.TargetAccountID) || in.ExpectedVersion < 1 {
		return Attempt{}, ErrAttemptInput
	}
	existing, err := s.store.GetAttemptByRequest(ctx, s.sourceID, articleID, in.RequestID)
	if err == nil {
		return matchAttemptInput(existing, in)
	}
	if !errors.Is(err, ErrStoreNotFound) {
		return Attempt{}, err
	}
	targets, err := s.client.Targets(ctx)
	if err != nil {
		return Attempt{}, err
	}
	if !slices.ContainsFunc(targets, func(target Target) bool {
		return target.Platform == "csdn" && target.AccountID == in.TargetAccountID &&
			slices.Contains(target.Operations, "save_draft") && slices.Contains(target.BodyFormats, "markdown")
	}) {
		return Attempt{}, ErrAttemptInput
	}
	record, err := s.store.CreateAttempt(ctx, s.sourceID, articleID, in.RequestID, func(article ArticleSource, seq int64) (AttemptRecord, error) {
		if article.Status != "published" || article.Version != in.ExpectedVersion {
			return AttemptRecord{}, ErrAttemptConflict
		}
		if article.CoverImageURL != "" && !in.TextOnly {
			return AttemptRecord{}, ErrCoverRequiresChoice
		}
		snapshot, fingerprint, err := ValidateSnapshot(articleSnapshot(article))
		if err != nil {
			return AttemptRecord{}, err
		}
		id := uuid.NewString()
		record := AttemptRecord{
			ID: id, ArticleID: article.ID, SourceID: s.sourceID, RequestID: in.RequestID,
			SourceVersion: article.Version, StateSeq: seq, TargetAccountID: in.TargetAccountID,
			AuthorizationID: "grant:" + id, Status: "preparing",
			AuthorizationExpiresAt: time.Now().UTC().Truncate(time.Second).Add(authorizationTTL),
		}
		body, err := marshalRequest(PreparationInput{
			RequestID: prepareRequestID(record), SourceID: s.sourceID, ArticleID: article.ID,
			SourceVersion: article.Version, StateSeq: seq, AuthorizationID: record.AuthorizationID,
			AuthorizationExpiresAt: record.AuthorizationExpiresAt, ContentFingerprint: fingerprint,
			Snapshot: snapshot, Targets: attemptTargets(record),
		})
		if err != nil {
			return AttemptRecord{}, err
		}
		record.Data, err = json.Marshal(attemptData{TextOnly: in.TextOnly, ContentFingerprint: fingerprint, PrepareBody: body})
		return record, err
	})
	if err != nil {
		return Attempt{}, err
	}
	return matchAttemptInput(record, in)
}

func matchAttemptInput(record AttemptRecord, in CreateAttemptInput) (Attempt, error) {
	data, err := loadAttemptData(record)
	if err != nil {
		return Attempt{}, err
	}
	if record.SourceVersion != in.ExpectedVersion || record.TargetAccountID != in.TargetAccountID || data.TextOnly != in.TextOnly {
		return Attempt{}, ErrAttemptConflict
	}
	return attemptView(record, data, time.Now()), nil
}

func articleSnapshot(article ArticleSource) Snapshot {
	return Snapshot{Title: article.Title, Summary: article.Summary, Body: article.Body, BodyFormat: "markdown", Tags: article.Tags}
}

func marshalRequest(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err == nil && len(body) > maxRequestSize {
		return nil, ErrRequestTooLarge
	}
	return body, err
}

func (s *Service) List(ctx context.Context, articleID, status string, page, pageSize int32) (AttemptPage, error) {
	if page < 1 || page > 100000 || pageSize < 1 || pageSize > 100 {
		return AttemptPage{}, ErrAttemptInput
	}
	switch status {
	case "", "preparing", "prepared", "confirmed", "submitting", "in_progress", "needs_attention", "completed", "cancelled", "revoked", "failed", "expired":
	default:
		return AttemptPage{}, ErrAttemptInput
	}
	rows, total, err := s.store.ListAttempts(ctx, s.sourceID, articleID, status, page, pageSize)
	if err != nil {
		return AttemptPage{}, err
	}
	now := time.Now()
	result := AttemptPage{Items: make([]Attempt, 0, len(rows)), Page: page, PageSize: pageSize, Total: total}
	for _, record := range rows {
		data, err := loadAttemptData(record)
		if err != nil {
			return AttemptPage{}, err
		}
		result.Items = append(result.Items, attemptView(record, data, now))
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, id string) (Attempt, error) {
	record, err := s.get(ctx, id)
	return viewRecord(record, err)
}

func (s *Service) get(ctx context.Context, id string) (AttemptRecord, error) {
	record, err := s.store.GetAttempt(ctx, id)
	if err == nil && record.SourceID != s.sourceID {
		return AttemptRecord{}, ErrStoreNotFound
	}
	return record, err
}

func (s *Service) Cancel(ctx context.Context, id string) (Attempt, error) {
	if _, err := s.get(ctx, id); err != nil {
		return Attempt{}, err
	}
	record, err := s.store.RevokeAttempt(ctx, id)
	return viewRecord(record, err)
}

func (s *Service) Refresh(ctx context.Context, id string) (Attempt, error) {
	record, err := s.get(ctx, id)
	if err != nil || time.Since(record.UpdatedAt) < manualRefreshInterval {
		return viewRecord(record, err)
	}
	record, err = s.store.ClaimAttempt(ctx, id)
	if errors.Is(err, ErrStoreBusy) {
		return s.Get(ctx, id)
	}
	if err != nil {
		return Attempt{}, err
	}
	record, err = s.process(ctx, record)
	return viewRecord(record, err)
}

func (s *Service) Submit(ctx context.Context, id string) (Attempt, error) {
	if !s.client.SigningConfigured() {
		return Attempt{}, ErrNotConfigured
	}
	record, err := s.get(ctx, id)
	if err != nil {
		return Attempt{}, err
	}
	record, err = s.store.ClaimAttempt(ctx, record.ID)
	if err != nil {
		return Attempt{}, err
	}
	data, err := loadAttemptData(record)
	if err != nil {
		return Attempt{}, err
	}
	now := time.Now()
	if now.Before(data.RetryNotBefore) {
		saved, err := s.save(ctx, record, data, &data.RetryNotBefore, true)
		if err != nil || len(data.SubmitBody) != 0 {
			return viewRecord(saved, err)
		}
		wait := (data.RetryNotBefore.Sub(now) + time.Second - 1) / time.Second
		return Attempt{}, &UpstreamError{Status: http.StatusTooManyRequests, RetryAfter: strconv.FormatInt(int64(wait), 10)}
	}
	if len(data.SubmitBody) != 0 {
		next := timePointer(now)
		if !record.CancelPending && record.Status != "submitting" && !data.needsPolling() {
			next = nil
		}
		return viewRecord(s.save(ctx, record, data, next, true))
	}
	if record.RevokedAt != nil || !now.Before(data.preparationDeadline(record)) || data.Preparation == nil {
		var next *time.Time
		if record.RevokedAt == nil && !now.Before(data.preparationDeadline(record)) {
			record.Status = "expired"
		}
		if record.CancelPending || record.Status == "preparing" {
			next = &now
		}
		_, err := s.save(ctx, record, data, next, true)
		if err != nil {
			return Attempt{}, err
		}
		return Attempt{}, ErrCannotSubmit
	}
	preparation, err := s.client.GetPreparation(ctx, data.Preparation.ID)
	if err == nil {
		err = data.acceptPreparation(record, preparation, time.Now())
	}
	if err != nil {
		if _, saveErr := s.saveFailure(ctx, record, data, err); saveErr != nil {
			return Attempt{}, saveErr
		}
		return Attempt{}, err
	}
	record.Status = data.Preparation.Status
	if !data.readyToSubmit(time.Now()) {
		_, err := s.save(ctx, record, data, data.nextPreparationSync(record), true)
		if err != nil {
			return Attempt{}, err
		}
		return Attempt{}, ErrCannotSubmit
	}
	body, err := marshalRequest(SubmitInput{
		RequestID: submitRequestID(record), SourceID: record.SourceID, ArticleID: record.ArticleID,
		SourceVersion: record.SourceVersion, StateSeq: record.StateSeq, ContentFingerprint: data.ContentFingerprint,
		PreparationID: preparation.ID, DraftRevisionIDs: preparation.DraftRevisionIDs,
		ConfirmationFingerprint: preparation.ConfirmationFingerprint, Targets: preparation.Targets,
	})
	if err != nil {
		return Attempt{}, err
	}
	data.SubmitBody, data.ErrorCode = body, ""
	record.Status = "submitting"
	return viewRecord(s.save(ctx, record, data, timePointer(time.Now()), true))
}

func viewRecord(record AttemptRecord, err error) (Attempt, error) {
	if err != nil {
		return Attempt{}, err
	}
	data, err := loadAttemptData(record)
	if err != nil {
		return Attempt{}, err
	}
	return attemptView(record, data, time.Now()), nil
}

func timePointer(t time.Time) *time.Time { return &t }

func (s *Service) save(ctx context.Context, record AttemptRecord, data attemptData, next *time.Time, release bool) (AttemptRecord, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return AttemptRecord{}, err
	}
	record.Data = raw
	return s.store.SaveAttempt(ctx, record, next, release)
}
