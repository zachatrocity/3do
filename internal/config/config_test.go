package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUsesPort(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("ADDR", ":8081")

	cfg := Load()
	if cfg.Port != "9090" {
		t.Fatalf("expected port 9090, got %q", cfg.Port)
	}
	if cfg.ListenAddr != ":9090" {
		t.Fatalf("expected listen addr :9090, got %q", cfg.ListenAddr)
	}
}

func TestLoadFallsBackToAddrForCompatibility(t *testing.T) {
	for _, addr := range []string{":8081", "0.0.0.0:8081"} {
		t.Setenv("ADDR", addr)

		cfg := Load()
		if cfg.Port != "8081" {
			t.Fatalf("expected port 8081 for addr %q, got %q", addr, cfg.Port)
		}
		if cfg.ListenAddr != ":8081" {
			t.Fatalf("expected listen addr :8081 for addr %q, got %q", addr, cfg.ListenAddr)
		}
	}
}

func TestEnsureDirsCreatesDatabaseParent(t *testing.T) {
	root := t.TempDir()
	cfg := Config{
		DataDir:      filepath.Join(root, "data"),
		UploadDir:    filepath.Join(root, "data", "uploads"),
		ThumbnailDir: filepath.Join(root, "data", "thumbnails"),
		DatabasePath: filepath.Join(root, "data", "db", "3do.db"),
	}

	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "uploads")); err != nil {
		t.Fatalf("expected uploads directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "thumbnails")); err != nil {
		t.Fatalf("expected thumbnails directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "data", "db")); err != nil {
		t.Fatalf("expected database parent directory: %v", err)
	}
	if err := cfg.CheckWritable(); err != nil {
		t.Fatalf("expected prepared directories to be writable: %v", err)
	}
}

func TestValidateForServeRequiresStrongSessionSecret(t *testing.T) {
	for _, secret := range []string{"", "change-me", "too-short"} {
		cfg := Config{SessionSecret: secret}
		if err := cfg.ValidateForServe(); err == nil {
			t.Fatalf("expected %q to be rejected", secret)
		}
	}

	cfg := Config{SessionSecret: "0123456789abcdef0123456789abcdef"}
	if err := cfg.ValidateForServe(); err != nil {
		t.Fatal(err)
	}
}
