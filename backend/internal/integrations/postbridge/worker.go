package postbridge

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

const (
	distributionPollInterval = 15 * time.Second
	cancelLookupWindow       = 2 * time.Minute
)

func (s *Service) Start(ctx context.Context, logger *slog.Logger) {
	if !s.client.SigningConfigured() || !identifier.MatchString(s.sourceID) {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.deliverEvent(ctx, logger); !idleOrStopped(err) {
					logger.Error("postbridge outbox operation failed")
				}
				record, err := s.store.ClaimNextAttempt(ctx, s.sourceID)
				if err == nil {
					_, err = s.process(ctx, record)
				}
				if !idleOrStopped(err) {
					logger.Error("postbridge distribution operation failed")
				}
			}
		}
	}()
}

func idleOrStopped(err error) bool {
	return err == nil || errors.Is(err, ErrStoreNotFound) || errors.Is(err, ErrStoreBusy) || errors.Is(err, context.Canceled)
}

func (s *Service) process(ctx context.Context, record AttemptRecord) (AttemptRecord, error) {
	data, err := loadAttemptData(record)
	if err != nil {
		return AttemptRecord{}, err
	}
	if time.Now().Before(data.RetryNotBefore) {
		return s.save(ctx, record, data, &data.RetryNotBefore, true)
	}
	if record.RevokedAt == nil && (data.Distribution == nil || data.needsPolling()) {
		if err := checkPublishedImages(ctx, s.store.queries, data.ImageURLs); err != nil {
			if errors.Is(err, ErrUnapprovedImage) {
				return s.revokeImageAttempt(ctx, record, data)
			}
			return AttemptRecord{}, err
		}
	}
	switch {
	case record.RevokedAt != nil && (record.CancelPending || data.CancelResult == "not_found"):
		return s.cancelRemote(ctx, record, data)
	case data.Distribution != nil:
		distribution, err := s.client.GetDistribution(ctx, data.Distribution.ID)
		return s.saveDistribution(ctx, record, data, distribution, err)
	case record.RevokedAt != nil:
		return s.save(ctx, record, data, nil, true)
	case len(data.SubmitBody) != 0:
		return s.submitRemote(ctx, record, data)
	case !time.Now().Before(data.preparationDeadline(record)):
		record.Status = "expired"
		return s.save(ctx, record, data, nil, true)
	}

	var preparation Preparation
	if data.Preparation != nil {
		preparation, err = s.client.GetPreparation(ctx, data.Preparation.ID)
	} else {
		current, err := s.store.GetAttempt(ctx, record.ID)
		if err != nil {
			return AttemptRecord{}, err
		}
		if current.RevokedAt != nil {
			return s.save(ctx, record, data, nil, true)
		}
		if !time.Now().Before(current.AuthorizationExpiresAt) {
			record.Status = "expired"
			return s.save(ctx, record, data, nil, true)
		}
		pending, err := s.store.HasPendingArticleEvents(ctx, record.SourceID, record.ArticleID, record.StateSeq)
		if err != nil {
			return AttemptRecord{}, err
		}
		if pending {
			return s.save(ctx, record, data, timePointer(time.Now().Add(manualRefreshInterval)), true)
		}
		preparation, err = s.client.CreatePreparation(ctx, data.PrepareBody)
		if err != nil {
			return s.saveFailure(ctx, record, data, err)
		}
	}
	if err == nil {
		err = data.acceptPreparation(record, preparation, time.Now())
	}
	if err != nil {
		return s.saveFailure(ctx, record, data, err)
	}
	record.Status = data.Preparation.Status
	if record.Status == "submitted" {
		record.Status = "needs_attention"
	}
	data.ErrorCode, data.Failures, data.RetryNotBefore = "", 0, time.Time{}
	return s.save(ctx, record, data, data.nextPreparationSync(record), true)
}

func (s *Service) revokeImageAttempt(ctx context.Context, record AttemptRecord, data attemptData) (AttemptRecord, error) {
	if record.LeaseToken == "" {
		return AttemptRecord{}, ErrStoreBusy
	}
	revoked, err := s.store.revokeAttempt(ctx, record.ID, record.LeaseToken)
	if err != nil {
		return AttemptRecord{}, err
	}
	data.ErrorCode = "image_unavailable"
	return s.save(ctx, revoked, data, timePointer(time.Now()), true)
}

