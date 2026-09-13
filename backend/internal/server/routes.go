package server

import (
	"net/http"

	"golang.org/x/time/rate"

	"cashflow-backend/internal/auth"
	"cashflow-backend/internal/ledger"
	"cashflow-backend/internal/receipts"
	"cashflow-backend/internal/reports"
	"cashflow-backend/internal/workspaces"
)

// Deps are the dependencies NewRouter needs to wire up every route.
type Deps struct {
	Pinger           Pinger
	AuthService      *auth.Service
	TokenIssuer      *auth.TokenIssuer
	LedgerService    *ledger.Service
	ReceiptService   *receipts.Service
	ReportsService   *reports.Service
	WorkspaceService *workspaces.Service
}

// NewRouter builds the full request handler: routes wrapped in the standard
// middleware chain. Keep it a flat list here as long as it stays readable;
// split into per-feature route files only once it doesn't.
func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /readyz", readyHandler(deps.Pinger))
	mux.Handle("GET /metrics", MetricsHandler())

	// A stricter, dedicated limiter for register/login (2/min sustained, a
	// burst of 5) — Phase 7 hardening against credential-stuffing/signup-spam
	// bots, which target exactly these two endpoints. General traffic gets
	// the much looser default below.
	authLimiter := newIPRateLimiter(rate.Limit(2.0/60.0), 5)
	mux.Handle("POST /auth/register", withRateLimit(authLimiter, auth.RegisterHandler(deps.AuthService)))
	mux.Handle("POST /auth/login", withRateLimit(authLimiter, auth.LoginHandler(deps.AuthService)))
	mux.HandleFunc("POST /auth/refresh", auth.RefreshHandler(deps.AuthService))

	requireAuth := auth.RequireAuth(deps.TokenIssuer)
	mux.Handle("POST /auth/logout", requireAuth(auth.LogoutHandler(deps.AuthService)))
	mux.Handle("GET /auth/me", requireAuth(auth.MeHandler(deps.AuthService)))
	mux.Handle("DELETE /auth/me", requireAuth(auth.DeleteMeHandler(deps.AuthService)))

	// Workspace management itself — create/list/rename, membership, invites.
	// Each handler resolves and verifies the {id} workspace itself (owner vs.
	// member checks vary per action), so these only need requireAuth, not the
	// membership middleware below.
	mux.Handle("POST /workspaces", requireAuth(workspaces.CreateWorkspaceHandler(deps.WorkspaceService)))
	mux.Handle("GET /workspaces", requireAuth(workspaces.ListWorkspacesHandler(deps.WorkspaceService)))
	mux.Handle("PATCH /workspaces/{id}", requireAuth(workspaces.UpdateWorkspaceHandler(deps.WorkspaceService)))
	mux.Handle("GET /workspaces/{id}/members", requireAuth(workspaces.ListMembersHandler(deps.WorkspaceService)))
	mux.Handle("DELETE /workspaces/{id}/members/{userId}", requireAuth(workspaces.RemoveMemberHandler(deps.WorkspaceService)))
	mux.Handle("POST /workspaces/{id}/invites", requireAuth(workspaces.InviteMemberHandler(deps.WorkspaceService)))
	mux.Handle("POST /workspaces/invites/accept", requireAuth(workspaces.AcceptInviteHandler(deps.WorkspaceService)))

	// Ledger/reports routes are nested under a workspace and additionally
	// require the caller to be a member of it (requireMembership), which
	// injects the workspace ID that every handler below scopes by.
	requireMembership := workspaces.RequireMembership(deps.WorkspaceService)
	inWorkspace := func(h http.Handler) http.Handler { return requireAuth(requireMembership(h)) }

	mux.Handle("POST /workspaces/{workspaceId}/accounts", inWorkspace(ledger.CreateAccountHandler(deps.LedgerService)))
	mux.Handle("GET /workspaces/{workspaceId}/accounts", inWorkspace(ledger.ListAccountsHandler(deps.LedgerService)))
	mux.Handle("GET /workspaces/{workspaceId}/accounts/{id}", inWorkspace(ledger.GetAccountHandler(deps.LedgerService)))
	mux.Handle("PATCH /workspaces/{workspaceId}/accounts/{id}", inWorkspace(ledger.UpdateAccountHandler(deps.LedgerService)))
	mux.Handle("DELETE /workspaces/{workspaceId}/accounts/{id}", inWorkspace(ledger.DeleteAccountHandler(deps.LedgerService)))

	mux.Handle("POST /workspaces/{workspaceId}/categories", inWorkspace(ledger.CreateCategoryHandler(deps.LedgerService)))
	mux.Handle("GET /workspaces/{workspaceId}/categories", inWorkspace(ledger.ListCategoriesHandler(deps.LedgerService)))
	mux.Handle("PATCH /workspaces/{workspaceId}/categories/{id}", inWorkspace(ledger.UpdateCategoryHandler(deps.LedgerService)))
	mux.Handle("DELETE /workspaces/{workspaceId}/categories/{id}", inWorkspace(ledger.DeleteCategoryHandler(deps.LedgerService)))

	mux.Handle("POST /workspaces/{workspaceId}/transactions", inWorkspace(ledger.CreateTransactionHandler(deps.LedgerService)))
	mux.Handle("GET /workspaces/{workspaceId}/transactions", inWorkspace(ledger.ListTransactionsHandler(deps.LedgerService)))
	mux.Handle("GET /workspaces/{workspaceId}/transactions/{id}", inWorkspace(ledger.GetTransactionHandler(deps.LedgerService)))
	mux.Handle("PATCH /workspaces/{workspaceId}/transactions/{id}", inWorkspace(ledger.UpdateTransactionHandler(deps.LedgerService)))
	mux.Handle("DELETE /workspaces/{workspaceId}/transactions/{id}", inWorkspace(ledger.DeleteTransactionHandler(deps.LedgerService)))

	mux.Handle("GET /workspaces/{workspaceId}/reports/summary", inWorkspace(reports.SummaryHandler(deps.ReportsService)))
	mux.Handle("GET /workspaces/{workspaceId}/reports/by-category", inWorkspace(reports.ByCategoryHandler(deps.ReportsService)))
	mux.Handle("GET /workspaces/{workspaceId}/reports/monthly-trend", inWorkspace(reports.MonthlyTrendHandler(deps.ReportsService)))

	// Receipt scanning stays scoped by the calling user, not a workspace: its
	// rate limiting is per-user by design (docs/adr/0002-ai-receipt-scanning.md)
	// and it never writes a transaction itself, so it needs no workspace at all.
	mux.Handle("POST /receipts/scan", requireAuth(receipts.ScanHandler(deps.ReceiptService)))

	// A much looser general limit (10/sec, burst 20 per IP) across every
	// route, on top of the stricter one above — a backstop against generic
	// abuse/runaway clients, not meant to bother a real user of the app.
	generalLimiter := newIPRateLimiter(10, 20)
	return withMiddleware(withRateLimit(generalLimiter, mux))
}
