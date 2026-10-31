package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

type store interface {
	GetDashboardOverview(context.Context, int32) (dbgen.GetDashboardOverviewRow, error)
	ListDashboardRecentArticles(context.Context, dbgen.ListDashboardRecentArticlesParams) ([]dbgen.ListDashboardRecentArticlesRow, error)
	ListDashboardPendingComments(context.Context) ([]dbgen.ListDashboardPendingCommentsRow, error)
	ListDashboardRecentImages(context.Context) ([]dbgen.ListDashboardRecentImagesRow, error)
	ListDashboardArticles(context.Context, int32) ([]dbgen.ListDashboardArticlesRow, error)
	ListDashboardAlbums(context.Context, int32) ([]dbgen.ListDashboardAlbumsRow, error)
}

type Handler struct{ queries store }

func NewHandler(queries store) Handler { return Handler{queries: queries} }

// RegisterAdminRoutes 由主路由的后台身份与角色中间件保护。
func (h Handler) RegisterAdminRoutes(router chi.Router) {
	router.Get("/dashboard/overview", h.overview)
	router.Get("/dashboard/articles", h.articles)
	router.Get("/dashboard/albums", h.albums)
}

type metrics struct {
	TodayViews      int64 `json:"todayViews"`
	PeriodViews     int64 `json:"periodViews"`
	ArticleViews    int64 `json:"articleViews"`
	AlbumViews      int64 `json:"albumViews"`
	NewComments     int64 `json:"newComments"`
	PendingComments int64 `json:"pendingComments"`
}
type content struct {
	PublishedArticles    int64 `json:"publishedArticles"`
	DraftArticles        int64 `json:"draftArticles"`
	PrivateArticles      int64 `json:"privateArticles"`
	ArchivedArticles     int64 `json:"archivedArticles"`
	PublishedAlbums      int64 `json:"publishedAlbums"`
	DraftAlbums          int64 `json:"draftAlbums"`
	PhotoCount           int64 `json:"photoCount"`
	MissingCoverArticles int64 `json:"missingCoverArticles"`
}
type resources struct {
	AssetCount int64 `json:"assetCount"`
	ImageCount int64 `json:"imageCount"`
	TotalBytes int64 `json:"totalBytes"`
}
type articleSummary struct {
	ArticleID     pgtype.UUID         `json:"articleId"`
	Slug          string              `json:"slug"`
	Title         string              `json:"title"`
	Status        dbgen.ArticleStatus `json:"status"`
	CoverImageURL string              `json:"coverImageUrl"`
	Tags          []string            `json:"tags"`
	UpdatedAt     pgtype.Timestamptz  `json:"updatedAt"`
	PublishedAt   pgtype.Timestamptz  `json:"publishedAt"`
}
type commentSummary struct {
	CommentID    pgtype.UUID        `json:"commentId"`
	ArticleID    pgtype.UUID        `json:"articleId"`
	ArticleTitle string             `json:"articleTitle"`
	AuthorName   string             `json:"authorName"`
	Body         string             `json:"body"`
	CreatedAt    pgtype.Timestamptz `json:"createdAt"`
}
type imageSummary struct {
	AssetID   pgtype.UUID        `json:"assetId"`
	Filename  string             `json:"filename"`
	PublicURL string             `json:"publicUrl"`
	ByteSize  int64              `json:"byteSize"`
	CreatedAt pgtype.Timestamptz `json:"createdAt"`
}
type overviewResponse struct {
	Days            int32              `json:"days"`
	GeneratedAt     pgtype.Timestamptz `json:"generatedAt"`
	StartDate       string             `json:"startDate"`
	EndDate         string             `json:"endDate"`
	Metrics         metrics            `json:"metrics"`
	Content         content            `json:"content"`
	Resources       resources          `json:"resources"`
	RecentArticles  []articleSummary   `json:"recentArticles"`
	Drafts          []articleSummary   `json:"drafts"`
	PendingComments []commentSummary   `json:"pendingComments"`
	RecentImages    []imageSummary     `json:"recentImages"`
}

