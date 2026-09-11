package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/minoplhy/nodem/internal/agent"
	"github.com/minoplhy/nodem/internal/version"
)

func main() {
	// 1. Load environment variables from .env.agent, .env, or system service configuration if present
	_ = godotenv.Load(".env.agent", ".env", "/etc/nodem-agent/agent.env", "/etc/ech_agent/agent.env")

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "-v", "--version":
			fmt.Printf("nodem-agent %s\n", version.Full())
			return
		case "cluster":
			if err := handleClusterCommand(os.Args[1:]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
	}

	var cfg agent.Config
	finalize := agent.BindFlags(flag.CommandLine, &cfg)
	flag.Parse()
	finalize()

	if strings.TrimSpace(cfg.ServerPublicKey) == "" {
		fmt.Fprintln(os.Stderr, "Error: missing required parameter: --server-public-key (or ECH_SERVER_PUBLIC_KEY)")
		os.Exit(1)
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(handler))

	slog.Info("Starting ECH Agent", "version", version.Full(), "transport", cfg.Transport, "proxy", cfg.ProxyType, "storage", cfg.StorageDir)

	// Ensure storage directory and master includes config exist immediately on startup
	if err := os.MkdirAll(cfg.StorageDir, 0755); err != nil {
		slog.Warn("Failed to create storage directory", "dir", cfg.StorageDir, "error", err)
	}
	if cfg.ProxyType == "nginx" {
		if err := agent.GenerateMasterIncludesConf(cfg.StorageDir); err != nil {
			slog.Warn("Failed initializing master includes config on startup", "error", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.Once {
		if err := agent.RunSyncCycle(ctx, cfg); err != nil {
			slog.Error("Sync cycle failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Single sync cycle completed successfully.")
		return
	}

	// Daemon mode: run periodic ticker
	if err := agent.RunSyncCycle(ctx, cfg); err != nil {
		slog.Warn("Initial sync cycle error", "error", err)
	}

	ticker := time.NewTicker(time.Duration(cfg.IntervalSecs) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := agent.RunSyncCycle(ctx, cfg); err != nil {
				slog.Error("Sync cycle error", "error", err)
			}
		case <-ctx.Done():
			slog.Info("ECH Agent exiting.")
			return
		}
	}
}
