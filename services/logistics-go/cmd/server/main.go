package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/config"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/httpapi"
	"github.com/yguerra/commerce-logistics-platform/services/logistics-go/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("connect to PostgreSQL: %v", err)
	}
	if err := storage.ApplyMigrations(ctx, db); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewHandler(storage.NewPostgresRepository(db)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("logistics service listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
