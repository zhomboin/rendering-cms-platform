package articles

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
)

func (h Handler) deleteArticle(w http.ResponseWriter, r *http.Request) {
	user, ok := httpapi.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "未登录")
		return
	}
	if user.Role != "admin" {
		writeError(w, http.StatusForbidden, "仅管理员可以删除文章")
		return
	}
	id, err := uuidFromString(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "文章 ID 无效")
		return
	}
	version, ok := expectedArticleVersion(w, r)
	if !ok {
		return
	}
	if !version.Valid {
		writeError(w, http.StatusBadRequest, "请提供待删除文章的版本号")
		return
	}
	count, err := h.queries.DeleteArticle(r.Context(), dbgen.DeleteArticleParams{
		ArticleID: id, ExpectedVersion: version.Int32,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "文章删除失败")
		return
	}
	if count > 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_, err = h.queries.GetArticleByID(r.Context(), id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "文章不存在")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "文章读取失败")
	default:
		writeError(w, http.StatusConflict, versionConflict)
	}
}
