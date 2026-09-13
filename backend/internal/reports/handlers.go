package reports

import (
	"net/http"
	"strconv"
	"time"

	"cashflow-backend/internal/workspaces"
)

const dateLayout = "2006-01-02"

type summaryResponse struct {
	From             string `json:"from"`
	To               string `json:"to"`
	IncomeMinor      int64  `json:"incomeMinor"`
	ExpenseMinor     int64  `json:"expenseMinor"`
	NetMinor         int64  `json:"netMinor"`
	TransactionCount int    `json:"transactionCount"`
}

func toSummaryResponse(s Summary) summaryResponse {
	return summaryResponse{
		From:             s.From.Format(dateLayout),
		To:               s.To.Format(dateLayout),
		IncomeMinor:      s.IncomeMinor,
		ExpenseMinor:     s.ExpenseMinor,
		NetMinor:         s.IncomeMinor - s.ExpenseMinor,
		TransactionCount: s.TransactionCount,
	}
}

type categoryBreakdownResponse struct {
	CategoryID       string `json:"categoryId"`
	CategoryName     string `json:"categoryName"`
	Kind             string `json:"kind"`
	TotalMinor       int64  `json:"totalMinor"`
	TransactionCount int    `json:"transactionCount"`
}

type byCategoryResponse struct {
	Categories []categoryBreakdownResponse `json:"categories"`
}

type monthlyTrendPointResponse struct {
	Month        string `json:"month"`
	IncomeMinor  int64  `json:"incomeMinor"`
	ExpenseMinor int64  `json:"expenseMinor"`
	NetMinor     int64  `json:"netMinor"`
}

// parseDateRange reads optional "from"/"to" query params (YYYY-MM-DD),
// returning nil for whichever is absent so Service can apply its defaults.
func parseDateRange(w http.ResponseWriter, r *http.Request) (from, to *time.Time, ok bool) {
	q := r.URL.Query()
	if raw := q.Get("from"); raw != "" {
		parsed, err := time.Parse(dateLayout, raw)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, errorBody("from must be a YYYY-MM-DD date"))
			return nil, nil, false
		}
		from = &parsed
	}
	if raw := q.Get("to"); raw != "" {
		parsed, err := time.Parse(dateLayout, raw)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, errorBody("to must be a YYYY-MM-DD date"))
			return nil, nil, false
		}
		to = &parsed
	}
	return from, to, true
}

// SummaryHandler handles GET /reports/summary?from=&to= — defaults to "this
// month so far" when neither is given.
func SummaryHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, ok := workspaces.WorkspaceIDFromContext(r.Context())
		if !ok {
			respondJSON(w, http.StatusUnauthorized, errorBody("not authenticated"))
			return
		}
		from, to, ok := parseDateRange(w, r)
		if !ok {
			return
		}

		summary, err := svc.Summary(r.Context(), workspaceID, from, to)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, toSummaryResponse(summary))
	}
}

// ByCategoryHandler handles GET /reports/by-category?from=&to=.
func ByCategoryHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, ok := workspaces.WorkspaceIDFromContext(r.Context())
		if !ok {
			respondJSON(w, http.StatusUnauthorized, errorBody("not authenticated"))
			return
		}
		from, to, ok := parseDateRange(w, r)
		if !ok {
			return
		}

		breakdown, err := svc.ByCategory(r.Context(), workspaceID, from, to)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		items := make([]categoryBreakdownResponse, 0, len(breakdown))
		for _, b := range breakdown {
			items = append(items, categoryBreakdownResponse{
				CategoryID:       b.CategoryID,
				CategoryName:     b.CategoryName,
				Kind:             b.Kind,
				TotalMinor:       b.TotalMinor,
				TransactionCount: b.TransactionCount,
			})
		}
		respondJSON(w, http.StatusOK, byCategoryResponse{Categories: items})
	}
}

// MonthlyTrendHandler handles GET /reports/monthly-trend?months=N (default 6, max 24).
func MonthlyTrendHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID, ok := workspaces.WorkspaceIDFromContext(r.Context())
		if !ok {
			respondJSON(w, http.StatusUnauthorized, errorBody("not authenticated"))
			return
		}

		months := 0
		if raw := r.URL.Query().Get("months"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				respondJSON(w, http.StatusBadRequest, errorBody("months must be an integer"))
				return
			}
			months = n
		}

		points, err := svc.MonthlyTrend(r.Context(), workspaceID, months)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		resp := make([]monthlyTrendPointResponse, 0, len(points))
		for _, p := range points {
			resp = append(resp, monthlyTrendPointResponse{
				Month:        p.Month,
				IncomeMinor:  p.IncomeMinor,
				ExpenseMinor: p.ExpenseMinor,
				NetMinor:     p.IncomeMinor - p.ExpenseMinor,
			})
		}
		respondJSON(w, http.StatusOK, resp)
	}
}
