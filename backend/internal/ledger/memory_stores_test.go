package ledger

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// memoryDB backs all three in-memory fakes below with one shared set of maps,
// the same way a single Postgres database backs all three Postgres stores —
// so GetBalance/HasTransactions can see transactions created through the
// transaction fake.
type memoryDB struct {
	accounts     map[string]Account
	categories   map[string]Category
	transactions map[string]Transaction
	nextID       int
}

func newMemoryDB() *memoryDB {
	return &memoryDB{
		accounts:     map[string]Account{},
		categories:   map[string]Category{},
		transactions: map[string]Transaction{},
	}
}

func (d *memoryDB) genID(prefix string) string {
	d.nextID++
	return fmt.Sprintf("%s-%d", prefix, d.nextID)
}

func newTestService() *Service {
	db := newMemoryDB()
	return NewService(&memoryAccountStore{db}, &memoryCategoryStore{db}, &memoryTransactionStore{db})
}

// ----- accounts -----

type memoryAccountStore struct{ db *memoryDB }

func (s *memoryAccountStore) Create(_ context.Context, a Account) (Account, error) {
	a.ID = s.db.genID("acct")
	a.CreatedAt = time.Now()
	s.db.accounts[a.ID] = a
	return a, nil
}

func (s *memoryAccountStore) GetByID(_ context.Context, workspaceID, id string) (Account, error) {
	a, ok := s.db.accounts[id]
	if !ok || a.WorkspaceID != workspaceID {
		return Account{}, ErrNotFound
	}
	return a, nil
}

func (s *memoryAccountStore) ListByWorkspace(ctx context.Context, workspaceID string) ([]AccountBalance, error) {
	var result []AccountBalance
	for _, a := range s.db.accounts {
		if a.WorkspaceID != workspaceID {
			continue
		}
		balance, _ := s.GetBalance(ctx, workspaceID, a.ID)
		result = append(result, AccountBalance{Account: a, BalanceMinor: balance})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (s *memoryAccountStore) GetBalance(_ context.Context, workspaceID, id string) (int64, error) {
	var balance int64
	for _, t := range s.db.transactions {
		if t.AccountID != id || t.WorkspaceID != workspaceID || t.DeletedAt != nil {
			continue
		}
		if t.Direction == DirectionIncome {
			balance += t.AmountMinor
		} else {
			balance -= t.AmountMinor
		}
	}
	return balance, nil
}

func (s *memoryAccountStore) Update(_ context.Context, a Account) (Account, error) {
	existing, ok := s.db.accounts[a.ID]
	if !ok || existing.WorkspaceID != a.WorkspaceID {
		return Account{}, ErrNotFound
	}
	existing.Name = a.Name
	existing.ArchivedAt = a.ArchivedAt
	s.db.accounts[a.ID] = existing
	return existing, nil
}

func (s *memoryAccountStore) Delete(_ context.Context, workspaceID, id string) error {
	a, ok := s.db.accounts[id]
	if !ok || a.WorkspaceID != workspaceID {
		return ErrNotFound
	}
	delete(s.db.accounts, id)
	return nil
}

func (s *memoryAccountStore) HasTransactions(_ context.Context, workspaceID, id string) (bool, error) {
	for _, t := range s.db.transactions {
		if t.AccountID == id && t.WorkspaceID == workspaceID {
			return true, nil
		}
	}
	return false, nil
}

// ----- categories -----

type memoryCategoryStore struct{ db *memoryDB }

func (s *memoryCategoryStore) Create(_ context.Context, c Category) (Category, error) {
	c.ID = s.db.genID("cat")
	c.CreatedAt = time.Now()
	s.db.categories[c.ID] = c
	return c, nil
}

func (s *memoryCategoryStore) CreateMany(ctx context.Context, cs []Category) error {
	for _, c := range cs {
		if _, err := s.Create(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func (s *memoryCategoryStore) GetByID(_ context.Context, workspaceID, id string) (Category, error) {
	c, ok := s.db.categories[id]
	if !ok || c.WorkspaceID != workspaceID {
		return Category{}, ErrNotFound
	}
	return c, nil
}

func (s *memoryCategoryStore) ListByWorkspace(_ context.Context, workspaceID string) ([]Category, error) {
	var result []Category
	for _, c := range s.db.categories {
		if c.WorkspaceID == workspaceID {
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result, nil
}

func (s *memoryCategoryStore) Update(_ context.Context, c Category) (Category, error) {
	existing, ok := s.db.categories[c.ID]
	if !ok || existing.WorkspaceID != c.WorkspaceID {
		return Category{}, ErrNotFound
	}
	existing.Name = c.Name
	existing.ArchivedAt = c.ArchivedAt
	s.db.categories[c.ID] = existing
	return existing, nil
}

func (s *memoryCategoryStore) Delete(_ context.Context, workspaceID, id string) error {
	c, ok := s.db.categories[id]
	if !ok || c.WorkspaceID != workspaceID {
		return ErrNotFound
	}
	delete(s.db.categories, id)
	return nil
}

func (s *memoryCategoryStore) HasTransactions(_ context.Context, workspaceID, id string) (bool, error) {
	for _, t := range s.db.transactions {
		if t.CategoryID == id && t.WorkspaceID == workspaceID {
			return true, nil
		}
	}
	return false, nil
}

// ----- transactions -----

type memoryTransactionStore struct{ db *memoryDB }

func (s *memoryTransactionStore) Create(_ context.Context, t Transaction) (Transaction, error) {
	t.ID = s.db.genID("txn")
	t.CreatedAt = time.Now()
	s.db.transactions[t.ID] = t
	return t, nil
}

func (s *memoryTransactionStore) GetByID(_ context.Context, workspaceID, id string) (Transaction, error) {
	t, ok := s.db.transactions[id]
	if !ok || t.WorkspaceID != workspaceID || t.DeletedAt != nil {
		return Transaction{}, ErrNotFound
	}
	return t, nil
}

func (s *memoryTransactionStore) ListByWorkspace(_ context.Context, workspaceID string, filter TransactionFilter) ([]Transaction, int, error) {
	var all []Transaction
	for _, t := range s.db.transactions {
		if t.WorkspaceID != workspaceID || t.DeletedAt != nil {
			continue
		}
		if filter.AccountID != "" && t.AccountID != filter.AccountID {
			continue
		}
		if filter.CategoryID != "" && t.CategoryID != filter.CategoryID {
			continue
		}
		if filter.From != nil && t.OccurredAt.Before(*filter.From) {
			continue
		}
		if filter.To != nil && t.OccurredAt.After(*filter.To) {
			continue
		}
		all = append(all, t)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].OccurredAt.After(all[j].OccurredAt) })

	total := len(all)
	start := filter.Offset
	if start > total {
		start = total
	}
	end := start + filter.Limit
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}

func (s *memoryTransactionStore) Update(_ context.Context, t Transaction) (Transaction, error) {
	existing, ok := s.db.transactions[t.ID]
	if !ok || existing.WorkspaceID != t.WorkspaceID || existing.DeletedAt != nil {
		return Transaction{}, ErrNotFound
	}
	t.CreatedAt = existing.CreatedAt
	s.db.transactions[t.ID] = t
	return t, nil
}

func (s *memoryTransactionStore) SoftDelete(_ context.Context, workspaceID, id string) error {
	t, ok := s.db.transactions[id]
	if !ok || t.WorkspaceID != workspaceID || t.DeletedAt != nil {
		return ErrNotFound
	}
	now := time.Now()
	t.DeletedAt = &now
	s.db.transactions[id] = t
	return nil
}
