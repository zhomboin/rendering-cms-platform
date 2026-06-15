package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	JWTSecret       string
	FrontendOrigin  string
	FrontendOrigins []string
	LogDir          string
	S3              S3Config
}

type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:       envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		FrontendOrigin: envOrDefault("FRONTEND_ORIGIN", "http://127.0.0.1:5173"),
		FrontendOrigins: parseCSVEnvOrFallback(
			"FRONTEND_ORIGINS",
			envOrDefault("FRONTEND_ORIGIN", "http://127.0.0.1:5173"),
		),
		LogDir: envOrDefault("LOG_DIR", "logs"),
		S3: S3Config{
			// S3_* 同时兼容本地 MinIO 和生产 Cloudflare R2，生产环境由 deploy/production.env 提供。
			Endpoint:        os.Getenv("S3_ENDPOINT"),
			Region:          envOrDefault("S3_REGION", "us-east-1"),
			Bucket:          os.Getenv("S3_BUCKET"),
			AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
			// R2 使用虚拟主机风格寻址，应设为 false；本地 MinIO 使用路径风格，应设为 true。
			UsePathStyle: parseBoolEnv("S3_USE_PATH_STYLE"),
		},
	}

	if cfg.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}

	return cfg, nil
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func parseBoolEnv(key string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func parseCSVEnvOrFallback(key string, fallback string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		raw = fallback
	}

	values := strings.Split(raw, ",")
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
