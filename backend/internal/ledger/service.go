package ledger

import (
	"context"
	"strings"
	"time"
)

const (
	defaultListLimit = 50
	maxListLimit     = 200
)

// Service orchestrates the three stores plus every validation and ownership
// rule from docs/business-rules.md. It is the only thing HTTP handlers talk
// to. Every method takes a workspaceID that the caller has already been
// verified to be a member of (internal/workspaces, checked in
// internal/server middleware before these ever run) — Service itself does
// not re-check membership, only that referenced rows belong to that same
// workspace.
type Service struct {
	accounts     AccountStore
	categories   CategoryStore
	transactions TransactionStore
}

func NewService(accounts AccountStore, categories CategoryStore, transactions TransactionStore) *Service {
	return &Service{accounts: accounts, categories: categories, transactions: transactions}
}

// SeedDefaultCategories gives a newly created workspace the starter category
// set. Called once, right after a workspace is created (personal, at
// registration, or a user-created shared one — see cmd/api/main.go and
// internal/workspaces.Service).
func (s *Service) SeedDefaultCategories(ctx context.Context, workspaceID, createdByUserID string) error {
	return s.categories.CreateMany(ctx, defaultCategoriesFor(workspaceID, createdByUserID))
}

// ----- Accounts -----

func (s *Service) CreateAccount(ctx context.Context, workspaceID, createdByUserID, name, currency string) (Account, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Account{}, ErrInvalidName
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return Account{}, ErrInvalidCurrency
	}
	return s.accounts.Create(ctx, Account{WorkspaceID: workspaceID, CreatedByUserID: createdByUserID, Name: name, Currency: currency})
}

func (s *Service) ListAccounts(ctx context.Context, workspaceID string) ([]AccountBalance, error) {
	return s.accounts.ListByWorkspace(ctx, workspaceID)
}

func (s *Service) GetAccount(ctx context.Context, workspaceID, id string) (AccountBalance, error) {
	a, err := s.accounts.GetByID(ctx, workspaceID, id)
	if err != nil {
		return AccountBalance{}, err
	}
	balance, err := s.accounts.GetBalance(ctx, workspaceID, id)
	if err != nil {
		return AccountBalance{}, err
	}
	return AccountBalance{Account: a, BalanceMinor: balance}, nil
}

// UpdateAccount applies whichever of name/archived is non-nil. Setting
// archived=true is the recommended way to retire an account that still has
// transaction history — see DeleteAccount.
func (s *Service) UpdateAccount(ctx context.Context, workspaceID, id string, name *string, archived *bool) (Account, error) {
	a, err := s.accounts.GetByID(ctx, workspaceID, id)
	if err != nil {
		return Account{}, err
	}

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return Account{}, ErrInvalidName
		}
		a.Name = trimmed
	}
	applyArchived(archived, &a.ArchivedAt)

	return s.accounts.Update(ctx, a)
}

// DeleteAccount hard-deletes only when nothing references it; otherwise the
// caller must archive it instead (docs/business-rules.md).
func (s *Service) DeleteAccount(ctx context.Context, workspaceID, id string) error {
	if _, err := s.accounts.GetByID(ctx, workspaceID, id); err != nil {
		return err
	}
	inUse, err := s.accounts.HasTransactions(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if inUse {
		return ErrInUse
	}
	return s.accounts.Delete(ctx, workspaceID, id)
}

// ----- Categories -----

func (s *Service) CreateCategory(ctx context.Context, workspaceID, createdByUserID, name string, kind Direction) (Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Category{}, ErrInvalidName
	}
	if !kind.Valid() {
		return Category{}, ErrInvalidDirection
	}
	return s.categories.Create(ctx, Category{WorkspaceID: workspaceID, CreatedByUserID: createdByUserID, Name: name, Kind: kind})
}

func (s *Service) ListCategories(ctx context.Context, workspaceID string) ([]Category, error) {
	return s.categories.ListByWorkspace(ctx, workspaceID)
}

func (s *Service) GetCategory(ctx context.Context, workspaceID, id string) (Category, error) {
	return s.categories.GetByID(ctx, workspaceID, id)
}

func (s *Service) UpdateCategory(ctx context.Context, workspaceID, id string, name *string, archived *bool) (Category, error) {
	c, err := s.categories.GetByID(ctx, workspaceID, id)
	if err != nil {
		return Category{}, err
	}

	if name != nil {
		trimmed := strings.TrimSpace(*name)
		if trimmed == "" {
			return Category{}, ErrInvalidName
		}
		c.Name = trimmed
	}
	applyArchived(archived, &c.ArchivedAt)

	return s.categories.Update(ctx, c)
}

// DeleteCategory mirrors DeleteAccount: refuses while any transaction still
// references the category.
func (s *Service) DeleteCategory(ctx context.Context, workspaceID, id string) error {
	if _, err := s.categories.GetByID(ctx, workspaceID, id); err != nil {
		return err
	}
	inUse, err := s.categories.HasTransactions(ctx, workspaceID, id)
	if err != nil {
		return err
	}
	if inUse {
		return ErrInUse
	}
	return s.categories.Delete(ctx, workspaceID, id)
}

