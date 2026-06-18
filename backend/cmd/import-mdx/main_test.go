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

	if !isShortSlugForTest(post.Slug) {
		t.Fatalf("Slug = %q, want six Base62 characters", post.Slug)
	}
	if post.ArticleName != "redis-sentinel-with-docker" {
		t.Fatalf("ArticleName = %q", post.ArticleName)
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

func TestImportPostsSkipsDraftsAndUpsertsPublishedArticles(t *testing.T) {
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
	if len(store.upserts) != 1 || !isShortSlugForTest(store.upserts[0].Slug) {
		t.Fatalf("upserts = %#v", store.upserts)
	}
	if store.upserts[0].ArticleName != "published-post" {
		t.Fatalf("ArticleName = %q, want original imported slug", store.upserts[0].ArticleName)
	}
}

type recordingStore struct {
	authorID  pgtype.UUID
	articleID pgtype.UUID
	upserts   []ImportedPost
}

func (s *recordingStore) ResolveImportAuthor(ctx context.Context, email string) (pgtype.UUID, error) {
	return s.authorID, nil
}

func (s *recordingStore) UpsertPublishedArticle(ctx context.Context, post ImportedPost, authorID pgtype.UUID) (pgtype.UUID, error) {
	s.upserts = append(s.upserts, post)
	return s.articleID, nil
}

func isShortSlugForTest(value string) bool {
	if len(value) != 6 {
		return false
	}
	for _, char := range value {
		if char >= '0' && char <= '9' {
			continue
		}
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		return false
	}
	return true
}
