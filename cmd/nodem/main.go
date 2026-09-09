package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"github.com/minoplhy/nodem/internal/api"
	"github.com/minoplhy/nodem/internal/cli"
	"github.com/minoplhy/nodem/internal/daemon"
	"github.com/minoplhy/nodem/internal/db"
	"github.com/minoplhy/nodem/internal/db/sqlite"
	"github.com/minoplhy/nodem/internal/misc"
	"github.com/minoplhy/nodem/internal/version"
)

func main() {
	// 1. Load environment variables from .env
	_ = godotenv.Load()

	// 2. Configure structured logging level based on DEBUG env
	debugMode := strings.EqualFold(strings.TrimSpace(os.Getenv("DEBUG")), "true")
	logLevel := slog.LevelInfo
	if debugMode {
		logLevel = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	slog.SetDefault(slog.New(handler))

	var repo *sqlite.SqliteRepository

	getRepo := func() (db.Repository, error) {
		if repo != nil {
			return repo, nil
		}
		var err error
		repo, err = sqlite.New(cli.GetDBPath())
		if err != nil {
			return nil, err
		}
		if err := repo.InitDB(context.Background()); err != nil {
			_ = repo.Close()
			return nil, err
		}
		return repo, nil
	}

	runDaemon := func(port uint16, sshPort uint16) error {
		r, err := getRepo()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		bootstrapToken := misc.GenerateBootstrapToken()
		userExists, err := r.UserExists(ctx)
		if err != nil {
			return fmt.Errorf("failed to check existing users: %w", err)
		}

		basePathRaw := strings.Trim(os.Getenv("BASE_PATH"), "/")
		basePath := ""
		if basePathRaw != "" {
			basePath = "/" + basePathRaw
		}

		if !userExists {
			cyan := color.New(color.FgCyan).SprintFunc()
			cyanBold := color.New(color.FgCyan, color.Bold).SprintFunc()
			yellowBold := color.New(color.FgYellow, color.Bold).SprintFunc()
			greenBold := color.New(color.FgGreen, color.Bold).SprintFunc()

			setupPath := fmt.Sprintf("http://localhost:%d%s/setup?token=%s", port, basePath, bootstrapToken)

			fmt.Println("\n" + cyan("=================================================================="))
			fmt.Println(cyanBold("  🚀 NODEM (NodeManager + ECH Manager) BOOTSTRAP PROTOCOL"))
			fmt.Printf("  Version: %s\n", greenBold(version.Full()))
			fmt.Println("  No users registered yet. To register the administrator account,")
			fmt.Println("  navigate to the setup URL in your browser:")
			fmt.Println()
			fmt.Printf("  👉 %s\n", yellowBold(setupPath))
			fmt.Printf("  Bootstrap Token: %s\n", greenBold(bootstrapToken))
			fmt.Println(cyan("==================================================================") + "\n")
		}

		// Start background daemon coordinator
		d := daemon.New(r)
		daemonCtx, daemonCancel := context.WithCancel(context.Background())
		daemonDone := make(chan struct{})
		go func() {
			defer close(daemonDone)
			d.Start(daemonCtx)
		}()

		// Start background ECH coordinator (handles rotation schedule, SSH pull server, and two-phase DNS sync)
		go daemon.RunECHCoordinator(daemonCtx, r, sshPort)

		// Start REST API server
		state := &api.AppState{
			Repo:           r,
			BootstrapToken: bootstrapToken,
			BasePath:       basePath,
		}

		router := api.BuildRouter(state)
		addr := fmt.Sprintf(":%d", port)
		server := &http.Server{
			Addr:    addr,
			Handler: router,
		}

		slog.Info("HTTP server starting", "addr", addr, "version", version.Full())

		serverErr := make(chan error, 1)
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				serverErr <- err
			}
		}()

		select {
		case err := <-serverErr:
			daemonCancel()
			<-daemonDone
			return err
		case <-ctx.Done():
			slog.Info("Received shutdown signal, starting shutdown...")
		}

		// Graceful shutdown of HTTP server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		}
		slog.Info("HTTP server shut down gracefully. Stopping daemon coordinator...")

		daemonCancel()
		<-daemonDone

		return nil
	}

	rootCmd := cli.RootCmd(getRepo, runDaemon)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}

	if repo != nil {
		slog.Info("Closing database connection pool...")
		if err := repo.Close(); err != nil {
			slog.Error("Failed closing database", "error", err)
		}
		slog.Info("Database closed. Exiting.")
	}
}