func (s *Service) submitRemote(ctx context.Context, record AttemptRecord, data attemptData) (AttemptRecord, error) {
	if data.SubmitStarted || record.Status != "submitting" {
		distribution, err := s.client.FindDistribution(ctx, submitRequestID(record))
		if !hasUpstreamStatus(err, http.StatusNotFound) {
			return s.saveDistribution(ctx, record, data, distribution, err)
		}
		if record.Status != "submitting" {
			return s.save(ctx, record, data, nil, true)
		}
		// Separate a successful recovery lookup from replay, including quota-one clients.
		data.SubmitStarted = false
		return s.save(ctx, record, data, timePointer(time.Now().Add(manualRefreshInterval)), true)
	}
	current, err := s.store.GetAttempt(ctx, record.ID)
	if err != nil {
		return AttemptRecord{}, err
	}
	if current.RevokedAt != nil {
		return s.save(ctx, record, data, timePointer(time.Now()), true)
	}
	now := time.Now()
	if !now.Before(current.AuthorizationExpiresAt) || data.Preparation == nil || !now.Before(data.Preparation.ExpiresAt) {
		record.Status = "expired"
		return s.save(ctx, record, data, nil, true)
	}
	if !data.SubmitStarted {
		data.SubmitStarted, data.SubmitAttempted = true, true
		record, err = s.save(ctx, record, data, timePointer(now), false)
		if err != nil {
			return AttemptRecord{}, err
		}
		if record.RevokedAt != nil {
			return s.save(ctx, record, data, timePointer(now), true)
		}
	}
	distribution, err := s.client.Submit(ctx, data.SubmitBody)
	if hasUpstreamStatus(err, http.StatusTooManyRequests) {
		// PostBridge rejected the request before accepting a distribution.
		data.SubmitStarted = false
	}
	return s.saveDistribution(ctx, record, data, distribution, err)
}

func (s *Service) cancelRemote(ctx context.Context, record AttemptRecord, data attemptData) (AttemptRecord, error) {
	if !data.SubmitAttempted && data.Distribution == nil {
		record.Status, record.CancelPending = "revoked", false
		data.CancelResult = "not_submitted"
		return s.save(ctx, record, data, nil, true)
	}
	if data.Distribution == nil {
		distribution, err := s.client.FindDistribution(ctx, submitRequestID(record))
		if hasUpstreamStatus(err, http.StatusNotFound) {
			if time.Since(*record.RevokedAt) < cancelLookupWindow {
				return s.save(ctx, record, data, timePointer(time.Now().Add(distributionPollInterval)), true)
			}
			record.Status, record.CancelPending = "needs_attention", false
			data.CancelResult = "not_found"
			return s.save(ctx, record, data, nil, true)
		}
		if err == nil {
			err = data.acceptDistribution(record, distribution)
		}
		if err != nil {
			return s.saveFailure(ctx, record, data, err)
		}
		// Persist the recovered ID even if cancellation subsequently times out.
		record, err = s.save(ctx, record, data, timePointer(time.Now()), false)
		if err != nil {
			return AttemptRecord{}, err
		}
	}
	distribution, err := s.client.CancelDistribution(ctx, data.Distribution.ID)
	var upstream *UpstreamError
	if errors.As(err, &upstream) && upstream.Status == http.StatusConflict && upstream.Code == "distribution_not_cancellable" {
		record.CancelPending = false
		data.CancelResult = "not_cancellable"
		return s.save(ctx, record, data, timePointer(time.Now().Add(distributionPollInterval)), true)
	}
	if err == nil {
		err = data.acceptDistribution(record, distribution)
	}
	if err != nil {
		return s.saveFailure(ctx, record, data, err)
	}
	record.CancelPending = false
	data.CancelResult = "accepted"
	return s.storeDistribution(ctx, record, data)
}

func (s *Service) saveDistribution(ctx context.Context, record AttemptRecord, data attemptData, distribution Distribution, err error) (AttemptRecord, error) {
	if err == nil {
		err = data.acceptDistribution(record, distribution)
	}
	if err != nil {
		return s.saveFailure(ctx, record, data, err)
	}
	return s.storeDistribution(ctx, record, data)
}

func (s *Service) storeDistribution(ctx context.Context, record AttemptRecord, data attemptData) (AttemptRecord, error) {
	record.Status = data.Distribution.Status
	switch data.Distribution.Child.Status {
	case "failed":
		record.Status = "failed"
	case "cancelled":
		record.Status = "cancelled"
	}
	data.ErrorCode, data.Failures, data.RetryNotBefore = "", 0, time.Time{}
	var next *time.Time
	if data.needsPolling() {
		next = timePointer(time.Now().Add(distributionPollInterval))
	}
	return s.save(ctx, record, data, next, true)
}

