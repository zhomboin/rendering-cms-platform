package comments

import "testing"

func TestNewCommentDefaultsToPending(t *testing.T) {
	comment := NewComment("Alice", "hello")

	if comment.AuthorName != "Alice" {
		t.Fatalf("expected author name Alice, got %q", comment.AuthorName)
	}
	if comment.Body != "hello" {
		t.Fatalf("expected body hello, got %q", comment.Body)
	}
	if comment.Status != StatusPending {
		t.Fatalf("expected status %q, got %q", StatusPending, comment.Status)
	}
}
