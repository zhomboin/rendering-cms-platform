package articlemcp

import (
	"context"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type Visibility struct {
	CMS             string `json:"cms"`
	Version         int32  `json:"version"`
	Page            string `json:"page"`
	Search          string `json:"search"`
	SearchCheckedAt string `json:"searchCheckedAt,omitempty"`
	PublicURL       string `json:"publicUrl,omitempty"`
	CheckedAt       string `json:"checkedAt"`
}

func (s *Server) status(ctx context.Context, p principal, in ArticleInput) (Visibility, error) {
	a, err := s.store.read(ctx, p, in.ArticleID, statusTool)
	if err != nil {
		return Visibility{}, err
	}
	result := Visibility{CMS: a.Status, Version: a.Version, Page: "not_checked", Search: "not_checked", PublicURL: a.PublicURL, CheckedAt: time.Now().UTC().Format(time.RFC3339)}
	if a.Status != "published" {
		return result, nil
	}
	if a.searchResult != "" {
		result.Search = a.searchResult
		result.SearchCheckedAt = a.searchCheckedAt
	}
	result.Page = s.probe(ctx, a.PublicURL)
	return result, nil
}

func (s *Server) probe(ctx context.Context, addr string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return "unverified"
	}
	req.Header.Set("Accept", "text/html")
	res, err := s.client.Do(req)
	if err != nil {
		return "unverified"
	}
	defer res.Body.Close()
	if res.StatusCode == 404 || res.StatusCode == 410 {
		return "not_found"
	}
	mimeType, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if res.StatusCode != 200 || mimeType != "text/html" {
		return "unverified"
	}
	if hasCanonical(io.LimitReader(res.Body, 1<<20), req.URL) {
		return "reachable"
	}
	return "unverified"
}

func hasCanonical(body io.Reader, expected *url.URL) bool {
	z := html.NewTokenizer(body)
	for {
		switch z.Next() {
		case html.ErrorToken:
			return false
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if t.Data == "body" || t.Data == "base" {
				return false
			}
			if t.Data != "link" {
				continue
			}
			var rel, href string
			for _, a := range t.Attr {
				switch a.Key {
				case "rel":
					rel = a.Val
				case "href":
					href = a.Val
				}
			}
			if !slices.Contains(strings.Fields(strings.ToLower(rel)), "canonical") {
				continue
			}
			u, err := url.Parse(href)
			return err == nil && href != "" && expected.ResolveReference(u).String() == expected.String()
		}
	}
}
