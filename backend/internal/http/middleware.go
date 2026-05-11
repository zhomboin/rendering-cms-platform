package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	rscors "github.com/rs/cors"

	"rendering-cms-platform/backend/internal/auth"
)

type AuthenticatedUser struct {
	UserID string
	Role   string
}

type authContextKey struct{}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rec *statusRecorder) WriteHeader(status int) {
	if rec.status != 0 {
		return
	}
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *statusRecorder) Write(body []byte) (int, error) {
	if rec.status == 0 {
		rec.status = http.StatusOK
	}
	n, err := rec.ResponseWriter.Write(body)
	rec.bytes += n
	return n, err
}

func RequestLogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			rec := &statusRecorder{ResponseWriter: w}

			next.ServeHTTP(rec, r)

			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			logger.InfoContext(r.Context(), "http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"bytes", rec.bytes,
				"duration_ms", time.Since(startedAt).Milliseconds(),
				"remote_addr", clientIP(r),
				"user_agent", r.UserAgent(),
			)
		})
	}
}

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

func CORSMiddleware(frontendOrigins []string) func(http.Handler) http.Handler {
	allowedOrigins := make([]string, 0, len(frontendOrigins))
	for _, origin := range frontendOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowedOrigins = append(allowedOrigins, origin)
		}
	}

	return rscors.New(rscors.Options{
		AllowedOrigins:       allowedOrigins,
		AllowedMethods:       []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:       []string{"*"},
		AllowCredentials:     true,
		OptionsSuccessStatus: http.StatusNoContent,
	}).Handler
}

func UserFromContext(ctx context.Context) (AuthenticatedUser, bool) {
	user, ok := ctx.Value(authContextKey{}).(AuthenticatedUser)
	return user, ok
}

func clientIP(r *http.Request) string {
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		first, _, _ := strings.Cut(forwardedFor, ",")
		return strings.TrimSpace(first)
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func writeHTTPError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
