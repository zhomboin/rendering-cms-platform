package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

const (
	testMFASecret = "secret-32-characters-minimum-value"
	testMFAUserID = "11111111-1111-1111-1111-111111111111"
)

func testMFADeps(finder UserFinder, attempts LoginAttemptStore, codes EmailCodeStore, sender EmailSender, now time.Time) LoginDeps {
	deps := testLoginDeps(finder, attempts, now)
	deps.Codes = codes
	deps.Sender = sender
	deps.MFAEnabled = true
	return deps
}

func testMFAUser(passwordHash string, lastLoginIpHash string) UserRecord {
	return UserRecord{
		UserID:          testMFAUserID,
		Email:           "admin@example.com",
		Name:            "Admin",
		PasswordHash:    passwordHash,
		Role:            "admin",
		LastLoginIpHash: lastLoginIpHash,
	}
}

func newMFALoginRequest(password string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{
		"email": "admin@example.com",
		"password": "`+password+`"
	}`))
	req.RemoteAddr = "192.0.2.10:12345"
	return req
}

func TestCountConsecutiveFailures(t *testing.T) {
	cases := []struct {
		name     string
		outcomes []bool
		want     int
	}{
		{name: "空记录", outcomes: nil, want: 0},
		{name: "三次失败", outcomes: []bool{false, false, false}, want: 3},
		{name: "成功后重置", outcomes: []bool{false, false, true, false, false}, want: 2},
		{name: "全部成功", outcomes: []bool{true, true}, want: 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := CountConsecutiveFailures(testCase.outcomes); got != testCase.want {
				t.Fatalf("CountConsecutiveFailures(%v) = %d, want %d", testCase.outcomes, got, testCase.want)
			}
		})
	}
}

func TestEvaluateLoginRisk(t *testing.T) {
	if !EvaluateLoginRisk(true, nil) {
		t.Fatal("IP 变更应触发验证码")
	}
	if !EvaluateLoginRisk(false, []bool{false, false, false}) {
		t.Fatal("连续失败达到 3 次应触发验证码")
	}
	if EvaluateLoginRisk(false, []bool{false, false}) {
		t.Fatal("连续失败不足 3 次不应触发验证码")
	}
	if EvaluateLoginRisk(false, []bool{false, false, true, false, false}) {
		t.Fatal("中间成功登录后连续失败被重置，不应触发验证码")
	}
}

func TestGenerateLoginCode(t *testing.T) {
	format := regexp.MustCompile(`^\d{6}$`)
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		code, err := GenerateLoginCode()
		if err != nil {
			t.Fatalf("GenerateLoginCode() returned error: %v", err)
		}
		if !format.MatchString(code) {
			t.Fatalf("code = %q, want 6 位数字", code)
		}
		seen[code] = true
	}
	if len(seen) < 2 {
		t.Fatal("多次生成的验证码应具有随机性")
	}
}

func TestHashLoginCode(t *testing.T) {
	first := HashLoginCode(testMFASecret, "123456")
	if first == "" || first == "123456" {
		t.Fatalf("hash = %q, want 非空且不等于明文", first)
	}
	if HashLoginCode(testMFASecret, "123456") != first {
		t.Fatal("同一 secret 与验证码的哈希应稳定")
	}
	if HashLoginCode(testMFASecret, "654321") == first {
		t.Fatal("不同验证码的哈希应不同")
	}
	if HashLoginCode("another-secret-32-characters-long!!", "123456") == first {
		t.Fatal("不同 secret 的哈希应不同")
	}
}

func TestLoginHandlerIssuesMFAChallengeOnIPChange(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, sha256String("198.51.100.7"))}
	attempts := &loginAttemptStoreStub{}
	codes := &emailCodeStoreStub{}
	sender := &emailSenderStub{}
	handler := NewLoginHandler(testMFADeps(finder, attempts, codes, sender, now))

	handler.ServeHTTP(httptest.NewRecorder(), newMFALoginRequest("correct-password"))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFALoginRequest("correct-password"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response MFAChallengeResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode challenge response: %v", err)
	}
	if !response.MFARequired || response.ChallengeToken == "" || response.Email != "admin@example.com" || response.ExpiresIn != 600 {
		t.Fatalf("challenge response = %#v, want mfaRequired challenge for admin@example.com", response)
	}
	claims, err := ParseMFAChallengeToken(testMFASecret, response.ChallengeToken)
	if err != nil {
		t.Fatalf("challenge token invalid: %v", err)
	}
	if claims.Subject != "admin@example.com" || claims.UserID != testMFAUserID {
		t.Fatalf("challenge claims = %#v, want subject email and user id", claims)
	}
	if len(sender.sent) == 0 || sender.sent[len(sender.sent)-1].to != "admin@example.com" {
		t.Fatalf("sent emails = %#v, want code email to admin@example.com", sender.sent)
	}
	if !regexp.MustCompile(`^\d{6}$`).MatchString(sender.sent[len(sender.sent)-1].code) {
		t.Fatalf("sent code = %q, want 6 位数字", sender.sent[len(sender.sent)-1].code)
	}
	if len(codes.createdCodes) == 0 {
		t.Fatal("验证码应写入数据库")
	}
	if attempts.createdAttempt.Success {
		t.Fatal("验证码挑战阶段不应记录登录成功")
	}
}

func TestLoginHandlerIssuesMFAChallengeAfterConsecutiveFailures(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, sha256String("192.0.2.10"))}
	attempts := &loginAttemptStoreStub{outcomes: []bool{false, false, false}}
	handler := NewLoginHandler(testMFADeps(finder, attempts, &emailCodeStoreStub{}, &emailSenderStub{}, now))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFALoginRequest("correct-password"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response MFAChallengeResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode challenge response: %v", err)
	}
	if !response.MFARequired {
		t.Fatalf("response = %#v, want mfaRequired challenge", response)
	}
}

func TestLoginHandlerSkipsMFAWhenNoRisk(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, sha256String("192.0.2.10"))}
	attempts := &loginAttemptStoreStub{outcomes: []bool{true, false, false}}
	codes := &emailCodeStoreStub{}
	sender := &emailSenderStub{}
	handler := NewLoginHandler(testMFADeps(finder, attempts, codes, sender, now))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFALoginRequest("correct-password"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response LoginResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if response.Token == "" || response.RefreshToken == "" {
		t.Fatalf("response = %#v, want direct token pair without mfa", response)
	}
	if len(sender.sent) != 0 || len(codes.createdCodes) != 0 {
		t.Fatal("无风险登录不应发送验证码")
	}
	if !attempts.createdAttempt.Success {
		t.Fatal("无风险登录应记录成功")
	}
	if len(codes.updatedIPs) != 1 {
		t.Fatal("无风险登录成功后应更新登录 IP 基线")
	}
}

func TestLoginHandlerMFASendFailure(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, sha256String("198.51.100.7"))}
	handler := NewLoginHandler(testMFADeps(finder, &loginAttemptStoreStub{}, &emailCodeStoreStub{}, &emailSenderStub{err: errBoom}, now))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFALoginRequest("correct-password"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestLoginHandlerMFAHourlyLimit(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, sha256String("198.51.100.7"))}
	handler := NewLoginHandler(testMFADeps(finder, &loginAttemptStoreStub{}, &emailCodeStoreStub{recentCount: 5}, &emailSenderStub{}, now))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFALoginRequest("correct-password"))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestLoginHandlerMFADevLog(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, sha256String("198.51.100.7"))}
	logged := []string{}
	deps := testMFADeps(finder, &loginAttemptStoreStub{}, &emailCodeStoreStub{}, nil, now)
	deps.DevLog = func(message string, args ...any) {
		logged = append(logged, message)
	}

	rec := httptest.NewRecorder()
	NewLoginHandler(deps).ServeHTTP(rec, newMFALoginRequest("correct-password"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(logged) != 1 || !strings.Contains(logged[0], "登录验证码") {
		t.Fatalf("dev log = %#v, want one login code log entry", logged)
	}
}

func newMFAVerifyRequest(challengeToken string, code string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/verify", strings.NewReader(`{
		"challengeToken": "`+challengeToken+`",
		"code": "`+code+`"
	}`))
	req.RemoteAddr = "192.0.2.10:12345"
	return req
}

func testMFACodeRecord(code string, now time.Time, attempts int32) dbgen.LoginEmailCode {
	codeID, _ := uuid.Parse(testMFAUserID)
	return dbgen.LoginEmailCode{
		CodeID:    pgtype.UUID{Bytes: codeID, Valid: true},
		Email:     "admin@example.com",
		CodeHash:  HashLoginCode(testMFASecret, code),
		ExpiresAt: pgtype.Timestamptz{Time: now.Add(loginCodeTTL), Valid: true},
		Attempts:  attempts,
	}
}

func testMFAChallengeToken(now time.Time) string {
	token, err := IssueMFAChallengeToken(testMFASecret, testMFAUserID, "admin", "admin@example.com", func() time.Time {
		return now
	})
	if err != nil {
		panic(err)
	}
	return token
}

func TestMFAVerifyHandlerIssuesTokensOnCorrectCode(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, sha256String("198.51.100.7"))}
	attempts := &loginAttemptStoreStub{}
	codes := &emailCodeStoreStub{activeCode: testMFACodeRecord("654321", now, 0), claimResults: 1}
	handler := NewMFAVerifyHandler(testMFADeps(finder, attempts, codes, &emailSenderStub{}, now))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFAVerifyRequest(testMFAChallengeToken(now), "654321"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response LoginResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if response.Token == "" || response.RefreshToken == "" || response.User.Email != "admin@example.com" {
		t.Fatalf("response = %#v, want full login response", response)
	}
	if _, err := ParseAccessToken(testMFASecret, response.Token); err != nil {
		t.Fatalf("access token invalid: %v", err)
	}
	if !attempts.createdAttempt.Success || attempts.createdAttempt.Email != "admin@example.com" {
		t.Fatalf("created attempt = %#v, want successful attempt", attempts.createdAttempt)
	}
	if len(codes.updatedIPs) != 1 || codes.updatedIPs[0].IpHash.String != sha256String("192.0.2.10") {
		t.Fatalf("updated ips = %#v, want verify request ip baseline", codes.updatedIPs)
	}
	if len(codes.incremented) != 0 {
		t.Fatal("正确验证码不应累加失败次数")
	}
}

func TestMFAVerifyHandlerRejectsWrongCode(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, "")}
	codes := &emailCodeStoreStub{activeCode: testMFACodeRecord("654321", now, 0)}
	attempts := &loginAttemptStoreStub{}
	handler := NewMFAVerifyHandler(testMFADeps(finder, attempts, codes, &emailSenderStub{}, now))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFAVerifyRequest(testMFAChallengeToken(now), "000000"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if len(codes.incremented) != 1 {
		t.Fatal("错误验证码应累加失败次数")
	}
	if len(codes.claimed) != 0 {
		t.Fatal("错误验证码不应占用验证码")
	}
	if attempts.createdAttempt.Success {
		t.Fatal("错误验证码不应记录登录成功")
	}
}

func TestMFAVerifyHandlerRejectsExhaustedCode(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, "")}
	codes := &emailCodeStoreStub{activeCode: testMFACodeRecord("654321", now, loginCodeMaxVerifyAttempts)}
	handler := NewMFAVerifyHandler(testMFADeps(finder, &loginAttemptStoreStub{}, codes, &emailSenderStub{}, now))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFAVerifyRequest(testMFAChallengeToken(now), "654321"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if len(codes.incremented) != 0 || len(codes.claimed) != 0 {
		t.Fatal("超过尝试上限的验证码应直接拒绝且不再计数")
	}
}

func TestMFAVerifyHandlerRejectsExpiredCode(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, "")}
	expired := testMFACodeRecord("654321", now, 0)
	expired.ExpiresAt = pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true}
	codes := &emailCodeStoreStub{activeCode: expired}
	handler := NewMFAVerifyHandler(testMFADeps(finder, &loginAttemptStoreStub{}, codes, &emailSenderStub{}, now))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFAVerifyRequest(testMFAChallengeToken(now), "654321"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMFAVerifyHandlerRejectsClaimedCode(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, "")}
	codes := &emailCodeStoreStub{activeCode: testMFACodeRecord("654321", now, 0), claimResults: 0}
	handler := NewMFAVerifyHandler(testMFADeps(finder, &loginAttemptStoreStub{}, codes, &emailSenderStub{}, now))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFAVerifyRequest(testMFAChallengeToken(now), "654321"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMFAVerifyHandlerRejectsInvalidChallengeToken(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	handler := NewMFAVerifyHandler(testMFADeps(&userFinderStub{}, &loginAttemptStoreStub{}, &emailCodeStoreStub{}, &emailSenderStub{}, now))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFAVerifyRequest("not-a-token", "654321"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func newMFAResendRequest(challengeToken string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/mfa/resend", strings.NewReader(`{
		"challengeToken": "`+challengeToken+`"
	}`))
	req.RemoteAddr = "192.0.2.10:12345"
	return req
}

func TestMFAResendHandlerRateLimitsWithinInterval(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, "")}
	latest := testMFACodeRecord("654321", now, 0)
	latest.CreatedAt = pgtype.Timestamptz{Time: now.Add(-30 * time.Second), Valid: true}
	codes := &emailCodeStoreStub{activeCode: latest}
	sender := &emailSenderStub{}
	handler := NewMFAResendHandler(testMFADeps(finder, &loginAttemptStoreStub{}, codes, sender, now))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFAResendRequest(testMFAChallengeToken(now)))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if len(sender.sent) != 0 || len(codes.createdCodes) != 0 {
		t.Fatal("限流期内不应重发验证码")
	}
}

func TestMFAResendHandlerIssuesNewCode(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	passwordHash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}
	finder := &userFinderStub{user: testMFAUser(passwordHash, "")}
	latest := testMFACodeRecord("654321", now, 0)
	latest.CreatedAt = pgtype.Timestamptz{Time: now.Add(-2 * time.Minute), Valid: true}
	codes := &emailCodeStoreStub{activeCode: latest}
	sender := &emailSenderStub{}
	handler := NewMFAResendHandler(testMFADeps(finder, &loginAttemptStoreStub{}, codes, sender, now))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newMFAResendRequest(testMFAChallengeToken(now)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response MFAResendResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode resend response: %v", err)
	}
	if response.ExpiresIn != 600 {
		t.Fatalf("expiresIn = %d, want 600", response.ExpiresIn)
	}
	if len(codes.claimed) != 1 {
		t.Fatal("重发前应作废旧验证码")
	}
	if len(codes.createdCodes) != 1 || len(sender.sent) != 1 {
		t.Fatalf("重发应创建并投递一个新验证码：created=%d sent=%d", len(codes.createdCodes), len(sender.sent))
	}
}

var errBoom = &boomError{}

type boomError struct{}

func (*boomError) Error() string { return "boom" }

type emailMessage struct {
	to   string
	code string
}

type emailSenderStub struct {
	sent []emailMessage
	err  error
}

func (s *emailSenderStub) SendLoginCode(to string, code string, ttl time.Duration) error {
	if s.err != nil {
		return s.err
	}
	s.sent = append(s.sent, emailMessage{to: to, code: code})
	return nil
}

type emailCodeStoreStub struct {
	recentCount  int64
	createdCodes []dbgen.CreateLoginEmailCodeParams
	activeCode   dbgen.LoginEmailCode
	activeErr    error
	claimResults int64
	claimed      []pgtype.UUID
	incremented  []pgtype.UUID
	updatedIPs   []dbgen.UpdateUserLastLoginIPHashParams
}

func (s *emailCodeStoreStub) CreateLoginEmailCode(ctx context.Context, arg dbgen.CreateLoginEmailCodeParams) (dbgen.LoginEmailCode, error) {
	s.createdCodes = append(s.createdCodes, arg)
	return dbgen.LoginEmailCode{}, nil
}

func (s *emailCodeStoreStub) GetLatestActiveLoginEmailCode(ctx context.Context, email string) (dbgen.LoginEmailCode, error) {
	if s.activeErr != nil {
		return dbgen.LoginEmailCode{}, s.activeErr
	}
	return s.activeCode, nil
}

func (s *emailCodeStoreStub) ClaimLoginEmailCode(ctx context.Context, codeID pgtype.UUID) (int64, error) {
	s.claimed = append(s.claimed, codeID)
	return s.claimResults, nil
}

func (s *emailCodeStoreStub) IncrementLoginEmailCodeAttempts(ctx context.Context, codeID pgtype.UUID) (int64, error) {
	s.incremented = append(s.incremented, codeID)
	return int64(len(s.incremented)), nil
}

func (s *emailCodeStoreStub) CountRecentLoginEmailCodes(ctx context.Context, arg dbgen.CountRecentLoginEmailCodesParams) (int64, error) {
	return s.recentCount, nil
}

func (s *emailCodeStoreStub) UpdateUserLastLoginIPHash(ctx context.Context, arg dbgen.UpdateUserLastLoginIPHashParams) error {
	s.updatedIPs = append(s.updatedIPs, arg)
	return nil
}
