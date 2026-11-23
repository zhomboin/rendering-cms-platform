package articlemcp

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"rendering-cms-platform/backend/internal/config"
	"rendering-cms-platform/backend/internal/database/dbgen"
)

type serviceSettings struct {
	Enabled     bool   `json:"enabled"`
	Origin      string `json:"origin"`
	AdminOrigin string `json:"adminOrigin"`
	BlogOrigin  string `json:"blogOrigin"`
	Revision    int32  `json:"revision"`
}

func (s *Server) current(ctx context.Context) (serviceSettings, error) {
	row, err := dbgen.New(s.store.pool).GetMCPSettings(ctx)
	if err != nil {
		return serviceSettings{}, err
	}
	if row.Revision == 0 {
		return serviceSettings{Enabled: s.cfg.Origin != "", Origin: s.cfg.Origin, AdminOrigin: s.cfg.AdminOrigin, BlogOrigin: s.cfg.BlogOrigin}, nil
	}
	v := serviceSettings{Enabled: row.Enabled, Origin: row.Origin, AdminOrigin: row.AdminOrigin, BlogOrigin: row.BlogOrigin, Revision: row.Revision}
	if v.Enabled {
		cfg := s.cfg
		v.Enabled = cfg.SetOrigins(v.Origin, v.AdminOrigin, v.BlogOrigin) == nil
	}
	return v, nil
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	v, err := s.current(r.Context())
	if err != nil {
		http.Error(w, "无法读取 MCP 设置", 503)
		return
	}
	address := ""
	if v.Origin != "" {
		address = v.Origin + endpoint
	}
	respond(w, 200, struct {
		serviceSettings
		Endpoint string `json:"endpoint"`
		Issuer   string `json:"issuer"`
	}{v, address, s.cfg.Issuer})
}

func (s *Server) saveSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Enabled     *bool  `json:"enabled"`
		Revision    *int32 `json:"revision"`
		Origin      string `json:"origin"`
		AdminOrigin string `json:"adminOrigin"`
		BlogOrigin  string `json:"blogOrigin"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Enabled == nil || in.Revision == nil {
		respond(w, 400, map[string]string{"error": "请提交开关状态和设置版本"})
		return
	}
	cfg := config.ArticleMCPConfig{Development: s.cfg.Development}
	if *in.Enabled || in.Origin != "" || in.AdminOrigin != "" || in.BlogOrigin != "" {
		if err := cfg.SetOrigins(strings.TrimSpace(in.Origin), strings.TrimSpace(in.AdminOrigin), strings.TrimSpace(in.BlogOrigin)); err != nil {
			respond(w, 400, map[string]string{"error": err.Error()})
			return
		}
	}
	count, err := dbgen.New(s.store.pool).SaveMCPSettings(r.Context(), dbgen.SaveMCPSettingsParams{
		Enabled: *in.Enabled, Revision: *in.Revision, Origin: cfg.Origin, AdminOrigin: cfg.AdminOrigin, BlogOrigin: cfg.BlogOrigin,
	})
	if err != nil {
		http.Error(w, "设置保存失败", 500)
		return
	}
	if count == 0 {
		respond(w, 409, map[string]string{"error": "设置已变更，请刷新后重新编辑"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) configured(v serviceSettings) *Server {
	cfg := s.cfg
	cfg.Origin, cfg.AdminOrigin, cfg.BlogOrigin = v.Origin, v.AdminOrigin, v.BlogOrigin
	instance := New(s.store.pool, cfg)
	if v.BlogOrigin != s.cfg.BlogOrigin {
		instance.client.Transport = publicBlogTransport
	}
	return instance
}

func (s *Server) withSettings(next func(*Server, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := s.current(r.Context())
		if err != nil {
			http.Error(w, "无法读取 MCP 设置", 503)
			return
		}
		if v.Origin == "" {
			respond(w, 409, map[string]string{"error": "请先配置服务地址"})
			return
		}
		next(s.configured(v), w, r)
	}
}

func (s *Server) Register(r chi.Router) {
	var mu sync.Mutex
	var active http.Handler
	var revision int32
	handler := func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		v, err := s.current(req.Context())
		if err != nil {
			http.Error(w, "无法读取 MCP 设置", 503)
			return
		}
		if !v.Enabled {
			http.NotFound(w, req)
			return
		}
		mu.Lock()
		if active == nil || revision != v.Revision {
			router := chi.NewRouter()
			s.configured(v).registerEnabled(router)
			active, revision = router, v.Revision
		}
		current := active
		mu.Unlock()
		current.ServeHTTP(w, req)
	}
	r.HandleFunc(endpoint, handler)
	if s.cfg.Issuer != "" {
		r.Get(metadata, handler)
	}
}
