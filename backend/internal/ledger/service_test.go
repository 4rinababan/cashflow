package ledger

import (
	"context"
	"errors"
	"testing"
	"time"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(dateLayout, s)
	if err != nil {
		t.Fatalf("parse date %q: %v", s, err)
	}
	return parsed
}

// ----- Accounts -----

func TestService_CreateAccount_Succeeds(t *testing.T) {
	svc := newTestService()
	a, err := svc.CreateAccount(context.Background(), "user-1", "user-1", "Cash", "idr")
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if a.Currency != "IDR" {
		t.Errorf("Currency = %q, want upper-cased IDR", a.Currency)
	}
}

func TestService_CreateAccount_RejectsAnEmptyName(t *testing.T) {
	svc := newTestService()
	if _, err := svc.CreateAccount(context.Background(), "user-1", "user-1", "  ", "IDR"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("error = %v, want ErrInvalidName", err)
	}
}

func TestService_GetAccount_NeverReturnsAnotherUsersAccount(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a, err := svc.CreateAccount(ctx, "user-a", "user-a", "Cash", "IDR")
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}

	if _, err := svc.GetAccount(ctx, "user-b", a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("user-b GetAccount(user-a's account) error = %v, want ErrNotFound", err)
	}
	if _, err := svc.GetAccount(ctx, "user-a", a.ID); err != nil {
		t.Errorf("user-a GetAccount(own account) error = %v, want nil", err)
	}
}

func TestService_UpdateAccount_RenamesAndArchives(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a, _ := svc.CreateAccount(ctx, "user-1", "user-1", "Cash", "IDR")

	newName := "Wallet"
	archived := true
	updated, err := svc.UpdateAccount(ctx, "user-1", a.ID, &newName, &archived)
	if err != nil {
		t.Fatalf("UpdateAccount() error = %v", err)
	}
	if updated.Name != "Wallet" || updated.ArchivedAt == nil {
		t.Errorf("updated = %+v, want renamed and archived", updated)
	}

	unarchived := false
	updated, err = svc.UpdateAccount(ctx, "user-1", a.ID, nil, &unarchived)
	if err != nil {
		t.Fatalf("UpdateAccount() unarchive error = %v", err)
	}
	if updated.ArchivedAt != nil {
		t.Errorf("ArchivedAt = %v, want nil after unarchiving", updated.ArchivedAt)
	}
}

func TestService_DeleteAccount_FailsWhileTransactionsReferenceIt(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a, _ := svc.CreateAccount(ctx, "user-1", "user-1", "Cash", "IDR")
	c, _ := svc.CreateCategory(ctx, "user-1", "user-1", "Salary", DirectionIncome)

	if _, err := svc.CreateTransaction(ctx, "user-1", "user-1", CreateTransactionInput{
		AccountID: a.ID, CategoryID: c.ID, Direction: DirectionIncome,
		AmountMinor: 10000, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-01"),
	}); err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}

	if err := svc.DeleteAccount(ctx, "user-1", a.ID); !errors.Is(err, ErrInUse) {
		t.Errorf("DeleteAccount() error = %v, want ErrInUse", err)
	}
}

func TestService_DeleteAccount_SucceedsWhenUnreferenced(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a, _ := svc.CreateAccount(ctx, "user-1", "user-1", "Cash", "IDR")

	if err := svc.DeleteAccount(ctx, "user-1", a.ID); err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}
	if _, err := svc.GetAccount(ctx, "user-1", a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetAccount() after delete error = %v, want ErrNotFound", err)
	}
}

// ----- Categories -----

func TestService_SeedDefaultCategories_CreatesTheStarterSet(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	if err := svc.SeedDefaultCategories(ctx, "user-1", "user-1"); err != nil {
		t.Fatalf("SeedDefaultCategories() error = %v", err)
	}

	cats, err := svc.ListCategories(ctx, "user-1")
	if err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}
	if len(cats) != 6 {
		t.Fatalf("len(cats) = %d, want 6", len(cats))
	}
}

func TestService_CreateCategory_RejectsAnInvalidKind(t *testing.T) {
	svc := newTestService()
	if _, err := svc.CreateCategory(context.Background(), "user-1", "user-1", "Food", Direction("sideways")); !errors.Is(err, ErrInvalidDirection) {
		t.Errorf("error = %v, want ErrInvalidDirection", err)
	}
}

func TestService_DeleteCategory_FailsWhileTransactionsReferenceIt(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a, _ := svc.CreateAccount(ctx, "user-1", "user-1", "Cash", "IDR")
	c, _ := svc.CreateCategory(ctx, "user-1", "user-1", "Food", DirectionExpense)

	if _, err := svc.CreateTransaction(ctx, "user-1", "user-1", CreateTransactionInput{
		AccountID: a.ID, CategoryID: c.ID, Direction: DirectionExpense,
		AmountMinor: 5000, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-02"),
	}); err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}

	if err := svc.DeleteCategory(ctx, "user-1", c.ID); !errors.Is(err, ErrInUse) {
		t.Errorf("DeleteCategory() error = %v, want ErrInUse", err)
	}
}

