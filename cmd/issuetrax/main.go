// Command issuetrax runs the IssueTrax server: a JSON API under /api plus the
// embedded React frontend.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nilswitt.dev/issuetrax/frontend"
	"nilswitt.dev/issuetrax/internal/adapter/httpapi"
	"nilswitt.dev/issuetrax/internal/adapter/storage/memory"
	"nilswitt.dev/issuetrax/internal/config"
	"nilswitt.dev/issuetrax/internal/core/service"
	"nilswitt.dev/issuetrax/internal/version"
)

// shutdownTimeout bounds how long in-flight requests get to finish after
// SIGINT/SIGTERM.
const shutdownTimeout = 15 * time.Second

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("issuetrax exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load(args)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Wiring: driven adapters -> core services -> driving adapters.
	issueRepo := memory.NewIssueRepository()
	issueService := service.NewIssueService(issueRepo)

	dist, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		return fmt.Errorf("open embedded frontend: %w", err)
	}

	handler, err := httpapi.NewHandler(issueService, dist, logger)
	if err != nil {
		return fmt.Errorf("build http handler: %w", err)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	errCh := make(chan error, 1)

	go func() {
		logger.InfoContext(ctx, "listening",
			slog.String("addr", cfg.Addr),
			slog.String("version", version.Version),
			slog.String("commit", version.Commit),
		)

		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}

	return nil
}
