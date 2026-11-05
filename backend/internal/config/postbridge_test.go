package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPostBridgeOptionalAndReadsSecretFile(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only")
	t.Setenv("POSTBRIDGE_ORIGIN", "")
	cfg, err := Load()
	if err != nil || cfg.PostBridge.Origin != "" {
		t.Fatalf("disabled PostBridge: %v", err)
	}

	file := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(file, []byte("synthetic-test-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("POSTBRIDGE_ORIGIN", "https://bridge.example")
	t.Setenv("POSTBRIDGE_ENVIRONMENT", "test")
	t.Setenv("POSTBRIDGE_SERVICE_ID", "bridge-test")
	t.Setenv("POSTBRIDGE_SOURCE_ID", "cms-test")
	t.Setenv("POSTBRIDGE_ACCOUNTS", " account-test , ")
	t.Setenv("POSTBRIDGE_SCOPES", "targets:read, preparations:write")
	t.Setenv("POSTBRIDGE_CLIENT_ID", "client-test")
	t.Setenv("POSTBRIDGE_KEY_ID", "key-test")
	t.Setenv("POSTBRIDGE_CLIENT_SECRET_FILE", file)
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	bridge := cfg.PostBridge
	if bridge.Origin != "https://bridge.example" || bridge.Environment != "test" || bridge.ServiceID != "bridge-test" ||
		bridge.SourceID != "cms-test" || len(bridge.Accounts) != 1 || bridge.Accounts[0] != "account-test" ||
		len(bridge.Scopes) != 2 || bridge.Scopes[1] != "preparations:write" ||
		bridge.ClientID != "client-test" || bridge.KeyID != "key-test" || bridge.Secret != "synthetic-test-secret" {
		t.Fatal("unexpected PostBridge configuration")
	}
}

func TestLoadPostBridgeDoesNotExposeSecretFilePath(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only")
	t.Setenv("POSTBRIDGE_ORIGIN", "https://bridge.example")
	file := filepath.Join(t.TempDir(), "missing-secret")
	t.Setenv("POSTBRIDGE_CLIENT_SECRET_FILE", file)
	_, err := Load()
	if err == nil || strings.Contains(err.Error(), file) {
		t.Fatalf("expected a redacted file error, got %v", err)
	}
}
