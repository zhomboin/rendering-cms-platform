package postbridge

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidRequest  = errors.New("invalid PostBridge request")
	ErrRequestTooLarge = errors.New("PostBridge request exceeds 1 MiB")
	requestKey         = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

type DistributionTarget struct {
	Platform  string `json:"platform"`
	AccountID string `json:"account_id"`
	Operation string `json:"operation"`
}

type PreparationInput struct {
	RequestID              string               `json:"request_id"`
	SourceID               string               `json:"source_id"`
	ArticleID              string               `json:"article_id"`
	SourceVersion          int64                `json:"source_version"`
	StateSeq               int64                `json:"state_seq"`
	AuthorizationID        string               `json:"authorization_id"`
	AuthorizationExpiresAt time.Time            `json:"authorization_expires_at"`
	ContentFingerprint     string               `json:"content_fingerprint"`
	Snapshot               Snapshot             `json:"snapshot"`
	Targets                []DistributionTarget `json:"targets"`
}

type Preparation struct {
	PreparationInput
	ID                      string    `json:"id"`
	ClientID                string    `json:"client_id"`
	PreviewURL              string    `json:"preview_url"`
	Status                  string    `json:"status"`
	Revision                int64     `json:"revision"`
	DraftRevisionIDs        []string  `json:"draft_revision_ids"`
	ConfirmationFingerprint string    `json:"confirmation_fingerprint"`
	Confirmed               bool      `json:"confirmed"`
	GrantVersion            int64     `json:"grant_version"`
	ExpiresAt               time.Time `json:"expires_at"`
	CreatedAt               time.Time `json:"created_at"`
}

type SubmitInput struct {
	RequestID               string               `json:"request_id"`
	SourceID                string               `json:"source_id"`
	ArticleID               string               `json:"article_id"`
	SourceVersion           int64                `json:"source_version"`
	StateSeq                int64                `json:"state_seq"`
	ContentFingerprint      string               `json:"content_fingerprint"`
	PreparationID           string               `json:"preparation_id"`
	DraftRevisionIDs        []string             `json:"draft_revision_ids"`
	ConfirmationFingerprint string               `json:"confirmation_fingerprint"`
	Targets                 []DistributionTarget `json:"targets"`
}

type Distribution struct {
	ID                      string               `json:"id"`
	ClientID                string               `json:"client_id"`
	SourceID                string               `json:"source_id"`
	ArticleID               string               `json:"article_id"`
	RequestID               string               `json:"request_id"`
	PreparationID           string               `json:"preparation_id"`
	SourceVersion           int64                `json:"source_version"`
	StateSeq                int64                `json:"state_seq"`
	ContentFingerprint      string               `json:"content_fingerprint"`
	ConfirmationFingerprint string               `json:"confirmation_fingerprint"`
	GrantVersion            int64                `json:"grant_version"`
	Targets                 []DistributionTarget `json:"targets"`
	JobIDs                  []string             `json:"job_ids"`
	CreatedAt               time.Time            `json:"created_at"`
	Children                []DistributionChild  `json:"children"`
	Status                  string               `json:"status"`
	CompletedChildren       int                  `json:"completed_children"`
	ResumeAllowed           bool                 `json:"resume_allowed"`
	DetailURL               string               `json:"detail_url"`
}

type DistributionChild struct {
	ID              string     `json:"id"`
	Platform        string     `json:"platform"`
	AccountID       string     `json:"account_id"`
	Operation       string     `json:"operation"`
	Status          string     `json:"status"`
	ExternalStatus  string     `json:"external_status"`
	Outcome         string     `json:"outcome"`
	LastConfirmedAt *time.Time `json:"last_confirmed_at"`
	RemoteID        string     `json:"remote_id"`
	RemoteURL       string     `json:"remote_url"`
	ErrorCode       string     `json:"error_code"`
	ErrorMessage    string     `json:"error_message"`
	DetailURL       string     `json:"detail_url"`
}

type SourceEvent struct {
	EventID            string `json:"event_id"`
	SourceID           string `json:"source_id"`
	ArticleID          string `json:"article_id"`
	StateSeq           int64  `json:"state_seq"`
	SourceVersion      int64  `json:"source_version"`
	Type               string `json:"type"`
	Status             string `json:"status"`
	AuthorizationID    string `json:"authorization_id,omitempty"`
	AuthorizationValid bool   `json:"authorization_valid"`
	ContentFingerprint string `json:"content_fingerprint,omitempty"`
}