// ----- Transactions -----

func TestService_CreateTransaction_RejectsADirectionCategoryKindMismatch(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a, _ := svc.CreateAccount(ctx, "user-1", "user-1", "Cash", "IDR")
	salary, _ := svc.CreateCategory(ctx, "user-1", "user-1", "Salary", DirectionIncome)

	_, err := svc.CreateTransaction(ctx, "user-1", "user-1", CreateTransactionInput{
		AccountID: a.ID, CategoryID: salary.ID, Direction: DirectionExpense,
		AmountMinor: 1000, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-01"),
	})
	if !errors.Is(err, ErrCategoryKindMismatch) {
		t.Errorf("error = %v, want ErrCategoryKindMismatch", err)
	}
}

func TestService_CreateTransaction_RejectsANonPositiveAmount(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a, _ := svc.CreateAccount(ctx, "user-1", "user-1", "Cash", "IDR")
	c, _ := svc.CreateCategory(ctx, "user-1", "user-1", "Food", DirectionExpense)

	_, err := svc.CreateTransaction(ctx, "user-1", "user-1", CreateTransactionInput{
		AccountID: a.ID, CategoryID: c.ID, Direction: DirectionExpense,
		AmountMinor: 0, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-01"),
	})
	if !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("error = %v, want ErrInvalidAmount", err)
	}
}

func TestService_CreateTransaction_RejectsAnArchivedAccount(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a, _ := svc.CreateAccount(ctx, "user-1", "user-1", "Cash", "IDR")
	c, _ := svc.CreateCategory(ctx, "user-1", "user-1", "Food", DirectionExpense)
	archived := true
	if _, err := svc.UpdateAccount(ctx, "user-1", a.ID, nil, &archived); err != nil {
		t.Fatalf("archive account: %v", err)
	}

	_, err := svc.CreateTransaction(ctx, "user-1", "user-1", CreateTransactionInput{
		AccountID: a.ID, CategoryID: c.ID, Direction: DirectionExpense,
		AmountMinor: 1000, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-01"),
	})
	if !errors.Is(err, ErrAccountArchived) {
		t.Errorf("error = %v, want ErrAccountArchived", err)
	}
}

