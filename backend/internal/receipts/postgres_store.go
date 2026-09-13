package receipts

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresScanStore is the pgx-backed ScanStore.
type PostgresScanStore struct {
	pool *pgxpool.Pool
}

func NewPostgresScanStore(pool *pgxpool.Pool) *PostgresScanStore {
	return &PostgresScanStore{pool: pool}
}

func (s *PostgresScanStore) CountSince(ctx context.Context, userID string, since time.Time) (int, error) {
	const q = `SELECT COUNT(*) FROM receipt_scans WHERE user_id = $1 AND created_at >= $2`
	var count int
	err := s.pool.QueryRow(ctx, q, userID, since).Scan(&count)
	return count, err
}

func (s *PostgresScanStore) Record(ctx context.Context, r ScanRecord) error {
	const q = `INSERT INTO receipt_scans (user_id, status, error_code) VALUES ($1, $2, NULLIF($3, ''))`
	_, err := s.pool.Exec(ctx, q, r.UserID, string(r.Status), r.ErrorCode)
	return err
}
