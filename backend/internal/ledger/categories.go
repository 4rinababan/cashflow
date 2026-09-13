package ledger

import (
	"context"
	"time"
)

// Direction is a category's kind, or the direction of money on a transaction.
// A transaction's Direction must match its Category's Kind — enforced by
// Service, not the database.
type Direction string

const (
	DirectionIncome  Direction = "income"
	DirectionExpense Direction = "expense"
)

// Valid reports whether d is one of the two allowed directions.
func (d Direction) Valid() bool {
	return d == DirectionIncome || d == DirectionExpense
}

// Category groups transactions (e.g. "Food", "Salary"). A category cannot be
// hard-deleted while any transaction references it — archive instead (see
// docs/business-rules.md). Scoped by WorkspaceID like Account.
type Category struct {
	ID              string
	WorkspaceID     string
	CreatedByUserID string
	Name            string
	Kind            Direction
	ArchivedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       *time.Time
}

// CategoryStore persists categories, scoped by WorkspaceID like AccountStore.
type CategoryStore interface {
	Create(ctx context.Context, c Category) (Category, error)
	CreateMany(ctx context.Context, cs []Category) error
	GetByID(ctx context.Context, workspaceID, id string) (Category, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]Category, error)
	Update(ctx context.Context, c Category) (Category, error)
	Delete(ctx context.Context, workspaceID, id string) error
	HasTransactions(ctx context.Context, workspaceID, id string) (bool, error)
}

// defaultCategoriesFor is the starter set every new workspace gets at
// creation (docs/business-rules.md: "ship a small set of sensible
// defaults"). Members can rename, archive, or add their own on top.
func defaultCategoriesFor(workspaceID, createdByUserID string) []Category {
	seed := []struct {
		name string
		kind Direction
	}{
		{"Salary", DirectionIncome},
		{"Other Income", DirectionIncome},
		{"Food", DirectionExpense},
		{"Transport", DirectionExpense},
		{"Bills", DirectionExpense},
		{"Other Expense", DirectionExpense},
	}

	cats := make([]Category, 0, len(seed))
	for _, s := range seed {
		cats = append(cats, Category{WorkspaceID: workspaceID, CreatedByUserID: createdByUserID, Name: s.name, Kind: s.kind})
	}
	return cats
}
