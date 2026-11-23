package articlemcp

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"rendering-cms-platform/backend/internal/config"
)

type Server struct {
	cfg    config.ArticleMCPConfig
	store  store
	client *http.Client
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
		out, err := call(ctx, principal{id: id, publishScope: slices.Contains(info.Scopes, publishScope)}, input)
		if err != nil {
			for _, allowed := range []error{errDenied, errMissing, errVersion, errKey, errInput, errName} {
				if errors.Is(err, allowed) {
					return nil, empty, allowed
				}
			}
			return nil, empty, errors.New("文章操作失败，请按原请求查询或重试")
		}
		return nil, out, nil
	})
}
func (s *Server) tools(publish bool) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "rendering-cms", Version: "1.0.0"}, nil)
	registerTool(server, "create_draft", "创建草稿，重试使用相同 requestKey。", s.store.create)
	registerTool(server, "get_article", "读取此连接创建的文章及当前版本。", func(ctx context.Context, p principal, in ArticleInput) (Article, error) {
		return s.store.read(ctx, p, in.ArticleID)
	})
	registerTool(server, "update_draft", "按版本替换草稿标题、摘要、正文和标签，未提供的摘要和标签会清空。", func(ctx context.Context, p principal, in EditInput) (Article, error) {
		return s.store.change(ctx, p, in.VersionInput, &in.Content)
	})
	if publish {
		registerTool(server, "publish_article", "公开发布指定版本的草稿。", func(ctx context.Context, p principal, in VersionInput) (Article, error) {
			return s.store.change(ctx, p, in, nil)
		})
	}
	registerTool(server, "article_status", "检查 CMS 状态与博客公开页面，并说明搜索的验证状态。", s.status)
	return server
}
func (s *Server) Register(r chi.Router) {
	draft, publish := s.tools(false), s.tools(true)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		info := mcpauth.TokenInfoFromContext(r.Context())
		if info != nil && slices.Contains(info.Scopes, publishScope) {
			return publish
		}
		return draft
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 2 << 20})
	opts := &mcpauth.RequireBearerTokenOptions{Scopes: []string{draftScope}, AllowMissingExpiration: true}
	if s.cfg.Issuer != "" {
		opts.ResourceMetadataURL = s.cfg.Origin + metadata
		r.Get(metadata, func(w http.ResponseWriter, _ *http.Request) {
			respond(w, 200, map[string]any{
				"resource":                 s.cfg.Origin + endpoint,
				"authorization_servers":    []string{s.cfg.Issuer},
				"scopes_supported":         []string{draftScope, publishScope},
				"bearer_methods_supported": []string{"header"},
			})
		})
	}
	protected := mcpauth.RequireBearerToken(s.verify, opts)(handler)
	r.HandleFunc(endpoint, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if origin := r.Header.Get("Origin"); origin != "" && origin != s.cfg.Origin && origin != s.cfg.AdminOrigin {
			http.Error(w, "来源无效", 403)
			return
		}
		protected.ServeHTTP(w, r)
	})
}
