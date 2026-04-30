package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type RouterConfig struct {
	LoginHandler http.HandlerFunc
	JWTSecret    string
}

type RouterOption func(*RouterConfig)

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

func NewRouter(options ...RouterOption) http.Handler {
	var config RouterConfig
	for _, option := range options {
		option(&config)
	}

	router := chi.NewRouter()

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

	return router
}
