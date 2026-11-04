package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// Config holds every runtime setting of the Drydock control plane.
// Values are resolved from environment variables (optionally preloaded from a
// local .env file) so that secrets never live inside the repository.
type Config struct {
	Addr          string
	DataDir       string
	DBPath        string
	WebDir        string
	MasterKey     string
	JWTSecret     string
	TokenTTLHours int
	CORSOrigins   []string
	LogLevel      string
	// FeatureToggles keeps optional capabilities switchable at runtime.
	EnableTerminal bool
	EnableSFTP     bool
	HealthInterval int // seconds between background health sweeps
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return def
}

// Load builds a Config from the environment and applies safe defaults.
func Load() *Config {
	dataDir := env("DRYDOCK_DATA_DIR", "./data")
	c := &Config{
		Addr:           env("DRYDOCK_ADDR", "127.0.0.1:8788"),
		DataDir:        dataDir,
		DBPath:         env("DRYDOCK_DB", dataDir+"/drydock.db"),
		WebDir:         env("DRYDOCK_WEB_DIR", "./web"),
		MasterKey:      env("DRYDOCK_MASTER_KEY", ""),
		JWTSecret:      env("DRYDOCK_JWT_SECRET", ""),
		TokenTTLHours:  envInt("DRYDOCK_TOKEN_TTL_HOURS", 12),
		CORSOrigins:    strings.Split(env("DRYDOCK_CORS_ORIGINS", "http://127.0.0.1:5173,http://localhost:5173"), ","),
		LogLevel:       env("DRYDOCK_LOG_LEVEL", "info"),
		EnableTerminal: envBool("DRYDOCK_ENABLE_TERMINAL", true),
		EnableSFTP:     envBool("DRYDOCK_ENABLE_SFTP", true),
		HealthInterval: envInt("DRYDOCK_HEALTH_INTERVAL", 60),
	}

	if c.MasterKey == "" {
		log.Fatal("DRYDOCK_MASTER_KEY is required — refusing to start without an encryption key")
	}
	if len(c.MasterKey) < 32 {
		log.Fatal("DRYDOCK_MASTER_KEY must be at least 32 characters long")
	}
	if c.JWTSecret == "" {
		log.Fatal("DRYDOCK_JWT_SECRET is required — refusing to start without a signing secret")
	}
	if err := os.MkdirAll(c.DataDir, 0o750); err != nil {
		log.Fatalf("cannot create data directory %q: %v", c.DataDir, err)
	}
	return c
}
