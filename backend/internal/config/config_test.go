package config

import "testing"

func TestLoadUsesDefaultsForOptionalRuntimeValues(t *testing.T) {
	t.Setenv("JWT_SECRET", "replace-with-32-plus-character-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
	if cfg.FrontendOrigin != "http://127.0.0.1:5173" {
		t.Fatalf("FrontendOrigin = %q, want default frontend origin", cfg.FrontendOrigin)
	}
	if cfg.LogDir != "logs" {
		t.Fatalf("LogDir = %q, want logs", cfg.LogDir)
	}
}

func TestLoadRequiresJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() returned nil error, want JWT secret validation error")
	}
}

func TestLoadReadsDatabaseAndS3Settings(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DATABASE_URL", "postgres://rendering:secret@127.0.0.1:5432/rendering_cms?sslmode=disable")
	t.Setenv("JWT_SECRET", "replace-with-32-plus-character-secret")
	t.Setenv("FRONTEND_ORIGIN", "http://localhost:5173")
	t.Setenv("LOG_DIR", "/var/log/rendering-cms-platform")
	t.Setenv("S3_ENDPOINT", "http://127.0.0.1:9000")
	t.Setenv("S3_REGION", "us-east-1")
	t.Setenv("S3_BUCKET", "rendering-assets")
	t.Setenv("S3_ACCESS_KEY_ID", "rendering")
	t.Setenv("S3_SECRET_ACCESS_KEY", "rendering_dev_password")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("DatabaseURL is empty")
	}
	if cfg.S3.Bucket != "rendering-assets" {
		t.Fatalf("S3.Bucket = %q, want rendering-assets", cfg.S3.Bucket)
	}
	if cfg.LogDir != "/var/log/rendering-cms-platform" {
		t.Fatalf("LogDir = %q, want configured log dir", cfg.LogDir)
	}
}
