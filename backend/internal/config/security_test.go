package config

import "testing"

func TestMFARejectsUnsafeConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name        string
		environment string
		enabled     string
		devLog      string
		smtp        bool
		wantError   bool
	}{
		{"production logging without SMTP", "production", "true", "true", false, true},
		{"production logging with SMTP", "production", "true", "true", true, true},
		{"production enabled without delivery", "production", "true", "false", false, true},
		{"development enabled without delivery", "development", "true", "false", false, true},
		{"development code logging", "development", "true", "true", false, false},
		{"production SMTP", "production", "true", "false", true, false},
		{"disabled without SMTP", "production", "false", "false", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "synthetic-configuration-test-secret")
			t.Setenv("APP_ENV", tc.environment)
			t.Setenv("LOGIN_MFA_ENABLED", tc.enabled)
			t.Setenv("LOGIN_MFA_DEV_LOG", tc.devLog)
			t.Setenv("SMTP_HOST", "")
			t.Setenv("SMTP_USERNAME", "")
			t.Setenv("SMTP_PASSWORD", "")
			if tc.smtp {
				t.Setenv("SMTP_HOST", "smtp.example.invalid")
				t.Setenv("SMTP_USERNAME", "sender@example.invalid")
				t.Setenv("SMTP_PASSWORD", "synthetic")
			}
			_, err := Load()
			if (err != nil) != tc.wantError {
				t.Fatalf("Load error = %v, want error=%v", err, tc.wantError)
			}
		})
	}
}

func TestDefaultAdminBootstrapOnlyRunsInDevelopment(t *testing.T) {
	for _, environment := range []string{"production", "staging", "development"} {
		t.Run(environment, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "synthetic-configuration-test-secret")
			t.Setenv("APP_ENV", environment)
			t.Setenv("DEV_BOOTSTRAP_ADMIN", "true")
			_, err := Load()
			if (err != nil) != (environment != "development") {
				t.Fatalf("environment=%s error=%v", environment, err)
			}
		})
	}
}
