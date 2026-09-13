package config

import "testing"

func withEnv(t *testing.T, kv map[string]string, fn func()) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
	fn()
}

func TestLoad_UsesDefaultsWhenOptionalVarsAreMissing(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}
		if cfg.Port != "8080" {
			t.Errorf("Port = %q, want 8080", cfg.Port)
		}
		if cfg.Env != "development" {
			t.Errorf("Env = %q, want development", cfg.Env)
		}
		if cfg.ReceiptScanDailyLimitPerUser != 10 {
			t.Errorf("ReceiptScanDailyLimitPerUser = %d, want 10", cfg.ReceiptScanDailyLimitPerUser)
		}
	})
}

func TestLoad_RequiresDatabaseURL(t *testing.T) {
	withEnv(t, map[string]string{"DATABASE_URL": ""}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("Load() error = nil, want an error for a missing DATABASE_URL")
		}
	})
}

func TestLoad_RequiresJWTSigningKeyOutsideDevelopment(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":    "postgres://user:pass@localhost:5432/db",
		"ENV":             "production",
		"JWT_SIGNING_KEY": "",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("Load() error = nil, want an error for a missing JWT_SIGNING_KEY in production")
		}
	})
}

func TestLoad_RejectsANonPositiveReceiptScanLimit(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":                      "postgres://user:pass@localhost:5432/db",
		"RECEIPT_SCAN_DAILY_LIMIT_PER_USER": "0",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("Load() error = nil, want an error for a non-positive scan limit")
		}
	})
}

func TestLoad_RejectsANonNumericReceiptScanLimit(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":                      "postgres://user:pass@localhost:5432/db",
		"RECEIPT_SCAN_DAILY_LIMIT_PER_USER": "not-a-number",
	}, func() {
		if _, err := Load(); err == nil {
			t.Fatal("Load() error = nil, want an error for a non-numeric scan limit")
		}
	})
}

func TestLoad_SMTPImplicitTLSDefaultsFromThePort(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
		"SMTP_PORT":    "465",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if !cfg.SMTPImplicitTLS {
			t.Error("SMTPImplicitTLS = false, want true by default on port 465")
		}
	})

	withEnv(t, map[string]string{
		"DATABASE_URL": "postgres://user:pass@localhost:5432/db",
		"SMTP_PORT":    "587",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.SMTPImplicitTLS {
			t.Error("SMTPImplicitTLS = true, want false by default on port 587")
		}
	})
}

func TestLoad_SMTPImplicitTLSIgnoresAnEmptyOverride(t *testing.T) {
	// Mirrors docker-compose's ${VAR:-} form, which sets an empty-but-present
	// env var rather than leaving it unset when the user hasn't overridden it.
	withEnv(t, map[string]string{
		"DATABASE_URL":      "postgres://user:pass@localhost:5432/db",
		"SMTP_PORT":         "465",
		"SMTP_IMPLICIT_TLS": "",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v, want the empty override to be ignored, not rejected", err)
		}
		if !cfg.SMTPImplicitTLS {
			t.Error("SMTPImplicitTLS = false, want the port-465 default to still apply")
		}
	})
}

func TestLoad_SMTPImplicitTLSExplicitOverrideWins(t *testing.T) {
	withEnv(t, map[string]string{
		"DATABASE_URL":      "postgres://user:pass@localhost:5432/db",
		"SMTP_PORT":         "465",
		"SMTP_IMPLICIT_TLS": "false",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if cfg.SMTPImplicitTLS {
			t.Error("SMTPImplicitTLS = true, want the explicit false override to win over the port-465 default")
		}
	})
}
