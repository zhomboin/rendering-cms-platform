package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"rendering-cms-platform/backend/internal/database/dbgen"
)

func TestLoginHandlerRejectsLockedLoginBeforePasswordLookup(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	store := &loginAttemptStoreStub{
		emailFailures: []pgtype.Timestamptz{
			{Time: now.Add(-4 * time.Minute), Valid: true},
			{Time: now.Add(-3 * time.Minute), Valid: true},
			{Time: now.Add(-2 * time.Minute), Valid: true},
			{Time: now.Add(-1 * time.Minute), Valid: true},
			{Time: now.Add(-10 * time.Second), Valid: true},
		},
	}
	finder := &userFinderStub{}
	handler := NewLoginHandlerWithClock("secret-32-characters-minimum-value", finder, store, func() time.Time {
		return now
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{
		"email": "admin@example.com",
		"password": "wrong-password"
	}`))
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if finder.called {
		t.Fatal("user finder should not be called while login is locked")
	}
}

func TestLoginHandlerRecordsFailedAttempt(t *testing.T) {
	store := &loginAttemptStoreStub{}
	finder := &userFinderStub{returnErr: ErrUserNotFound}
	handler := NewLoginHandlerWithClock("secret-32-characters-minimum-value", finder, store, func() time.Time {
		return time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{
		"email": "missing@example.com",
		"password": "wrong-password"
	}`))
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if store.createdAttempt.Email == "" || store.createdAttempt.Success {
		t.Fatalf("created attempt = %#v, want failed valid attempt", store.createdAttempt)
	}
	if store.createdAttempt.Email != "missing@example.com" {
		t.Fatalf("email = %q, want missing@example.com", store.createdAttempt.Email)
	}
}

func TestLoginHandlerDoesNotCombineEmailAndIPFailures(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	store := &loginAttemptStoreStub{
		emailFailures: []pgtype.Timestamptz{
			{Time: now.Add(-4 * time.Minute), Valid: true},
			{Time: now.Add(-3 * time.Minute), Valid: true},
			{Time: now.Add(-2 * time.Minute), Valid: true},
		},
		ipFailures: []pgtype.Timestamptz{
			{Time: now.Add(-1 * time.Minute), Valid: true},
			{Time: now.Add(-10 * time.Second), Valid: true},
		},
	}
	finder := &userFinderStub{returnErr: ErrUserNotFound}
	handler := NewLoginHandlerWithClock("secret-32-characters-minimum-value", finder, store, func() time.Time {
		return now
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{
		"email": "missing@example.com",
		"password": "wrong-password"
	}`))
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusTooManyRequests {
		t.Fatal("login should not be locked when email and IP failures are separately below threshold")
	}
	if !finder.called {
		t.Fatal("user finder should be called when neither independent lockout threshold is reached")
	}
}

type userFinderStub struct {
	called    bool
	returnErr error
	user      UserRecord
}

func (s *userFinderStub) FindUserByEmail(ctx context.Context, email string) (UserRecord, error) {
	s.called = true
	if s.returnErr != nil {
		return UserRecord{}, s.returnErr
	}
	return s.user, nil
}

type loginAttemptStoreStub struct {
	emailFailures  []pgtype.Timestamptz
	ipFailures     []pgtype.Timestamptz
	createdAttempt dbgen.CreateLoginAttemptParams
}

func (s *loginAttemptStoreStub) ListRecentFailedLoginAttemptsByEmail(ctx context.Context, arg dbgen.ListRecentFailedLoginAttemptsByEmailParams) ([]pgtype.Timestamptz, error) {
	return s.emailFailures, nil
}

func (s *loginAttemptStoreStub) ListRecentFailedLoginAttemptsByIP(ctx context.Context, arg dbgen.ListRecentFailedLoginAttemptsByIPParams) ([]pgtype.Timestamptz, error) {
	return s.ipFailures, nil
}

func (s *loginAttemptStoreStub) CreateLoginAttempt(ctx context.Context, arg dbgen.CreateLoginAttemptParams) (dbgen.LoginAttempt, error) {
	s.createdAttempt = arg
	return dbgen.LoginAttempt{}, nil
}
