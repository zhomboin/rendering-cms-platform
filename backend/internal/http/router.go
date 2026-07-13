package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type RouterConfig struct {
	LoginHandler              http.HandlerFunc
	RefreshHandler            http.HandlerFunc
	JWTSecret                 string
	FrontendOrigin            string
	FrontendOrigins           []string
	Logger                    *slog.Logger
	RequestBodyLimit          int64
	PublicRoutes              []RouteRegistrar
	PublicArticleReadRoutes   []RouteRegistrar
	PublicArticleSearchRoutes []RouteRegistrar
	PublicTrafficLimits       PublicTrafficLimits
	AdminRoutes               []RouteRegistrar
}

type PublicTrafficLimits struct {
	ReadRatePerSecond   float64
	ReadBurst           int
	SearchRatePerSecond float64
	SearchBurst         int
	MaxInFlight         int
	MaxClients          int
	ClientTTL           time.Duration
}

type RouterOption func(*RouterConfig)
type RouteRegistrar func(chi.Router)

func WithLoginHandler(handler http.HandlerFunc) RouterOption {
	return func(config *RouterConfig) {
		config.LoginHandler = handler
	}
}

func WithRefreshHandler(handler http.HandlerFunc) RouterOption {
	return func(config *RouterConfig) {
		config.RefreshHandler = handler
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
		config.FrontendOrigins = []string{origin}
	}
}

func WithFrontendOrigins(origins []string) RouterOption {
	return func(config *RouterConfig) {
		config.FrontendOrigins = append([]string(nil), origins...)
	}
}

func WithLogger(logger *slog.Logger) RouterOption {
	return func(config *RouterConfig) {
		config.Logger = logger
	}
}

func WithRequestBodyLimit(limit int64) RouterOption {
	return func(config *RouterConfig) {
		config.RequestBodyLimit = limit
	}
}

func WithPublicRoutes(registrar RouteRegistrar) RouterOption {
	return func(config *RouterConfig) {
		config.PublicRoutes = append(config.PublicRoutes, registrar)
	}
}

func WithPublicArticleReadRoutes(registrar RouteRegistrar) RouterOption {
	return func(config *RouterConfig) {
		config.PublicArticleReadRoutes = append(config.PublicArticleReadRoutes, registrar)
	}
}

func WithPublicArticleSearchRoutes(registrar RouteRegistrar) RouterOption {
	return func(config *RouterConfig) {
		config.PublicArticleSearchRoutes = append(config.PublicArticleSearchRoutes, registrar)
	}
}

func WithPublicTrafficLimits(limits PublicTrafficLimits) RouterOption {
	return func(config *RouterConfig) {
		config.PublicTrafficLimits = limits
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
	if config.RequestBodyLimit == 0 {
		config.RequestBodyLimit = 25 << 20
	}

	router := chi.NewRouter()
	router.Use(RequestSizeLimitMiddleware(config.RequestBodyLimit))
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
	if config.RefreshHandler != nil {
		router.Post("/api/v1/auth/refresh", config.RefreshHandler)
	}
	for _, registrar := range config.PublicRoutes {
		registrar(router)
	}
	registerLimitedPublicRoutes(router, config)
	if config.JWTSecret != "" && len(config.AdminRoutes) > 0 {
		router.Route("/api/v1/admin", func(admin chi.Router) {
			admin.Use(AdminAuthMiddleware(config.JWTSecret))
			for _, registrar := range config.AdminRoutes {
				registrar(admin)
			}
		})
	}

	var handler http.Handler = router
	if len(config.FrontendOrigins) > 0 {
		handler = CORSMiddleware(config.FrontendOrigins)(handler)
	}
	if config.Logger != nil {
		handler = RequestLogMiddleware(config.Logger)(handler)
	}

	return handler
}

func registerLimitedPublicRoutes(router chi.Router, config RouterConfig) {
	if len(config.PublicArticleReadRoutes) == 0 && len(config.PublicArticleSearchRoutes) == 0 {
		return
	}
	limits := config.PublicTrafficLimits
	if limits.ReadRatePerSecond <= 0 {
		limits.ReadRatePerSecond = 20
	}
	if limits.ReadBurst <= 0 {
		limits.ReadBurst = 40
	}
	if limits.SearchRatePerSecond <= 0 {
		limits.SearchRatePerSecond = 5
	}
	if limits.SearchBurst <= 0 {
		limits.SearchBurst = 10
	}
	if limits.MaxInFlight <= 0 {
		limits.MaxInFlight = 128
	}
	if limits.MaxClients <= 0 {
		limits.MaxClients = 10000
	}
	if limits.ClientTTL <= 0 {
		limits.ClientTTL = 10 * time.Minute
	}

	readLimiter := NewClientRateLimiter(ClientRateLimitOptions{
		RatePerSecond: limits.ReadRatePerSecond, Burst: limits.ReadBurst,
		MaxClients: limits.MaxClients, ClientTTL: limits.ClientTTL,
	})
	searchLimiter := NewClientRateLimiter(ClientRateLimitOptions{
		RatePerSecond: limits.SearchRatePerSecond, Burst: limits.SearchBurst,
		MaxClients: limits.MaxClients, ClientTTL: limits.ClientTTL,
	})
	concurrencyLimit := ConcurrencyLimitMiddleware(limits.MaxInFlight)

	if len(config.PublicArticleReadRoutes) > 0 {
		router.Group(func(public chi.Router) {
			public.Use(concurrencyLimit)
			public.Use(RateLimitMiddleware(readLimiter))
			for _, registrar := range config.PublicArticleReadRoutes {
				registrar(public)
			}
		})
	}
	if len(config.PublicArticleSearchRoutes) > 0 {
		router.Group(func(public chi.Router) {
			public.Use(concurrencyLimit)
			public.Use(RateLimitMiddleware(searchLimiter))
			for _, registrar := range config.PublicArticleSearchRoutes {
				registrar(public)
			}
		})
	}
}
