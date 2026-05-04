package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestParseMDXPostReadsRenderingFrontMatter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "redis-sentinel-with-docker.mdx")
	content := "\ufeff" + `---
title: "Redis Sentinel with Docker"
description: "A practical Sentinel deployment note."
publishedAt: "2026-03-19"
tags:
  - redis
  - docker
draft: false
featured: true
coverImageUrl: "/images/blog/redis-sentinel-with-docker/cover.png"
---

## Body

Content here.
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	post, err := ParseMDXPost(path)
	if err != nil {
		t.Fatal(err)
	}

	if post.Slug != "redis-sentinel-with-docker" {
		t.Fatalf("Slug = %q", post.Slug)
	}
	if post.Title != "Redis Sentinel with Docker" {
		t.Fatalf("Title = %q", post.Title)
	}
	if post.Summary != "A practical Sentinel deployment note." {
		t.Fatalf("Summary = %q", post.Summary)
	}
	if got := post.PublishedAt.Format(time.DateOnly); got != "2026-03-19" {
		t.Fatalf("PublishedAt = %q", got)
	}
	if len(post.Tags) != 2 || post.Tags[0] != "redis" || post.Tags[1] != "docker" {
		t.Fatalf("Tags = %#v", post.Tags)
	}
	if !post.Featured {
		t.Fatal("Featured = false")
	}
	if post.CoverImageURL != "/images/blog/redis-sentinel-with-docker/cover.png" {
		t.Fatalf("CoverImageURL = %q", post.CoverImageURL)
	}
	if post.BodyMDX != "## Body\n\nContent here.\n" {
		t.Fatalf("BodyMDX = %q", post.BodyMDX)
	}
}

func TestParseMDXPostRejectsMissingRequiredMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing-title.mdx")
	content := `---
description: "Missing title"
publishedAt: "2026-03-19"
tags:
  - redis
---

Body
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ParseMDXPost(path); err == nil {
		t.Fatal("expected missing title error")
	}
}

func TestImportPostsSkipsDraftsAndWritesRevisions(t *testing.T) {
	publishedAt := time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC)
	store := &recordingStore{
		authorID:  pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		articleID: pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
	}
	posts := []ImportedPost{
		{
			Slug:        "published-post",
			Title:       "Published",
			Summary:     "Summary",
			BodyMDX:     "Body",
			Tags:        []string{"go"},
			PublishedAt: publishedAt,
		},
		{
			Slug:        "draft-post",
			Title:       "Draft",
			Summary:     "Summary",
			BodyMDX:     "Body",
			Tags:        []string{"go"},
			PublishedAt: publishedAt,
			Draft:       true,
		},
	}

	result, err := ImportPosts(context.Background(), store, posts, "")
	if err != nil {
		t.Fatal(err)
	}

	if result.Imported != 1 || result.SkippedDrafts != 1 {
		t.Fatalf("result = %#v", result)
	}
	if len(store.upserts) != 1 || store.upserts[0].Slug != "published-post" {
		t.Fatalf("upserts = %#v", store.upserts)
	}
	if len(store.revisions) != 1 || store.revisions[0].Title != "Published" {
		t.Fatalf("revisions = %#v", store.revisions)
	}
}

type recordingStore struct {
	authorID  pgtype.UUID
	articleID pgtype.UUID
	upserts   []ImportedPost
	revisions []ImportedPost
}

func (s *recordingStore) ResolveImportAuthor(ctx context.Context, email string) (pgtype.UUID, error) {
	return s.authorID, nil
}

func (s *recordingStore) UpsertPublishedArticle(ctx context.Context, post ImportedPost, authorID pgtype.UUID) (pgtype.UUID, error) {
	s.upserts = append(s.upserts, post)
	return s.articleID, nil
}

func (s *recordingStore) CreateArticleRevision(ctx context.Context, articleID pgtype.UUID, post ImportedPost, authorID pgtype.UUID) error {
	s.revisions = append(s.revisions, post)
	return nil
}
