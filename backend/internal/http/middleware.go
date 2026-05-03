package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"rendering-cms-platform/backend/internal/auth"
)

type AuthenticatedUser struct {
	UserID string
	Role   string
}

type authContextKey struct{}

func AdminAuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(raw, "Bearer ") {
				writeHTTPError(w, http.StatusUnauthorized, "缺少 Bearer token")
				return
			}

			claims, err := auth.ParseToken(secret, strings.TrimSpace(strings.TrimPrefix(raw, "Bearer ")))
			if err != nil {
				writeHTTPError(w, http.StatusUnauthorized, "无效或过期的 token")
				return
			}
			if claims.Role != "admin" && claims.Role != "editor" {
				writeHTTPError(w, http.StatusForbidden, "用户无后台访问权限")
				return
			}

			ctx := context.WithValue(r.Context(), authContextKey{}, AuthenticatedUser{
				UserID: claims.UserID,
				Role:   claims.Role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func CORSMiddleware(frontendOrigin string) func(http.Handler) http.Handler {
	allowedMethods := "GET, POST, PATCH, DELETE, OPTIONS"
	allowedHeaders := "Authorization, Content-Type"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == frontendOrigin {
				w.Header().Set("Access-Control-Allow-Origin", frontendOrigin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
				w.Header().Add("Vary", "Origin")
			}

			if r.Method == http.MethodOptions && origin == frontendOrigin {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func UserFromContext(ctx context.Context) (AuthenticatedUser, bool) {
	user, ok := ctx.Value(authContextKey{}).(AuthenticatedUser)
	return user, ok
}

func writeHTTPError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
