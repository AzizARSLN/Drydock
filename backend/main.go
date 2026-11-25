package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AzizARSLN/drydock/internal/api"
	"github.com/AzizARSLN/drydock/internal/app"
	"github.com/AzizARSLN/drydock/internal/audit"
	"github.com/AzizARSLN/drydock/internal/auth"
	"github.com/AzizARSLN/drydock/internal/config"
	"github.com/AzizARSLN/drydock/internal/crypto"
	"github.com/AzizARSLN/drydock/internal/store"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		os.Exit(resetPassword(os.Args[2:]))
	}

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

// resetPassword implements `drydock reset-password <kullanıcı-adı>` for the
// "I lost the administrator password" case. It only rewrites a single bcrypt
// hash and never touches the rest of the database, so it is safe to run on a
// live install while the server is stopped.
func resetPassword(args []string) int {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(os.Stderr, "kullanım: drydock reset-password <kullanıcı-adı>")
		return 2
	}
	username := strings.TrimSpace(args[0])

	plain, err := readNewPassword()
	if err != nil {
		fmt.Fprintln(os.Stderr, "şifre okunamadı:", err)
		return 1
	}
	if len(plain) < 8 {
		fmt.Fprintln(os.Stderr, "şifre en az 8 karakter olmalı")
		return 1
	}

	config.LoadDotEnv(".env", "backend/.env")
	st, err := store.Open(config.Load().DBPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "veritabanı açılamadı:", err)
		return 1
	}
	defer st.Close()

	user, err := st.GetUserByUsername(username)
	if err != nil || user == nil {
		fmt.Fprintf(os.Stderr, "%q adlı kullanıcı bulunamadı\n", username)
		return 1
	}
	hash, err := auth.HashPassword(plain)
	if err != nil {
		fmt.Fprintln(os.Stderr, "şifre hash'lenemedi:", err)
		return 1
	}
	if err := st.UpdatePassword(user.ID, hash); err != nil {
		fmt.Fprintln(os.Stderr, "şifre güncellenemedi:", err)
		return 1
	}
	fmt.Printf("%q kullanıcısının şifresi güncellendi.\n", user.Username)
	return 0
}

// readNewPassword takes the new password from DRYDOCK_NEW_PASSWORD (scripted
// use) or prompts for it twice on the terminal without echoing.
func readNewPassword() (string, error) {
	if v := os.Getenv("DRYDOCK_NEW_PASSWORD"); v != "" {
		return v, nil
	}
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, "Yeni şifre: ")
		first, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		fmt.Fprint(os.Stderr, "Yeni şifre (tekrar): ")
		again, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", err
		}
		if string(first) != string(again) {
			return "", errors.New("şifreler eşleşmiyor")
		}
		return string(first), nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
