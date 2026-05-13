package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string    `json:"token"`
	User  LoginUser `json:"user"`
}

type LoginUser struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

type UserRecord struct {
	UserID       string
	Email        string
	Name         string
	PasswordHash string
	Role         string
}

type UserFinder interface {
	FindUserByEmail(ctx context.Context, email string) (UserRecord, error)
}

type LoginAttemptStore interface {
	ListRecentFailedLoginAttemptsByEmail(ctx context.Context, arg dbgen.ListRecentFailedLoginAttemptsByEmailParams) ([]pgtype.Timestamptz, error)
	ListRecentFailedLoginAttemptsByIP(ctx context.Context, arg dbgen.ListRecentFailedLoginAttemptsByIPParams) ([]pgtype.Timestamptz, error)
	CreateLoginAttempt(ctx context.Context, arg dbgen.CreateLoginAttemptParams) (dbgen.LoginAttempt, error)
}

type DatabaseUserFinder struct {
	queries *dbgen.Queries
}

func NewDatabaseUserFinder(queries *dbgen.Queries) DatabaseUserFinder {
	return DatabaseUserFinder{queries: queries}
}

func (finder DatabaseUserFinder) FindUserByEmail(ctx context.Context, email string) (UserRecord, error) {
	user, err := finder.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return UserRecord{}, err
	}

	return UserRecord{
		UserID:       user.UserID.String(),
		Email:        user.Email,
		Name:         user.Name,
		PasswordHash: user.PasswordHash,
		Role:         string(user.Role),
	}, nil
}

func NewLoginHandler(secret string, finder UserFinder, stores ...LoginAttemptStore) http.HandlerFunc {
	var store LoginAttemptStore
	if len(stores) > 0 {
		store = stores[0]
	}
	return NewLoginHandlerWithClock(secret, finder, store, time.Now)
}

func NewLoginHandlerWithClock(secret string, finder UserFinder, store LoginAttemptStore, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeAuthError(w, http.StatusBadRequest, "请求体格式不正确")
			return
		}
		request.Email = strings.TrimSpace(strings.ToLower(request.Email))
		if request.Email == "" || request.Password == "" {
			writeAuthError(w, http.StatusBadRequest, "邮箱和密码不能为空")
			return
		}
		ipHash := ipHashFromRequest(r)

		if store != nil {
			locked, err := isLoginLocked(r.Context(), store, request.Email, ipHash, now())
			if err != nil {
				writeAuthError(w, http.StatusInternalServerError, "登录安全检查失败")
				return
			}
			if locked {
				writeAuthError(w, http.StatusTooManyRequests, "登录失败次数过多，请稍后再试")
				return
			}
		}

		user, err := finder.FindUserByEmail(r.Context(), request.Email)
		if err != nil {
			_ = VerifyPassword(dummyPasswordHash, request.Password)
			recordLoginAttempt(r.Context(), store, request.Email, ipHash, false, "invalid_credentials")
			writeAuthError(w, http.StatusUnauthorized, "邮箱或密码错误")
			return
		}
		if !VerifyPassword(user.PasswordHash, request.Password) {
			recordLoginAttempt(r.Context(), store, request.Email, ipHash, false, "invalid_credentials")
			writeAuthError(w, http.StatusUnauthorized, "邮箱或密码错误")
			return
		}
		if user.Role != "admin" && user.Role != "editor" {
			recordLoginAttempt(r.Context(), store, request.Email, ipHash, false, "forbidden_role")
			writeAuthError(w, http.StatusForbidden, "用户无后台访问权限")
			return
		}

		token, err := IssueToken(secret, user.UserID, user.Role)
		if err != nil {
			recordLoginAttempt(r.Context(), store, request.Email, ipHash, false, "token_issue_failed")
			writeAuthError(w, http.StatusInternalServerError, "登录令牌生成失败")
			return
		}
		recordLoginAttempt(r.Context(), store, request.Email, ipHash, true, "")

		writeJSON(w, http.StatusOK, LoginResponse{
			Token: token,
			User: LoginUser{
				UserID: user.UserID,
				Email:  user.Email,
				Name:   user.Name,
				Role:   user.Role,
			},
		})
	}
}

func isLoginLocked(ctx context.Context, store LoginAttemptStore, email string, ipHash string, currentTime time.Time) (bool, error) {
	cutoff := pgtype.Timestamptz{Time: currentTime.Add(-time.Hour), Valid: true}
	emailFailures, err := store.ListRecentFailedLoginAttemptsByEmail(ctx, dbgen.ListRecentFailedLoginAttemptsByEmailParams{
		CreatedAt: cutoff,
		Email:     email,
	})
	if err != nil {
		return false, err
	}
	if decision := EvaluateLoginLockout(currentTime, loginAttemptTimes(emailFailures)); decision.Locked {
		return true, nil
	}

	ipFailures, err := store.ListRecentFailedLoginAttemptsByIP(ctx, dbgen.ListRecentFailedLoginAttemptsByIPParams{
		CreatedAt: cutoff,
		IpHash:    ipHash,
	})
	if err != nil {
		return false, err
	}
	return EvaluateLoginLockout(currentTime, loginAttemptTimes(ipFailures)).Locked, nil
}

func recordLoginAttempt(ctx context.Context, store LoginAttemptStore, email string, ipHash string, success bool, reason string) {
	if store == nil {
		return
	}
	_, _ = store.CreateLoginAttempt(ctx, dbgen.CreateLoginAttemptParams{
		Email:         email,
		IpHash:        ipHash,
		Success:       success,
		FailureReason: nullableText(reason),
	})
}

func loginAttemptTimes(values []pgtype.Timestamptz) []time.Time {
	result := make([]time.Time, 0, len(values))
	for _, value := range values {
		if value.Valid {
			result = append(result, value.Time)
		}
	}
	return result
}

func ipHashFromRequest(r *http.Request) string {
	host := r.RemoteAddr
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		host, _, _ = strings.Cut(forwardedFor, ",")
		host = strings.TrimSpace(host)
	} else if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		host = realIP
	} else if parsedHost, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = parsedHost
	}
	sum := sha256.Sum256([]byte(host))
	return hex.EncodeToString(sum[:])
}

func nullableText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

var ErrUserNotFound = errors.New("user not found")

const dummyPasswordHash = "$2a$10$CwTycUXWue0Thq9StjUM0uJ8cEzXM0FUqrkLwZT72b1Q9n8TaQd8u"