func (c *Client) CreatePreparation(ctx context.Context, body []byte) (Preparation, error) {
	if len(body) == 0 {
		return Preparation{}, ErrInvalidRequest
	}
	return c.preparation(ctx, http.MethodPost, "/preparations", body, http.StatusCreated, http.StatusOK)
}

func (c *Client) GetPreparation(ctx context.Context, id string) (Preparation, error) {
	if !requestKey.MatchString(id) {
		return Preparation{}, ErrInvalidRequest
	}
	out, err := c.preparation(ctx, http.MethodGet, "/preparations/"+id, nil, http.StatusOK)
	if err == nil && out.ID != id {
		return Preparation{}, ErrUnavailable
	}
	return out, err
}

func (c *Client) preparation(ctx context.Context, method, path string, body []byte, status int, alternative ...int) (Preparation, error) {
	var out Preparation
	if err := c.request(ctx, method, path, body, "", status, &out, alternative...); err != nil {
		return Preparation{}, err
	}
	if !requestKey.MatchString(out.ID) {
		return Preparation{}, ErrUnavailable
	}
	out.PreviewURL = safeHTTPSLink(out.PreviewURL, strings.TrimPrefix(c.origin, "https://"))
	return out, nil
}

func (c *Client) Submit(ctx context.Context, body []byte) (Distribution, error) {
	if len(body) == 0 {
		return Distribution{}, ErrInvalidRequest
	}
	return c.distribution(ctx, http.MethodPost, "/distributions", body, http.StatusAccepted, http.StatusOK)
}

func (c *Client) GetDistribution(ctx context.Context, id string) (Distribution, error) {
	return c.distributionByID(ctx, http.MethodGet, id, "")
}

func (c *Client) FindDistribution(ctx context.Context, requestID string) (Distribution, error) {
	if !requestKey.MatchString(requestID) {
		return Distribution{}, ErrInvalidRequest
	}
	query := url.Values{"request_id": []string{requestID}}
	out, err := c.distribution(ctx, http.MethodGet, "/distributions?"+query.Encode(), nil, http.StatusOK)
	if err == nil && out.RequestID != requestID {
		return Distribution{}, ErrUnavailable
	}
	return out, err
}

func (c *Client) CancelDistribution(ctx context.Context, id string) (Distribution, error) {
	return c.distributionByID(ctx, http.MethodPost, id, "/cancel")
}

func (c *Client) distributionByID(ctx context.Context, method, id, suffix string) (Distribution, error) {
	if !requestKey.MatchString(id) {
		return Distribution{}, ErrInvalidRequest
	}
	out, err := c.distribution(ctx, method, "/distributions/"+id+suffix, nil, http.StatusOK)
	if err == nil && out.ID != id {
		return Distribution{}, ErrUnavailable
	}
	return out, err
}

func (c *Client) distribution(ctx context.Context, method, path string, body []byte, status int, alternative ...int) (Distribution, error) {
	var out Distribution
	if err := c.request(ctx, method, path, body, "", status, &out, alternative...); err != nil {
		return Distribution{}, err
	}
	if !requestKey.MatchString(out.ID) {
		return Distribution{}, ErrUnavailable
	}
	host := strings.TrimPrefix(c.origin, "https://")
	out.DetailURL = safeHTTPSLink(out.DetailURL, host)
	for i := range out.Children {
		child := &out.Children[i]
		child.DetailURL = safeHTTPSLink(child.DetailURL, host)
		child.RemoteURL = safeHTTPSLink(child.RemoteURL, "editor.csdn.net")
	}
	return out, nil
}

func (c *Client) SendSourceEvent(ctx context.Context, body []byte) error {
	if len(body) == 0 {
		return ErrInvalidRequest
	}
	var result struct {
		Received bool `json:"received"`
	}
	if err := c.request(ctx, http.MethodPost, "/source-events", body, "", http.StatusOK, &result); err != nil {
		return err
	}
	if !result.Received {
		return ErrUnavailable
	}
	return nil
}

func safeHTTPSLink(raw, host string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || !strings.EqualFold(u.Host, host) {
		return ""
	}
	return raw
}
