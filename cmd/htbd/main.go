package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/kazerlelutin/htb/internal/auth"
	"github.com/kazerlelutin/htb/internal/httpapi"
	"github.com/kazerlelutin/htb/internal/store"
)

var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	databaseURL := required("HTBD_DATABASE_URL")
	issuer, audience := required("HTBD_ZITADEL_ISSUER"), required("HTBD_ZITADEL_AUDIENCE")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	store := &store.Store{DB: db}
	if err := store.Migrate(ctx); err != nil {
		logger.Error("migrate database", "error", err)
		os.Exit(1)
	}
	verifier, err := auth.NewZitadelVerifier(ctx, issuer, audience, os.Getenv("HTBD_ZITADEL_SUPERADMIN_CLAIM"), os.Getenv("HTBD_ZITADEL_SUPERADMIN_ROLE"))
	if err != nil {
		logger.Error("initialize Zitadel verifier", "error", err)
		os.Exit(1)
	}
	deviceConfig := auth.DeviceConfig{Issuer: issuer, ClientID: os.Getenv("HTBD_ZITADEL_DEVICE_CLIENT_ID"), Audience: audience}
	server := httpapi.New(store, verifier, deviceConfig, os.Getenv("HTBD_RELEASE_URL"), logger)
	httpServer := &http.Server{Addr: valueOr("HTBD_LISTEN_ADDR", ":8080"), Handler: server.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		logger.Info("server started", "address", httpServer.Addr, "version", version)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdown); err != nil {
		logger.Error("graceful shutdown", "error", err)
	}
}

func required(key string) string {
	value := os.Getenv(key)
	if value == "" {
		slog.Error("required environment variable is missing", "name", key)
		os.Exit(2)
	}
	return value
}
func valueOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
