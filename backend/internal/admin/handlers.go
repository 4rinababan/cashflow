package admin

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

type dashboardData struct {
	Cashflow     CashflowStats
	Gawean       GaweanStats
	Errors       []ErrorLogEntry
	DailyChart   template.HTML
	WeeklyChart  template.HTML
	MonthlyChart template.HTML
}

// DashboardHandler renders the whole operator dashboard as one self-contained
// HTML page (inline CSS, inline SVG charts, zero external requests) — an
// internal tool, not something that needs a build pipeline.
func DashboardHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		cfStats, err := svc.CashflowStats(ctx)
		if err != nil {
			http.Error(w, "failed to load Cashflow stats: "+err.Error(), http.StatusInternalServerError)
			return
		}
		gwStats, err := svc.GaweanStats(ctx)
		if err != nil {
			// Degrade, don't fail the whole page — the rest of the dashboard
			// is still useful even if the sibling app's DB is unreachable.
			gwStats = GaweanStats{Connected: false}
		}
		errors, err := svc.RecentErrors(50)
		if err != nil {
			errors = nil
		}

		data := dashboardData{
			Cashflow:     cfStats,
			Gawean:       gwStats,
			Errors:       errors,
			DailyChart:   barChartSVG(cfStats.SignupsDaily, "#1f6f5c"),
			WeeklyChart:  barChartSVG(cfStats.SignupsWeekly, "#1f6f5c"),
			MonthlyChart: barChartSVG(cfStats.SignupsMonthly, "#1f6f5c"),
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := dashboardTemplate.Execute(w, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// StatsAPIHandler serves the same numbers as plain JSON, for anyone who'd
// rather poll it than look at the HTML (e.g. a future proper dashboard).
func StatsAPIHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		cfStats, err := svc.CashflowStats(ctx)
		if err != nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		gwStats, _ := svc.GaweanStats(ctx)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"cashflow": cfStats,
			"gawean":   gwStats,
		})
	}
}

// barChartSVG renders a minimal, dependency-free bar chart. Each bar carries
// a <title> tooltip (label + count) since there's no JS here to build a
// richer hover — plain SVG native tooltips are the honest baseline.
func barChartSVG(points []SeriesPoint, color string) template.HTML {
	if len(points) == 0 {
		return template.HTML(`<p class="empty">No data yet.</p>`)
	}
	const width, height, barGap = 720, 160, 4
	barWidth := float64(width) / float64(len(points))

	max := 1
	for _, p := range points {
		if p.Count > max {
			max = p.Count
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" class="chart" role="img" aria-label="signup chart">`, width, height+24)
	for i, p := range points {
		barHeight := float64(p.Count) / float64(max) * float64(height-4)
		x := float64(i) * barWidth
		y := float64(height) - barHeight
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s" rx="2"><title>%s: %d</title></rect>`,
			x+barGap/2, y, barWidth-barGap, barHeight+1, color, template.HTMLEscapeString(p.Label), p.Count)
		// Label every Nth bar so long series don't overlap.
		step := len(points)/12 + 1
		if i%step == 0 {
			fmt.Fprintf(&b, `<text x="%.1f" y="%d" class="chart-label">%s</text>`,
				x+barWidth/2, height+16, template.HTMLEscapeString(p.Label))
		}
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String())
}

var dashboardTemplate = template.Must(template.New("dashboard").Parse(`<!doctype html>
<html lang="en" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>Cashflow — Admin</title>
<style>
  :root {
    --bg: #101512; --surface: #171e1a; --border: #2a332c; --text: #e7ece7;
    --muted: #93a397; --accent: #59c1a3; --danger: #e79483;
  }
  * { box-sizing: border-box; }
  body {
    background: var(--bg); color: var(--text); margin: 0; padding: 24px;
    font-family: ui-sans-serif, system-ui, sans-serif; font-size: 14px;
  }
  h1 { font-size: 1.3rem; margin: 0 0 4px; }
  h2 { font-size: 0.95rem; text-transform: uppercase; letter-spacing: 0.05em; color: var(--muted); margin: 0 0 12px; }
  .sub { color: var(--muted); margin: 0 0 24px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; margin-bottom: 28px; }
  .tile { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; padding: 16px; }
  .tile .n { font-size: 1.8rem; font-weight: 600; font-variant-numeric: tabular-nums; }
  .tile .l { color: var(--muted); font-size: 0.8rem; margin-top: 2px; }
  .tile.disconnected { opacity: 0.6; }
  section { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; padding: 20px; margin-bottom: 20px; }
  .chart { width: 100%; height: auto; }
  .chart-label { fill: var(--muted); font-size: 8px; text-anchor: middle; }
  .empty { color: var(--muted); }
  table { width: 100%; border-collapse: collapse; font-size: 0.85rem; }
  th { text-align: left; color: var(--muted); font-weight: 500; padding: 6px 8px; border-bottom: 1px solid var(--border); }
  td { padding: 6px 8px; border-bottom: 1px solid var(--border); vertical-align: top; }
  td.app { white-space: nowrap; font-family: ui-monospace, monospace; }
  td.msg { font-family: ui-monospace, monospace; font-size: 0.78rem; color: var(--danger); white-space: pre-wrap; word-break: break-word; }
  td.ts { white-space: nowrap; color: var(--muted); font-variant-numeric: tabular-nums; }
</style>
</head>
<body>
  <h1>Cashflow — Admin dashboard</h1>
  <p class="sub">Internal only. Not linked from any public page.</p>

  <div class="grid">
    <div class="tile"><div class="n">{{.Cashflow.TotalUsers}}</div><div class="l">Cashflow — total users</div></div>
    <div class="tile"><div class="n">{{.Cashflow.TotalWorkspaces}}</div><div class="l">Cashflow — total workspaces</div></div>
    <div class="tile"><div class="n">{{.Cashflow.ActiveUsers7Days}}</div><div class="l">Cashflow — active users (7d)</div></div>
    {{if .Gawean.Connected}}
    <div class="tile"><div class="n">{{.Gawean.TotalUsers}}</div><div class="l">Gawean — total users</div></div>
    {{else}}
    <div class="tile disconnected"><div class="n">—</div><div class="l">Gawean — not connected (set GAWEAN_DATABASE_URL)</div></div>
    {{end}}
  </div>

  <section>
    <h2>New signups — daily (30d)</h2>
    {{.DailyChart}}
  </section>
  <section>
    <h2>New signups — weekly (12w)</h2>
    {{.WeeklyChart}}
  </section>
  <section>
    <h2>New signups — monthly (12mo)</h2>
    {{.MonthlyChart}}
  </section>

  <section>
    <h2>Recent errors (both apps)</h2>
    {{if .Errors}}
    <table>
      <thead><tr><th>When</th><th>App</th><th>Message</th></tr></thead>
      <tbody>
        {{range .Errors}}
        <tr>
          <td class="ts">{{.Timestamp.Format "2006-01-02 15:04:05"}}</td>
          <td class="app">{{.App}}</td>
          <td class="msg">{{.Message}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
    {{else}}
    <p class="empty">No errors logged yet — either everything's fine, or scripts/log-watch.py hasn't run yet.</p>
    {{end}}
  </section>
</body>
</html>
`))
