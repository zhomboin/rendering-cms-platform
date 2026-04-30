package auth

import "testing"

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
