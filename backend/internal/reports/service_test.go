package reports

import (
	"context"
	"errors"
	"testing"
	"time"
)

func fixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestService_Summary_DefaultsToTheCurrentMonthSoFarWhenNoRangeIsGiven(t *testing.T) {
	store := &memoryStore{}
	svc := NewService(store)
	svc.now = fixedNow(time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC))

	if _, err := svc.Summary(context.Background(), "user-1", nil, nil); err != nil {
		t.Fatalf("Summary() error = %v", err)
	}

	wantFrom := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	if !store.gotFrom.Equal(wantFrom) {
		t.Errorf("from = %v, want %v (first of the current month)", store.gotFrom, wantFrom)
	}
	if !store.gotTo.Equal(wantTo) {
		t.Errorf("to = %v, want %v (now)", store.gotTo, wantTo)
	}
}

func TestService_Summary_HonoursAnExplicitRange(t *testing.T) {
	store := &memoryStore{}
	svc := NewService(store)

	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
	if _, err := svc.Summary(context.Background(), "user-1", &from, &to); err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if !store.gotFrom.Equal(from) || !store.gotTo.Equal(to) {
		t.Errorf("range = [%v, %v], want [%v, %v]", store.gotFrom, store.gotTo, from, to)
	}
}

func TestService_Summary_RejectsAToBeforeFrom(t *testing.T) {
	svc := NewService(&memoryStore{})
	from := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	if _, err := svc.Summary(context.Background(), "user-1", &from, &to); !errors.Is(err, ErrInvalidRange) {
		t.Errorf("error = %v, want ErrInvalidRange", err)
	}
}

func TestService_Summary_NetIsIncomeMinusExpenseAtTheHandlerLevel(t *testing.T) {
	// Net is computed in the HTTP response layer, not Store — Summary itself
	// only carries income/expense; this documents that split so a future
	// change doesn't accidentally compute Net twice or drop it.
	store := &memoryStore{summary: Summary{IncomeMinor: 100, ExpenseMinor: 40}}
	svc := NewService(store)

	got, err := svc.Summary(context.Background(), "user-1", nil, nil)
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if got.IncomeMinor != 100 || got.ExpenseMinor != 40 {
		t.Errorf("got = %+v", got)
	}
}

func TestService_ByCategory_UsesTheSameDefaultRangeAsSummary(t *testing.T) {
	store := &memoryStore{byCategory: []CategoryBreakdown{{CategoryName: "Food", TotalMinor: 500}}}
	svc := NewService(store)
	svc.now = fixedNow(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC))

	got, err := svc.ByCategory(context.Background(), "user-1", nil, nil)
	if err != nil {
		t.Fatalf("ByCategory() error = %v", err)
	}
	if len(got) != 1 || got[0].CategoryName != "Food" {
		t.Errorf("got = %+v", got)
	}
	wantFrom := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !store.gotFrom.Equal(wantFrom) {
		t.Errorf("from = %v, want %v", store.gotFrom, wantFrom)
	}
}

func TestService_MonthlyTrend_DefaultsToSixMonths(t *testing.T) {
	store := &memoryStore{}
	svc := NewService(store)
	svc.now = fixedNow(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC))

	points, err := svc.MonthlyTrend(context.Background(), "user-1", 0)
	if err != nil {
		t.Fatalf("MonthlyTrend() error = %v", err)
	}
	if len(points) != 6 {
		t.Fatalf("len(points) = %d, want 6", len(points))
	}
	want := []string{"2025-10", "2025-11", "2025-12", "2026-01", "2026-02", "2026-03"}
	for i, p := range points {
		if p.Month != want[i] {
			t.Errorf("points[%d].Month = %q, want %q", i, p.Month, want[i])
		}
	}
}

func TestService_MonthlyTrend_FillsMonthsWithNoActivityAsZero(t *testing.T) {
	store := &memoryStore{trendPoints: map[string]MonthlyTrendPoint{
		"2026-03": {Month: "2026-03", IncomeMinor: 1000, ExpenseMinor: 200},
	}}
	svc := NewService(store)
	svc.now = fixedNow(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))

	points, err := svc.MonthlyTrend(context.Background(), "user-1", 2)
	if err != nil {
		t.Fatalf("MonthlyTrend() error = %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("len(points) = %d, want 2", len(points))
	}
	if points[0].Month != "2026-02" || points[0].IncomeMinor != 0 || points[0].ExpenseMinor != 0 {
		t.Errorf("points[0] = %+v, want a zeroed February", points[0])
	}
	if points[1].IncomeMinor != 1000 || points[1].ExpenseMinor != 200 {
		t.Errorf("points[1] = %+v, want March's real totals", points[1])
	}
}

func TestService_MonthlyTrend_RejectsMoreThanTheMax(t *testing.T) {
	svc := NewService(&memoryStore{})
	if _, err := svc.MonthlyTrend(context.Background(), "user-1", 25); !errors.Is(err, ErrInvalidMonths) {
		t.Errorf("error = %v, want ErrInvalidMonths", err)
	}
}
