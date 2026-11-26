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

	"nilswitt.dev/issuetrax/docs"
	"nilswitt.dev/issuetrax/frontend"
	"nilswitt.dev/issuetrax/internal/adapter/changefeed"
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

// cleanupInterval is how often expired sessions and audit entries are purged.
const cleanupInterval = time.Hour

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			fmt.Fprintln(os.Stderr, "unhealthy:", err)
			os.Exit(1)
		}

		return
	}

	if err := run(os.Args[1:]); err != nil {
		slog.Error("issuetrax exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}

// wire builds the application graph: driven adapters -> core services ->
// driving adapters.
func wire(ctx context.Context, cfg config.Config, db *postgres.DB, logger *slog.Logger) (http.Handler, cleaner, error) {
	repos := service.Repositories{
		Tx:          db,
		Projects:    postgres.NewProjectRepository(db),
		Issues:      postgres.NewIssueRepository(db),
		Tags:        postgres.NewTagRepository(db),
		Comments:    postgres.NewCommentRepository(db),
		Attachments: postgres.NewAttachmentRepository(db),
		Users:       postgres.NewUserRepository(db),
		Sessions:    postgres.NewSessionRepository(db),
		APIKeys:     postgres.NewAPIKeyRepository(db),
		Audit:       postgres.NewAuditRepository(db),
	}

	idp, err := oidc.New(ctx, oidc.Config{
		Issuer:                cfg.OIDCIssuer,
		ClientID:              cfg.OIDCClientID,
		ClientSecret:          cfg.OIDCClientSecret,
		RedirectURL:           cfg.PublicURL + "/auth/callback",
		PostLogoutRedirectURL: cfg.PublicURL + "/",
	})
	if err != nil {
		return nil, cleaner{}, err
	}

	if cfg.OIDCAdminGroup == "" {
		logger.WarnContext(ctx, "ISSUETRAX_OIDC_ADMIN_GROUP is not set; nobody can read the audit log")
	}

	// Services announce committed changes on the hub; /api/events streams
	// them to browsers.
	changes := changefeed.New()
	notify := service.WithChangeNotifier(changes)

	auth := service.NewAuthService(repos, idp, service.WithAdminGroup(cfg.OIDCAdminGroup))
	audit := service.NewAuditService(repos, cfg.AuditRetention)

	dist, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		return nil, cleaner{}, fmt.Errorf("open embedded frontend: %w", err)
	}

	handler, err := httpapi.NewHandler(httpapi.Config{
		Auth:            auth,
		Projects:        service.NewProjectService(repos, notify),
		Issues:          service.NewIssueService(repos, notify),
		Tags:            service.NewTagService(repos, notify),
		Comments:        service.NewCommentService(repos, notify),
		Attachments:     service.NewAttachmentService(repos, notify),
		Users:           service.NewUserService(repos),
		ServiceAccounts: service.NewBotService(repos),
		Audit:           audit,
		Changes:         changes,
		Frontend:        dist,
		OpenAPISpec:     docs.OpenAPI,
		Logger:          logger,
		PublicURL:       cfg.PublicURL,
		CookieKey:       cfg.CookieKey,
		InsecureCookies: cfg.Dev,
	})
	if err != nil {
		return nil, cleaner{}, fmt.Errorf("build http handler: %w", err)
	}

	return handler, cleaner{auth: auth, audit: audit}, nil
}

// cleaner holds the services with periodic cleanup work.
type cleaner struct {
	auth  *service.AuthService
	audit *service.AuditService
}

// run deletes expired sessions and audit entries now and then every hour
// until ctx is done.
func (c cleaner) run(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		logCleanup(ctx, logger, "sessions", c.auth.DeleteExpiredSessions)
		logCleanup(ctx, logger, "audit entries", c.audit.DeleteExpired)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// logCleanup runs one cleanup step and logs its outcome.
func logCleanup(ctx context.Context, logger *slog.Logger, what string, deleteExpired func(context.Context) (int64, error)) {
	n, err := deleteExpired(ctx)

	switch {
	case err != nil && ctx.Err() == nil:
		logger.ErrorContext(ctx, "cleanup failed", slog.String("what", what), slog.Any("error", err))
	case n > 0:
		logger.InfoContext(ctx, "expired "+what+" deleted", slog.Int64("count", n))
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

	handler, cleanup, err := wire(ctx, cfg, db, logger)
	if err != nil {
		return err
	}

	go cleanup.run(ctx, logger)

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
