package postbridge

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	apiPrefix           = "/api/integrations/v1"
	maxRequestSize      = 1 << 20
	maxResponseSize     = 4 << 20
	signatureComponents = `("@method" "@target-uri" "content-digest" "x-integration-client" "x-integration-environment" "x-integration-service")`
)

var (
	ErrNotConfigured       = errors.New("PostBridge signing credentials are not configured")
	ErrInvalidRegistration = errors.New("invalid PostBridge registration input")
	ErrUnavailable         = errors.New("PostBridge request could not be completed")
	identifier             = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	errorCode              = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

type Identity struct {
	ClientID    string
	KeyID       string
	Environment string
	ServiceID   string
	Secret      string
}

type Client struct {
	origin      string
	clientID    string
	keyID       string
	environment string
	serviceID   string
	secret      []byte
	http        *http.Client
}

func NewClient(origin string, identity Identity) (*Client, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawPath != "" || u.ForceQuery || u.RawQuery != "" || strings.Contains(origin, "#") {
		return nil, errors.New("PostBridge origin must be an HTTPS origin")
	}
	if !identifier.MatchString(identity.Environment) {
		return nil, errors.New("invalid PostBridge environment")
	}
	client := &Client{
		origin:      u.Scheme + "://" + u.Host,
		environment: identity.Environment,
		http: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	if identity.ClientID == "" && identity.KeyID == "" && identity.ServiceID == "" && identity.Secret == "" {
		return client, nil
	}
	if !identifier.MatchString(identity.ClientID) || !identifier.MatchString(identity.KeyID) || !identifier.MatchString(identity.ServiceID) {
		return nil, errors.New("invalid PostBridge signing identity")
	}
	key, err := base64.StdEncoding.Strict().DecodeString(identity.Secret)
	if err != nil || len(key) != sha256.Size {
		return nil, errors.New("PostBridge secret must encode 32 bytes using standard Base64")
	}
	client.clientID, client.keyID, client.serviceID, client.secret = identity.ClientID, identity.KeyID, identity.ServiceID, key
	return client, nil
}

func (c *Client) SigningConfigured() bool {
	return len(c.secret) != 0
}

type RegistrationGrant struct {
	SourceID string
	Accounts []string
	Scopes   []string
}

type registrationInput struct {
	Name        string   `json:"name"`
	Environment string   `json:"environment"`
	SourceID    string   `json:"source_id"`
	Scopes      []string `json:"scopes"`
	Accounts    []string `json:"accounts"`
	Operations  []string `json:"operations"`
}

type Registration struct {
	ID     string `json:"registration_id"`
	Status string `json:"status"`
}

func (c *Client) Register(ctx context.Context, invitation, name string, grant RegistrationGrant) (Registration, error) {
	name = strings.TrimSpace(name)
	if invitation == "" || strings.ContainsAny(invitation, "\r\n") || name == "" || !identifier.MatchString(grant.SourceID) || len(grant.Accounts) == 0 {
		return Registration{}, ErrInvalidRegistration
	}
	scopes := grant.Scopes
	if len(scopes) == 0 {
		scopes = []string{"targets:read"}
	}
	allowedScopes := []string{"targets:read", "preparations:write", "preparations:read", "distributions:write", "distributions:read", "source-events:write"}
	for _, scope := range scopes {
		if !slices.Contains(allowedScopes, scope) {
			return Registration{}, ErrInvalidRegistration
		}
	}
	payload, err := json.Marshal(registrationInput{
		Name: name, Environment: c.environment, SourceID: grant.SourceID, Accounts: grant.Accounts,
		Scopes:     scopes,
		Operations: []string{"save_draft"},
	})
	if err != nil {
		return Registration{}, err
	}
	var result Registration
	if err := c.request(ctx, http.MethodPost, "/registrations", payload, invitation, http.StatusCreated, &result); err != nil {
		return Registration{}, err
	}
	if result.ID == "" || result.Status != "pending" {
		return Registration{}, ErrUnavailable
	}
	return result, nil
}

func (c *Client) Activate(ctx context.Context) error {
	var result struct {
		Status string `json:"status"`
	}
	if err := c.request(ctx, http.MethodPost, "/clients/self/activate", nil, "", http.StatusOK, &result); err != nil {
		return err
	}
	if result.Status != "active" {
		return ErrUnavailable
	}
	return nil
}

type Target struct {
	Platform      string     `json:"platform"`
	AccountID     string     `json:"account_id"`
	DisplayName   string     `json:"display_name"`
	Status        string     `json:"status"`
	LastCheckedAt *time.Time `json:"last_checked_at"`
	Operations    []string   `json:"operations"`
	BodyFormats   []string   `json:"body_formats"`
	Images        bool       `json:"images"`
	TitleMin      int        `json:"title_min"`
	TitleMax      int        `json:"title_max"`
	BodyMax       int        `json:"body_max"`
}

func (c *Client) Targets(ctx context.Context) ([]Target, error) {
	var result struct {
		Items []Target `json:"items"`
	}
	if err := c.request(ctx, http.MethodGet, "/targets", nil, "", http.StatusOK, &result); err != nil {
		return nil, err
	}
	if result.Items == nil {
		return nil, ErrUnavailable
	}
	return result.Items, nil
}

type UpstreamError struct {
	Status     int
	Code       string
	RetryAfter string
}

func (e *UpstreamError) Error() string {
	return fmt.Sprintf("PostBridge returned HTTP %d (%s)", e.Status, e.Code)
}

func (c *Client) request(ctx context.Context, method, path string, body []byte, invitation string, wantStatus int, result any, alternateStatuses ...int) error {
	if len(body) > maxRequestSize {
		return ErrRequestTooLarge
	}
	req, err := http.NewRequestWithContext(ctx, method, c.origin+apiPrefix+path, bytes.NewReader(body))
	if err != nil {
		return ErrUnavailable
	}
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if invitation != "" {
		req.Header.Set("X-Integration-Invitation", invitation)
	} else {
		if !c.SigningConfigured() {
			return ErrNotConfigured
		}
		c.sign(req, body, time.Now(), rand.Text())
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil || len(raw) > maxResponseSize {
		return ErrUnavailable
	}
	if resp.StatusCode != wantStatus && !slices.Contains(alternateStatuses, resp.StatusCode) {
		var upstream struct {
			Code string `json:"code"`
		}
		var code string
		if json.Unmarshal(raw, &upstream) == nil && errorCode.MatchString(upstream.Code) {
			code = upstream.Code
		}
		return &UpstreamError{Status: resp.StatusCode, Code: code, RetryAfter: resp.Header.Get("Retry-After")}
	}
	if err := json.Unmarshal(raw, result); err != nil {
		return ErrUnavailable
	}
	return nil
}

func (c *Client) sign(req *http.Request, body []byte, now time.Time, nonce string) {
	sum := sha256.Sum256(body)
	digest := "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
	params := fmt.Sprintf(`%s;created=%d;keyid="%s";nonce="%s";alg="hmac-sha256"`, signatureComponents, now.Unix(), c.keyID, nonce)
	base := strings.Join([]string{
		`"@method": ` + req.Method,
		`"@target-uri": ` + req.URL.String(),
		`"content-digest": ` + digest,
		`"x-integration-client": ` + c.clientID,
		`"x-integration-environment": ` + c.environment,
		`"x-integration-service": ` + c.serviceID,
		`"@signature-params": ` + params,
	}, "\n")
	mac := hmac.New(sha256.New, c.secret)
	mac.Write([]byte(base))

	req.Header.Set("Content-Digest", digest)
	req.Header.Set("X-Integration-Client", c.clientID)
	req.Header.Set("X-Integration-Environment", c.environment)
	req.Header.Set("X-Integration-Service", c.serviceID)
	req.Header.Set("Signature-Input", "sig1="+params)
	req.Header.Set("Signature", "sig1=:"+base64.StdEncoding.EncodeToString(mac.Sum(nil))+":")
}
