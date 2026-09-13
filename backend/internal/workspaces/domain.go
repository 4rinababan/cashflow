// Package workspaces lets more than one user share the same ledger — the
// family/UMKM case (docs/business-rules.md). A workspace is what
// accounts/categories/transactions actually belong to (internal/ledger);
// a user can be a member of more than one. Every brand-new user gets
// exactly one workspace automatically at registration (their personal one)
// so solo use needs nothing extra — sharing is opt-in via an invite.
package workspaces

import (
	"context"
	"time"
)

// Role is a member's permission level within one workspace. Deliberately
// just two: Owner is whoever created the workspace — permanent for v1, no
// ownership transfer, no removing an owner — and can invite/remove members
// and rename the workspace; Member can read/write ledger data but cannot
// manage membership.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
)

type Workspace struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type Member struct {
	WorkspaceID string
	UserID      string
	Role        Role
	CreatedAt   time.Time
}

// MemberWithUser is a Member plus enough of the user's own profile to show
// in a member list, without every caller having to join against users
// separately.
type MemberWithUser struct {
	Member
	Email       string
	DisplayName string
}

// WorkspaceWithRole is one workspace plus the calling user's role in it —
// what ListWorkspacesForUser returns, since a workspace switcher needs both.
type WorkspaceWithRole struct {
	Workspace
	Role Role
}

// Invite is a pending (or already-used) invitation to join a workspace by
// email. The raw token is emailed to the invitee and never stored — only
// its hash is, matching this codebase's existing refresh-token discipline
// (internal/auth.RefreshToken).
type Invite struct {
	ID          string
	WorkspaceID string
	Email       string
	TokenHash   string
	InvitedBy   string
	ExpiresAt   time.Time
	AcceptedAt  *time.Time
	CreatedAt   time.Time
}

func (i Invite) isUsable(now time.Time) bool {
	return i.AcceptedAt == nil && now.Before(i.ExpiresAt)
}

// WorkspaceStore persists workspaces themselves.
type WorkspaceStore interface {
	Create(ctx context.Context, w Workspace) (Workspace, error)
	GetByID(ctx context.Context, id string) (Workspace, error)
	UpdateName(ctx context.Context, id, name string) (Workspace, error)
	// Delete removes a workspace outright — only ever called for one whose
	// sole member is about to have their own account deleted (see
	// Service.RemoveUserFromAllWorkspaces). Its ledger data cascades away
	// with it (ON DELETE CASCADE on workspace_id, migrations/0005).
	Delete(ctx context.Context, id string) error
}

// MemberStore persists workspace membership.
type MemberStore interface {
	Add(ctx context.Context, m Member) error
	Remove(ctx context.Context, workspaceID, userID string) error
	Get(ctx context.Context, workspaceID, userID string) (Member, error)
	ListForWorkspace(ctx context.Context, workspaceID string) ([]MemberWithUser, error)
	ListForUser(ctx context.Context, userID string) ([]WorkspaceWithRole, error)
}

// InviteStore persists workspace invites.
type InviteStore interface {
	Create(ctx context.Context, i Invite) (Invite, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (Invite, error)
	MarkAccepted(ctx context.Context, id string) error
}
