package admin

import (
	"context"
)

// SeriesPoint is one bar in a signup-growth chart — Label is a pre-formatted
// date (e.g. "2026-09-14", "2026-W37", "2026-09"), Count is zero-filled for
// gaps so a chart's x-axis stays continuous (same convention as
// internal/reports' monthly trend).
type SeriesPoint struct {
	Label string
	Count int
}

// CashflowStats is everything the dashboard shows about Cashflow itself —
// always available, no optional/degraded state, since this project owns
// its own database outright.
type CashflowStats struct {
	TotalUsers       int
	TotalWorkspaces  int
	ActiveUsers7Days int
	SignupsDaily     []SeriesPoint // last 30 days
	SignupsWeekly    []SeriesPoint // last 12 weeks
	SignupsMonthly   []SeriesPoint // last 12 months
}

func (s *Service) CashflowStats(ctx context.Context) (CashflowStats, error) {
	var stats CashflowStats

	if err := s.cashflow.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&stats.TotalUsers); err != nil {
		return stats, err
	}
	if err := s.cashflow.QueryRow(ctx, `SELECT count(*) FROM workspaces`).Scan(&stats.TotalWorkspaces); err != nil {
		return stats, err
	}
	if err := s.cashflow.QueryRow(ctx, `
		SELECT count(DISTINCT created_by_user_id) FROM transactions
		WHERE created_at >= now() - interval '7 days'
	`).Scan(&stats.ActiveUsers7Days); err != nil {
		return stats, err
	}

	var err error
	stats.SignupsDaily, err = s.signupSeries(ctx, "day", "29 days", "YYYY-MM-DD")
	if err != nil {
		return stats, err
	}
	stats.SignupsWeekly, err = s.signupSeries(ctx, "week", "11 weeks", "IYYY-\"W\"IW")
	if err != nil {
		return stats, err
	}
	stats.SignupsMonthly, err = s.signupSeries(ctx, "month", "11 months", "YYYY-MM")
	if err != nil {
		return stats, err
	}
	return stats, nil
}

// signupSeries returns a zero-filled series of new-signup counts, one point
// per unit (day/week/month) from `lookback` ago through now, labeled with
// format. unit and format are always one of the fixed literals this file
// calls it with, never request input — safe to interpolate directly.
func (s *Service) signupSeries(ctx context.Context, unit, lookback, format string) ([]SeriesPoint, error) {
	query := `
		SELECT to_char(d.bucket, '` + format + `') AS label, COALESCE(u.cnt, 0) AS count
		FROM generate_series(
			date_trunc('` + unit + `', now()) - interval '` + lookback + `',
			date_trunc('` + unit + `', now()),
			interval '1 ` + unit + `'
		) AS d(bucket)
		LEFT JOIN (
			SELECT date_trunc('` + unit + `', created_at) AS bucket, count(*) AS cnt
			FROM users
			GROUP BY 1
		) u ON u.bucket = d.bucket
		ORDER BY d.bucket
	`
	rows, err := s.cashflow.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []SeriesPoint
	for rows.Next() {
		var p SeriesPoint
		if err := rows.Scan(&p.Label, &p.Count); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

// GaweanStats is a best-effort snapshot of the sibling gawean-management
// app — Connected is false whenever GAWEAN_DATABASE_URL isn't configured
// (or the connection failed at startup), in which case the dashboard shows
// that plainly rather than a zero that could be mistaken for a real count.
// No growth chart: ASP.NET Identity's AspNetUsers table has no signup
// timestamp column in that app's current schema, so there is no history to
// chart — only ever a current total.
type GaweanStats struct {
	Connected  bool
	TotalUsers int
}

func (s *Service) GaweanStats(ctx context.Context) (GaweanStats, error) {
	if s.gawean == nil {
		return GaweanStats{Connected: false}, nil
	}
	var total int
	if err := s.gawean.QueryRow(ctx, `SELECT count(*) FROM "AspNetUsers"`).Scan(&total); err != nil {
		return GaweanStats{Connected: false}, err
	}
	return GaweanStats{Connected: true, TotalUsers: total}, nil
}
