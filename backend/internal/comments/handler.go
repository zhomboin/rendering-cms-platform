package comments

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

type Handler struct {
	queries *dbgen.Queries
}

type CreateCommentPayload struct {
	AuthorName  string `json:"authorName"`
	AuthorEmail string `json:"authorEmail"`
	Body        string `json:"body"`
}

type ReviewCommentPayload struct {
	Status string `json:"status"`
}

func NewHandler(queries *dbgen.Queries) Handler {
	return Handler{queries: queries}
}

func (h Handler) RegisterPublicRoutes(router chi.Router) {
	router.Get("/api/v1/articles/{slug}/comments", h.listApprovedComments)
	router.Post("/api/v1/articles/{slug}/comments", h.createComment)
}

func (h Handler) RegisterAdminRoutes(router chi.Router) {
	router.Get("/comments", h.listAdminComments)
	router.Patch("/comments/{id}", h.reviewComment)
}

func (h Handler) listApprovedComments(w http.ResponseWriter, r *http.Request) {
	comments, err := h.queries.ListApprovedCommentsByArticleSlug(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "评论列表读取失败")
		return
	}
	response := make([]map[string]interface{}, 0, len(comments))
	for _, comment := range comments {
		response = append(response, mapPublicComment(comment))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h Handler) createComment(w http.ResponseWriter, r *http.Request) {
	var payload CreateCommentPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式不正确")
		return
	}
	comment := NewComment(strings.TrimSpace(payload.AuthorName), strings.TrimSpace(payload.Body))
	if comment.AuthorName == "" || comment.Body == "" {
		writeError(w, http.StatusBadRequest, "昵称和评论内容不能为空")
		return
	}

	created, err := h.queries.CreateComment(r.Context(), dbgen.CreateCommentParams{
		Slug:        chi.URLParam(r, "slug"),
		AuthorName: comment.AuthorName,
		AuthorEmail: nullableText(strings.TrimSpace(payload.AuthorEmail)),
		Body:        comment.Body,
		IpHash:      ipHashFromRequest(r),
		UserAgent:   nullableText(r.UserAgent()),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "文章不存在或未发布")
			return
		}
		writeError(w, http.StatusInternalServerError, "评论提交失败")
		return
	}
	writeJSON(w, http.StatusCreated, mapComment(created))
}

func (h Handler) listAdminComments(w http.ResponseWriter, r *http.Request) {
	comments, err := h.queries.ListAdminComments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "后台评论列表读取失败")
		return
	}
	response := make([]map[string]interface{}, 0, len(comments))
	for _, comment := range comments {
		response = append(response, mapAdminComment(comment))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h Handler) reviewComment(w http.ResponseWriter, r *http.Request) {
	var payload ReviewCommentPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式不正确")
		return
	}
	status := strings.TrimSpace(payload.Status)
	if status != StatusApproved && status != StatusRejected {
		writeError(w, http.StatusBadRequest, "审核状态只能为 approved 或 rejected")
		return
	}
	commentID, err := uuidFromString(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "评论 ID 无效")
		return
	}

	comment, err := h.queries.ReviewComment(r.Context(), dbgen.ReviewCommentParams{
		CommentID: commentID,
		Status:    dbgen.CommentStatus(status),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "评论不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "评论审核失败")
		return
	}
	writeJSON(w, http.StatusOK, mapComment(comment))
}

func uuidFromString(value string) (pgtype.UUID, error) {
	var uuid pgtype.UUID
	if err := uuid.Scan(value); err != nil {
		return pgtype.UUID{}, err
	}
	return uuid, nil
}

func nullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func ipHashFromRequest(r *http.Request) string {
	host := r.RemoteAddr
	if parsedHost, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = parsedHost
	}
	sum := sha256.Sum256([]byte(host))
	return hex.EncodeToString(sum[:])
}

func mapPublicComment(comment dbgen.ListApprovedCommentsByArticleSlugRow) map[string]interface{} {
	return map[string]interface{}{
		"commentId":  comment.CommentID.String(),
		"authorName": comment.AuthorName,
		"body":       comment.Body,
		"createdAt":  timestamptzValue(comment.CreatedAt),
	}
}

func mapAdminComment(comment dbgen.ListAdminCommentsRow) map[string]interface{} {
	return map[string]interface{}{
		"commentId":    comment.CommentID.String(),
		"articleId":    comment.ArticleID.String(),
		"articleSlug":  comment.ArticleSlug,
		"articleTitle": comment.ArticleTitle,
		"authorName":   comment.AuthorName,
		"authorEmail":  textValue(comment.AuthorEmail),
		"body":         comment.Body,
		"status":       string(comment.Status),
		"userAgent":    textValue(comment.UserAgent),
		"createdAt":    timestamptzValue(comment.CreatedAt),
		"reviewedAt":   timestamptzValue(comment.ReviewedAt),
	}
}

func mapComment(comment dbgen.Comment) map[string]interface{} {
	return map[string]interface{}{
		"commentId":   comment.CommentID.String(),
		"articleId":   comment.ArticleID.String(),
		"authorName":  comment.AuthorName,
		"authorEmail": textValue(comment.AuthorEmail),
		"body":        comment.Body,
		"status":      string(comment.Status),
		"userAgent":   textValue(comment.UserAgent),
		"createdAt":   timestamptzValue(comment.CreatedAt),
		"reviewedAt":  timestamptzValue(comment.ReviewedAt),
	}
}

func textValue(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func timestamptzValue(value pgtype.Timestamptz) interface{} {
	if !value.Valid {
		return nil
	}
	return value.Time
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
