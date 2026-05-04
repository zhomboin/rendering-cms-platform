package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type RouterConfig struct {
	LoginHandler   http.HandlerFunc
	JWTSecret      string
	FrontendOrigin string
	Logger         *slog.Logger
	PublicRoutes   []RouteRegistrar
	AdminRoutes    []RouteRegistrar
}

type RouterOption func(*RouterConfig)
type RouteRegistrar func(chi.Router)

func WithLoginHandler(handler http.HandlerFunc) RouterOption {
	return func(config *RouterConfig) {
		config.LoginHandler = handler
	}
}

func WithJWTSecret(secret string) RouterOption {
	return func(config *RouterConfig) {
		config.JWTSecret = secret
	}
}

func WithFrontendOrigin(origin string) RouterOption {
	return func(config *RouterConfig) {
		config.FrontendOrigin = origin
	}
}

func WithLogger(logger *slog.Logger) RouterOption {
	return func(config *RouterConfig) {
		config.Logger = logger
	}
}

func WithPublicRoutes(registrar RouteRegistrar) RouterOption {
	return func(config *RouterConfig) {
		config.PublicRoutes = append(config.PublicRoutes, registrar)
	}
}

func WithAdminRoutes(registrar RouteRegistrar) RouterOption {
	return func(config *RouterConfig) {
		config.AdminRoutes = append(config.AdminRoutes, registrar)
	}
}

func NewRouter(options ...RouterOption) http.Handler {
	var config RouterConfig
	for _, option := range options {
		option(&config)
	}

	router := chi.NewRouter()
	if config.Logger != nil {
		router.Use(RequestLogMiddleware(config.Logger))
	}
	if config.FrontendOrigin != "" {
		router.Use(CORSMiddleware(config.FrontendOrigin))
	}

	router.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})
	if config.LoginHandler != nil {
		router.Post("/api/v1/auth/login", config.LoginHandler)
	}
	for _, registrar := range config.PublicRoutes {
		registrar(router)
	}
	if config.JWTSecret != "" && len(config.AdminRoutes) > 0 {
		router.Route("/api/v1/admin", func(admin chi.Router) {
			admin.Use(AdminAuthMiddleware(config.JWTSecret))
			for _, registrar := range config.AdminRoutes {
				registrar(admin)
			}
		})
	}

	return router
}
