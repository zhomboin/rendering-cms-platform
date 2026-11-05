package postbridge

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestSnapshotFingerprintMatchesContract(t *testing.T) {
	var fixture struct {
		Content struct {
			Snapshot    Snapshot `json:"snapshot"`
			Fingerprint string   `json:"fingerprint"`
		} `json:"content"`
	}
	raw, err := os.ReadFile("testdata/integration-contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	in := fixture.Content.Snapshot
	in.Title = "  " + in.Title + "  "
	normalized, got, err := ValidateSnapshot(in)
	if err != nil || got != fixture.Content.Fingerprint || normalized.Title != fixture.Content.Snapshot.Title || strings.Contains(normalized.Body, "\r") {
		t.Fatalf("fingerprint = %s, normalized = %+v, err = %v", got, normalized, err)
	}
	in.OriginalURL = "https://blog.example/other"
	_, got, err = ValidateSnapshot(in)
	if err != nil || got != fixture.Content.Fingerprint {
		t.Fatalf("original URL changed fingerprint: %s, %v", got, err)
	}
}

func TestSnapshotContentPolicy(t *testing.T) {
	cases := []struct {
		name, body string
		want       error
	}{
		{"plain", "比较 1 < 2\n正文", nil},
		{"fenced code", "   ```go\nimport \"fmt\"\nfunc f() {}\n   ```", nil},
		{"tilde fence", "~~~~\n<Example />\n~~~\n{still code}\n~~~~\n尾段", nil},
		{"image", "![说明](https://image.example/sample.png)", ErrUnsupportedContent},
		{"html", "<strong>文本</strong>", ErrUnsupportedContent},
		{"mdx expression", "hello {name}", ErrUnsupportedContent},
		{"mdx import", " import Example from './example'", ErrUnsupportedContent},
		{"mdx export", "export const example = 1", ErrUnsupportedContent},
		{"inline braces", "`{literal}`", ErrUnsupportedContent},
		{"too much indentation", "    ```\n{expression}", ErrUnsupportedContent},
		{"tab indentation", "\t```\n{expression}", ErrUnsupportedContent},
		{"invalid backtick fence info", "```bad`lang\n{expression}", ErrUnsupportedContent},
		{"empty", "\t\n ", ErrUnsupportedContent},
		{"too long", strings.Repeat("文", 200001), ErrUnsupportedContent},
		{"invalid utf8", "\xff", ErrInvalidRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			normalized, _, err := ValidateSnapshot(Snapshot{Title: "合成文章标题", Body: tc.body, BodyFormat: "markdown"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if err == nil && (normalized.Assets == nil || normalized.Tags == nil) {
				t.Fatal("empty arrays were not normalized")
			}
		})
	}
}

func TestSnapshotRejectsIncompatibleMetadata(t *testing.T) {
	base := Snapshot{Title: "合成文章标题", Body: "正文", BodyFormat: "markdown"}
	cases := []struct {
		name string
		set  func(*Snapshot)
		want error
	}{
		{"short title", func(s *Snapshot) { s.Title = " 标题 " }, ErrUnsupportedContent},
		{"long title", func(s *Snapshot) { s.Title = strings.Repeat("文", 101) }, ErrUnsupportedContent},
		{"long summary", func(s *Snapshot) { s.Summary = strings.Repeat("文", 2001) }, ErrInvalidRequest},
		{"blank tag", func(s *Snapshot) { s.Tags = []string{" "} }, ErrInvalidRequest},
		{"long tag", func(s *Snapshot) { s.Tags = []string{strings.Repeat("文", 65)} }, ErrInvalidRequest},
		{"null byte", func(s *Snapshot) { s.Title += "\x00" }, ErrInvalidRequest},
		{"invalid original link", func(s *Snapshot) { s.OriginalURL = "javascript:alert(1)" }, ErrInvalidRequest},
		{"cover", func(s *Snapshot) { s.CoverImageURL = "https://image.example/cover" }, ErrUnsupportedContent},
		{"asset", func(s *Snapshot) { s.Assets = []SnapshotAsset{{ID: "test-image"}} }, ErrUnsupportedContent},
		{"format", func(s *Snapshot) { s.BodyFormat = "mdx" }, ErrUnsupportedContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := base
			tc.set(&s)
			if _, _, err := ValidateSnapshot(s); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}
