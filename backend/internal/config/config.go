// Package config loads server configuration from the environment. There is no
// framework here on purpose — Phase 1 has few settings and a plain loader is
// easy to read and test.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config is the fully-resolved, validated set of settings the server needs to
// start. Load returns an error rather than letting a bad value surface later
// as a confusing runtime failure.
type Config struct {
	// Port the HTTP server listens on.
	Port string
	// Env is "development", "staging", or "production". Some checks (e.g. a
	// non-empty JWT signing key) are only enforced outside development.
	Env string
	// DatabaseURL is a Postgres connection string (see pgx docs for the format).
	DatabaseURL string
	// JWTSigningKey signs auth tokens (Phase 2). Required outside development.
	JWTSigningKey string
	// AIEndpoint is the base URL of an OpenAI-compatible chat-completions API
	// (Phase 4 receipt scanning) — currently Gemini's OpenAI-compatibility
	// layer, chosen because it is live-verified to actually read images (see
	// docs/adr/0002-ai-receipt-scanning.md; the original choice, Biznet
	// Gio's openai/gpt-oss-20b, was live-tested and confirmed NOT to support
	// vision at all). Must not include a trailing slash; "/chat/completions"
	// is appended to it.
	AIEndpoint string
	// AIAPIKey authenticates to AIEndpoint. Optional until Phase 4's scan
	// endpoint is actually used; the endpoint refuses requests without it.
	AIAPIKey string
	// AIModel is the model name passed to AIEndpoint. Configurable (not
	// hardcoded) so the provider/model can change without a code change.
	// Defaults to Gemini's "-latest" alias rather than a pinned version:
	// live-testing hit a pinned model (gemini-2.5-flash) being deprecated
	// mid-session, so pinning a specific version is a real, not theoretical,
	// maintenance burden here.
	AIModel string
	// ReceiptScanDailyLimitPerUser caps AI receipt scans per user per day
	// (Phase 4), so a free public app has a cost ceiling.
	ReceiptScanDailyLimitPerUser int

	// SMTP* configure outbound email (Phase 9 workspace invites). SMTPHost
	// empty means email is not configured: main.go falls back to a
	// logging-only Mailer instead of failing startup, since a fresh dev
	// environment has no mail server and invites are not required to run
	// the app solo.
	SMTPHost        string
	SMTPPort        int
	SMTPUsername    string
	SMTPPassword    string
	SMTPFromAddress string
	SMTPFromName    string
	// SMTPImplicitTLS selects implicit-TLS-from-connect (typically port 465,
	// "SslOnConnect") vs. plaintext-then-STARTTLS (typically port 587).
	SMTPImplicitTLS bool

	// AppBaseURL is where a workspace-invite link points (e.g.
	// "https://app.cashflow.example"); the mobile app / a web landing page
	// owns the actual "/invites/{token}" route.
	AppBaseURL string

	// Admin* configure the internal /admin dashboard (Phase 9d — operator
	// analytics/log panel, never exposed to end users). Empty
	// AdminUser/AdminPassword disables the dashboard entirely (main.go
	// checks this) rather than serving it wide open.
	AdminUser     string
	AdminPassword string
	// GaweanDatabaseURL optionally points at a READ-ONLY role on the
	// sibling gawean-management app's own Postgres, purely to show its
	// total user count on the same dashboard. Left empty by default — that
	// app's database is not this project's to reach into without the
	// credentials being explicitly handed over for this purpose; the
	// dashboard shows "not connected" until it's set.
	GaweanDatabaseURL string
	// AdminErrorLogPath is where scripts/log-watch.py writes newline-delimited
	// JSON error/crash entries it scrapes from both apps' container logs
	// (see docs/ops/production-deployment.md). Read-only from here.
	AdminErrorLogPath string
}

// Load reads and validates configuration from environment variables.
func Load() (Config, error) {
	cfg := Config{
		Port:              getEnv("PORT", "8080"),
		Env:               getEnv("ENV", "development"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTSigningKey:     os.Getenv("JWT_SIGNING_KEY"),
		AIEndpoint:        getEnv("AI_ENDPOINT", "https://generativelanguage.googleapis.com/v1beta/openai"),
		AIAPIKey:          os.Getenv("AI_API_KEY"),
		AIModel:           getEnv("AI_MODEL", "gemini-flash-latest"),
		SMTPHost:          os.Getenv("SMTP_HOST"),
		SMTPUsername:      os.Getenv("SMTP_USER"),
		SMTPPassword:      os.Getenv("SMTP_PASSWORD"),
		SMTPFromAddress:   os.Getenv("SMTP_FROM_ADDRESS"),
		SMTPFromName:      getEnv("SMTP_FROM_NAME", "Cashflow"),
		AppBaseURL:        getEnv("APP_BASE_URL", "https://app.cashflow.example"),
		AdminUser:         os.Getenv("ADMIN_USER"),
		AdminPassword:     os.Getenv("ADMIN_PASSWORD"),
		GaweanDatabaseURL: os.Getenv("GAWEAN_DATABASE_URL"),
		AdminErrorLogPath: getEnv("ADMIN_ERROR_LOG_PATH", "/data/monitor/error-log.jsonl"),
	}

	limitRaw := getEnv("RECEIPT_SCAN_DAILY_LIMIT_PER_USER", "10")
	limit, err := strconv.Atoi(limitRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid RECEIPT_SCAN_DAILY_LIMIT_PER_USER %q: %w", limitRaw, err)
	}
	if limit <= 0 {
		return Config{}, fmt.Errorf("RECEIPT_SCAN_DAILY_LIMIT_PER_USER must be positive, got %d", limit)
	}
	cfg.ReceiptScanDailyLimitPerUser = limit

	smtpPortRaw := getEnv("SMTP_PORT", "465")
	smtpPort, err := strconv.Atoi(smtpPortRaw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid SMTP_PORT %q: %w", smtpPortRaw, err)
	}
	cfg.SMTPPort = smtpPort
	// SMTP_IMPLICIT_TLS defaults to true on port 465 ("SslOnConnect", this
	// project's actual mail host) and false otherwise (e.g. 587/STARTTLS);
	// an explicit SMTP_IMPLICIT_TLS always wins.
	implicitTLSDefault := smtpPort == 465
	// Checked against "" as well as unset: docker-compose's ${VAR:-} form sets
	// an empty-but-present env var when the user hasn't overridden it, which
	// os.LookupEnv alone would otherwise mistake for an explicit "false".
	if raw, ok := os.LookupEnv("SMTP_IMPLICIT_TLS"); ok && raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SMTP_IMPLICIT_TLS %q: %w", raw, err)
		}
		cfg.SMTPImplicitTLS = parsed
	} else {
		cfg.SMTPImplicitTLS = implicitTLSDefault
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.Env != "development" && cfg.JWTSigningKey == "" {
		return Config{}, fmt.Errorf("JWT_SIGNING_KEY is required outside development")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
