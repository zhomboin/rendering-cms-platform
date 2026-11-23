package articles

import (
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
)

const versionConflict = "文章状态或版本已变更，请重新读取后确认"

func expectedArticleVersion(w http.ResponseWriter, r *http.Request) (pgtype.Int4, bool) {
	q := r.URL.Query()
	if !q.Has("expectedVersion") {
		return pgtype.Int4{}, true
	}
	version, err := strconv.ParseInt(q.Get("expectedVersion"), 10, 32)
	if err != nil || version < 1 || len(q["expectedVersion"]) != 1 {
		writeError(w, http.StatusBadRequest, "文章版本号无效")
		return pgtype.Int4{}, false
	}
	return pgtype.Int4{Int32: int32(version), Valid: true}, true
}
