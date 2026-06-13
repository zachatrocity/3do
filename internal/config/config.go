package config

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port          string
	ListenAddr    string
	AppURL        string
	DataDir       string
	DatabasePath  string
	UploadDir     string
	UploadMaxSize int64
	SessionSecret string
}

func Load() Config {
	dataDir := env("DATA_DIR", "./data")
	uploadMaxMB := envInt("UPLOAD_MAX_MB", 512)
	port := listenPort()
	return Config{
		Port:          port,
		ListenAddr:    ":" + port,
		AppURL:        env("APP_URL", "http://localhost:8080"),
		DataDir:       dataDir,
		DatabasePath:  env("DATABASE_PATH", filepath.Join(dataDir, "3do.db")),
		UploadDir:     filepath.Join(dataDir, "uploads"),
		UploadMaxSize: int64(uploadMaxMB) * 1024 * 1024,
		SessionSecret: strings.TrimSpace(os.Getenv("SESSION_SECRET")),
	}
}

func listenPort() string {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port != "" {
		return strings.TrimPrefix(port, ":")
	}

	addr := strings.TrimSpace(os.Getenv("ADDR"))
	if addr == "" {
		return "8080"
	}
	if _, addrPort, err := net.SplitHostPort(addr); err == nil {
		return addrPort
	}
	return strings.TrimPrefix(addr, ":")
}

func (c Config) ValidateForServe() error {
	if c.SessionSecret == "" {
		return errors.New("SESSION_SECRET is required before starting 3do")
	}
	if c.SessionSecret == "change-me" {
		return errors.New("SESSION_SECRET must not use the .env.example placeholder")
	}
	if len(c.SessionSecret) < 32 {
		return errors.New("SESSION_SECRET must be at least 32 characters")
	}
	return nil
}

func (c Config) EnsureDirs() error {
	if err := os.MkdirAll(c.DataDir, 0o755); err != nil {
		return err
	}
	return os.MkdirAll(c.UploadDir, 0o755)
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
