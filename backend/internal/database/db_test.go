package database

import (
	"context"
	"testing"
)

func TestOpenCreatesPoolFromDatabaseURL(t *testing.T) {
	pool, err := Open(context.Background(), "postgres://rendering:secret@127.0.0.1:5432/rendering_cms?sslmode=disable")
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer pool.Close()
}
