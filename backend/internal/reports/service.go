package reports

import (
	"context"
	"time"
)

const (
	defaultTrendMonths = 6
	maxTrendMonths     = 24
)

// Service resolves defaults/validates input, then delegates to Store.
type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

// Summary returns totals for [from, to]. A nil from defaults to the first
// day of the current month; a nil to defaults to today — together, "this
// month so far" when neither is given, matching the roadmap's "monthly
// summary" as the default view.
func (s *Service) Summary(ctx context.Context, workspaceID string, from, to *time.Time) (Summary, error) {
	start, end, err := s.resolveRange(from, to)
	if err != nil {
		return Summary{}, err
	}
	return s.store.Summary(ctx, workspaceID, start, end)
}

func (s *Service) ByCategory(ctx context.Context, workspaceID string, from, to *time.Time) ([]CategoryBreakdown, error) {
	start, end, err := s.resolveRange(from, to)
	if err != nil {
		return nil, err
	}
	return s.store.ByCategory(ctx, workspaceID, start, end)
}

// MonthlyTrend returns exactly `months` points, oldest first, ending with
// the current month — including months with no activity, since a chart
// needs a continuous x-axis rather than gaps.
func (s *Service) MonthlyTrend(ctx context.Context, workspaceID string, months int) ([]MonthlyTrendPoint, error) {
	if months <= 0 {
		months = defaultTrendMonths
	}
	if months > maxTrendMonths {
		return nil, ErrInvalidMonths
	}

	now := s.now().UTC()
	currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	oldestMonthStart := currentMonthStart.AddDate(0, -(months - 1), 0)
	rangeEnd := currentMonthStart.AddDate(0, 1, 0)

	labels := make([]string, months)
	for i := 0; i < months; i++ {
		labels[i] = oldestMonthStart.AddDate(0, i, 0).Format("2006-01")
	}

	byMonth, err := s.store.MonthlyTrend(ctx, workspaceID, labels, oldestMonthStart, rangeEnd)
	if err != nil {
		return nil, err
	}

	points := make([]MonthlyTrendPoint, months)
	for i, label := range labels {
		if p, ok := byMonth[label]; ok {
			points[i] = p
		} else {
			points[i] = MonthlyTrendPoint{Month: label}
		}
	}
	return points, nil
}

func (s *Service) resolveRange(from, to *time.Time) (time.Time, time.Time, error) {
	now := s.now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	if from != nil {
		start = *from
	}
	end := now
	if to != nil {
		end = *to
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, ErrInvalidRange
	}
	return start, end, nil
}
