package postbridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrUnsupportedContent = errors.New("PostBridge does not support this content")
	unsupportedMarkdown   = regexp.MustCompile(`<\s*/?\s*[A-Za-z]|\{|\}|^\s*(import|export)\s`)
)

const (
	maxImageOccurrences = 20
	maxDistinctImages   = 10
)

var (
	inlineImage = regexp.MustCompile(`!\[([^\[\]\\\r\n]*)\]\((https://[^()<>\s"\\]+)\)`)
	dnsHostname = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$`)
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
	snapshot, fingerprint, _, err := validateSnapshot(s, false)
	return snapshot, fingerprint, err
}

func validateSnapshot(s Snapshot, allowImages bool) (Snapshot, string, []string, error) {
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
		return Snapshot{}, "", nil, ErrInvalidRequest
	}
	for _, tag := range s.Tags {
		if !validSnapshotText(tag) || strings.TrimSpace(tag) == "" || utf8.RuneCountInString(tag) > 64 {
			return Snapshot{}, "", nil, ErrInvalidRequest
		}
	}
	if s.OriginalURL != "" {
		u, err := url.Parse(s.OriginalURL)
		if err != nil || u.User != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") {
			return Snapshot{}, "", nil, ErrInvalidRequest
		}
	}
	titleLength := utf8.RuneCountInString(s.Title)
	if s.BodyFormat != "markdown" || s.CoverImageURL != "" || len(s.Assets) != 0 ||
		titleLength < 5 || titleLength > 100 || strings.TrimSpace(s.Body) == "" ||
		utf8.RuneCountInString(s.Body) > 200000 {
		return Snapshot{}, "", nil, ErrUnsupportedContent
	}

	imageURLs, ok := markdownImages(s.Body, allowImages)
	if !ok {
		return Snapshot{}, "", nil, ErrUnsupportedContent
	}

	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode([]any{
		"postbridge-content-v1", s.Title, s.Summary, s.Body, s.BodyFormat, s.Tags, s.CoverImageURL, s.Assets,
	}); err != nil {
		return Snapshot{}, "", nil, err
	}
	digest := sha256.Sum256(bytes.TrimSuffix(canonical.Bytes(), []byte{'\n'}))
	return s, hex.EncodeToString(digest[:]), imageURLs, nil
}

func validSnapshotText(s string) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

func markdownImages(body string, allowImages bool) ([]string, bool) {
	var fence byte
	length, occurrences := 0, 0
	var inlineContext markdownInlineContext
	var urls []string
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(body, "\n") {
		// A fence indented in a container may end before a following image.
		// Reject image-like syntax in fenced lines when image transfer is enabled.
		hasImageMarker := strings.Contains(line, "![")
		if allowImages && fence != 0 && hasImageMarker {
			return nil, false
		}
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
					if allowImages && hasImageMarker {
						return nil, false
					}
					fence, length = ch, n
					inlineContext = markdownInlineContext{}
					continue
				}
				if ch == fence && n >= length && strings.Trim(rest, " \t") == "" {
					fence = 0
					continue
				}
			}
		}
		if fence != 0 {
			continue
		}
		if unsupportedMarkdown.MatchString(line) || (allowImages && (strings.Contains(line, "<!") || strings.Contains(line, "<?"))) {
			return nil, false
		}
		if strings.TrimSpace(line) == "" {
			inlineContext = markdownInlineContext{}
			continue
		}
		lineContext := inlineContext
		if allowImages {
			inlineContext = inlineContext.scan(line)
		}
		if !hasImageMarker {
			continue
		}
		if !allowImages || indent >= 4 || strings.HasPrefix(trimmed, "\t") {
			return nil, false
		}
		matches := inlineImage.FindAllStringSubmatchIndex(line, maxImageOccurrences+1)
		if len(matches) != strings.Count(line, "![") {
			return nil, false
		}
		for _, match := range matches {
			raw := line[match[4]:match[5]]
			occurrences++
			prefix := line[:match[0]]
			if occurrences > maxImageOccurrences || !validImageURL(raw) || strings.HasSuffix(prefix, "\\") ||
				lineContext.scan(prefix) != (markdownInlineContext{}) {
				return nil, false
			}
			if !seen[raw] {
				seen[raw] = true
				urls = append(urls, raw)
			}
		}
		if len(urls) > maxDistinctImages {
			return nil, false
		}
	}
	return urls, true
}

// Track the inline constructs in which a literal image token is unsupported.
// Carry the context across soft line breaks so multiline labels are also rejected.
type markdownInlineContext struct {
	codeTicks  int
	linkLabels int
	linkParens int
}

func (c markdownInlineContext) scan(text string) markdownInlineContext {
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if c.codeTicks == 0 && ch == '\\' {
			i++
			continue
		}
		if c.linkParens > 0 {
			switch ch {
			case '(':
				c.linkParens++
			case ')':
				c.linkParens--
			}
			continue
		}
		if ch == '`' {
			start := i
			for i+1 < len(text) && text[i+1] == '`' {
				i++
			}
			length := i - start + 1
			switch c.codeTicks {
			case 0:
				c.codeTicks = length
			case length:
				c.codeTicks = 0
			}
			continue
		}
		if c.codeTicks != 0 {
			continue
		}
		switch ch {
		case '[':
			c.linkLabels++
		case ']':
			if c.linkLabels == 0 {
				continue
			}
			c.linkLabels--
			if i+1 < len(text) && text[i+1] == '(' {
				c.linkParens++
				i++
			}
		}
	}
	return c
}

func validImageURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || !dnsHostname.MatchString(u.Host) ||
		net.ParseIP(u.Host) != nil || u.Path == "" || u.String() != raw ||
		u.ForceQuery || u.RawQuery != "" || strings.ContainsAny(raw, "#&") ||
		strings.ContainsRune(u.Path, '\\') || strings.Contains(strings.ToLower(u.EscapedPath()), "%2f") {
		return false
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return !strings.ContainsFunc(u.Path, func(ch rune) bool { return ch < 32 || ch == 127 })
}
