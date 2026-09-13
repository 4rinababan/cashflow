package ledger

import (
	"context"
	"time"
)

// Transaction is one posted income or expense. Amounts are fixed-point
// (AmountMinor, integer minor currency units) — never a float, per
// docs/business-rules.md. Deleting one is a soft delete (DeletedAt set) so
// reports can still be reconstructed from history. Scoped by WorkspaceID
// like Account/Category — any member of the workspace can see/edit it,
// not just whoever created it; CreatedByUserID is who actually did,
// recorded for display only.
type Transaction struct {
	ID              string
	WorkspaceID     string
	CreatedByUserID string
	AccountID       string
	CategoryID      string
	Direction       Direction
	AmountMinor     int64
	Currency        string
	OccurredAt      time.Time
	Note            string
	DeletedAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       *time.Time
}

// TransactionFilter narrows ListByWorkspace. Zero values mean "no filter" for
// AccountID/CategoryID/From/To; Limit/Offset are normalized by Service before
// reaching the store.
type TransactionFilter struct {
	AccountID  string
	CategoryID string
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

// TransactionStore persists transactions, scoped by WorkspaceID like the
// other stores. ListByWorkspace returns the page plus the total matching
// count (for pagination), never including soft-deleted rows.
type TransactionStore interface {
	Create(ctx context.Context, t Transaction) (Transaction, error)
	GetByID(ctx context.Context, workspaceID, id string) (Transaction, error)
	ListByWorkspace(ctx context.Context, workspaceID string, filter TransactionFilter) ([]Transaction, int, error)
	Update(ctx context.Context, t Transaction) (Transaction, error)
	SoftDelete(ctx context.Context, workspaceID, id string) error
}