func (s *Service) saveFailure(ctx context.Context, record AttemptRecord, data attemptData, err error) (AttemptRecord, error) {
	code, transient, retryAfter := remoteFailure(err)
	data.ErrorCode, data.Failures = code, data.Failures+1
	next := data.nextPreparationSync(record)
	switch {
	case transient:
		data.RetryNotBefore = time.Now().Add(max(backoff(data.Failures), retryAfter))
		next = &data.RetryNotBefore
	case record.CancelPending:
		next = timePointer(time.Now().Add(5 * time.Minute))
	case hasUpstreamStatus(err, http.StatusGone):
		record.Status, next = "expired", nil
	case record.Status == "preparing" || record.Status == "submitting":
		record.Status = "failed"
	}
	return s.save(ctx, record, data, next, true)
}

func (s *Service) deliverEvent(ctx context.Context, logger *slog.Logger) error {
	event, err := s.store.ClaimEvent(ctx, s.sourceID)
	if err != nil {
		return err
	}
	if len(event.Snapshot) != 0 {
		var article ArticleSource
		var body SourceEvent
		if err := json.Unmarshal(event.Snapshot, &article); err != nil {
			return s.failEvent(ctx, logger, event, ErrRemoteMismatch)
		}
		if err := json.Unmarshal(event.Body, &body); err != nil {
			return s.failEvent(ctx, logger, event, ErrRemoteMismatch)
		}
		_, fingerprint, err := ValidateSnapshot(articleSnapshot(article))
		if err != nil && article.Status == "published" {
			var imageURLs []string
			_, fingerprint, imageURLs, err = validateSnapshot(articleSnapshot(article), true)
			if err == nil {
				err = checkPublishedImages(ctx, s.store.queries, imageURLs)
			}
		}
		if err != nil {
			// The same transaction already queued invalidation of the old grants.
			if errors.Is(err, ErrUnsupportedContent) || errors.Is(err, ErrInvalidRequest) || errors.Is(err, ErrUnapprovedImage) {
				return s.store.FinishEvent(ctx, event, nil, "")
			}
			return s.failEvent(ctx, logger, event, ErrUnavailable)
		}
		body.ContentFingerprint = fingerprint
		payload, err := marshalRequest(body)
		if err != nil {
			return s.failEvent(ctx, logger, event, err)
		}
		if err := s.store.FreezeEvent(ctx, event, payload); err != nil {
			return err
		}
		event.Body = payload
	}
	if err := s.client.SendSourceEvent(ctx, event.Body); err != nil {
		return s.failEvent(ctx, logger, event, err)
	}
	return s.store.FinishEvent(ctx, event, nil, "")
}

func (s *Service) failEvent(ctx context.Context, logger *slog.Logger, event EventRecord, err error) error {
	code, transient, retryAfter := remoteFailure(err)
	delay := max(backoff(int(event.Attempts)), retryAfter)
	if !transient {
		delay = max(delay, time.Hour)
	}
	if !transient || event.Attempts >= 5 {
		logger.Warn("postbridge source event pending", "event_id", event.ID, "attempts", event.Attempts, "code", code)
	}
	return s.store.FinishEvent(ctx, event, timePointer(time.Now().Add(delay)), code)
}

func hasUpstreamStatus(err error, status int) bool {
	var upstream *UpstreamError
	return errors.As(err, &upstream) && upstream.Status == status
}

func remoteFailure(err error) (string, bool, time.Duration) {
	var upstream *UpstreamError
	if errors.As(err, &upstream) {
		code := upstream.Code
		if code == "" {
			code = "upstream_http_" + strconv.Itoa(upstream.Status)
		}
		delay := time.Duration(0)
		if seconds, err := strconv.ParseUint(upstream.RetryAfter, 10, 32); err == nil {
			delay = time.Duration(seconds) * time.Second
		}
		return code, upstream.Status == http.StatusTooManyRequests || upstream.Status >= 500, delay
	}
	switch {
	case errors.Is(err, ErrUnavailable):
		return "upstream_unavailable", true, 0
	case errors.Is(err, ErrNotConfigured):
		return "not_configured", false, 0
	default:
		return "invalid_upstream_response", false, 0
	}
}

func backoff(attempts int) time.Duration {
	return min(5*time.Minute, 5*time.Second<<min(max(attempts-1, 0), 6))
}
