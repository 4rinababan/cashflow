// Package ledger owns the core money-tracking domain: accounts, categories,
// and transactions. It knows nothing about HTTP, receipt scanning, or
// workspace membership itself; it depends only on being told a workspace ID
// to scope by, already verified as one the caller belongs to (upstream,
// internal/workspaces via internal/server middleware).
package ledger

import (
	"context"
	"time"
)

// Account is a workspace's money container (e.g. "Cash", "BCA") — not a
// login account. Scoped by WorkspaceID, not directly by user: since Phase 9
// more than one user can share a workspace (docs/business-rules.md).
// CreatedByUserID records who actually created the row, for display only —
// it plays no part in access control.
type Account struct {
	ID              string
	WorkspaceID     string
	CreatedByUserID string
	Name            string
	Currency        string
	ArchivedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       *time.Time
}

// AccountBalance is an Account plus its running balance: the sum of every
// non-deleted transaction posted to it (income adds, expense subtracts), in
// the same fixed-point minor units as Transaction.AmountMinor.
type AccountBalance struct {
	Account
	BalanceMinor int64
}

// AccountStore persists accounts. Every method is scoped by WorkspaceID so a
// lookup for a workspace the caller isn't a member of behaves exactly like a
// lookup for a row that doesn't exist (ErrNotFound) — never a distinguishable
// 403. Workspace membership itself is verified upstream, before any of
// these run.
type AccountStore interface {
	Create(ctx context.Context, a Account) (Account, error)
	GetByID(ctx context.Context, workspaceID, id string) (Account, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]AccountBalance, error)
	GetBalance(ctx context.Context, workspaceID, id string) (int64, error)
	Update(ctx context.Context, a Account) (Account, error)
	Delete(ctx context.Context, workspaceID, id string) error
	HasTransactions(ctx context.Context, workspaceID, id string) (bool, error)
}
