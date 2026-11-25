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
	"nilswitt.dev/issuetrax/internal/adapter/oidc"
	"nilswitt.dev/issuetrax/internal/adapter/storage/postgres"
	"nilswitt.dev/issuetrax/internal/config"
	"nilswitt.dev/issuetrax/internal/core/service"
	"nilswitt.dev/issuetrax/internal/version"
)

// shutdownTimeout bounds how long in-flight requests get to finish after
// SIGINT/SIGTERM.
const shutdownTimeout = 15 * time.Second

// sessionCleanupInterval is how often expired sessions are purged.
const sessionCleanupInterval = time.Hour

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("issuetrax exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}

// wire builds the application graph: driven adapters -> core services ->
// driving adapters.
func wire(ctx context.Context, cfg config.Config, db *postgres.DB, logger *slog.Logger) (http.Handler, *service.AuthService, error) {
	repos := service.Repositories{
		Tx:       db,
		Projects: postgres.NewProjectRepository(db),
		Issues:   postgres.NewIssueRepository(db),
		Tags:     postgres.NewTagRepository(db),
		Comments: postgres.NewCommentRepository(db),
		Users:    postgres.NewUserRepository(db),
		Sessions: postgres.NewSessionRepository(db),
	}

	idp, err := oidc.New(ctx, oidc.Config{
		Issuer:                cfg.OIDCIssuer,
		ClientID:              cfg.OIDCClientID,
		ClientSecret:          cfg.OIDCClientSecret,
		RedirectURL:           cfg.PublicURL + "/auth/callback",
		PostLogoutRedirectURL: cfg.PublicURL + "/",
	})
	if err != nil {
		return nil, nil, err
	}

	auth := service.NewAuthService(repos, idp)

	dist, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		return nil, nil, fmt.Errorf("open embedded frontend: %w", err)
	}

	handler, err := httpapi.NewHandler(httpapi.Config{
		Auth:            auth,
		Projects:        service.NewProjectService(repos),
		Issues:          service.NewIssueService(repos),
		Tags:            service.NewTagService(repos),
		Comments:        service.NewCommentService(repos),
		Users:           service.NewUserService(repos),
		Frontend:        dist,
		Logger:          logger,
		PublicURL:       cfg.PublicURL,
		CookieKey:       cfg.CookieKey,
		InsecureCookies: cfg.Dev,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("build http handler: %w", err)
	}

	return handler, auth, nil
}

// cleanupSessions deletes expired sessions now and then every hour until
// ctx is done.
func cleanupSessions(ctx context.Context, auth *service.AuthService, logger *slog.Logger) {
	ticker := time.NewTicker(sessionCleanupInterval)
	defer ticker.Stop()

	for {
		n, err := auth.DeleteExpiredSessions(ctx)

		switch {
		case err != nil && ctx.Err() == nil:
			logger.ErrorContext(ctx, "session cleanup failed", slog.Any("error", err))
		case n > 0:
			logger.InfoContext(ctx, "expired sessions deleted", slog.Int64("count", n))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
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

	if cfg.MigrateOnStart {
		if err := postgres.Migrate(ctx, cfg.DatabaseURL); err != nil {
			return err
		}

		logger.InfoContext(ctx, "database migrations applied")
	}

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	handler, auth, err := wire(ctx, cfg, db, logger)
	if err != nil {
		return err
	}

	go cleanupSessions(ctx, auth, logger)

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
