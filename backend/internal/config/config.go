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
			Endpoint:        os.Getenv("S3_ENDPOINT"),
			Region:          envOrDefault("S3_REGION", "us-east-1"),
			Bucket:          os.Getenv("S3_BUCKET"),
			AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
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
