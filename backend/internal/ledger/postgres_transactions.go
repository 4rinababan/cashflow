package ledger

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresTransactionStore is the pgx-backed TransactionStore.
type PostgresTransactionStore struct {
	pool *pgxpool.Pool
}

func NewPostgresTransactionStore(pool *pgxpool.Pool) *PostgresTransactionStore {
	return &PostgresTransactionStore{pool: pool}
}

func (s *PostgresTransactionStore) Create(ctx context.Context, t Transaction) (Transaction, error) {
	const q = `
		INSERT INTO transactions (workspace_id, created_by_user_id, account_id, category_id, direction, amount_minor, currency, occurred_at, note)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, workspace_id, created_by_user_id, account_id, category_id, direction, amount_minor, currency, occurred_at, note, deleted_at, created_at, updated_at
	`
	return scanTransaction(s.pool.QueryRow(ctx, q,
		t.WorkspaceID, t.CreatedByUserID, t.AccountID, t.CategoryID, string(t.Direction), t.AmountMinor, t.Currency, t.OccurredAt, t.Note))
}

func (s *PostgresTransactionStore) GetByID(ctx context.Context, workspaceID, id string) (Transaction, error) {
	const q = `
		SELECT id, workspace_id, created_by_user_id, account_id, category_id, direction, amount_minor, currency, occurred_at, note, deleted_at, created_at, updated_at
		FROM transactions WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL
	`
	t, err := scanTransaction(s.pool.QueryRow(ctx, q, id, workspaceID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Transaction{}, ErrNotFound
		}
		return Transaction{}, err
	}
	return t, nil
}

// ListByWorkspace builds its WHERE clause from whichever filter fields are
// set. Every value is still passed as a placeholder argument — only the
// placeholder numbers are string-built — so this stays injection-safe.
func (s *PostgresTransactionStore) ListByWorkspace(ctx context.Context, workspaceID string, filter TransactionFilter) ([]Transaction, int, error) {
	where := []string{"workspace_id = $1", "deleted_at IS NULL"}
	args := []any{workspaceID}
	addArg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.AccountID != "" {
		where = append(where, "account_id = "+addArg(filter.AccountID))
	}
	if filter.CategoryID != "" {
		where = append(where, "category_id = "+addArg(filter.CategoryID))
	}
	if filter.From != nil {
		where = append(where, "occurred_at >= "+addArg(*filter.From))
	}
	if filter.To != nil {
		where = append(where, "occurred_at <= "+addArg(*filter.To))
	}
	whereClause := strings.Join(where, " AND ")

	var total int
	if err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM transactions WHERE "+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limitPlaceholder := addArg(filter.Limit)
	offsetPlaceholder := addArg(filter.Offset)
	listQuery := fmt.Sprintf(`
		SELECT id, workspace_id, created_by_user_id, account_id, category_id, direction, amount_minor, currency, occurred_at, note, deleted_at, created_at, updated_at
		FROM transactions WHERE %s
		ORDER BY occurred_at DESC, created_at DESC
		LIMIT %s OFFSET %s
	`, whereClause, limitPlaceholder, offsetPlaceholder)

	rows, err := s.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []Transaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return result, total, nil
}

func (s *PostgresTransactionStore) Update(ctx context.Context, t Transaction) (Transaction, error) {
	const q = `
		UPDATE transactions
		SET account_id = $3, category_id = $4, direction = $5, amount_minor = $6, occurred_at = $7, note = $8, updated_at = now()
		WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL
		RETURNING id, workspace_id, created_by_user_id, account_id, category_id, direction, amount_minor, currency, occurred_at, note, deleted_at, created_at, updated_at
	`
	updated, err := scanTransaction(s.pool.QueryRow(ctx, q,
		t.ID, t.WorkspaceID, t.AccountID, t.CategoryID, string(t.Direction), t.AmountMinor, t.OccurredAt, t.Note))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Transaction{}, ErrNotFound
		}
		return Transaction{}, err
	}
	return updated, nil
}

func (s *PostgresTransactionStore) SoftDelete(ctx context.Context, workspaceID, id string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE transactions SET deleted_at = now() WHERE id = $1 AND workspace_id = $2 AND deleted_at IS NULL`, id, workspaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanTransaction(row rowScanner) (Transaction, error) {
	var t Transaction
	var direction string
	err := row.Scan(&t.ID, &t.WorkspaceID, &t.CreatedByUserID, &t.AccountID, &t.CategoryID, &direction, &t.AmountMinor,
		&t.Currency, &t.OccurredAt, &t.Note, &t.DeletedAt, &t.CreatedAt, &t.UpdatedAt)
	t.Direction = Direction(direction)
	return t, err
}
