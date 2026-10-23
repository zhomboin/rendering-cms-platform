package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"rendering-cms-platform/backend/internal/albums"
	"rendering-cms-platform/backend/internal/analytics"
	"rendering-cms-platform/backend/internal/articles"
	"rendering-cms-platform/backend/internal/assets"
	"rendering-cms-platform/backend/internal/auth"
	"rendering-cms-platform/backend/internal/comments"
	"rendering-cms-platform/backend/internal/config"
	"rendering-cms-platform/backend/internal/database"
	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
	"rendering-cms-platform/backend/internal/logging"
	"rendering-cms-platform/backend/internal/mail"
	"rendering-cms-platform/backend/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	logger, closeLogger := logging.NewDailyFileLogger(cfg.LogDir, cfg.LogRetentionDays)
	defer closeLogger.Close()

	db, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := auth.EnsureDevAdmin(context.Background(), db, auth.DevAdminConfig{
		Enabled:  cfg.AppEnv == "development" || cfg.DevBootstrapAdmin,
		Email:    cfg.DevAdminEmail,
		Name:     cfg.DevAdminName,
		Password: cfg.DevAdminPassword,
	}); err != nil {
		log.Fatalf("bootstrap dev admin: %v", err)
	}

	queries := dbgen.New(db)
	storageClient, err := storage.NewS3Client(cfg.S3)
	if err != nil {
		log.Fatalf("open object storage: %v", err)
	}
	userFinder := auth.NewDatabaseUserFinder(queries)

	// SMTP 未配置齐全时邮件验证码功能在 config 层已自动关闭；dev log 仅在 LOGIN_MFA_DEV_LOG 开启时注入。
	var emailSender auth.EmailSender
	if cfg.SMTP.Configured() {
		emailSender = mail.NewSMTPSender(mail.SMTPConfig{
			Host:     cfg.SMTP.Host,
			Port:     cfg.SMTP.Port,
			Username: cfg.SMTP.Username,
			Password: cfg.SMTP.Password,
			From:     cfg.SMTP.From,
		})
	}
	var codeLogger auth.CodeLogger
	if cfg.LoginMFADevLog {
		codeLogger = logger.Info
	}
	loginDeps := auth.LoginDeps{
		Secret:     cfg.JWTSecret,
		Finder:     userFinder,
		Attempts:   queries,
		Codes:      queries,
		Sender:     emailSender,
		DevLog:     codeLogger,
		MFAEnabled: cfg.LoginMFAEnabled,
	}

	articleHandler := articles.NewHandler(queries)
	analyticsHandler := analytics.NewHandler(queries)
	commentHandler := comments.NewHandler(queries)
	assetHandler := assets.NewHandlerWithOptions(queries, storageClient, assets.HandlerOptions{
		PublicBaseURL:   cfg.S3.PublicBaseURL,
		BlogImagePrefix: cfg.S3.BlogImagePrefix,
		AssetFilePrefix: cfg.S3.AssetFilePrefix,
	})
	// 相册照片为公开内容：走独立 photos 前缀并要求配置公共访问基地址。
	albumHandler := albums.NewHandlerWithOptions(queries, storageClient, albums.HandlerOptions{
		PublicBaseURL: cfg.S3.PublicBaseURL,
		PhotoPrefix:   cfg.S3.PhotoPrefix,
	})

	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	defer stopScheduler()
	analytics.StartDailyViewArchiveScheduler(schedulerCtx, queries, logger)
	articles.StartDailyArticleLogPruneScheduler(schedulerCtx, queries, cfg.ArticleLogRetention, logger)

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(
			httpapi.WithJWTSecret(cfg.JWTSecret),
			httpapi.WithFrontendOrigins(cfg.FrontendOrigins),
			httpapi.WithLogger(logger),
			httpapi.WithLoginHandler(auth.NewLoginHandler(loginDeps)),
			httpapi.WithMFAHandlers(auth.NewMFAVerifyHandler(loginDeps), auth.NewMFAResendHandler(loginDeps)),
			httpapi.WithRefreshHandler(auth.NewRefreshHandler(cfg.JWTSecret)),
			httpapi.WithPublicTrafficLimits(httpapi.PublicTrafficLimits{
				ReadRatePerSecond:   cfg.PublicReadRatePerSecond,
				ReadBurst:           cfg.PublicReadBurst,
				SearchRatePerSecond: cfg.PublicSearchRatePerSecond,
				SearchBurst:         cfg.PublicSearchBurst,
				MaxInFlight:         cfg.PublicMaxInFlight,
				MaxClients:          cfg.PublicRateLimitMaxClients,
				ClientTTL:           10 * time.Minute,
			}),
			httpapi.WithPublicArticleReadRoutes(articleHandler.RegisterPublicReadRoutes),
			httpapi.WithPublicArticleSearchRoutes(articleHandler.RegisterPublicSearchRoutes),
			httpapi.WithPublicPhotoReadRoutes(albumHandler.RegisterPublicReadRoutes),
			httpapi.WithPublicRoutes(analyticsHandler.RegisterPublicRoutes),
			httpapi.WithPublicRoutes(commentHandler.RegisterPublicRoutes),
			httpapi.WithAdminRoutes(articleHandler.RegisterAdminRoutes),
			httpapi.WithAdminRoutes(analyticsHandler.RegisterAdminRoutes),
			httpapi.WithAdminRoutes(commentHandler.RegisterAdminRoutes),
			httpapi.WithAdminRoutes(assetHandler.RegisterAdminRoutes),
			httpapi.WithAdminRoutes(albumHandler.RegisterAdminRoutes),
		),
		// 照片上传走 20MB 后端代理链路，ReadTimeout/WriteTimeout 需覆盖慢速
		// 上行的整个请求周期（与前端 120s 超时对齐）；slowloris 头部防护由
		// ReadHeaderTimeout 单独承担。
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	log.Printf("starting server on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server stopped: %v", err)
	}
}
