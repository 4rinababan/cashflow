package workspaces

import (
	"context"
	"net/http"

	"cashflow-backend/internal/auth"
)

type contextKey string

const (
	workspaceIDContextKey contextKey = "workspace_id"
	roleContextKey        contextKey = "workspace_role"
)

// RequireMembership resolves {workspaceId} from the request path and
// verifies the already-authenticated caller (auth.RequireAuth must run
// first) is a member of it, putting the workspace ID and the caller's role
// in context for every ledger/reports/receipts handler nested under
// /workspaces/{workspaceId}/... A non-member gets 404, never a
// distinguishable 403 — same anti-enumeration discipline as everything else
// in this API.
func RequireMembership(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := auth.UserIDFromContext(r.Context())
			if !ok {
				respondJSON(w, http.StatusUnauthorized, errorBody("not authenticated"))
				return
			}

			workspaceID := r.PathValue("workspaceId")
			member, err := svc.RequireMembership(r.Context(), workspaceID, userID)
			if err != nil {
				writeServiceError(w, err)
				return
			}

			ctx := context.WithValue(r.Context(), workspaceIDContextKey, workspaceID)
			ctx = context.WithValue(ctx, roleContextKey, member.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WorkspaceIDFromContext returns the workspace ID set by RequireMembership.
func WorkspaceIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(workspaceIDContextKey).(string)
	return id, ok
}

// RoleFromContext returns the caller's role in the current workspace, set
// by RequireMembership.
func RoleFromContext(ctx context.Context) (Role, bool) {
	role, ok := ctx.Value(roleContextKey).(Role)
	return role, ok
}
