package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

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

func NewLoginHandler(secret string, finder UserFinder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeAuthError(w, http.StatusBadRequest, "请求体格式不正确")
			return
		}
		if request.Email == "" || request.Password == "" {
			writeAuthError(w, http.StatusBadRequest, "邮箱和密码不能为空")
			return
		}

		user, err := finder.FindUserByEmail(r.Context(), request.Email)
		if err != nil || !VerifyPassword(user.PasswordHash, request.Password) {
			writeAuthError(w, http.StatusUnauthorized, "邮箱或密码错误")
			return
		}
		if user.Role != "admin" && user.Role != "editor" {
			writeAuthError(w, http.StatusForbidden, "用户无后台访问权限")
			return
		}

		token, err := IssueToken(secret, user.UserID, user.Role)
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "登录令牌生成失败")
			return
		}

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

func writeAuthError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

var ErrUserNotFound = errors.New("user not found")