func TestService_CreateTransaction_RejectsAnotherUsersAccountOrCategory(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a, _ := svc.CreateAccount(ctx, "user-a", "user-a", "Cash", "IDR")
	c, _ := svc.CreateCategory(ctx, "user-a", "user-a", "Food", DirectionExpense)

	_, err := svc.CreateTransaction(ctx, "user-b", "user-b", CreateTransactionInput{
		AccountID: a.ID, CategoryID: c.ID, Direction: DirectionExpense,
		AmountMinor: 1000, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-01"),
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound (user-b must not be able to post against user-a's account)", err)
	}
}

func TestService_AccountBalance_ReflectsIncomeAndExpenseAndIgnoresDeletedRows(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a, _ := svc.CreateAccount(ctx, "user-1", "user-1", "Cash", "IDR")
	salary, _ := svc.CreateCategory(ctx, "user-1", "user-1", "Salary", DirectionIncome)
	food, _ := svc.CreateCategory(ctx, "user-1", "user-1", "Food", DirectionExpense)

	income, err := svc.CreateTransaction(ctx, "user-1", "user-1", CreateTransactionInput{
		AccountID: a.ID, CategoryID: salary.ID, Direction: DirectionIncome,
		AmountMinor: 100000, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-01"),
	})
	if err != nil {
		t.Fatalf("CreateTransaction(income) error = %v", err)
	}
	if _, err := svc.CreateTransaction(ctx, "user-1", "user-1", CreateTransactionInput{
		AccountID: a.ID, CategoryID: food.ID, Direction: DirectionExpense,
		AmountMinor: 30000, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-02"),
	}); err != nil {
		t.Fatalf("CreateTransaction(expense) error = %v", err)
	}
	// A soft-deleted expense must not affect the balance.
	toDelete, err := svc.CreateTransaction(ctx, "user-1", "user-1", CreateTransactionInput{
		AccountID: a.ID, CategoryID: food.ID, Direction: DirectionExpense,
		AmountMinor: 999999, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-03"),
	})
	if err != nil {
		t.Fatalf("CreateTransaction(to-delete) error = %v", err)
	}
	if err := svc.DeleteTransaction(ctx, "user-1", toDelete.ID); err != nil {
		t.Fatalf("DeleteTransaction() error = %v", err)
	}

	got, err := svc.GetAccount(ctx, "user-1", a.ID)
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	want := int64(100000 - 30000)
	if got.BalanceMinor != want {
		t.Errorf("BalanceMinor = %d, want %d (soft-deleted transaction must be excluded)", got.BalanceMinor, want)
	}
	_ = income
}

func TestService_UpdateTransaction_RevalidatesAgainstTheNewCategory(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a, _ := svc.CreateAccount(ctx, "user-1", "user-1", "Cash", "IDR")
	food, _ := svc.CreateCategory(ctx, "user-1", "user-1", "Food", DirectionExpense)
	salary, _ := svc.CreateCategory(ctx, "user-1", "user-1", "Salary", DirectionIncome)

	txn, err := svc.CreateTransaction(ctx, "user-1", "user-1", CreateTransactionInput{
		AccountID: a.ID, CategoryID: food.ID, Direction: DirectionExpense,
		AmountMinor: 1000, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-01"),
	})
	if err != nil {
		t.Fatalf("CreateTransaction() error = %v", err)
	}

	// Moving it to an income category without also changing direction must fail.
	newCategoryID := salary.ID
	if _, err := svc.UpdateTransaction(ctx, "user-1", txn.ID, UpdateTransactionInput{CategoryID: &newCategoryID}); !errors.Is(err, ErrCategoryKindMismatch) {
		t.Errorf("error = %v, want ErrCategoryKindMismatch", err)
	}

	// Changing both together is fine.
	newDirection := DirectionIncome
	updated, err := svc.UpdateTransaction(ctx, "user-1", txn.ID, UpdateTransactionInput{CategoryID: &newCategoryID, Direction: &newDirection})
	if err != nil {
		t.Fatalf("UpdateTransaction() error = %v", err)
	}
	if updated.CategoryID != salary.ID || updated.Direction != DirectionIncome {
		t.Errorf("updated = %+v, want moved to salary/income", updated)
	}
}

func TestService_ListTransactions_FiltersByAccountAndPaginates(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a1, _ := svc.CreateAccount(ctx, "user-1", "user-1", "Cash", "IDR")
	a2, _ := svc.CreateAccount(ctx, "user-1", "user-1", "Bank", "IDR")
	food, _ := svc.CreateCategory(ctx, "user-1", "user-1", "Food", DirectionExpense)

	for i := 0; i < 3; i++ {
		if _, err := svc.CreateTransaction(ctx, "user-1", "user-1", CreateTransactionInput{
			AccountID: a1.ID, CategoryID: food.ID, Direction: DirectionExpense,
			AmountMinor: 1000, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-01"),
		}); err != nil {
			t.Fatalf("seed a1 transaction: %v", err)
		}
	}
	if _, err := svc.CreateTransaction(ctx, "user-1", "user-1", CreateTransactionInput{
		AccountID: a2.ID, CategoryID: food.ID, Direction: DirectionExpense,
		AmountMinor: 2000, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-01"),
	}); err != nil {
		t.Fatalf("seed a2 transaction: %v", err)
	}

	items, total, err := svc.ListTransactions(ctx, "user-1", TransactionFilter{AccountID: a1.ID})
	if err != nil {
		t.Fatalf("ListTransactions() error = %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Errorf("total = %d, len(items) = %d, want 3 and 3", total, len(items))
	}

	paged, total, err := svc.ListTransactions(ctx, "user-1", TransactionFilter{AccountID: a1.ID, Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("ListTransactions() paged error = %v", err)
	}
	if total != 3 || len(paged) != 2 {
		t.Errorf("paged: total = %d, len(items) = %d, want 3 and 2", total, len(paged))
	}
}

func TestService_DeleteTransaction_IsASoftDelete(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	a, _ := svc.CreateAccount(ctx, "user-1", "user-1", "Cash", "IDR")
	c, _ := svc.CreateCategory(ctx, "user-1", "user-1", "Food", DirectionExpense)
	txn, _ := svc.CreateTransaction(ctx, "user-1", "user-1", CreateTransactionInput{
		AccountID: a.ID, CategoryID: c.ID, Direction: DirectionExpense,
		AmountMinor: 1000, Currency: "IDR", OccurredAt: mustDate(t, "2026-01-01"),
	})

	if err := svc.DeleteTransaction(ctx, "user-1", txn.ID); err != nil {
		t.Fatalf("DeleteTransaction() error = %v", err)
	}
	if _, err := svc.GetTransaction(ctx, "user-1", txn.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTransaction() after delete error = %v, want ErrNotFound", err)
	}
	// The account itself is now free to hard-delete... no it is not: the
	// soft-deleted row still references it, and HasTransactions does not
	// distinguish soft-deleted rows (the FK still exists) — this is
	// deliberate: history is kept, so the account isn't either.
	if err := svc.DeleteAccount(ctx, "user-1", a.ID); !errors.Is(err, ErrInUse) {
		t.Errorf("DeleteAccount() after soft-deleting its only transaction: error = %v, want ErrInUse", err)
	}
}
