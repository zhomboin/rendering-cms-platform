package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

// 邮件验证码的固定安全参数：有效期、单码验证上限、重发间隔、每小时发送上限与连续失败触发阈值。
const (
	loginCodeTTL                = 10 * time.Minute
	loginCodeMaxVerifyAttempts  = 5
	loginCodeResendInterval     = 60 * time.Second
	loginCodeHourlySendLimit    = 5
	loginRiskConsecutiveFailure = 3
	recentOutcomesLimit         = 10
)

// EmailSender 邮件发送接口，由 internal/mail.SMTPSender 实现。
type EmailSender interface {
	SendLoginCode(to string, code string, ttl time.Duration) error
}

// CodeLogger 开发调试日志函数；仅 LOGIN_MFA_DEV_LOG 开启时由 main 注入 logger.Info。
type CodeLogger func(message string, args ...any)

// EmailCodeStore 邮件验证码与登录 IP 基线的存取接口，*dbgen.Queries 满足。
type EmailCodeStore interface {
	CreateLoginEmailCode(ctx context.Context, arg dbgen.CreateLoginEmailCodeParams) (dbgen.LoginEmailCode, error)
	GetLatestActiveLoginEmailCode(ctx context.Context, email string) (dbgen.LoginEmailCode, error)
	ClaimLoginEmailCode(ctx context.Context, codeID pgtype.UUID) (int64, error)
	IncrementLoginEmailCodeAttempts(ctx context.Context, codeID pgtype.UUID) (int64, error)
	CountRecentLoginEmailCodes(ctx context.Context, arg dbgen.CountRecentLoginEmailCodesParams) (int64, error)
	UpdateUserLastLoginIPHash(ctx context.Context, arg dbgen.UpdateUserLastLoginIPHashParams) error
}

// LoginDeps 汇集登录、验证码校验与重发三个 handler 的全部依赖。
type LoginDeps struct {
	Secret     string
	Finder     UserFinder
	Attempts   LoginAttemptStore
	Codes      EmailCodeStore
	Sender     EmailSender
	DevLog     CodeLogger
	MFAEnabled bool
	Now        func() time.Time
}

type loginService struct {
	secret     string
	finder     UserFinder
	attempts   LoginAttemptStore
	codes      EmailCodeStore
	sender     EmailSender
	devLog     CodeLogger
	mfaEnabled bool
	now        func() time.Time
}

func newLoginService(deps LoginDeps) *loginService {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &loginService{
		secret:     deps.Secret,
		finder:     deps.Finder,
		attempts:   deps.Attempts,
		codes:      deps.Codes,
		sender:     deps.Sender,
		devLog:     deps.DevLog,
		mfaEnabled: deps.MFAEnabled,
		now:        deps.Now,
	}
}

type MFAChallengeResponse struct {
	MFARequired    bool   `json:"mfaRequired"`
	ChallengeToken string `json:"challengeToken"`
	Email          string `json:"email"`
	ExpiresIn      int    `json:"expiresIn"`
}

type MFAVerifyRequest struct {
	ChallengeToken string `json:"challengeToken"`
	Code           string `json:"code"`
}

type MFAResendRequest struct {
	ChallengeToken string `json:"challengeToken"`
}

type MFAResendResponse struct {
	ExpiresIn int `json:"expiresIn"`
}

type mfaDispatchError struct {
	status  int
	message string
}

func (err *mfaDispatchError) Error() string {
	return err.message
}

// GenerateLoginCode 使用 crypto/rand 生成 6 位随机数字验证码（补前导零）。
func GenerateLoginCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", fmt.Errorf("generate login code: %w", err)
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

// HashLoginCode 以 JWT_SECRET 作为 pepper 计算验证码哈希，验证码不落明文。
func HashLoginCode(secret string, code string) string {
	sum := sha256.Sum256([]byte(secret + ":" + code))
	return hex.EncodeToString(sum[:])
}

// CountConsecutiveFailures 统计自最近一次成功登录以来的连续失败次数；outcomes 按时间倒序。
func CountConsecutiveFailures(outcomes []bool) int {
	count := 0
	for _, success := range outcomes {
		if success {
			break
		}
		count++
	}
	return count
}

