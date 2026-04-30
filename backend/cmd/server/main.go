package main

import (
	"context"
	"log"
	"net/http"

	"rendering-cms-platform/backend/internal/articles"
	"rendering-cms-platform/backend/internal/auth"
	"rendering-cms-platform/backend/internal/config"
	"rendering-cms-platform/backend/internal/database"
	"rendering-cms-platform/backend/internal/database/dbgen"
	httpapi "rendering-cms-platform/backend/internal/http"
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
	userFinder := auth.NewDatabaseUserFinder(queries)
	articleHandler := articles.NewHandler(queries)

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(
			httpapi.WithJWTSecret(cfg.JWTSecret),
			httpapi.WithLoginHandler(auth.NewLoginHandler(cfg.JWTSecret, userFinder)),
			httpapi.WithPublicRoutes(articleHandler.RegisterPublicRoutes),
			httpapi.WithAdminRoutes(articleHandler.RegisterAdminRoutes),
		),
	}

	log.Printf("starting server on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}
}
