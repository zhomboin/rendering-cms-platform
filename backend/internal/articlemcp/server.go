package articlemcp

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"rendering-cms-platform/backend/internal/albums"
	"rendering-cms-platform/backend/internal/config"
)

type Server struct {
	cfg    config.ArticleMCPConfig
	store  store
	client *http.Client
	albums albums.Handler
}

func New(pool *pgxpool.Pool, c config.ArticleMCPConfig) *Server {
	return &Server{
		cfg:   c,
		store: store{pool: pool, adminOrigin: c.AdminOrigin, blogOrigin: c.BlogOrigin},
		client: &http.Client{
			Timeout:       5 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (s *Server) WithAlbums(h albums.Handler) *Server {
	s.albums = h
	return s
}

func registerTool[I, O any](server *mcp.Server, name, description string, call func(context.Context, principal, I) (O, error)) {
	mcp.AddTool(server, &mcp.Tool{Name: name, Description: description}, func(ctx context.Context, req *mcp.CallToolRequest, input I) (*mcp.CallToolResult, O, error) {
		var empty O
		if req.Extra == nil || req.Extra.TokenInfo == nil {
			return nil, empty, errDenied
		}
		info := req.Extra.TokenInfo
		id, err := parseID(info.UserID)
		if err != nil {
			return nil, empty, errDenied
		}
		out, err := call(ctx, principal{id: id, publishScope: slices.Contains(info.Scopes, publishScope), albumScope: slices.Contains(info.Scopes, albumScope)}, input)
		if err != nil {
			for _, allowed := range []error{errDenied, errMissing, errVersion, errKey, errInput, errName, errAlbumMissing, errAlbumInput, errAlbumState, albums.ErrAlbumEmpty, albums.ErrPhotoOrder} {
				if errors.Is(err, allowed) {
					return nil, empty, allowed
				}
			}
			var invalid albums.PhotoInputError
			if errors.As(err, &invalid) {
				return nil, empty, invalid
			}
			return nil, empty, errors.New("操作失败，请查询结果后重试")
		}
		return nil, out, nil
	})
}
func (s *Server) tools(permissions []string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "rendering-cms", Version: "1.0.0"}, &mcp.ServerOptions{Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}, Logging: &mcp.LoggingCapabilities{}}})
	server.AddReceivingMiddleware(s.observe)
	if slices.Contains(permissions, createTool) {
		registerTool(server, createTool, "创建草稿，重试使用相同 requestKey。", s.store.create)
	}
	if slices.Contains(permissions, readTool) {
		registerTool(server, readTool, "读取此连接创建的文章及当前版本。", func(ctx context.Context, p principal, in ArticleInput) (Article, error) {
			return s.store.read(ctx, p, in.ArticleID, readTool)
		})
	}
	if slices.Contains(permissions, editTool) {
		registerTool(server, editTool, "按版本替换草稿标题、摘要、正文和标签，未提供的摘要和标签会清空。", func(ctx context.Context, p principal, in EditInput) (Article, error) {
			return s.store.change(ctx, p, in.VersionInput, &in.Content)
		})
	}
	if slices.Contains(permissions, publishTool) {
		registerTool(server, publishTool, "公开发布指定版本的草稿。", func(ctx context.Context, p principal, in VersionInput) (Article, error) {
			return s.store.change(ctx, p, in, nil)
		})
	}
	if slices.Contains(permissions, statusTool) {
		registerTool(server, statusTool, "检查 CMS 状态与博客公开页面，并说明搜索的验证状态。", s.status)
	}
	s.registerAlbumTools(server, permissions)
	return server
}
func (s *Server) registerEnabled(r chi.Router) {
	var mu sync.Mutex
	var keys []string
	servers := map[string]*mcp.Server{}
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		info := mcpauth.TokenInfoFromContext(r.Context())
		if info == nil {
			return nil
		}
		permissions, _ := info.Extra["permissions"].([]string)
		key := strings.Join(permissions, ",")
		mu.Lock()
		defer mu.Unlock()
		if servers[key] == nil {
			if len(keys) == 32 {
				delete(servers, keys[0])
				keys = keys[1:]
			}
			keys = append(keys, key)
			servers[key] = s.tools(permissions)
		}
		return servers[key]
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 2 << 20})
	opts := &mcpauth.RequireBearerTokenOptions{Scopes: []string{draftScope}, AllowMissingExpiration: true}
	if s.cfg.Issuer != "" {
		opts.ResourceMetadataURL = s.cfg.Origin + metadata
		r.Get(metadata, func(w http.ResponseWriter, _ *http.Request) {
			respond(w, 200, map[string]any{
				"resource":                 s.cfg.Origin + endpoint,
				"authorization_servers":    []string{s.cfg.Issuer},
				"scopes_supported":         []string{draftScope, publishScope, albumScope},
				"bearer_methods_supported": []string{"header"},
			})
		})
	}
	protect := mcpauth.RequireBearerToken(s.verify, opts)
	protected := protect(handler)
	upload := protect(http.HandlerFunc(s.multipartPhoto))
	r.Post(uploadRoute, func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && origin != s.cfg.Origin && origin != s.cfg.AdminOrigin {
			http.Error(w, "来源无效", 403)
			return
		}
		upload.ServeHTTP(w, r)
	})
	r.HandleFunc(endpoint, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if origin := r.Header.Get("Origin"); origin != "" && origin != s.cfg.Origin && origin != s.cfg.AdminOrigin {
			http.Error(w, "来源无效", 403)
			return
		}
		protected.ServeHTTP(w, r)
	})
}