// EvaluateLoginRisk 判定本次登录是否需要邮箱验证码：IP 变更或连续失败达到阈值。
func EvaluateLoginRisk(ipChanged bool, outcomes []bool) bool {
	return ipChanged || CountConsecutiveFailures(outcomes) >= loginRiskConsecutiveFailure
}

// mfaRequired 评估密码验证成功后的风险判定；查询失败时返回错误由上层以 500 拒绝。
func (svc *loginService) mfaRequired(ctx context.Context, user UserRecord, ipHash string) (bool, error) {
	if !svc.mfaEnabled {
		return false, nil
	}
	outcomes := []bool(nil)
	if svc.attempts != nil {
		var err error
		outcomes, err = svc.attempts.ListRecentLoginAttemptOutcomes(ctx, dbgen.ListRecentLoginAttemptOutcomesParams{
			Email:    user.Email,
			RowLimit: recentOutcomesLimit,
		})
		if err != nil {
			return false, err
		}
	}
	ipChanged := user.LastLoginIpHash != "" && user.LastLoginIpHash != ipHash
	return EvaluateLoginRisk(ipChanged, outcomes), nil
}

// dispatchLoginCode 生成验证码、入库并发送；已知限流与发送失败以 *mfaDispatchError 返回状态码。
func (svc *loginService) dispatchLoginCode(ctx context.Context, user UserRecord) error {
	count, err := svc.codes.CountRecentLoginEmailCodes(ctx, dbgen.CountRecentLoginEmailCodesParams{
		Email:     user.Email,
		CreatedAt: pgtype.Timestamptz{Time: svc.now().Add(-time.Hour), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("count recent login codes: %w", err)
	}
	if count >= loginCodeHourlySendLimit {
		return &mfaDispatchError{status: http.StatusTooManyRequests, message: "验证码发送过于频繁，请稍后再试"}
	}

	code, err := GenerateLoginCode()
	if err != nil {
		return err
	}
	userID, err := uuid.Parse(user.UserID)
	if err != nil {
		return fmt.Errorf("parse user id: %w", err)
	}
	_, err = svc.codes.CreateLoginEmailCode(ctx, dbgen.CreateLoginEmailCodeParams{
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
		Email:     user.Email,
		CodeHash:  HashLoginCode(svc.secret, code),
		ExpiresAt: pgtype.Timestamptz{Time: svc.now().Add(loginCodeTTL), Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create login code: %w", err)
	}

	if svc.sender != nil {
		if err := svc.sender.SendLoginCode(user.Email, code, loginCodeTTL); err != nil {
			return &mfaDispatchError{status: http.StatusInternalServerError, message: "验证码邮件发送失败"}
		}
		return nil
	}
	if svc.devLog != nil {
		svc.devLog("登录验证码（LOGIN_MFA_DEV_LOG）：email=%s code=%s", user.Email, code)
		return nil
	}
	return &mfaDispatchError{status: http.StatusInternalServerError, message: "邮件验证码功能未正确配置"}
}

func (svc *loginService) writeDispatchError(w http.ResponseWriter, err error) {
	var dispatchErr *mfaDispatchError
	if errors.As(err, &dispatchErr) {
		writeAuthError(w, dispatchErr.status, dispatchErr.message)
		return
	}
	writeAuthError(w, http.StatusInternalServerError, "登录安全检查失败")
}

// NewMFAVerifyHandler 校验邮箱验证码并完成登录。
func NewMFAVerifyHandler(deps LoginDeps) http.HandlerFunc {
	svc := newLoginService(deps)
	return func(w http.ResponseWriter, r *http.Request) {
		var request MFAVerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeAuthError(w, http.StatusBadRequest, "请求体格式不正确")
			return
		}
		request.ChallengeToken = strings.TrimSpace(request.ChallengeToken)
		request.Code = strings.TrimSpace(request.Code)
		if request.ChallengeToken == "" || request.Code == "" {
			writeAuthError(w, http.StatusBadRequest, "challengeToken 和验证码不能为空")
			return
		}

		claims, err := ParseMFAChallengeToken(svc.secret, request.ChallengeToken)
		if err != nil || claims.Subject == "" {
			writeAuthError(w, http.StatusUnauthorized, "登录验证已失效，请重新登录")
			return
		}
		user, err := svc.finder.FindUserByEmail(r.Context(), claims.Subject)
		if err != nil || user.Role != "admin" && user.Role != "editor" {
			writeAuthError(w, http.StatusUnauthorized, "登录验证已失效，请重新登录")
			return
		}

		record, err := svc.codes.GetLatestActiveLoginEmailCode(r.Context(), user.Email)
		if err != nil {
			writeAuthError(w, http.StatusUnauthorized, "验证码错误或已过期")
			return
		}
		if !record.ExpiresAt.Valid || !record.ExpiresAt.Time.After(svc.now()) || record.Attempts >= loginCodeMaxVerifyAttempts {
			writeAuthError(w, http.StatusUnauthorized, "验证码错误或已过期")
			return
		}
		if HashLoginCode(svc.secret, request.Code) != record.CodeHash {
			_, _ = svc.codes.IncrementLoginEmailCodeAttempts(r.Context(), record.CodeID)
			writeAuthError(w, http.StatusUnauthorized, "验证码错误或已过期")
			return
		}
		claimed, err := svc.codes.ClaimLoginEmailCode(r.Context(), record.CodeID)
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "登录安全检查失败")
			return
		}
		if claimed == 0 {
			writeAuthError(w, http.StatusUnauthorized, "验证码错误或已过期")
			return
		}

		ipHash := ipHashFromRequest(r)
		recordLoginAttempt(r.Context(), svc.attempts, user.Email, ipHash, true, "")
		svc.updateLastLoginIP(r.Context(), user.UserID, ipHash)

		tokens, err := IssueTokenPairWithClock(svc.secret, user.UserID, user.Role, svc.now)
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "登录令牌生成失败")
			return
		}
		writeJSON(w, http.StatusOK, LoginResponse{
			Token:        tokens.Token,
			RefreshToken: tokens.RefreshToken,
			User:         LoginUser{UserID: user.UserID, Email: user.Email, Name: user.Name, Role: user.Role},
		})
	}
}

