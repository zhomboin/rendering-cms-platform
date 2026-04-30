package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("HashPassword() returned error: %v", err)
	}

	if !VerifyPassword(hash, "correct-password") {
		t.Fatal("expected correct password to verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("expected wrong password to fail")
	}
}
