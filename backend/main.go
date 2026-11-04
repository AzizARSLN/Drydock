package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AzizARSLN/drydock/internal/api"
	"github.com/AzizARSLN/drydock/internal/app"
	"github.com/AzizARSLN/drydock/internal/audit"
	"github.com/AzizARSLN/drydock/internal/auth"
	"github.com/AzizARSLN/drydock/internal/config"
	"github.com/AzizARSLN/drydock/internal/crypto"
	"github.com/AzizARSLN/drydock/internal/store"
)

func main() {
	config.LoadDotEnv(".env", "backend/.env")

	cfg := config.Load()

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer st.Close()

	cipher, err := crypto.New(cfg.MasterKey)
	if err != nil {
		log.Fatalf("init cipher: %v", err)
	}

	authSvc := auth.NewService(cfg.JWTSecret, cfg.TokenTTLHours)
	application := app.New(cfg, st, authSvc, cipher)
	auditLog := audit.New(st)

	webDir := cfg.WebDir
	if info, err := os.Stat(webDir); err != nil || !info.IsDir() {
		log.Printf("web assets not found at %q — API only mode", webDir)
		webDir = ""
	}

	srv := api.NewServer(application, auditLog)
	handler := srv.Routes(webDir)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout is intentionally unset: the terminal WebSocket is long lived.
	}

	go func() {
		log.Printf("Drydock %s listening on http://%s (web assets: %q)", api.Version, cfg.Addr, webDir)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down…")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
