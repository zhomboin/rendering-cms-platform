package config

import (
	"errors"
	"math"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	HTTPAddr                  string
	DatabaseURL               string
	JWTSecret                 string
	FrontendOrigin            string
	FrontendOrigins           []string
	LogDir                    string
	AppEnv                    string
	DevBootstrapAdmin         bool
	DevAdminEmail             string
	DevAdminName              string
	DevAdminPassword          string
	PublicReadRatePerSecond   float64
	PublicReadBurst           int
	PublicSearchRatePerSecond float64
	PublicSearchBurst         int
	PublicMaxInFlight         int
	PublicRateLimitMaxClients int
	S3                        S3Config
}

type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
	PublicBaseURL   string
	BlogImagePrefix string
	AssetFilePrefix string
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
		LogDir:                    envOrDefault("LOG_DIR", "logs"),
		AppEnv:                    envOrDefault("APP_ENV", "production"),
		DevBootstrapAdmin:         parseBoolEnv("DEV_BOOTSTRAP_ADMIN"),
		DevAdminEmail:             envOrDefault("DEV_ADMIN_EMAIL", "admin@rendering.me"),
		DevAdminName:              envOrDefault("DEV_ADMIN_NAME", "Dev Admin"),
		DevAdminPassword:          os.Getenv("DEV_ADMIN_PASSWORD"),
		PublicReadRatePerSecond:   positiveFloatEnv("PUBLIC_READ_RATE_PER_SECOND", 20),
		PublicReadBurst:           positiveIntEnv("PUBLIC_READ_BURST", 40),
		PublicSearchRatePerSecond: positiveFloatEnv("PUBLIC_SEARCH_RATE_PER_SECOND", 5),
		PublicSearchBurst:         positiveIntEnv("PUBLIC_SEARCH_BURST", 10),
		PublicMaxInFlight:         positiveIntEnv("PUBLIC_MAX_IN_FLIGHT", 128),
		PublicRateLimitMaxClients: positiveIntEnv("PUBLIC_RATE_LIMIT_MAX_CLIENTS", 10000),
		S3: S3Config{
			// S3_* 同时兼容本地 MinIO 和生产 Cloudflare R2，生产环境由 deploy/production.env 提供。
			Endpoint:        os.Getenv("S3_ENDPOINT"),
			Region:          envOrDefault("S3_REGION", "us-east-1"),
			Bucket:          os.Getenv("S3_BUCKET"),
			AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
			// R2 使用虚拟主机风格寻址，应设为 false；本地 MinIO 使用路径风格，应设为 true。
			UsePathStyle:    parseBoolEnv("S3_USE_PATH_STYLE"),
			PublicBaseURL:   strings.TrimRight(os.Getenv("S3_PUBLIC_BASE_URL"), "/"),
			BlogImagePrefix: cleanPrefixEnv("S3_BLOG_IMAGE_PREFIX", "blog"),
			AssetFilePrefix: cleanPrefixEnv("S3_ASSET_FILE_PREFIX", "assets"),
		},
	}

	if cfg.JWTSecret == "" {
		return Config{}, errors.New("JWT_SECRET is required")
	}

	return cfg, nil
}

func positiveIntEnv(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func positiveFloatEnv(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(key)), 64)
	if err != nil || value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return fallback
	}
	return value
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

func cleanPrefixEnv(key string, fallback string) string {
	value := strings.Trim(strings.TrimSpace(os.Getenv(key)), "/")
	if value == "" || value == "." {
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
