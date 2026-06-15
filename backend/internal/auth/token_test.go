package auth

import (
	"testing"
	"time"
)

func TestIssueAndParseToken(t *testing.T) {
	token, err := IssueToken("secret-32-characters-minimum-value", "user-1", "admin")
	if err != nil {
		t.Fatalf("IssueToken() returned error: %v", err)
	}

	claims, err := ParseToken("secret-32-characters-minimum-value", token)
	if err != nil {
		t.Fatalf("ParseToken() returned error: %v", err)
	}

	if claims.UserID != "user-1" {
		t.Fatalf("UserID = %q, want user-1", claims.UserID)
	}
	if claims.Role != "admin" {
		t.Fatalf("Role = %q, want admin", claims.Role)
	}
}

func TestIssueTokenPairUsesSeparateAccessAndRefreshTypes(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	pair, err := IssueTokenPairWithClock("secret-32-characters-minimum-value", "user-1", "admin", func() time.Time {
		return now
	})
	if err != nil {
		t.Fatalf("IssueTokenPairWithClock() returned error: %v", err)
	}

	accessClaims, err := ParseAccessToken("secret-32-characters-minimum-value", pair.Token)
	if err != nil {
		t.Fatalf("ParseAccessToken() returned error: %v", err)
	}
	if accessClaims.TokenType != TokenTypeAccess {
		t.Fatalf("access token type = %q, want %q", accessClaims.TokenType, TokenTypeAccess)
	}
	if accessClaims.ExpiresAt == nil || !accessClaims.ExpiresAt.Time.Equal(now.Add(accessTokenTTL)) {
		t.Fatalf("access expiry = %v, want %v", accessClaims.ExpiresAt, now.Add(accessTokenTTL))
	}

	refreshClaims, err := ParseRefreshToken("secret-32-characters-minimum-value", pair.RefreshToken)
	if err != nil {
		t.Fatalf("ParseRefreshToken() returned error: %v", err)
	}
	if refreshClaims.TokenType != TokenTypeRefresh {
		t.Fatalf("refresh token type = %q, want %q", refreshClaims.TokenType, TokenTypeRefresh)
	}
	if refreshClaims.ExpiresAt == nil || !refreshClaims.ExpiresAt.Time.Equal(now.Add(refreshTokenTTL)) {
		t.Fatalf("refresh expiry = %v, want %v", refreshClaims.ExpiresAt, now.Add(refreshTokenTTL))
	}
}

func TestParseRefreshTokenRejectsAccessToken(t *testing.T) {
	pair, err := IssueTokenPair("secret-32-characters-minimum-value", "user-1", "admin")
	if err != nil {
		t.Fatalf("IssueTokenPair() returned error: %v", err)
	}

	if _, err := ParseRefreshToken("secret-32-characters-minimum-value", pair.Token); err == nil {
		t.Fatal("ParseRefreshToken() should reject access token")
	}
}
