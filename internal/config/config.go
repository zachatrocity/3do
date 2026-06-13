package config

import (
	"errors"
	"fmt"
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
	ThumbnailDir  string
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
		ThumbnailDir:  filepath.Join(dataDir, "thumbnails"),
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
	for _, dir := range uniqueDirs(c.DataDir, c.UploadDir, c.ThumbnailDir, filepath.Dir(c.DatabasePath)) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (c Config) CheckWritable() error {
	for name, dir := range map[string]string{
		"data directory":      c.DataDir,
		"upload directory":    c.UploadDir,
		"thumbnail directory": c.ThumbnailDir,
		"database directory":  filepath.Dir(c.DatabasePath),
	} {
		if dir == "" || dir == "." {
			continue
		}
		if err := checkDirWritable(dir); err != nil {
			return fmt.Errorf("%s %q is not writable: %w", name, dir, err)
		}
	}
	return nil
}

func uniqueDirs(dirs ...string) []string {
	seen := make(map[string]bool, len(dirs))
	var unique []string
	for _, dir := range dirs {
		if dir == "" || dir == "." || seen[dir] {
			continue
		}
		seen[dir] = true
		unique = append(unique, dir)
	}
	return unique
}

func checkDirWritable(dir string) error {
	file, err := os.CreateTemp(dir, ".3do-write-test-*")
	if err != nil {
		return err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
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
