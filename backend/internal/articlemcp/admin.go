package articlemcp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
)

func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	dec.DisallowUnknownFields()
	if dec.Decode(v) != nil || dec.Decode(new(any)) != io.EOF {
		http.Error(w, "请求格式无效", 400)
		return false
	}
	return true
}
func (s *Server) RegisterAdmin(r chi.Router) {
	r.Route("/mcp", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				u, ok := httpapi.UserFromContext(r.Context())
				if !ok || u.Role != "admin" {
					http.Error(w, "需要管理员权限", 403)
					return
				}
				w.Header().Set("Cache-Control", "no-store")
				next.ServeHTTP(w, r)
			})
		})
		r.Get("/connections", s.list)
		r.Post("/connections", s.issue)
		r.Patch("/connections/{id}", s.modify)
		r.Delete("/connections/{id}", s.modify)
		r.Get("/articles/{id}/search-check", s.searchCheck)
		r.Post("/articles/{id}/search-check", s.searchCheck)
	})
}

func owner(r *http.Request) pgtype.UUID {
	u, _ := httpapi.UserFromContext(r.Context())
	id, _ := parseID(u.UserID)
	return id
}

type grantInfo struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	AllowPublish bool   `json:"allowPublish"`
	Revoked      bool   `json:"revoked"`
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	rows, err := dbgen.New(s.store.pool).ListMCPConnections(r.Context(), owner(r))
	if err != nil {
		http.Error(w, "读取连接失败", 500)
		return
	}
	result := make([]grantInfo, 0, len(rows))
	for _, row := range rows {
		result = append(result, grantInfo{row.ID.String(), row.Label, row.AllowPublish, row.Revoked})
	}
	respond(w, 200, result)
}
func (s *Server) issue(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Label        string `json:"label"`
		AllowPublish bool   `json:"allowPublish"`
		OAuthClient  string `json:"oauthClient"`
		OAuthSubject string `json:"oauthSubject"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Label = strings.TrimSpace(in.Label)
	if in.Label == "" || len(in.Label) > 160 {
		http.Error(w, "连接名称无效", 400)
		return
	}
	args := dbgen.CreateMCPConnectionParams{OwnerID: owner(r), Label: in.Label, AllowPublish: in.AllowPublish}
	var token string
	if in.OAuthClient == "" && in.OAuthSubject == "" {
		var key [32]byte
		rand.Read(key[:])
		token = tokenPrefix + base64.RawURLEncoding.EncodeToString(key[:])
		hash := sha256.Sum256([]byte(token))
		args.TokenHash = hash[:]
	} else {
		if s.cfg.Issuer == "" || strings.TrimSpace(in.OAuthClient) == "" || strings.TrimSpace(in.OAuthSubject) == "" {
			http.Error(w, "OAuth 连接配置不完整", 400)
			return
		}
		args.OauthIssuer, args.OauthClient, args.OauthSubject = textValue(s.cfg.Issuer), textValue(in.OAuthClient), textValue(in.OAuthSubject)
	}
	id, err := dbgen.New(s.store.pool).CreateMCPConnection(r.Context(), args)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			http.Error(w, "连接身份已登记", 409)
			return
		}
		http.Error(w, "创建连接失败", 500)
		return
	}
	result := map[string]any{"id": id.String(), "allowPublish": in.AllowPublish, "endpoint": s.cfg.Origin + endpoint}
	if token != "" {
		result["token"] = token
	}
	respond(w, 201, result)
}
func (s *Server) modify(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	q := dbgen.New(s.store.pool)
	var count int64
	if r.Method == http.MethodDelete {
		count, err = q.RevokeMCPConnection(r.Context(), dbgen.RevokeMCPConnectionParams{OwnerID: owner(r), ID: id})
	} else {
		var in struct {
			AllowPublish *bool `json:"allowPublish"`
		}
		if !decode(w, r, &in) {
			return
		}
		if in.AllowPublish == nil {
			http.Error(w, "请指定发布权限", 400)
			return
		}
		count, err = q.ChangeMCPPublish(r.Context(), dbgen.ChangeMCPPublishParams{OwnerID: owner(r), ID: id, AllowPublish: *in.AllowPublish})
	}
	if err != nil {
		http.Error(w, "保存连接失败", 500)
		return
	}
	if count == 0 {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
