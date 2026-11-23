package articlemcp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

func (s *Server) searchCheck(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	q := dbgen.New(s.store.pool)
	if r.Method == http.MethodGet {
		a, err := q.MCPVerificationTarget(r.Context(), dbgen.MCPVerificationTargetParams{OwnerID: owner(r), ArticleID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, "查询核验目标失败", 500)
			return
		}
		respond(w, 200, map[string]any{"publicUrl": s.cfg.BlogOrigin + "/blog/" + a.Slug, "title": a.Title, "version": a.Version})
		return
	}
	var in struct {
		Version int32  `json:"version"`
		Result  string `json:"result"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Version < 1 || (in.Result != "found" && in.Result != "not_found") {
		http.Error(w, "核验结果无效", 400)
		return
	}
	count, err := q.RecordMCPSearch(r.Context(), dbgen.RecordMCPSearchParams{OwnerID: owner(r), ArticleID: id, Version: in.Version, SearchResult: textValue(in.Result)})
	if err != nil {
		http.Error(w, "记录核验结果失败", 500)
		return
	}
	if count == 0 {
		http.Error(w, "核验版本或状态已变更", 409)
		return
	}
	w.WriteHeader(204)
}
