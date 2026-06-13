package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/zachatrocity/3do/internal/app"
	"github.com/zachatrocity/3do/internal/config"
	"github.com/zachatrocity/3do/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		cfg := config.Load()
		client := http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get("http://127.0.0.1:" + cfg.Port + "/healthz")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(os.Stderr, "unexpected health status: %s\n", resp.Status)
			os.Exit(1)
		}
		return
	}

	cfg := config.Load()
	if err := cfg.ValidateForServe(); err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}
	if err := cfg.EnsureDirs(); err != nil {
		slog.Error("failed to prepare data directories", "err", err)
		os.Exit(1)
	}

	db, err := store.Open(cfg.DatabasePath)
	if err != nil {
		slog.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		slog.Error("failed to migrate database", "err", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.NewServer(cfg, db),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("starting 3do", "port", cfg.Port, "data_dir", cfg.DataDir, "database", cfg.DatabasePath)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
