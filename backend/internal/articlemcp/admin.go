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
	"time"

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
		r.Get("/settings", s.settings)
		r.Patch("/settings", s.saveSettings)
		r.Get("/operations", s.operations)
		r.Get("/connections", s.list)
		r.Post("/connections", s.withSettings((*Server).issue))
		r.Patch("/connections/{id}", s.modify)
		r.Delete("/connections/{id}", s.modify)
		r.Get("/articles/{id}/search-check", s.withSettings((*Server).searchCheck))
		r.Post("/articles/{id}/search-check", s.withSettings((*Server).searchCheck))
	})
}

func owner(r *http.Request) pgtype.UUID {
	u, _ := httpapi.UserFromContext(r.Context())
	id, _ := parseID(u.UserID)
	return id
}

type grantInfo struct {
	ID                 string     `json:"id"`
	Label              string     `json:"label"`
	AllowPublish       bool       `json:"allowPublish"`
	Permissions        []string   `json:"permissions"`
	PermissionRevision int32      `json:"permissionRevision"`
	Revoked            bool       `json:"revoked"`
	OAuth              bool       `json:"oauth"`
	ClientName         string     `json:"clientName"`
	LastSeenAt         *time.Time `json:"lastSeenAt"`
}

func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	rows, err := dbgen.New(s.store.pool).ListMCPConnections(r.Context(), owner(r))
	if err != nil {
		http.Error(w, "读取连接失败", 500)
		return
	}
	result := make([]grantInfo, 0, len(rows))
	for _, row := range rows {
		g := grantInfo{ID: row.ID.String(), Label: row.Label, AllowPublish: row.AllowPublish, Revoked: row.Revoked, OAuth: row.Oauth, ClientName: row.ClientName}
		g.Permissions = allPermissions(row.DraftPermissions, row.AllowPublish)
		g.PermissionRevision = row.PermissionRevision
		if row.LastSeenAt.Valid {
			g.LastSeenAt = &row.LastSeenAt.Time
		}
		result = append(result, g)
	}
	respond(w, 200, result)
}
func (s *Server) issue(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Label        string         `json:"label"`
		AllowPublish *bool          `json:"allowPublish"`
		Permissions  permissionList `json:"permissions"`
		OAuthClient  string         `json:"oauthClient"`
		OAuthSubject string         `json:"oauthSubject"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Label = strings.TrimSpace(in.Label)
	if in.Label == "" || len(in.Label) > 160 {
		respond(w, 400, map[string]string{"error": "请填写 160 字节以内的连接名称"})
		return
	}
	args := dbgen.CreateMCPConnectionParams{OwnerID: owner(r), Label: in.Label, DraftPermissions: draftTools}
	if in.Permissions != nil && in.AllowPublish != nil {
		respond(w, 400, map[string]string{"error": "请使用单一授权方式"})
		return
	}
	if in.Permissions != nil {
		args.DraftPermissions, args.AllowPublish = in.Permissions.split()
	} else if in.AllowPublish != nil {
		args.AllowPublish = *in.AllowPublish
	}
	var token string
	if in.OAuthClient == "" && in.OAuthSubject == "" {
		var key [32]byte
		rand.Read(key[:])
		token = tokenPrefix + base64.RawURLEncoding.EncodeToString(key[:])
		hash := sha256.Sum256([]byte(token))
		args.TokenHash = hash[:]
	} else {
		if s.cfg.Issuer == "" || strings.TrimSpace(in.OAuthClient) == "" || strings.TrimSpace(in.OAuthSubject) == "" {
			respond(w, 400, map[string]string{"error": "OAuth 连接配置不完整"})
			return
		}
		args.OauthIssuer, args.OauthClient, args.OauthSubject = textValue(s.cfg.Issuer), textValue(in.OAuthClient), textValue(in.OAuthSubject)
	}
	id, err := dbgen.New(s.store.pool).CreateMCPConnection(r.Context(), args)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			respond(w, 409, map[string]string{"error": "此 OAuth 身份已有有效连接"})
			return
		}
		http.Error(w, "创建连接失败", 500)
		return
	}
	result := map[string]any{"id": id.String(), "allowPublish": args.AllowPublish, "permissions": allPermissions(args.DraftPermissions, args.AllowPublish), "permissionRevision": 1, "endpoint": s.cfg.Origin + endpoint}
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
			AllowPublish     *bool          `json:"allowPublish"`
			Permissions      permissionList `json:"permissions"`
			ExpectedRevision *int32         `json:"expectedRevision"`
		}
		if !decode(w, r, &in) {
			return
		}
		if (in.AllowPublish == nil) == (in.Permissions == nil) {
			respond(w, 400, map[string]string{"error": "请指定权限列表或发布开关"})
			return
		}
		if in.ExpectedRevision != nil && (in.Permissions == nil || *in.ExpectedRevision < 1) {
			respond(w, 400, map[string]string{"error": "无效的权限版本"})
			return
		}
		if in.Permissions != nil {
			drafts, publish := in.Permissions.split()
			args := dbgen.ChangeMCPPermissionsParams{OwnerID: owner(r), ID: id, DraftPermissions: drafts, AllowPublish: publish}
			if in.ExpectedRevision != nil {
				args.ExpectedRevision = *in.ExpectedRevision
			}
			count, err = q.ChangeMCPPermissions(r.Context(), args)
			if count == 0 && err == nil && in.ExpectedRevision != nil {
				respond(w, 409, map[string]string{"error": "连接或权限已变更，请刷新后重新编辑"})
				return
			}
		} else {
			count, err = q.ChangeMCPPublish(r.Context(), dbgen.ChangeMCPPublishParams{OwnerID: owner(r), ID: id, AllowPublish: *in.AllowPublish})
		}
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
