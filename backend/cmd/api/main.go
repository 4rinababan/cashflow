// Command api runs the Cashflow HTTP API.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cashflow-backend/internal/admin"
	"cashflow-backend/internal/auth"
	"cashflow-backend/internal/config"
	"cashflow-backend/internal/ledger"
	"cashflow-backend/internal/mail"
	"cashflow-backend/internal/platform/db"
	"cashflow-backend/internal/receipts"
	"cashflow-backend/internal/reports"
	"cashflow-backend/internal/server"
	"cashflow-backend/internal/users"
	"cashflow-backend/internal/workspaces"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal startup error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, os.DirFS("migrations")); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	slog.Info("migrations applied")

	signingKey := cfg.JWTSigningKey
	if signingKey == "" {
		// Only reachable in development — config.Load requires a real key
		// otherwise. Ephemeral: every restart invalidates existing access
		// tokens. Fine for local dev, never for a shared/production server.
		generated, err := auth.GenerateSigningKey()
		if err != nil {
			return fmt.Errorf("generate development signing key: %w", err)
		}
		signingKey = generated
		slog.Warn("JWT_SIGNING_KEY not set — using an ephemeral development-only key; tokens will not survive a restart")
	}

	tokenIssuer := auth.NewTokenIssuer(signingKey)
	authService := auth.NewService(
		users.NewPostgresStore(pool),
		auth.NewPostgresRefreshTokenStore(pool),
		tokenIssuer,
	)

	ledgerService := ledger.NewService(
		ledger.NewPostgresAccountStore(pool),
		ledger.NewPostgresCategoryStore(pool),
		ledger.NewPostgresTransactionStore(pool),
	)

	var mailer mail.Mailer = mail.NoopMailer{}
	if cfg.SMTPPassword != "" {
		mailer = mail.NewSMTPMailer(mail.SMTPConfig{
			Host:        cfg.SMTPHost,
			Port:        cfg.SMTPPort,
			Username:    cfg.SMTPUsername,
			Password:    cfg.SMTPPassword,
			FromAddress: cfg.SMTPFromAddress,
			FromName:    cfg.SMTPFromName,
			ImplicitTLS: cfg.SMTPImplicitTLS,
		})
	} else {
		slog.Warn("SMTP_PASSWORD not set — workspace invite emails will only be logged, not sent")
	}

	workspaceService := workspaces.NewService(
		workspaces.NewPostgresWorkspaceStore(pool),
		workspaces.NewPostgresMemberStore(pool),
		workspaces.NewPostgresInviteStore(pool),
		mailer,
	)
	workspaceService.AppBaseURL = cfg.AppBaseURL

	// Every new user gets a personal workspace automatically (solo use needs
	// no extra setup), then it's seeded with the default category set —
	// sharing beyond it is opt-in via a workspace invite (internal/workspaces).
	authService.SetAfterRegister(func(ctx context.Context, userID, displayName string) {
		ws, err := workspaceService.CreatePersonalWorkspace(ctx, userID, displayName)
		if err != nil {
			slog.Error("create personal workspace failed", "user_id", userID, "error", err)
			return
		}
		if err := ledgerService.SeedDefaultCategories(ctx, ws.ID, userID); err != nil {
			slog.Error("seed default categories failed", "workspace_id", ws.ID, "error", err)
		}
	})
	authService.SetBeforeDeleteAccount(workspaceService.RemoveUserFromAllWorkspaces)

	receiptService := receipts.NewService(
		receipts.NewOpenAIScanner(cfg.AIEndpoint, cfg.AIAPIKey, cfg.AIModel, nil),
		receipts.NewPostgresScanStore(pool),
		cfg.ReceiptScanDailyLimitPerUser,
	)
	if cfg.AIAPIKey == "" {
		slog.Warn("AI_API_KEY not set — POST /receipts/scan will return 503 until it is configured")
	}

	reportsService := reports.NewService(reports.NewPostgresStore(pool))

	var adminService *admin.Service
	if cfg.AdminUser != "" && cfg.AdminPassword != "" {
		gaweanPool, err := admin.ConnectGawean(ctx, cfg.GaweanDatabaseURL)
		if err != nil {
			slog.Error("connect to gawean-management database failed — admin dashboard will show it as not connected", "error", err)
			gaweanPool = nil
		}
		adminService = admin.NewService(pool, gaweanPool, cfg.AdminErrorLogPath)
	} else {
		slog.Warn("ADMIN_USER/ADMIN_PASSWORD not set — the /admin dashboard is disabled")
	}

	srv := server.New(cfg.Port, server.Deps{
		Pinger:           pool,
		AuthService:      authService,
		TokenIssuer:      tokenIssuer,
		LedgerService:    ledgerService,
		ReceiptService:   receiptService,
		ReportsService:   reportsService,
		WorkspaceService: workspaceService,
		AdminService:     adminService,
		AdminUser:        cfg.AdminUser,
		AdminPassword:    cfg.AdminPassword,
	})

	errCh := make(chan error, 1)
	go func() {
		slog.Info("listening", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("serve: %w", err)
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