func (h Handler) overview(w http.ResponseWriter, r *http.Request) {
	days := normalizeDays(r.URL.Query().Get("days"))
	row, err := h.queries.GetDashboardOverview(r.Context(), days)
	if err != nil {
		readError(w)
		return
	}
	recent, err := h.queries.ListDashboardRecentArticles(r.Context(), dbgen.ListDashboardRecentArticlesParams{Status: "", RowLimit: 5})
	if err != nil {
		readError(w)
		return
	}
	drafts, err := h.queries.ListDashboardRecentArticles(r.Context(), dbgen.ListDashboardRecentArticlesParams{Status: "draft", RowLimit: 3})
	if err != nil {
		readError(w)
		return
	}
	comments, err := h.queries.ListDashboardPendingComments(r.Context())
	if err != nil {
		readError(w)
		return
	}
	images, err := h.queries.ListDashboardRecentImages(r.Context())
	if err != nil {
		readError(w)
		return
	}
	response := overviewResponse{
		Days: days, GeneratedAt: row.GeneratedAt, StartDate: row.StartDate.Time.Format("2006-01-02"), EndDate: row.EndDate.Time.Format("2006-01-02"),
		Metrics:        metrics{row.TodayViews, row.PeriodViews, row.ArticleViews, row.AlbumViews, row.NewComments, row.PendingComments},
		Content:        content{row.PublishedArticles, row.DraftArticles, row.PrivateArticles, row.ArchivedArticles, row.PublishedAlbums, row.DraftAlbums, row.PhotoCount, row.MissingCoverArticles},
		Resources:      resources{row.AssetCount, row.ImageCount, row.TotalBytes},
		RecentArticles: mapArticleSummaries(recent, 5), Drafts: mapArticleSummaries(drafts, 3), PendingComments: make([]commentSummary, 0, 3), RecentImages: make([]imageSummary, 0, 6),
	}
	for _, c := range comments[:min(len(comments), 3)] {
		body := []rune(c.Body)
		if len(body) > 120 {
			body = body[:120]
		}
		response.PendingComments = append(response.PendingComments, commentSummary{c.CommentID, c.ArticleID, c.ArticleTitle, c.AuthorName, string(body), c.CreatedAt})
	}
	for _, i := range images {
		if u := publicURL(i.PublicUrl); u != "" {
			response.RecentImages = append(response.RecentImages, imageSummary{i.AssetID, i.Filename, u, i.ByteSize, i.CreatedAt})
			if len(response.RecentImages) == 6 {
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, response)
}
func mapArticleSummaries(rows []dbgen.ListDashboardRecentArticlesRow, limit int) []articleSummary {
	result := make([]articleSummary, 0, min(len(rows), limit))
	for _, r := range rows[:min(len(rows), limit)] {
		result = append(result, articleSummary{r.ArticleID, r.Slug, r.Title, r.Status, publicURL(r.CoverImageUrl), nonNilTags(r.Tags), r.UpdatedAt, r.PublishedAt})
	}
	return result
}

type articleRanking struct {
	ArticleID     pgtype.UUID        `json:"articleId"`
	Slug          string             `json:"slug"`
	Title         string             `json:"title"`
	CoverImageURL string             `json:"coverImageUrl"`
	Tags          []string           `json:"tags"`
	PeriodViews   int64              `json:"periodViews"`
	TotalViews    int64              `json:"totalViews"`
	PublishedAt   pgtype.Timestamptz `json:"publishedAt"`
}

func (h Handler) articles(w http.ResponseWriter, r *http.Request) {
	days := normalizeDays(r.URL.Query().Get("days"))
	rows, err := h.queries.ListDashboardArticles(r.Context(), days)
	if err != nil {
		readError(w)
		return
	}
	result := make([]articleRanking, 0, min(len(rows), 5))
	for _, a := range rows[:min(len(rows), 5)] {
		result = append(result, articleRanking{a.ArticleID, a.Slug, a.Title, publicURL(a.CoverImageUrl), nonNilTags(a.Tags), a.PeriodViews, a.TotalViews, a.PublishedAt})
	}
	writeJSON(w, http.StatusOK, struct {
		Days     int32            `json:"days"`
		Articles []articleRanking `json:"articles"`
	}{days, result})
}

type albumSummary struct {
	AlbumID     pgtype.UUID            `json:"albumId"`
	Slug        string                 `json:"slug"`
	Title       string                 `json:"title"`
	Status      dbgen.PhotoAlbumStatus `json:"status"`
	CoverURL    string                 `json:"coverUrl"`
	PhotoCount  int64                  `json:"photoCount"`
	PeriodViews int64                  `json:"periodViews"`
	UpdatedAt   pgtype.Timestamptz     `json:"updatedAt"`
}

func (h Handler) albums(w http.ResponseWriter, r *http.Request) {
	days := normalizeDays(r.URL.Query().Get("days"))
	rows, err := h.queries.ListDashboardAlbums(r.Context(), days)
	if err != nil {
		readError(w)
		return
	}
	result := make([]albumSummary, 0, min(len(rows), 4))
	for _, a := range rows[:min(len(rows), 4)] {
		result = append(result, albumSummary{a.AlbumID, a.Slug, a.Title, a.Status, publicURL(a.CoverUrl), a.PhotoCount, a.PeriodViews, a.UpdatedAt})
	}
	writeJSON(w, http.StatusOK, struct {
		Days   int32          `json:"days"`
		Albums []albumSummary `json:"albums"`
	}{days, result})
}
func normalizeDays(raw string) int32 {
	switch raw {
	case "7":
		return 7
	case "90":
		return 90
	default:
		return 30
	}
}
func nonNilTags(tags []string) []string {
	if tags == nil {
		return []string{}
	}
	return tags
}
func publicURL(raw string) string {
	if strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\r\n\t ") {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return raw
}
func readError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "驾驶舱数据读取失败"})
}
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
