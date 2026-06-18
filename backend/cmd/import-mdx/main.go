package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/articles"
	"rendering-cms-platform/backend/internal/database"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

func main() {
	source := flag.String("source", "", "MDX source directory")
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL database URL")
	authorEmail := flag.String("author-email", os.Getenv("IMPORT_AUTHOR_EMAIL"), "import author email; defaults to first admin/editor user")
	dryRun := flag.Bool("dry-run", false, "parse files without writing to database")
	flag.Parse()

	if *source == "" {
		fmt.Fprintln(os.Stderr, "missing -source")
		os.Exit(2)
	}
	if *databaseURL == "" && !*dryRun {
		fmt.Fprintln(os.Stderr, "missing -database-url or DATABASE_URL")
		os.Exit(2)
	}

	posts, err := ScanMDXPosts(*source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan posts: %v\n", err)
		os.Exit(1)
	}

	if *dryRun {
		for _, post := range posts {
			status := "published"
			if post.Draft {
				status = "draft"
			}
			fmt.Printf("%s\t%s\t%s\n", status, post.Slug, post.Title)
		}
		return
	}

	ctx := context.Background()
	pool, err := database.Open(ctx, *databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	store := NewDatabaseImportStore(dbgen.New(pool))
	result, err := ImportPosts(ctx, store, posts, *authorEmail)
	if err != nil {
		fmt.Fprintf(os.Stderr, "import posts: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("imported=%d skipped_drafts=%d\n", result.Imported, result.SkippedDrafts)
}

type ImportedPost struct {
	Slug          string
	ArticleName   string
	Title         string
	Summary       string
	BodyMDX       string
	Tags          []string
	Draft         bool
	Featured      bool
	CoverImageURL string
	PublishedAt   time.Time
}

type ImportResult struct {
	Imported      int
	SkippedDrafts int
}

type ImportStore interface {
	ResolveImportAuthor(ctx context.Context, email string) (pgtype.UUID, error)
	UpsertPublishedArticle(ctx context.Context, post ImportedPost, authorID pgtype.UUID) (pgtype.UUID, error)
}

type DatabaseImportStore struct {
	queries *dbgen.Queries
}

func NewDatabaseImportStore(queries *dbgen.Queries) DatabaseImportStore {
	return DatabaseImportStore{queries: queries}
}

func (s DatabaseImportStore) ResolveImportAuthor(ctx context.Context, email string) (pgtype.UUID, error) {
	if strings.TrimSpace(email) != "" {
		user, err := s.queries.GetUserByEmail(ctx, strings.TrimSpace(email))
		if err != nil {
			return pgtype.UUID{}, err
		}
		return user.UserID, nil
	}

	user, err := s.queries.GetDefaultImportAuthor(ctx)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return user.UserID, nil
}

func (s DatabaseImportStore) UpsertPublishedArticle(ctx context.Context, post ImportedPost, authorID pgtype.UUID) (pgtype.UUID, error) {
	article, err := s.queries.UpsertPublishedArticleFromImport(ctx, dbgen.UpsertPublishedArticleFromImportParams{
		Slug:          post.Slug,
		ArticleName:   post.ArticleName,
		Title:         post.Title,
		Summary:       post.Summary,
		BodyMdx:       post.BodyMDX,
		Tags:          post.Tags,
		Featured:      post.Featured,
		CoverImageUrl: nullableText(post.CoverImageURL),
		PublishedAt:   pgtype.Timestamptz{Time: post.PublishedAt, Valid: true},
		AuthorID:      authorID,
	})
	if err != nil {
		return pgtype.UUID{}, err
	}
	return article.ArticleID, nil
}

func ImportPosts(ctx context.Context, store ImportStore, posts []ImportedPost, authorEmail string) (ImportResult, error) {
	authorID, err := store.ResolveImportAuthor(ctx, authorEmail)
	if err != nil {
		return ImportResult{}, fmt.Errorf("resolve import author: %w", err)
	}

	var result ImportResult
	for _, post := range posts {
		if post.Draft {
			result.SkippedDrafts++
			continue
		}
		originalSlug := post.Slug
		post.Slug = normalizeImportedSlug(post.Slug)
		if strings.TrimSpace(post.ArticleName) == "" {
			post.ArticleName = originalSlug
		}

		_, err := store.UpsertPublishedArticle(ctx, post, authorID)
		if err != nil {
			return result, fmt.Errorf("upsert %s: %w", post.Slug, err)
		}
		result.Imported++
	}
	return result, nil
}

func normalizeImportedSlug(slug string) string {
	if articles.ValidSlug(slug) {
		return slug
	}
	return articles.StableShortSlugFromString(slug)
}

func ScanMDXPosts(source string) ([]ImportedPost, error) {
	var files []string
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".mdx" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	posts := make([]ImportedPost, 0, len(files))
	for _, file := range files {
		post, err := ParseMDXPost(file)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func ParseMDXPost(path string) (ImportedPost, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ImportedPost{}, err
	}

	frontMatter, body, err := splitFrontMatter(string(data))
	if err != nil {
		return ImportedPost{}, fmt.Errorf("%s: %w", path, err)
	}
	metadata, err := parseFrontMatter(frontMatter)
	if err != nil {
		return ImportedPost{}, fmt.Errorf("%s: %w", path, err)
	}

	publishedAt, err := parseDate(required(metadata, "publishedAt"))
	if err != nil {
		return ImportedPost{}, fmt.Errorf("%s: publishedAt: %w", path, err)
	}

	post := ImportedPost{
		Slug:          articles.StableShortSlugFromString(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))),
		ArticleName:   strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Title:         firstString(required(metadata, "title")),
		Summary:       firstString(firstNonEmpty(metadata["description"], metadata["summary"])),
		BodyMDX:       body,
		Tags:          metadata["tags"],
		Draft:         parseBool(firstNonEmpty(metadata["draft"], []string{"false"})),
		Featured:      parseBool(metadata["featured"]),
		CoverImageURL: firstString(metadata["coverImageUrl"], metadata["coverImageURL"], metadata["cover"]),
		PublishedAt:   publishedAt,
	}

	if post.Title == "" {
		return ImportedPost{}, fmt.Errorf("%s: title is required", path)
	}
	if post.Summary == "" {
		return ImportedPost{}, fmt.Errorf("%s: description or summary is required", path)
	}
	if post.PublishedAt.IsZero() {
		return ImportedPost{}, fmt.Errorf("%s: publishedAt is required", path)
	}
	if len(post.Tags) == 0 {
		return ImportedPost{}, fmt.Errorf("%s: tags are required", path)
	}
	if strings.TrimSpace(post.BodyMDX) == "" {
		return ImportedPost{}, fmt.Errorf("%s: body is required", path)
	}

	return post, nil
}

func splitFrontMatter(content string) (string, string, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.TrimPrefix(normalized, "\ufeff")
	if !strings.HasPrefix(normalized, "---\n") {
		return "", "", errors.New("front matter is required")
	}
	rest := strings.TrimPrefix(normalized, "---\n")
	index := strings.Index(rest, "\n---\n")
	if index < 0 {
		return "", "", errors.New("front matter closing marker is required")
	}
	return rest[:index], strings.TrimPrefix(rest[index+5:], "\n"), nil
}

func parseFrontMatter(input string) (map[string][]string, error) {
	result := map[string][]string{}
	var currentListKey string

	for _, rawLine := range strings.Split(input, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if currentListKey != "" && strings.HasPrefix(line, "- ") {
			result[currentListKey] = append(result[currentListKey], unquote(strings.TrimSpace(strings.TrimPrefix(line, "- "))))
			continue
		}
		currentListKey = ""

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid front matter line %q", rawLine)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, fmt.Errorf("empty front matter key in line %q", rawLine)
		}
		if value == "" {
			result[key] = []string{}
			currentListKey = key
			continue
		}
		result[key] = parseValueList(value)
	}

	return result, nil
}

func parseValueList(value string) []string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimSuffix(strings.TrimPrefix(value, "["), "]")
		if strings.TrimSpace(value) == "" {
			return nil
		}
		parts := strings.Split(value, ",")
		values := make([]string, 0, len(parts))
		for _, part := range parts {
			values = append(values, unquote(strings.TrimSpace(part)))
		}
		return values
	}
	return []string{unquote(value)}
}

func required(metadata map[string][]string, key string) []string {
	return metadata[key]
}

func firstNonEmpty(values ...[]string) []string {
	for _, list := range values {
		if firstString(list) != "" {
			return list
		}
	}
	return nil
}

func firstString(values ...[]string) string {
	for _, list := range values {
		for _, value := range list {
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func parseBool(values []string) bool {
	switch strings.ToLower(firstString(values)) {
	case "true", "yes", "1":
		return true
	default:
		return false
	}
}

func parseDate(value []string) (time.Time, error) {
	raw := firstString(value)
	if raw == "" {
		return time.Time{}, errors.New("date is required")
	}
	for _, layout := range []string{time.DateOnly, time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q", raw)
}

func unquote(value string) string {
	if len(value) < 2 {
		return strings.TrimSpace(value)
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return strings.TrimSpace(value)
}

func nullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: strings.TrimSpace(value) != ""}
}
