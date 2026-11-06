package postbridge

import (
	"encoding/json"
	"errors"
	"slices"
	"time"
)

var (
	ErrAttemptInput        = errors.New("invalid distribution attempt input")
	ErrAttemptConflict     = errors.New("article version, state, or request changed")
	ErrUnapprovedImage     = errors.New("PostBridge image is not available in a published album")
	ErrCannotSubmit        = errors.New("preparation is not eligible for submission")
	ErrCoverRequiresChoice = errors.New("omitting the cover requires confirmation")
	ErrRemoteMismatch      = errors.New("PostBridge response does not match the distribution attempt")
)

type Attempt struct {
	ID                     string     `json:"id"`
	ArticleID              string     `json:"articleId"`
	SourceVersion          int64      `json:"sourceVersion"`
	TargetAccountID        string     `json:"targetAccountId"`
	Status                 string     `json:"status"`
	PreviewURL             string     `json:"previewUrl,omitempty"`
	DetailURL              string     `json:"detailUrl,omitempty"`
	ExternalStatus         string     `json:"externalStatus,omitempty"`
	ChildStatus            string     `json:"childStatus,omitempty"`
	Outcome                string     `json:"outcome,omitempty"`
	RemoteURL              string     `json:"remoteUrl,omitempty"`
	RemoteID               string     `json:"remoteId,omitempty"`
	ErrorCode              string     `json:"errorCode,omitempty"`
	Revoked                bool       `json:"revoked"`
	CancelPending          bool       `json:"cancelPending"`
	CancelResult           string     `json:"cancelResult,omitempty"`
	CanSubmit              bool       `json:"canSubmit"`
	CanCancel              bool       `json:"canCancel"`
	CanRefresh             bool       `json:"canRefresh"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
	AuthorizationExpiresAt time.Time  `json:"authorizationExpiresAt"`
	LastConfirmedAt        *time.Time `json:"lastConfirmedAt,omitempty"`
}

type CreateAttemptInput struct {
	RequestID       string `json:"requestId"`
	ExpectedVersion int64  `json:"expectedVersion"`
	TargetAccountID string `json:"targetAccountId"`
	TextOnly        bool   `json:"textOnly"`
}

type AttemptPage struct {
	Items    []Attempt `json:"items"`
	Page     int32     `json:"page"`
	PageSize int32     `json:"pageSize"`
	Total    int64     `json:"total"`
}

type preparationState struct {
	ID                      string
	PreviewURL              string
	Status                  string
	Confirmed               bool
	DraftRevisionIDs        []string
	ConfirmationFingerprint string
	ExpiresAt               time.Time
}

type distributionState struct {
	ID        string
	Status    string
	DetailURL string
	Child     DistributionChild
}

type attemptData struct {
	TextOnly           bool
	ContentFingerprint string
	PrepareBody        []byte
	ImageURLs          []string `json:",omitempty"`
	SubmitBody         []byte
	Preparation        *preparationState
	Distribution       *distributionState
	SubmitStarted      bool
	SubmitAttempted    bool
	RetryNotBefore     time.Time
	CancelResult       string
	ErrorCode          string
	Failures           int
}

func loadAttemptData(record AttemptRecord) (attemptData, error) {
	var data attemptData
	err := json.Unmarshal(record.Data, &data)
	return data, err
}

func attemptView(record AttemptRecord, data attemptData, now time.Time) Attempt {
	activeGrant := record.RevokedAt == nil && now.Before(data.preparationDeadline(record))
	view := Attempt{
		ID: record.ID, ArticleID: record.ArticleID, SourceVersion: record.SourceVersion,
		TargetAccountID: record.TargetAccountID, Status: record.Status, ErrorCode: data.ErrorCode,
		Revoked: record.RevokedAt != nil, CancelPending: record.CancelPending, CancelResult: data.CancelResult,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
		AuthorizationExpiresAt: record.AuthorizationExpiresAt,
		CanSubmit:              activeGrant && !now.Before(data.RetryNotBefore) && record.Status == "confirmed" && data.readyToSubmit(now),
		CanCancel:              record.RevokedAt == nil && record.Status != "completed" && record.Status != "cancelled",
		CanRefresh:             record.CancelPending || data.CancelResult == "not_found" || record.Status == "preparing" || record.Status == "submitting" || data.Distribution != nil || (activeGrant && data.Preparation != nil),
	}
	if data.Preparation != nil && activeGrant {
		view.PreviewURL = data.Preparation.PreviewURL
	}
	if data.CancelResult == "not_found" {
		view.ExternalStatus = "unknown"
	}
	if distribution := data.Distribution; distribution != nil {
		view.DetailURL = distribution.DetailURL
		child := distribution.Child
		view.ExternalStatus, view.ChildStatus, view.Outcome, view.LastConfirmedAt = child.ExternalStatus, child.Status, child.Outcome, child.LastConfirmedAt
		if child.ExternalStatus == "draft_saved" {
			view.RemoteURL, view.RemoteID = child.RemoteURL, child.RemoteID
		}
		if view.ErrorCode == "" {
			view.ErrorCode = child.ErrorCode
		}
	}
	return view
}

func attemptTargets(record AttemptRecord) []DistributionTarget {
	return []DistributionTarget{{Platform: "csdn", AccountID: record.TargetAccountID, Operation: "save_draft"}}
}

func prepareRequestID(record AttemptRecord) string { return "prepare:" + record.ID }
func submitRequestID(record AttemptRecord) string  { return "submit:" + record.ID }

func (data *attemptData) acceptPreparation(record AttemptRecord, prep Preparation, now time.Time) error {
	if prep.ID == "" || prep.RequestID != prepareRequestID(record) || prep.ArticleID != record.ArticleID ||
		prep.SourceID != record.SourceID || prep.SourceVersion != record.SourceVersion || prep.StateSeq != record.StateSeq ||
		prep.AuthorizationID != record.AuthorizationID || !prep.AuthorizationExpiresAt.Equal(record.AuthorizationExpiresAt) ||
		prep.ContentFingerprint != data.ContentFingerprint || !slices.Equal(prep.Targets, attemptTargets(record)) ||
		(data.Preparation != nil && data.Preparation.ID != prep.ID) {
		return ErrRemoteMismatch
	}
	switch prep.Status {
	case "prepared", "confirmed", "submitted", "revoked", "expired":
	default:
		return ErrRemoteMismatch
	}
	data.Preparation = &preparationState{
		ID: prep.ID, PreviewURL: prep.PreviewURL, Status: prep.Status,
		Confirmed: prep.Confirmed, ExpiresAt: prep.ExpiresAt,
		DraftRevisionIDs: prep.DraftRevisionIDs, ConfirmationFingerprint: prep.ConfirmationFingerprint,
	}
	if !now.Before(prep.ExpiresAt) && (prep.Status == "prepared" || prep.Status == "confirmed") {
		data.Preparation.Status = "expired"
	}
	return nil
}

func (data attemptData) preparationDeadline(record AttemptRecord) time.Time {
	if data.Preparation != nil && data.Preparation.ExpiresAt.Before(record.AuthorizationExpiresAt) {
		return data.Preparation.ExpiresAt
	}
	return record.AuthorizationExpiresAt
}

func (data attemptData) nextPreparationSync(record AttemptRecord) *time.Time {
	if record.Status == "prepared" || record.Status == "confirmed" {
		return timePointer(data.preparationDeadline(record))
	}
	return nil
}

func (data attemptData) readyToSubmit(now time.Time) bool {
	p := data.Preparation
	return len(data.SubmitBody) == 0 && p != nil && p.Status == "confirmed" && p.Confirmed &&
		now.Before(p.ExpiresAt) && len(p.DraftRevisionIDs) == 1 && p.DraftRevisionIDs[0] != "" && p.ConfirmationFingerprint != ""
}

func (data *attemptData) acceptDistribution(record AttemptRecord, d Distribution) error {
	if data.Preparation == nil || d.ID == "" || d.RequestID != submitRequestID(record) ||
		d.PreparationID != data.Preparation.ID || d.ArticleID != record.ArticleID || d.SourceID != record.SourceID ||
		d.SourceVersion != record.SourceVersion || d.StateSeq != record.StateSeq ||
		d.ContentFingerprint != data.ContentFingerprint || d.ConfirmationFingerprint != data.Preparation.ConfirmationFingerprint ||
		!slices.Equal(d.Targets, attemptTargets(record)) || (data.Distribution != nil && d.ID != data.Distribution.ID) {
		return ErrRemoteMismatch
	}
	switch d.Status {
	case "in_progress", "needs_attention", "completed":
	default:
		return ErrRemoteMismatch
	}
	if len(d.Children) != 1 {
		return ErrRemoteMismatch
	}
	child := d.Children[0]
	if child.Platform != "csdn" || child.AccountID != record.TargetAccountID || child.Operation != "save_draft" {
		return ErrRemoteMismatch
	}
	child.ErrorMessage = ""
	if !errorCode.MatchString(child.ErrorCode) {
		child.ErrorCode = ""
	}
	data.Distribution = &distributionState{ID: d.ID, Status: d.Status, DetailURL: d.DetailURL, Child: child}
	return nil
}

func (data attemptData) needsPolling() bool {
	if data.Distribution == nil {
		return false
	}
	child := data.Distribution.Child
	if child.ExternalStatus == "unknown" {
		return true
	}
	switch child.Status {
	case "failed", "cancelled", "succeeded":
		return false
	default:
		return data.Distribution.Status != "completed"
	}
}
