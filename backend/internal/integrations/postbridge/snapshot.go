package postbridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrUnsupportedContent = errors.New("PostBridge does not support this content")
	unsupportedMarkdown   = regexp.MustCompile(`!\[|<\s*/?\s*[A-Za-z]|\{|\}|^\s*(import|export)\s`)
)

type SnapshotAsset struct {
	ID       string `json:"id"`
	Checksum string `json:"checksum"`
}

type Snapshot struct {
	Title         string          `json:"title"`
	Summary       string          `json:"summary"`
	Body          string          `json:"body"`
	BodyFormat    string          `json:"body_format"`
	Tags          []string        `json:"tags"`
	CoverImageURL string          `json:"cover_image_url"`
	Assets        []SnapshotAsset `json:"assets"`
	OriginalURL   string          `json:"original_url,omitempty"`
}

func ValidateSnapshot(s Snapshot) (Snapshot, string, error) {
	s.Title = strings.TrimSpace(s.Title)
	s.Body = strings.ReplaceAll(strings.ReplaceAll(s.Body, "\r\n", "\n"), "\r", "\n")
	if s.Tags == nil {
		s.Tags = []string{}
	}
	if s.Assets == nil {
		s.Assets = []SnapshotAsset{}
	}
	if !validSnapshotText(s.Title) || !validSnapshotText(s.Summary) || !validSnapshotText(s.Body) ||
		utf8.RuneCountInString(s.Summary) > 2000 || len(s.Tags) > 20 {
		return Snapshot{}, "", ErrInvalidRequest
	}
	for _, tag := range s.Tags {
		if !validSnapshotText(tag) || strings.TrimSpace(tag) == "" || utf8.RuneCountInString(tag) > 64 {
			return Snapshot{}, "", ErrInvalidRequest
		}
	}
	if s.OriginalURL != "" {
		u, err := url.Parse(s.OriginalURL)
		if err != nil || u.User != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") {
			return Snapshot{}, "", ErrInvalidRequest
		}
	}
	titleLength := utf8.RuneCountInString(s.Title)
	if s.BodyFormat != "markdown" || s.CoverImageURL != "" || len(s.Assets) != 0 ||
		titleLength < 5 || titleLength > 100 || strings.TrimSpace(s.Body) == "" ||
		utf8.RuneCountInString(s.Body) > 200000 || !supportedMarkdown(s.Body) {
		return Snapshot{}, "", ErrUnsupportedContent
	}

	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode([]any{
		"postbridge-content-v1", s.Title, s.Summary, s.Body, s.BodyFormat, s.Tags, s.CoverImageURL, s.Assets,
	}); err != nil {
		return Snapshot{}, "", err
	}
	digest := sha256.Sum256(bytes.TrimSuffix(canonical.Bytes(), []byte{'\n'}))
	return s, hex.EncodeToString(digest[:]), nil
}

func validSnapshotText(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

func supportedMarkdown(body string) bool {
	var fence byte
	length := 0
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		if indent <= 3 && len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			ch := trimmed[0]
			n := 0
			for n < len(trimmed) && trimmed[n] == ch {
				n++
			}
			if n >= 3 {
				rest := trimmed[n:]
				if fence == 0 && (ch != '`' || !strings.ContainsRune(rest, '`')) {
					fence, length = ch, n
					continue
				}
				if ch == fence && n >= length && strings.Trim(rest, " \t") == "" {
					fence = 0
					continue
				}
			}
		}
		if fence == 0 && unsupportedMarkdown.MatchString(line) {
			return false
		}
	}
	return true
}
