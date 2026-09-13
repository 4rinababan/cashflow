// Package reports answers read-only questions over the ledger (Phase 3):
// how much came in/out over a period, broken down by category, and how that
// trends month over month. It never writes anything — it exists purely to
// aggregate what internal/ledger already owns.
package reports

import (
	"context"
	"time"
)

// Summary is totals over [From, To] (inclusive), across every account.
type Summary struct {
	From             time.Time
	To               time.Time
	IncomeMinor      int64
	ExpenseMinor     int64
	TransactionCount int
}

// CategoryBreakdown is one category's activity over the same period as a Summary.
type CategoryBreakdown struct {
	CategoryID       string
	CategoryName     string
	Kind             string // "income" | "expense"
	TotalMinor       int64
	TransactionCount int
}

// MonthlyTrendPoint is one calendar month's totals, keyed "YYYY-MM". A trend
// always returns one point per requested month, including months with no
// activity (zero totals) — a chart needs a continuous x-axis.
type MonthlyTrendPoint struct {
	Month        string
	IncomeMinor  int64
	ExpenseMinor int64
}

// Store computes the aggregates. The Postgres implementation queries
// internal/ledger's transactions/categories tables directly — reports has no
// tables of its own.
type Store interface {
	Summary(ctx context.Context, workspaceID string, from, to time.Time) (Summary, error)
	ByCategory(ctx context.Context, workspaceID string, from, to time.Time) ([]CategoryBreakdown, error)
	// MonthlyTrend returns activity for each of the given months (labeled
	// "YYYY-MM", oldest first) ending with the month containing "now".
	MonthlyTrend(ctx context.Context, workspaceID string, months []string, rangeStart, rangeEnd time.Time) (map[string]MonthlyTrendPoint, error)
}
