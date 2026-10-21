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
	// LogRetentionDays 按日日志文件保留天数，0 或非法值时使用后端默认 30。
	LogRetentionDays int
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
	// ArticleLogRetention 每篇文章保留的修订快照数量（不含首版），0 或非法值时使用后端默认 20。
	ArticleLogRetention int32
	S3                  S3Config
	SMTP                SMTPConfig
	// LoginMFAEnabled 未显式设置时由 SMTP 是否配置齐全决定；关闭后登录不触发邮箱验证码。
	LoginMFAEnabled bool
	// LoginMFADevLog 开发调试开关，开启后验证码写入后端日志而不实际发信。
	LoginMFADevLog bool
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

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	// From 为空时使用 Username 作为发件人。
	From string
}

// Configured 判断 SMTP 是否配置齐全，未配置齐全时邮件验证码功能自动关闭。
func (config SMTPConfig) Configured() bool {
	return config.Host != "" && config.Username != "" && config.Password != ""
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
		LogRetentionDays:          positiveIntEnv("LOG_RETENTION_DAYS", 30),
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
		ArticleLogRetention:       int32(positiveIntEnv("ARTICLE_LOG_RETENTION_LIMIT", 20)),
		SMTP: SMTPConfig{
			// SMTP_* 用于登录邮箱验证码发信，支持个人邮箱授权码或企业 SMTP；未配置齐全时邮件验证码自动关闭。
			Host:     strings.TrimSpace(os.Getenv("SMTP_HOST")),
			Port:     positiveIntEnv("SMTP_PORT", 465),
			Username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     strings.TrimSpace(os.Getenv("SMTP_FROM")),
		},
		LoginMFADevLog: parseBoolEnv("LOGIN_MFA_DEV_LOG"),
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

	// LOGIN_MFA_ENABLED 未设置时跟随 SMTP 配置：SMTP 配置齐全即启用，便于本地无 SMTP 环境自动关闭。
	if os.Getenv("LOGIN_MFA_ENABLED") != "" {
		cfg.LoginMFAEnabled = parseBoolEnv("LOGIN_MFA_ENABLED")
	} else {
		cfg.LoginMFAEnabled = cfg.SMTP.Configured()
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
