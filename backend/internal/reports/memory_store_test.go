package reports

import (
	"context"
	"time"
)

// memoryStore is an in-memory Store fake for Service tests, seeded directly
// with the aggregates it should return — internal/ledger already owns and
// tests the real aggregation logic (SQL run against Postgres, verified via
// the Docker e2e check); this fake only needs to prove Service's own
// default-range/validation/month-window behavior.
type memoryStore struct {
	summary     Summary
	summaryErr  error
	byCategory  []CategoryBreakdown
	trendPoints map[string]MonthlyTrendPoint

	gotWorkspaceID        string
	gotFrom, gotTo        time.Time
	gotMonths             []string
	gotRangeStart, gotEnd time.Time
}

func (s *memoryStore) Summary(_ context.Context, workspaceID string, from, to time.Time) (Summary, error) {
	s.gotWorkspaceID, s.gotFrom, s.gotTo = workspaceID, from, to
	if s.summaryErr != nil {
		return Summary{}, s.summaryErr
	}
	result := s.summary
	result.From, result.To = from, to
	return result, nil
}

func (s *memoryStore) ByCategory(_ context.Context, workspaceID string, from, to time.Time) ([]CategoryBreakdown, error) {
	s.gotWorkspaceID, s.gotFrom, s.gotTo = workspaceID, from, to
	return s.byCategory, nil
}

func (s *memoryStore) MonthlyTrend(_ context.Context, workspaceID string, months []string, rangeStart, rangeEnd time.Time) (map[string]MonthlyTrendPoint, error) {
	s.gotWorkspaceID, s.gotMonths, s.gotRangeStart, s.gotEnd = workspaceID, months, rangeStart, rangeEnd
	if s.trendPoints == nil {
		return map[string]MonthlyTrendPoint{}, nil
	}
	return s.trendPoints, nil
}