// NewMFAResendHandler 在限流允许时作废旧验证码并重新发送。
func NewMFAResendHandler(deps LoginDeps) http.HandlerFunc {
	svc := newLoginService(deps)
	return func(w http.ResponseWriter, r *http.Request) {
		var request MFAResendRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeAuthError(w, http.StatusBadRequest, "请求体格式不正确")
			return
		}
		request.ChallengeToken = strings.TrimSpace(request.ChallengeToken)
		if request.ChallengeToken == "" {
			writeAuthError(w, http.StatusBadRequest, "challengeToken 不能为空")
			return
		}

		claims, err := ParseMFAChallengeToken(svc.secret, request.ChallengeToken)
		if err != nil || claims.Subject == "" {
			writeAuthError(w, http.StatusUnauthorized, "登录验证已失效，请重新登录")
			return
		}
		user, err := svc.finder.FindUserByEmail(r.Context(), claims.Subject)
		if err != nil || user.Role != "admin" && user.Role != "editor" {
			writeAuthError(w, http.StatusUnauthorized, "登录验证已失效，请重新登录")
			return
		}

		latest, err := svc.codes.GetLatestActiveLoginEmailCode(r.Context(), user.Email)
		if err == nil {
			if latest.CreatedAt.Valid && svc.now().Sub(latest.CreatedAt.Time) < loginCodeResendInterval {
				writeAuthError(w, http.StatusTooManyRequests, "验证码发送过于频繁，请稍后再试")
				return
			}
			_, _ = svc.codes.ClaimLoginEmailCode(r.Context(), latest.CodeID)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			writeAuthError(w, http.StatusInternalServerError, "登录安全检查失败")
			return
		}

		if err := svc.dispatchLoginCode(r.Context(), user); err != nil {
			svc.writeDispatchError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, MFAResendResponse{ExpiresIn: int(loginCodeTTL.Seconds())})
	}
}

func (svc *loginService) updateLastLoginIP(ctx context.Context, userID string, ipHash string) {
	if svc.codes == nil {
		return
	}
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return
	}
	_ = svc.codes.UpdateUserLastLoginIPHash(ctx, dbgen.UpdateUserLastLoginIPHashParams{
		IpHash: pgtype.Text{String: ipHash, Valid: true},
		UserID: pgtype.UUID{Bytes: parsed, Valid: true},
	})
}
