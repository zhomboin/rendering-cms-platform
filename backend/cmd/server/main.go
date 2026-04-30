package main

import (
	"context"
	"log"
	"net/http"

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

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(
			httpapi.WithJWTSecret(cfg.JWTSecret),
			httpapi.WithLoginHandler(auth.NewLoginHandler(cfg.JWTSecret, userFinder)),
		),
	}

	log.Printf("starting server on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server stopped: %v", err)
	}
}
