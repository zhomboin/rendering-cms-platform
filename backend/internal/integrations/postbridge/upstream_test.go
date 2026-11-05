package postbridge

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestUpstreamUsesFrozenBytesAndSupportsIdempotentResponses(t *testing.T) {
	ctx := context.Background()
	body := []byte("{ \"request_id\": \"synthetic-request\" }\n")
	lookupID := "submit:article.1_2-3"
	const preparation = `{"id":"prep-test","preview_url":"https://bridge.example/integration-preparations/prep-test","draft_revision_ids":["draft-test"],"confirmed":true}`
	const distribution = `{"id":"dist-test","request_id":"submit:article.1_2-3","detail_url":"https://bridge.example/integrations?tab=distributions","children":[{"id":"job-test","external_status":"draft_saved","remote_url":"https://editor.csdn.net/md/?articleId=1"}]}`
	c := testClient(t)
	steps := []struct {
		method, uri, body string
		status            int
		response          string
		run               func() error
	}{
		{http.MethodPost, "/preparations", string(body), 201, preparation, func() error {
			got, err := c.CreatePreparation(ctx, body)
			if err == nil && (got.ID != "prep-test" || got.PreviewURL == "" || len(got.DraftRevisionIDs) != 1) {
				t.Errorf("unexpected preparation: %+v", got)
			}
			return err
		}},
		{http.MethodPost, "/preparations", string(body), 200, preparation, func() error {
			_, err := c.CreatePreparation(ctx, body)
			return err
		}},
		{http.MethodGet, "/preparations/prep-test", "", 200, preparation, func() error {
			_, err := c.GetPreparation(ctx, "prep-test")
			return err
		}},
		{http.MethodPost, "/distributions", string(body), 202, distribution, func() error {
			got, err := c.Submit(ctx, body)
			if err == nil && (got.ID != "dist-test" || got.DetailURL == "" || len(got.Children) != 1 || got.Children[0].RemoteURL == "") {
				t.Errorf("unexpected distribution: %+v", got)
			}
			return err
		}},
		{http.MethodPost, "/distributions", string(body), 200, distribution, func() error {
			_, err := c.Submit(ctx, body)
			return err
		}},
		{http.MethodGet, "/distributions/dist-test", "", 200, distribution, func() error {
			_, err := c.GetDistribution(ctx, "dist-test")
			return err
		}},
		{http.MethodGet, "/distributions?request_id=submit%3Aarticle.1_2-3", "", 200, distribution, func() error {
			_, err := c.FindDistribution(ctx, lookupID)
			return err
		}},
		{http.MethodPost, "/distributions/dist-test/cancel", "", 200, distribution, func() error {
			_, err := c.CancelDistribution(ctx, "dist-test")
			return err
		}},
		{http.MethodPost, "/source-events", string(body), 200, `{"received":true,"applied":false}`, func() error {
			return c.SendSourceEvent(ctx, body)
		}},
	}

	for _, step := range steps {
		t.Run(step.method+" "+step.uri, func(t *testing.T) {
			c.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != step.method || req.URL.RequestURI() != apiPrefix+step.uri {
					t.Errorf("request = %s %s", req.Method, req.URL.RequestURI())
				}
				var raw []byte
				if req.Body != nil {
					var err error
					raw, err = io.ReadAll(req.Body)
					if err != nil {
						t.Fatal(err)
					}
				}
				if string(raw) != step.body {
					t.Errorf("body = %q, want %q", raw, step.body)
				}
				digest := sha256.Sum256([]byte(step.body))
				wantDigest := "sha-256=:" + base64.StdEncoding.EncodeToString(digest[:]) + ":"
				if req.Header.Get("Content-Digest") != wantDigest || req.Header.Get("Signature") == "" {
					t.Error("request was not signed over the expected body")
				}
				return response(step.status, step.response), nil
			})
			if err := step.run(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUpstreamDropsUntrustedLinks(t *testing.T) {
	c := testClient(t)
	c.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case apiPrefix + "/preparations/prep-test":
			return response(200, `{"id":"prep-test","preview_url":"https://bridge.example@unexpected.example/preview"}`), nil
		default:
			return response(200, `{"id":"dist-test","detail_url":"javascript:alert(1)","children":[{"id":"job-test","remote_url":"https://unexpected.example/","detail_url":"//unexpected.example/job"}]}`), nil
		}
	})
	gotPreparation, err := c.GetPreparation(context.Background(), "prep-test")
	if err != nil || gotPreparation.PreviewURL != "" {
		t.Fatalf("preparation URL = %q, err = %v", gotPreparation.PreviewURL, err)
	}
	gotDistribution, err := c.GetDistribution(context.Background(), "dist-test")
	if err != nil || gotDistribution.DetailURL != "" || gotDistribution.Children[0].RemoteURL != "" || gotDistribution.Children[0].DetailURL != "" {
		t.Fatalf("unsafe distribution links or error: %+v, %v", gotDistribution, err)
	}
}

func TestUpstreamRejectsInvalidIdentifiersAndUnacknowledgedEvents(t *testing.T) {
	c := testClient(t)
	calls := 0
	c.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		return response(200, `{"received":false}`), nil
	})
	ctx := context.Background()
	if _, err := c.GetPreparation(ctx, "../targets"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("GetPreparation returned %v", err)
	}
	if _, err := c.FindDistribution(ctx, "id&extra=true"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("FindDistribution returned %v", err)
	}
	if _, err := c.Submit(ctx, []byte(strings.Repeat("x", maxRequestSize+1))); !errors.Is(err, ErrRequestTooLarge) {
		t.Fatalf("Submit returned %v", err)
	}
	if calls != 0 {
		t.Fatal("invalid request reached upstream")
	}
	if err := c.SendSourceEvent(ctx, []byte("{}")); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("SendSourceEvent returned %v", err)
	}
}

func TestUpstreamPreservesConflictCodeAndRejectsMismatchedRecords(t *testing.T) {
	c := testClient(t)
	c.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPost {
			return response(409, `{"code":"distribution_not_cancellable","message":"synthetic detail"}`), nil
		}
		return response(200, `{"id":"unrelated"}`), nil
	})
	ctx := context.Background()
	var upstream *UpstreamError
	if _, err := c.CancelDistribution(ctx, "dist-test"); !errors.As(err, &upstream) || upstream.Code != "distribution_not_cancellable" || upstream.Status != 409 {
		t.Fatalf("CancelDistribution returned %v", err)
	}
	if _, err := c.GetDistribution(ctx, "dist-test"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("GetDistribution returned %v", err)
	}
}