// ----- Transactions -----

// CreateTransactionInput is everything CreateTransaction needs from a caller.
type CreateTransactionInput struct {
	AccountID   string
	CategoryID  string
	Direction   Direction
	AmountMinor int64
	Currency    string
	OccurredAt  time.Time
	Note        string
}

func (s *Service) CreateTransaction(ctx context.Context, workspaceID, createdByUserID string, in CreateTransactionInput) (Transaction, error) {
	if !in.Direction.Valid() {
		return Transaction{}, ErrInvalidDirection
	}
	if in.AmountMinor <= 0 {
		return Transaction{}, ErrInvalidAmount
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		return Transaction{}, ErrInvalidCurrency
	}

	if err := s.checkPostable(ctx, workspaceID, in.AccountID, in.CategoryID, in.Direction); err != nil {
		return Transaction{}, err
	}

	return s.transactions.Create(ctx, Transaction{
		WorkspaceID:     workspaceID,
		CreatedByUserID: createdByUserID,
		AccountID:       in.AccountID,
		CategoryID:      in.CategoryID,
		Direction:       in.Direction,
		AmountMinor:     in.AmountMinor,
		Currency:        currency,
		OccurredAt:      in.OccurredAt,
		Note:            strings.TrimSpace(in.Note),
	})
}

func (s *Service) GetTransaction(ctx context.Context, workspaceID, id string) (Transaction, error) {
	return s.transactions.GetByID(ctx, workspaceID, id)
}

func (s *Service) ListTransactions(ctx context.Context, workspaceID string, filter TransactionFilter) ([]Transaction, int, error) {
	if filter.Limit <= 0 || filter.Limit > maxListLimit {
		filter.Limit = defaultListLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.transactions.ListByWorkspace(ctx, workspaceID, filter)
}

// UpdateTransactionInput applies whichever fields are non-nil; AccountID,
// CategoryID, and Direction are re-validated together whenever any of them
// changes, since the category-kind/direction match is a cross-field rule.
type UpdateTransactionInput struct {
	AccountID   *string
	CategoryID  *string
	Direction   *Direction
	AmountMinor *int64
	OccurredAt  *time.Time
	Note        *string
}

func (s *Service) UpdateTransaction(ctx context.Context, workspaceID, id string, in UpdateTransactionInput) (Transaction, error) {
	t, err := s.transactions.GetByID(ctx, workspaceID, id)
	if err != nil {
		return Transaction{}, err
	}

	accountID, categoryID, direction := t.AccountID, t.CategoryID, t.Direction
	if in.AccountID != nil {
		accountID = *in.AccountID
	}
	if in.CategoryID != nil {
		categoryID = *in.CategoryID
	}
	if in.Direction != nil {
		direction = *in.Direction
	}
	if !direction.Valid() {
		return Transaction{}, ErrInvalidDirection
	}

	if err := s.checkPostable(ctx, workspaceID, accountID, categoryID, direction); err != nil {
		return Transaction{}, err
	}

	if in.AmountMinor != nil {
		if *in.AmountMinor <= 0 {
			return Transaction{}, ErrInvalidAmount
		}
		t.AmountMinor = *in.AmountMinor
	}
	if in.OccurredAt != nil {
		t.OccurredAt = *in.OccurredAt
	}
	if in.Note != nil {
		t.Note = strings.TrimSpace(*in.Note)
	}
	t.AccountID, t.CategoryID, t.Direction = accountID, categoryID, direction

	return s.transactions.Update(ctx, t)
}

func (s *Service) DeleteTransaction(ctx context.Context, workspaceID, id string) error {
	if _, err := s.transactions.GetByID(ctx, workspaceID, id); err != nil {
		return err
	}
	return s.transactions.SoftDelete(ctx, workspaceID, id)
}

// checkPostable verifies the account and category belong to workspaceID,
// are not archived, and that the category's kind matches the transaction's
// direction.
func (s *Service) checkPostable(ctx context.Context, workspaceID, accountID, categoryID string, direction Direction) error {
	account, err := s.accounts.GetByID(ctx, workspaceID, accountID)
	if err != nil {
		return err
	}
	if account.ArchivedAt != nil {
		return ErrAccountArchived
	}

	category, err := s.categories.GetByID(ctx, workspaceID, categoryID)
	if err != nil {
		return err
	}
	if category.ArchivedAt != nil {
		return ErrCategoryArchived
	}
	if category.Kind != direction {
		return ErrCategoryKindMismatch
	}

	return nil
}

// applyArchived sets or clears *field per archived, leaving it untouched if
// archived is nil or already in the requested state.
func applyArchived(archived *bool, field **time.Time) {
	if archived == nil {
		return
	}
	if *archived {
		if *field == nil {
			now := time.Now()
			*field = &now
		}
	} else {
		*field = nil
	}
}
