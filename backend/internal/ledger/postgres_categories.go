package ledger

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresCategoryStore is the pgx-backed CategoryStore.
type PostgresCategoryStore struct {
	pool *pgxpool.Pool
}

func NewPostgresCategoryStore(pool *pgxpool.Pool) *PostgresCategoryStore {
	return &PostgresCategoryStore{pool: pool}
}

func (s *PostgresCategoryStore) Create(ctx context.Context, c Category) (Category, error) {
	const q = `
		INSERT INTO categories (workspace_id, created_by_user_id, name, kind)
		VALUES ($1, $2, $3, $4)
		RETURNING id, workspace_id, created_by_user_id, name, kind, archived_at, created_at, updated_at
	`
	return scanCategory(s.pool.QueryRow(ctx, q, c.WorkspaceID, c.CreatedByUserID, c.Name, string(c.Kind)))
}

// CreateMany inserts every category in cs in a single transaction — used to
// seed a new workspace's default categories atomically.
func (s *PostgresCategoryStore) CreateMany(ctx context.Context, cs []Category) error {
	if len(cs) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op once committed

	for _, c := range cs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO categories (workspace_id, created_by_user_id, name, kind) VALUES ($1, $2, $3, $4)`,
			c.WorkspaceID, c.CreatedByUserID, c.Name, string(c.Kind),
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *PostgresCategoryStore) GetByID(ctx context.Context, workspaceID, id string) (Category, error) {
	const q = `
		SELECT id, workspace_id, created_by_user_id, name, kind, archived_at, created_at, updated_at
		FROM categories WHERE id = $1 AND workspace_id = $2
	`
	c, err := scanCategory(s.pool.QueryRow(ctx, q, id, workspaceID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Category{}, ErrNotFound
		}
		return Category{}, err
	}
	return c, nil
}

func (s *PostgresCategoryStore) ListByWorkspace(ctx context.Context, workspaceID string) ([]Category, error) {
	const q = `
		SELECT id, workspace_id, created_by_user_id, name, kind, archived_at, created_at, updated_at
		FROM categories WHERE workspace_id = $1 ORDER BY created_at
	`
	rows, err := s.pool.Query(ctx, q, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Category
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (s *PostgresCategoryStore) Update(ctx context.Context, c Category) (Category, error) {
	const q = `
		UPDATE categories SET name = $3, archived_at = $4, updated_at = now()
		WHERE id = $1 AND workspace_id = $2
		RETURNING id, workspace_id, created_by_user_id, name, kind, archived_at, created_at, updated_at
	`
	updated, err := scanCategory(s.pool.QueryRow(ctx, q, c.ID, c.WorkspaceID, c.Name, c.ArchivedAt))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Category{}, ErrNotFound
		}
		return Category{}, err
	}
	return updated, nil
}

func (s *PostgresCategoryStore) Delete(ctx context.Context, workspaceID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM categories WHERE id = $1 AND workspace_id = $2`, id, workspaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresCategoryStore) HasTransactions(ctx context.Context, workspaceID, id string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM transactions WHERE category_id = $1 AND workspace_id = $2)`
	var exists bool
	err := s.pool.QueryRow(ctx, q, id, workspaceID).Scan(&exists)
	return exists, err
}

func scanCategory(row rowScanner) (Category, error) {
	var c Category
	var kind string
	err := row.Scan(&c.ID, &c.WorkspaceID, &c.CreatedByUserID, &c.Name, &kind, &c.ArchivedAt, &c.CreatedAt, &c.UpdatedAt)
	c.Kind = Direction(kind)
	return c, err
}
