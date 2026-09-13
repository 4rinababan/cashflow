package ledger

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresAccountStore is the pgx-backed AccountStore.
type PostgresAccountStore struct {
	pool *pgxpool.Pool
}

func NewPostgresAccountStore(pool *pgxpool.Pool) *PostgresAccountStore {
	return &PostgresAccountStore{pool: pool}
}

func (s *PostgresAccountStore) Create(ctx context.Context, a Account) (Account, error) {
	const q = `
		INSERT INTO accounts (workspace_id, created_by_user_id, name, currency)
		VALUES ($1, $2, $3, $4)
		RETURNING id, workspace_id, created_by_user_id, name, currency, archived_at, created_at, updated_at
	`
	return scanAccount(s.pool.QueryRow(ctx, q, a.WorkspaceID, a.CreatedByUserID, a.Name, a.Currency))
}

func (s *PostgresAccountStore) GetByID(ctx context.Context, workspaceID, id string) (Account, error) {
	const q = `
		SELECT id, workspace_id, created_by_user_id, name, currency, archived_at, created_at, updated_at
		FROM accounts WHERE id = $1 AND workspace_id = $2
	`
	a, err := scanAccount(s.pool.QueryRow(ctx, q, id, workspaceID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Account{}, ErrNotFound
		}
		return Account{}, err
	}
	return a, nil
}

// ListByWorkspace returns every account with its running balance computed
// in one query (a LEFT JOIN aggregate), avoiding an N+1 balance lookup per
// account.
func (s *PostgresAccountStore) ListByWorkspace(ctx context.Context, workspaceID string) ([]AccountBalance, error) {
	const q = `
		SELECT a.id, a.workspace_id, a.created_by_user_id, a.name, a.currency, a.archived_at, a.created_at, a.updated_at,
		       COALESCE(SUM(CASE WHEN t.direction = 'income' THEN t.amount_minor
		                          WHEN t.direction = 'expense' THEN -t.amount_minor
		                          ELSE 0 END), 0) AS balance_minor
		FROM accounts a
		LEFT JOIN transactions t ON t.account_id = a.id AND t.deleted_at IS NULL
		WHERE a.workspace_id = $1
		GROUP BY a.id
		ORDER BY a.created_at
	`
	rows, err := s.pool.Query(ctx, q, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []AccountBalance
	for rows.Next() {
		var ab AccountBalance
		if err := rows.Scan(&ab.ID, &ab.WorkspaceID, &ab.CreatedByUserID, &ab.Name, &ab.Currency, &ab.ArchivedAt, &ab.CreatedAt, &ab.UpdatedAt, &ab.BalanceMinor); err != nil {
			return nil, err
		}
		result = append(result, ab)
	}
	return result, rows.Err()
}

func (s *PostgresAccountStore) GetBalance(ctx context.Context, workspaceID, id string) (int64, error) {
	const q = `
		SELECT COALESCE(SUM(CASE WHEN direction = 'income' THEN amount_minor ELSE -amount_minor END), 0)
		FROM transactions
		WHERE account_id = $1 AND workspace_id = $2 AND deleted_at IS NULL
	`
	var balance int64
	err := s.pool.QueryRow(ctx, q, id, workspaceID).Scan(&balance)
	return balance, err
}

func (s *PostgresAccountStore) Update(ctx context.Context, a Account) (Account, error) {
	const q = `
		UPDATE accounts SET name = $3, archived_at = $4, updated_at = now()
		WHERE id = $1 AND workspace_id = $2
		RETURNING id, workspace_id, created_by_user_id, name, currency, archived_at, created_at, updated_at
	`
	updated, err := scanAccount(s.pool.QueryRow(ctx, q, a.ID, a.WorkspaceID, a.Name, a.ArchivedAt))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Account{}, ErrNotFound
		}
		return Account{}, err
	}
	return updated, nil
}

func (s *PostgresAccountStore) Delete(ctx context.Context, workspaceID, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM accounts WHERE id = $1 AND workspace_id = $2`, id, workspaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresAccountStore) HasTransactions(ctx context.Context, workspaceID, id string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM transactions WHERE account_id = $1 AND workspace_id = $2)`
	var exists bool
	err := s.pool.QueryRow(ctx, q, id, workspaceID).Scan(&exists)
	return exists, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAccount(row rowScanner) (Account, error) {
	var a Account
	err := row.Scan(&a.ID, &a.WorkspaceID, &a.CreatedByUserID, &a.Name, &a.Currency, &a.ArchivedAt, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}
