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
	if len(cfg.FrontendOrigins) != 1 || cfg.FrontendOrigins[0] != "http://127.0.0.1:5173" {
		t.Fatalf("FrontendOrigins = %#v, want default frontend origin list", cfg.FrontendOrigins)
	}
	if cfg.LogDir != "logs" {
		t.Fatalf("LogDir = %q, want logs", cfg.LogDir)
	}
	if cfg.AppEnv != "production" {
		t.Fatalf("AppEnv = %q, want production", cfg.AppEnv)
	}
	if cfg.DevBootstrapAdmin {
		t.Fatal("DevBootstrapAdmin = true, want false by default")
	}
	if cfg.PublicReadRatePerSecond != 20 || cfg.PublicReadBurst != 40 || cfg.PublicSearchRatePerSecond != 5 || cfg.PublicSearchBurst != 10 {
		t.Fatalf("unexpected public rate defaults: %#v", cfg)
	}
	if cfg.PublicMaxInFlight != 128 || cfg.PublicRateLimitMaxClients != 10000 {
		t.Fatalf("unexpected public capacity defaults: %#v", cfg)
	}
}

func TestLoadReadsPublicRateLimitSettings(t *testing.T) {
	t.Setenv("JWT_SECRET", "replace-with-32-plus-character-secret")
	t.Setenv("PUBLIC_READ_RATE_PER_SECOND", "12.5")
	t.Setenv("PUBLIC_READ_BURST", "25")
	t.Setenv("PUBLIC_SEARCH_RATE_PER_SECOND", "3")
	t.Setenv("PUBLIC_SEARCH_BURST", "6")
	t.Setenv("PUBLIC_MAX_IN_FLIGHT", "64")
	t.Setenv("PUBLIC_RATE_LIMIT_MAX_CLIENTS", "5000")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PublicReadRatePerSecond != 12.5 || cfg.PublicReadBurst != 25 || cfg.PublicSearchRatePerSecond != 3 || cfg.PublicSearchBurst != 6 {
		t.Fatalf("unexpected public rate config: %#v", cfg)
	}
	if cfg.PublicMaxInFlight != 64 || cfg.PublicRateLimitMaxClients != 5000 {
		t.Fatalf("unexpected public capacity config: %#v", cfg)
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
	t.Setenv("APP_ENV", "development")
	t.Setenv("DEV_BOOTSTRAP_ADMIN", "true")
	t.Setenv("DEV_ADMIN_EMAIL", "admin@rendering.me")
	t.Setenv("DEV_ADMIN_NAME", "Dev Admin")
	t.Setenv("DEV_ADMIN_PASSWORD", "rendering_dev_password")
	t.Setenv("S3_ENDPOINT", "http://127.0.0.1:9000")
	t.Setenv("S3_REGION", "us-east-1")
	t.Setenv("S3_BUCKET", "rendering-assets")
	t.Setenv("S3_ACCESS_KEY_ID", "rendering")
	t.Setenv("S3_SECRET_ACCESS_KEY", "rendering_dev_password")
	t.Setenv("S3_USE_PATH_STYLE", "true")
	t.Setenv("S3_PUBLIC_BASE_URL", "https://assets.rendering.me")
	t.Setenv("S3_BLOG_IMAGE_PREFIX", "blog-images")
	t.Setenv("S3_ASSET_FILE_PREFIX", "files")

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
	if !cfg.S3.UsePathStyle {
		t.Fatal("S3.UsePathStyle = false, want true")
	}
	if cfg.S3.PublicBaseURL != "https://assets.rendering.me" {
		t.Fatalf("S3.PublicBaseURL = %q, want configured public base URL", cfg.S3.PublicBaseURL)
	}
	if cfg.S3.BlogImagePrefix != "blog-images" {
		t.Fatalf("S3.BlogImagePrefix = %q, want blog-images", cfg.S3.BlogImagePrefix)
	}
	if cfg.S3.AssetFilePrefix != "files" {
		t.Fatalf("S3.AssetFilePrefix = %q, want files", cfg.S3.AssetFilePrefix)
	}
	if cfg.LogDir != "/var/log/rendering-cms-platform" {
		t.Fatalf("LogDir = %q, want configured log dir", cfg.LogDir)
	}
	if cfg.AppEnv != "development" {
		t.Fatalf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if !cfg.DevBootstrapAdmin {
		t.Fatal("DevBootstrapAdmin = false, want true")
	}
	if cfg.DevAdminEmail != "admin@rendering.me" {
		t.Fatalf("DevAdminEmail = %q, want admin@rendering.me", cfg.DevAdminEmail)
	}
	if cfg.DevAdminName != "Dev Admin" {
		t.Fatalf("DevAdminName = %q, want Dev Admin", cfg.DevAdminName)
	}
	if cfg.DevAdminPassword != "rendering_dev_password" {
		t.Fatalf("DevAdminPassword = %q, want configured dev password", cfg.DevAdminPassword)
	}
}

func TestLoadUsesDefaultS3Prefixes(t *testing.T) {
	t.Setenv("JWT_SECRET", "replace-with-32-plus-character-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.S3.BlogImagePrefix != "blog" {
		t.Fatalf("S3.BlogImagePrefix = %q, want blog", cfg.S3.BlogImagePrefix)
	}
	if cfg.S3.AssetFilePrefix != "assets" {
		t.Fatalf("S3.AssetFilePrefix = %q, want assets", cfg.S3.AssetFilePrefix)
	}
}

func TestLoadDefaultsS3PathStyleToFalse(t *testing.T) {
	t.Setenv("JWT_SECRET", "replace-with-32-plus-character-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.S3.UsePathStyle {
		t.Fatal("S3.UsePathStyle = true, want false")
	}
}

func TestLoadReadsMultipleFrontendOrigins(t *testing.T) {
	t.Setenv("JWT_SECRET", "replace-with-32-plus-character-secret")
	t.Setenv("FRONTEND_ORIGINS", "http://127.0.0.1:3000, http://127.0.0.1:5173")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	want := []string{"http://127.0.0.1:3000", "http://127.0.0.1:5173"}
	if len(cfg.FrontendOrigins) != len(want) {
		t.Fatalf("FrontendOrigins = %#v, want %#v", cfg.FrontendOrigins, want)
	}
	for i := range want {
		if cfg.FrontendOrigins[i] != want[i] {
			t.Fatalf("FrontendOrigins = %#v, want %#v", cfg.FrontendOrigins, want)
		}
	}
}
