// Package auth handles registration, login, token issuance/verification, and
// session (refresh token) lifecycle. It depends on users.Store but owns
// everything password- and token-shaped; users knows nothing about this package.
package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cashflow-backend/internal/users"
)

// AuthResult is returned by every operation that starts or continues a
// session: a fresh access token, its expiry, a fresh (rotated) refresh token,
// and the user it belongs to.
type AuthResult struct {
	User            users.User
	AccessToken     string
	AccessExpiresAt time.Time
	RefreshToken    string
}

// Service orchestrates users.Store + password hashing + token issuance. It
// is the only thing HTTP handlers talk to.
type Service struct {
	users               users.Store
	refresh             RefreshTokenStore
	tokens              *TokenIssuer
	afterRegister       func(ctx context.Context, userID, displayName string)
	beforeDeleteAccount func(ctx context.Context, userID string) error
}

func NewService(userStore users.Store, refreshStore RefreshTokenStore, tokens *TokenIssuer) *Service {
	return &Service{users: userStore, refresh: refreshStore, tokens: tokens}
}

// SetAfterRegister installs a hook run once a new user's session has been
// issued (e.g. Phase 9 creating the user's personal workspace, then Phase 3's
// ledger seeding default categories into it). Auth deliberately does not
// import the workspaces/ledger packages itself — main.go wires this so auth
// stays ignorant of what happens after registration. A hook error is not
// fatal to registration; the hook itself decides how to handle/log it.
func (s *Service) SetAfterRegister(fn func(ctx context.Context, userID, displayName string)) {
	s.afterRegister = fn
}

// Register creates a new user and immediately signs them in.
func (s *Service) Register(ctx context.Context, email, password, displayName string) (AuthResult, error) {
	normalizedEmail, err := users.NormalizeEmail(email)
	if err != nil {
		return AuthResult{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return AuthResult{}, err
	}
	name := strings.TrimSpace(displayName)
	if name == "" {
		return AuthResult{}, ErrDisplayNameRequired
	}

	hash, err := HashPassword(password)
	if err != nil {
		return AuthResult{}, err
	}

	created, err := s.users.Create(ctx, users.User{
		Email:        normalizedEmail,
		PasswordHash: hash,
		DisplayName:  name,
	})
	if err != nil {
		return AuthResult{}, err // may be users.ErrEmailTaken
	}

	if s.afterRegister != nil {
		s.afterRegister(ctx, created.ID, created.DisplayName)
	}

	return s.issueSession(ctx, created)
}

// Login verifies credentials and starts a new session. On any failure it
// returns ErrInvalidCredentials — never "no such user" vs "wrong password",
// so a login attempt can't be used to enumerate registered emails.
func (s *Service) Login(ctx context.Context, email, password string) (AuthResult, error) {
	normalizedEmail, err := users.NormalizeEmail(email)
	if err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}

	u, err := s.users.GetByEmail(ctx, normalizedEmail)
	if err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}

	if !VerifyPassword(u.PasswordHash, password) {
		return AuthResult{}, ErrInvalidCredentials
	}

	return s.issueSession(ctx, u)
}

// Refresh rotates a refresh token: the presented one is revoked (pointing at
// its replacement) and a new access + refresh pair is issued. A reused,
// expired, or unknown token is rejected — this also means a stolen-and-later
// refreshed token invalidates itself for the legitimate holder, which is a
// visible signal something is wrong, not a silent security hole.
func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (AuthResult, error) {
	existing, err := s.refresh.GetByHash(ctx, HashToken(rawRefreshToken))
	if err != nil {
		return AuthResult{}, ErrInvalidToken
	}
	if !existing.isUsable(time.Now()) {
		return AuthResult{}, ErrInvalidToken
	}

	u, err := s.users.GetByID(ctx, existing.UserID)
	if err != nil {
		return AuthResult{}, ErrInvalidToken
	}

	result, newID, err := s.issueSessionWithID(ctx, u)
	if err != nil {
		return AuthResult{}, err
	}

	if err := s.refresh.Revoke(ctx, existing.ID, &newID); err != nil {
		return AuthResult{}, err
	}

	return result, nil
}

