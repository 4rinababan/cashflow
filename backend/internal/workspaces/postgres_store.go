package workspaces

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rowScanner interface {
	Scan(dest ...any) error
}

// ----- workspaces -----

type PostgresWorkspaceStore struct {
	pool *pgxpool.Pool
}

func NewPostgresWorkspaceStore(pool *pgxpool.Pool) *PostgresWorkspaceStore {
	return &PostgresWorkspaceStore{pool: pool}
}

func (s *PostgresWorkspaceStore) Create(ctx context.Context, w Workspace) (Workspace, error) {
	const q = `INSERT INTO workspaces (name) VALUES ($1) RETURNING id, name, created_at`
	return scanWorkspace(s.pool.QueryRow(ctx, q, w.Name))
}

func (s *PostgresWorkspaceStore) GetByID(ctx context.Context, id string) (Workspace, error) {
	const q = `SELECT id, name, created_at FROM workspaces WHERE id = $1`
	w, err := scanWorkspace(s.pool.QueryRow(ctx, q, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Workspace{}, ErrNotFound
		}
		return Workspace{}, err
	}
	return w, nil
}

func (s *PostgresWorkspaceStore) UpdateName(ctx context.Context, id, name string) (Workspace, error) {
	const q = `UPDATE workspaces SET name = $2 WHERE id = $1 RETURNING id, name, created_at`
	w, err := scanWorkspace(s.pool.QueryRow(ctx, q, id, name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Workspace{}, ErrNotFound
		}
		return Workspace{}, err
	}
	return w, nil
}

func (s *PostgresWorkspaceStore) Delete(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, id)
	return err
}

func scanWorkspace(row rowScanner) (Workspace, error) {
	var w Workspace
	err := row.Scan(&w.ID, &w.Name, &w.CreatedAt)
	return w, err
}

// ----- members -----

type PostgresMemberStore struct {
	pool *pgxpool.Pool
}

func NewPostgresMemberStore(pool *pgxpool.Pool) *PostgresMemberStore {
	return &PostgresMemberStore{pool: pool}
}

func (s *PostgresMemberStore) Add(ctx context.Context, m Member) error {
	const q = `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, $3)`
	_, err := s.pool.Exec(ctx, q, m.WorkspaceID, m.UserID, string(m.Role))
	return err
}

func (s *PostgresMemberStore) Remove(ctx context.Context, workspaceID, userID string) error {
	const q = `DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`
	tag, err := s.pool.Exec(ctx, q, workspaceID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresMemberStore) Get(ctx context.Context, workspaceID, userID string) (Member, error) {
	const q = `SELECT workspace_id, user_id, role, created_at FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`
	m, err := scanMember(s.pool.QueryRow(ctx, q, workspaceID, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Member{}, ErrNotFound
		}
		return Member{}, err
	}
	return m, nil
}

func (s *PostgresMemberStore) ListForWorkspace(ctx context.Context, workspaceID string) ([]MemberWithUser, error) {
	const q = `
		SELECT wm.workspace_id, wm.user_id, wm.role, wm.created_at, u.email, u.display_name
		FROM workspace_members wm
		JOIN users u ON u.id = wm.user_id
		WHERE wm.workspace_id = $1
		ORDER BY wm.created_at
	`
	rows, err := s.pool.Query(ctx, q, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []MemberWithUser
	for rows.Next() {
		var mu MemberWithUser
		var role string
		if err := rows.Scan(&mu.WorkspaceID, &mu.UserID, &role, &mu.CreatedAt, &mu.Email, &mu.DisplayName); err != nil {
			return nil, err
		}
		mu.Role = Role(role)
		result = append(result, mu)
	}
	return result, rows.Err()
}

func (s *PostgresMemberStore) ListForUser(ctx context.Context, userID string) ([]WorkspaceWithRole, error) {
	const q = `
		SELECT w.id, w.name, w.created_at, wm.role
		FROM workspace_members wm
		JOIN workspaces w ON w.id = wm.workspace_id
		WHERE wm.user_id = $1
		ORDER BY w.created_at
	`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []WorkspaceWithRole
	for rows.Next() {
		var wr WorkspaceWithRole
		var role string
		if err := rows.Scan(&wr.ID, &wr.Name, &wr.CreatedAt, &role); err != nil {
			return nil, err
		}
		wr.Role = Role(role)
		result = append(result, wr)
	}
	return result, rows.Err()
}

func scanMember(row rowScanner) (Member, error) {
	var m Member
	var role string
	err := row.Scan(&m.WorkspaceID, &m.UserID, &role, &m.CreatedAt)
	m.Role = Role(role)
	return m, err
}

// ----- invites -----

type PostgresInviteStore struct {
	pool *pgxpool.Pool
}

func NewPostgresInviteStore(pool *pgxpool.Pool) *PostgresInviteStore {
	return &PostgresInviteStore{pool: pool}
}

func (s *PostgresInviteStore) Create(ctx context.Context, i Invite) (Invite, error) {
	const q = `
		INSERT INTO workspace_invites (workspace_id, email, token_hash, invited_by, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, workspace_id, email, token_hash, invited_by, expires_at, accepted_at, created_at
	`
	return scanInvite(s.pool.QueryRow(ctx, q, i.WorkspaceID, i.Email, i.TokenHash, i.InvitedBy, i.ExpiresAt))
}

func (s *PostgresInviteStore) GetByTokenHash(ctx context.Context, tokenHash string) (Invite, error) {
	const q = `
		SELECT id, workspace_id, email, token_hash, invited_by, expires_at, accepted_at, created_at
		FROM workspace_invites WHERE token_hash = $1
	`
	i, err := scanInvite(s.pool.QueryRow(ctx, q, tokenHash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Invite{}, ErrNotFound
		}
		return Invite{}, err
	}
	return i, nil
}

func (s *PostgresInviteStore) MarkAccepted(ctx context.Context, id string) error {
	const q = `UPDATE workspace_invites SET accepted_at = $2 WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id, time.Now())
	return err
}

func scanInvite(row rowScanner) (Invite, error) {
	var i Invite
	err := row.Scan(&i.ID, &i.WorkspaceID, &i.Email, &i.TokenHash, &i.InvitedBy, &i.ExpiresAt, &i.AcceptedAt, &i.CreatedAt)
	return i, err
}
