package reports

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore computes every aggregate directly against internal/ledger's
// tables (transactions, categories) — reports owns no tables of its own.
type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Summary(ctx context.Context, workspaceID string, from, to time.Time) (Summary, error) {
	const q = `
		SELECT
			COALESCE(SUM(CASE WHEN direction = 'income' THEN amount_minor ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN direction = 'expense' THEN amount_minor ELSE 0 END), 0),
			COUNT(*)
		FROM transactions
		WHERE workspace_id = $1 AND deleted_at IS NULL AND occurred_at BETWEEN $2 AND $3
	`
	summary := Summary{From: from, To: to}
	err := s.pool.QueryRow(ctx, q, workspaceID, from, to).Scan(&summary.IncomeMinor, &summary.ExpenseMinor, &summary.TransactionCount)
	return summary, err
}

func (s *PostgresStore) ByCategory(ctx context.Context, workspaceID string, from, to time.Time) ([]CategoryBreakdown, error) {
	const q = `
		SELECT c.id, c.name, c.kind, COALESCE(SUM(t.amount_minor), 0), COUNT(t.id)
		FROM categories c
		JOIN transactions t ON t.category_id = c.id
			AND t.workspace_id = c.workspace_id
			AND t.deleted_at IS NULL
			AND t.occurred_at BETWEEN $2 AND $3
		WHERE c.workspace_id = $1
		GROUP BY c.id
		ORDER BY SUM(t.amount_minor) DESC
	`
	rows, err := s.pool.Query(ctx, q, workspaceID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []CategoryBreakdown
	for rows.Next() {
		var b CategoryBreakdown
		if err := rows.Scan(&b.CategoryID, &b.CategoryName, &b.Kind, &b.TotalMinor, &b.TransactionCount); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

func (s *PostgresStore) MonthlyTrend(ctx context.Context, workspaceID string, months []string, rangeStart, rangeEnd time.Time) (map[string]MonthlyTrendPoint, error) {
	const q = `
		SELECT to_char(date_trunc('month', occurred_at), 'YYYY-MM') AS month,
			COALESCE(SUM(CASE WHEN direction = 'income' THEN amount_minor ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN direction = 'expense' THEN amount_minor ELSE 0 END), 0)
		FROM transactions
		WHERE workspace_id = $1 AND deleted_at IS NULL AND occurred_at >= $2 AND occurred_at < $3
		GROUP BY 1
	`
	rows, err := s.pool.Query(ctx, q, workspaceID, rangeStart, rangeEnd)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]MonthlyTrendPoint, len(months))
	for rows.Next() {
		var p MonthlyTrendPoint
		if err := rows.Scan(&p.Month, &p.IncomeMinor, &p.ExpenseMinor); err != nil {
			return nil, err
		}
		result[p.Month] = p
	}
	return result, rows.Err()
}