// Logout revokes the presented refresh token. Idempotent: an unknown or
// already-revoked token is not an error — logging out twice must not fail.
func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	existing, err := s.refresh.GetByHash(ctx, HashToken(rawRefreshToken))
	if err != nil {
		return nil
	}
	if existing.RevokedAt != nil {
		return nil
	}
	return s.refresh.Revoke(ctx, existing.ID, nil)
}

// CurrentUser looks up the user behind an already-verified access token's subject.
func (s *Service) CurrentUser(ctx context.Context, userID string) (users.User, error) {
	return s.users.GetByID(ctx, userID)
}

// SetBeforeDeleteAccount installs a hook run just before a user's row is
// permanently deleted (Phase 9: cleaning up any workspace they solely own —
// see internal/workspaces.Service.RemoveUserFromAllWorkspaces, wired from
// cmd/api/main.go). Unlike SetAfterRegister, a failing hook aborts the
// deletion — cleanup must succeed before the user row (and everything a
// solely-owned workspace's ON DELETE CASCADE hangs off it) is allowed to go.
func (s *Service) SetBeforeDeleteAccount(fn func(ctx context.Context, userID string) error) {
	s.beforeDeleteAccount = fn
}

// DeleteAccount permanently deletes userID's user row — and, via each
// table's ON DELETE CASCADE foreign key back to users(id), their refresh
// tokens and receipt-scan audit rows (Phase 8 self-service account
// deletion; see docs/privacy-policy.md §6). Their ledger data (accounts,
// categories, transactions) is handled by the beforeDeleteAccount hook, not
// a direct FK cascade from users: since Phase 9 that data belongs to a
// workspace, not the user directly, so a solely-owned workspace is deleted
// along with them, while a shared workspace survives for its other members.
// There is no undo and this package does not ask for confirmation — the
// caller (the HTTP handler, then the app's own confirmation UI) is
// responsible for making sure the user actually meant it before this is
// called.
//
// The user's already-issued access token(s) are not individually revoked —
// a JWT's signature still verifies fine after this runs, since RequireAuth
// never touches the database. What actually stops a stale token from being
// useful is that every row it could ever have pointed at is now gone: /auth/me
// 404s (the user row itself no longer exists) and every ledger/report/receipt
// call scoped to this user's ID returns empty or 404, exactly as if the user
// had simply never had any data. The token expires for good within 15
// minutes regardless (TokenIssuer.AccessTokenTTL).
func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	if s.beforeDeleteAccount != nil {
		if err := s.beforeDeleteAccount(ctx, userID); err != nil {
			return fmt.Errorf("clean up before account deletion: %w", err)
		}
	}
	return s.users.Delete(ctx, userID)
}

func (s *Service) issueSession(ctx context.Context, u users.User) (AuthResult, error) {
	result, _, err := s.issueSessionWithID(ctx, u)
	return result, err
}

func (s *Service) issueSessionWithID(ctx context.Context, u users.User) (AuthResult, string, error) {
	access, expiresAt, err := s.tokens.IssueAccessToken(u.ID)
	if err != nil {
		return AuthResult{}, "", err
	}

	rawRefresh, hash, err := NewRefreshToken()
	if err != nil {
		return AuthResult{}, "", err
	}

	stored, err := s.refresh.Create(ctx, RefreshToken{
		UserID:    u.ID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(RefreshTokenTTL),
	})
	if err != nil {
		return AuthResult{}, "", err
	}

	return AuthResult{
		User:            u,
		AccessToken:     access,
		AccessExpiresAt: expiresAt,
		RefreshToken:    rawRefresh,
	}, stored.ID, nil
}
