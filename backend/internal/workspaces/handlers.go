package workspaces

import (
	"net/http"
	"time"

	"cashflow-backend/internal/auth"
)

type workspaceResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role,omitempty"`
	CreatedAt string `json:"createdAt"`
}

func toWorkspaceResponse(w Workspace, role Role) workspaceResponse {
	return workspaceResponse{
		ID:        w.ID,
		Name:      w.Name,
		Role:      string(role),
		CreatedAt: w.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type memberResponse struct {
	UserID      string `json:"userId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	CreatedAt   string `json:"createdAt"`
}

func toMemberResponse(m MemberWithUser) memberResponse {
	return memberResponse{
		UserID:      m.UserID,
		Email:       m.Email,
		DisplayName: m.DisplayName,
		Role:        string(m.Role),
		CreatedAt:   m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func requireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		respondJSON(w, http.StatusUnauthorized, errorBody("not authenticated"))
		return "", false
	}
	return userID, true
}

type createWorkspaceRequest struct {
	Name string `json:"name"`
}

// CreateWorkspaceHandler handles POST /workspaces: the caller becomes Owner.
func CreateWorkspaceHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		var req createWorkspaceRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		created, err := svc.CreateWorkspace(r.Context(), userID, req.Name)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		respondJSON(w, http.StatusCreated, toWorkspaceResponse(created, RoleOwner))
	}
}

// ListWorkspacesHandler handles GET /workspaces: every workspace the caller
// belongs to, with their role in each — what a workspace switcher needs.
func ListWorkspacesHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}

		list, err := svc.ListWorkspacesForUser(r.Context(), userID)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		items := make([]workspaceResponse, 0, len(list))
		for _, wr := range list {
			items = append(items, toWorkspaceResponse(wr.Workspace, wr.Role))
		}
		respondJSON(w, http.StatusOK, items)
	}
}

type updateWorkspaceRequest struct {
	Name string `json:"name"`
}

// UpdateWorkspaceHandler handles PATCH /workspaces/{id}: rename. Owner-only.
func UpdateWorkspaceHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		var req updateWorkspaceRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		updated, err := svc.RenameWorkspace(r.Context(), r.PathValue("id"), userID, req.Name)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, toWorkspaceResponse(updated, RoleOwner))
	}
}

// ListMembersHandler handles GET /workspaces/{id}/members. Any member.
func ListMembersHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}

		members, err := svc.ListMembers(r.Context(), r.PathValue("id"), userID)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		items := make([]memberResponse, 0, len(members))
		for _, m := range members {
			items = append(items, toMemberResponse(m))
		}
		respondJSON(w, http.StatusOK, items)
	}
}

// RemoveMemberHandler handles DELETE /workspaces/{id}/members/{userId}. Owner-only.
func RemoveMemberHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}

		err := svc.RemoveMember(r.Context(), r.PathValue("id"), userID, r.PathValue("userId"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type inviteRequest struct {
	Email string `json:"email"`
}

// InviteMemberHandler handles POST /workspaces/{id}/invites. Owner-only;
// emails the invite (internal/mail).
func InviteMemberHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		var req inviteRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Email == "" {
			respondJSON(w, http.StatusBadRequest, errorBody("email is required"))
			return
		}

		if err := svc.InviteMember(r.Context(), r.PathValue("id"), userID, req.Email); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

type acceptInviteRequest struct {
	Token string `json:"token"`
}

// AcceptInviteHandler handles POST /workspaces/invites/accept. Any
// authenticated user; joins them to the invite's workspace as a Member.
func AcceptInviteHandler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		var req acceptInviteRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		workspace, err := svc.AcceptInvite(r.Context(), req.Token, userID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, toWorkspaceResponse(workspace, RoleMember))
	}
}
