package db

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationFile is one applyable ".up.sql" migration.
type migrationFile struct {
	// Version is the filename without the ".up.sql" suffix, e.g. "0001_init".
	// Migrations apply in ascending version order, so name files with a
	// sortable numeric prefix ("0001_init.up.sql", "0002_add_accounts.up.sql").
	Version string
	Name    string
}

// discoverMigrations lists the ".up.sql" files directly under migrationsFS,
// sorted by version. Pure (no I/O beyond reading the directory), so it is
// tested without a database.
func discoverMigrations(migrationsFS fs.FS) ([]migrationFile, error) {
	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	var files []migrationFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		files = append(files, migrationFile{
			Version: strings.TrimSuffix(name, ".up.sql"),
			Name:    name,
		})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].Version < files[j].Version })
	return files, nil
}

// Migrate applies every not-yet-applied ".up.sql" file in migrationsFS, in
// version order, each inside its own transaction, and records it in
// schema_migrations. Safe to call on every startup — already-applied
// migrations are skipped.
func Migrate(ctx context.Context, pool *pgxpool.Pool, migrationsFS fs.FS) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	files, err := discoverMigrations(migrationsFS)
	if err != nil {
		return err
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	for _, f := range files {
		if applied[f.Version] {
			continue
		}

		if err := applyMigration(ctx, pool, migrationsFS, f); err != nil {
			return err
		}
	}

	return nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, migrationsFS fs.FS, f migrationFile) error {
	sqlBytes, err := fs.ReadFile(migrationsFS, f.Name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", f.Name, err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction for migration %s: %w", f.Version, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("apply migration %s: %w", f.Version, err)
	}

	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, f.Version); err != nil {
		return fmt.Errorf("record migration %s: %w", f.Version, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %s: %w", f.Version, err)
	}

	return nil
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan migration version: %w", err)
		}
		applied[version] = true
	}

	return applied, rows.Err()
}
