package main

import (
	"context"
	"log"
	"net/http"

	"rendering-cms-platform/backend/internal/analytics"
	"rendering-cms-platform/backend/internal/articles"
	"rendering-cms-platform/backend/internal/assets"
	"rendering-cms-platform/backend/internal/auth"
	"rendering-cms-platform/backend/internal/comments"
	"rendering-cms-platform/backend/internal/config"
	"rendering-cms-platform/backend/internal/database"
	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
	"rendering-cms-platform/backend/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	queries := dbgen.New(db)
	storageClient, err := storage.NewS3Client(cfg.S3)
	if err != nil {
		log.Fatalf("open object storage: %v", err)
	}
	userFinder := auth.NewDatabaseUserFinder(queries)
	articleHandler := articles.NewHandler(queries)
	analyticsHandler := analytics.NewHandler(queries)
	commentHandler := comments.NewHandler(queries)
	assetHandler := assets.NewHandler(queries, storageClient)

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(
			httpapi.WithJWTSecret(cfg.JWTSecret),
			httpapi.WithFrontendOrigin(cfg.FrontendOrigin),
			httpapi.WithLoginHandler(auth.NewLoginHandler(cfg.JWTSecret, userFinder)),
			httpapi.WithPublicRoutes(articleHandler.RegisterPublicRoutes),
			httpapi.WithPublicRoutes(analyticsHandler.RegisterPublicRoutes),
			httpapi.WithPublicRoutes(commentHandler.RegisterPublicRoutes),
			httpapi.WithAdminRoutes(articleHandler.RegisterAdminRoutes),
			httpapi.WithAdminRoutes(analyticsHandler.RegisterAdminRoutes),
			httpapi.WithAdminRoutes(commentHandler.RegisterAdminRoutes),
			httpapi.WithAdminRoutes(assetHandler.RegisterAdminRoutes),
		),
	}

	log.Printf("starting server on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}
}
