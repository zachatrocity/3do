package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Addr          string
	AppURL        string
	DataDir       string
	DatabasePath  string
	UploadDir     string
	UploadMaxSize int64
}

func Load() Config {
	dataDir := env("DATA_DIR", "./data")
	uploadMaxMB := envInt("UPLOAD_MAX_MB", 512)
	return Config{
		Addr:          env("ADDR", ":8080"),
		AppURL:        env("APP_URL", "http://localhost:8080"),
		DataDir:       dataDir,
		DatabasePath:  env("DATABASE_PATH", filepath.Join(dataDir, "3do.db")),
		UploadDir:     filepath.Join(dataDir, "uploads"),
		UploadMaxSize: int64(uploadMaxMB) * 1024 * 1024,
	}
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
