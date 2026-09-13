package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RefreshToken is one issued refresh token, stored hashed. Revoked/expired
// rows are kept (not deleted) so a reused, already-revoked token can still be
// detected and rejected rather than silently vanishing.
type RefreshToken struct {
	ID         string
	UserID     string
	TokenHash  string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *string
	CreatedAt  time.Time
}

func (rt RefreshToken) isUsable(now time.Time) bool {
	return rt.RevokedAt == nil && now.Before(rt.ExpiresAt)
}

// RefreshTokenStore persists refresh tokens. The Postgres implementation is
// below; auth's tests use an in-memory fake.
type RefreshTokenStore interface {
	Create(ctx context.Context, rt RefreshToken) (RefreshToken, error)
	GetByHash(ctx context.Context, hash string) (RefreshToken, error)
	Revoke(ctx context.Context, id string, replacedBy *string) error
}

type PostgresRefreshTokenStore struct {
	pool *pgxpool.Pool
}

func NewPostgresRefreshTokenStore(pool *pgxpool.Pool) *PostgresRefreshTokenStore {
	return &PostgresRefreshTokenStore{pool: pool}
}

func (s *PostgresRefreshTokenStore) Create(ctx context.Context, rt RefreshToken) (RefreshToken, error) {
	const q = `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, expires_at, revoked_at, replaced_by, created_at
	`
	return scanRefreshToken(s.pool.QueryRow(ctx, q, rt.UserID, rt.TokenHash, rt.ExpiresAt))
}

func (s *PostgresRefreshTokenStore) GetByHash(ctx context.Context, hash string) (RefreshToken, error) {
	const q = `
		SELECT id, user_id, token_hash, expires_at, revoked_at, replaced_by, created_at
		FROM refresh_tokens WHERE token_hash = $1
	`
	rt, err := scanRefreshToken(s.pool.QueryRow(ctx, q, hash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RefreshToken{}, ErrInvalidToken
		}
		return RefreshToken{}, err
	}
	return rt, nil
}

func (s *PostgresRefreshTokenStore) Revoke(ctx context.Context, id string, replacedBy *string) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $2 WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id, replacedBy)
	return err
}

type refreshRowScanner interface {
	Scan(dest ...any) error
}

func scanRefreshToken(row refreshRowScanner) (RefreshToken, error) {
	var rt RefreshToken
	err := row.Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.RevokedAt, &rt.ReplacedBy, &rt.CreatedAt)
	return rt, err
}
