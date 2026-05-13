package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

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
				"remote_ip_hash", ClientIPHash(r),
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
	allowedOrigins := make(map[string]struct{}, len(frontendOrigins))
	for _, origin := range frontendOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowedOrigins[origin] = struct{}{}
		}
	}

	allowedMethods := []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	allowedHeaders := []string{"Content-Type", "Authorization", "X-Requested-With"}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if _, ok := allowedOrigins[origin]; ok {
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method != http.MethodOptions || r.Header.Get("Access-Control-Request-Method") == "" {
				next.ServeHTTP(w, r)
				return
			}

			if _, ok := allowedOrigins[origin]; !ok {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			requestMethod := strings.TrimSpace(r.Header.Get("Access-Control-Request-Method"))
			if !containsToken(allowedMethods, requestMethod) {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			w.Header().Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
			if requestedHeaders := allowedRequestedHeaders(r.Header.Get("Access-Control-Request-Headers"), allowedHeaders); requestedHeaders != "" {
				w.Header().Set("Access-Control-Allow-Headers", requestedHeaders)
			}
			w.WriteHeader(http.StatusNoContent)
		})
	}
}

func allowedRequestedHeaders(value string, allowedHeaders []string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	var result []string
	for _, header := range strings.Split(value, ",") {
		header = strings.TrimSpace(header)
		if containsToken(allowedHeaders, header) {
			result = append(result, header)
		}
	}
	return strings.Join(result, ", ")
}

func containsToken(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func RequestSizeLimitMiddleware(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limit > 0 && r.ContentLength > limit {
				writeHTTPError(w, http.StatusRequestEntityTooLarge, "请求体过大")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
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

func ClientIPHash(r *http.Request) string {
	sum := sha256.Sum256([]byte(clientIP(r)))
	return hex.EncodeToString(sum[:])
}

func writeHTTPError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		slog.Error("failed to encode JSON error response", "error", err, "status", status)
	}
}
