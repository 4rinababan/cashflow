package workspaces

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"cashflow-backend/internal/mail"
)

// InviteTTL is how long an emailed invite link stays valid.
const InviteTTL = 7 * 24 * time.Hour

// Service orchestrates workspaces, membership, and invites. It is the only
// thing HTTP handlers talk to.
type Service struct {
	workspaces WorkspaceStore
	members    MemberStore
	invites    InviteStore
	mailer     mail.Mailer
	now        func() time.Time
	// AppBaseURL is where an invite link points the invitee — e.g.
	// "https://app.cashflow.example" — set from config; the invite path
	// itself is app-defined (the mobile app / a web landing page owns it).
	AppBaseURL string
}

func NewService(workspaces WorkspaceStore, members MemberStore, invites InviteStore, mailer mail.Mailer) *Service {
	return &Service{workspaces: workspaces, members: members, invites: invites, mailer: mailer, now: time.Now}
}

// CreatePersonalWorkspace is called once, right after registration succeeds
// (see cmd/api/main.go's afterRegister hook) — every user gets exactly one
// of these automatically, so solo use requires no extra setup.
func (s *Service) CreatePersonalWorkspace(ctx context.Context, userID, displayName string) (Workspace, error) {
	return s.createWorkspaceWithOwner(ctx, userID, displayName+"'s Workspace")
}

// CreateWorkspace is user-initiated — e.g. a UMKM owner creating a shared
// workspace beyond their personal one.
func (s *Service) CreateWorkspace(ctx context.Context, ownerUserID, name string) (Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workspace{}, ErrInvalidName
	}
	return s.createWorkspaceWithOwner(ctx, ownerUserID, name)
}

func (s *Service) createWorkspaceWithOwner(ctx context.Context, ownerUserID, name string) (Workspace, error) {
	created, err := s.workspaces.Create(ctx, Workspace{Name: name})
	if err != nil {
		return Workspace{}, err
	}
	if err := s.members.Add(ctx, Member{WorkspaceID: created.ID, UserID: ownerUserID, Role: RoleOwner}); err != nil {
		return Workspace{}, err
	}
	return created, nil
}

// ListWorkspacesForUser returns every workspace userID belongs to, with
// their role in each — what a workspace switcher needs.
func (s *Service) ListWorkspacesForUser(ctx context.Context, userID string) ([]WorkspaceWithRole, error) {
	return s.members.ListForUser(ctx, userID)
}

// RequireMembership returns the caller's membership row, or ErrNotFound if
// they are not a member (never distinguishing "doesn't exist" from "you're
// not in it") — every ledger/report/receipt request calls this first (see
// internal/server middleware wiring).
func (s *Service) RequireMembership(ctx context.Context, workspaceID, userID string) (Member, error) {
	return s.members.Get(ctx, workspaceID, userID)
}

func (s *Service) ListMembers(ctx context.Context, workspaceID, callerUserID string) ([]MemberWithUser, error) {
	if _, err := s.RequireMembership(ctx, workspaceID, callerUserID); err != nil {
		return nil, err
	}
	return s.members.ListForWorkspace(ctx, workspaceID)
}

func (s *Service) RenameWorkspace(ctx context.Context, workspaceID, callerUserID, name string) (Workspace, error) {
	if err := s.requireOwner(ctx, workspaceID, callerUserID); err != nil {
		return Workspace{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Workspace{}, ErrInvalidName
	}
	return s.workspaces.UpdateName(ctx, workspaceID, name)
}

// RemoveMember lets an owner remove a member (never the owner themselves —
// v1 has no ownership transfer and no "leave" flow, see docs/roadmap.md).
func (s *Service) RemoveMember(ctx context.Context, workspaceID, callerUserID, targetUserID string) error {
	if err := s.requireOwner(ctx, workspaceID, callerUserID); err != nil {
		return err
	}
	target, err := s.members.Get(ctx, workspaceID, targetUserID)
	if err != nil {
		return err
	}
	if target.Role == RoleOwner {
		return ErrCannotRemoveOwner
	}
	return s.members.Remove(ctx, workspaceID, targetUserID)
}

// InviteMember creates an invite and emails it to email. Owner-only.
func (s *Service) InviteMember(ctx context.Context, workspaceID, callerUserID, email string) error {
	if err := s.requireOwner(ctx, workspaceID, callerUserID); err != nil {
		return err
	}

	workspace, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return err
	}

	rawToken, tokenHash, err := newInviteToken()
	if err != nil {
		return err
	}

	if _, err := s.invites.Create(ctx, Invite{
		WorkspaceID: workspaceID,
		Email:       strings.ToLower(strings.TrimSpace(email)),
		TokenHash:   tokenHash,
		InvitedBy:   callerUserID,
		ExpiresAt:   s.now().Add(InviteTTL),
	}); err != nil {
		return err
	}

	return s.mailer.Send(ctx, inviteEmail(email, workspace.Name, s.AppBaseURL, rawToken))
}

// AcceptInvite adds acceptingUserID to the invite's workspace as a member.
// The invite must be unexpired and not already used; accepting it twice (or
// with a wrong/expired token) returns the same ErrInviteInvalid either way.
func (s *Service) AcceptInvite(ctx context.Context, rawToken, acceptingUserID string) (Workspace, error) {
	invite, err := s.invites.GetByTokenHash(ctx, hashInviteToken(rawToken))
	if err != nil {
		return Workspace{}, ErrInviteInvalid
	}
	if !invite.isUsable(s.now()) {
		return Workspace{}, ErrInviteInvalid
	}

	if _, err := s.members.Get(ctx, invite.WorkspaceID, acceptingUserID); err == nil {
		return Workspace{}, ErrAlreadyAMember
	}

	if err := s.members.Add(ctx, Member{WorkspaceID: invite.WorkspaceID, UserID: acceptingUserID, Role: RoleMember}); err != nil {
		return Workspace{}, err
	}
	if err := s.invites.MarkAccepted(ctx, invite.ID); err != nil {
		return Workspace{}, err
	}

	return s.workspaces.GetByID(ctx, invite.WorkspaceID)
}

// RemoveUserFromAllWorkspaces is called once, just before userID's account is
// permanently deleted (see internal/auth.Service.DeleteAccount, wired from
// cmd/api/main.go). For every workspace they belong to: if they were its
// only member, the whole workspace — and, via ON DELETE CASCADE on
// workspace_id (migrations/0005), every account/category/transaction in it —
// is deleted along with them, since nobody else could ever reach it anyway.
// If other members remain, this leaves the workspace and its data alone;
// the user's own membership row disappears on its own once the user row
// itself is deleted (ON DELETE CASCADE on workspace_members.user_id).
func (s *Service) RemoveUserFromAllWorkspaces(ctx context.Context, userID string) error {
	memberships, err := s.members.ListForUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, ws := range memberships {
		members, err := s.members.ListForWorkspace(ctx, ws.ID)
		if err != nil {
			return err
		}
		if len(members) <= 1 {
			if err := s.workspaces.Delete(ctx, ws.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) requireOwner(ctx context.Context, workspaceID, userID string) error {
	member, err := s.RequireMembership(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if member.Role != RoleOwner {
		return ErrForbidden
	}
	return nil
}

func newInviteToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashInviteToken(raw), nil
}

func hashInviteToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func inviteEmail(to, workspaceName, appBaseURL, rawToken string) mail.Message {
	link := fmt.Sprintf("%s/invites/%s", strings.TrimSuffix(appBaseURL, "/"), rawToken)
	return mail.Message{
		To:      to,
		Subject: fmt.Sprintf("You're invited to join %q on Cashflow", workspaceName),
		HTMLBody: fmt.Sprintf(
			`<p>You've been invited to join <strong>%s</strong> on Cashflow.</p>`+
				`<p><a href="%s">Accept the invite</a> (expires in 7 days).</p>`+
				`<p>If you weren't expecting this, you can ignore this email.</p>`,
			workspaceName, link,
		),
	}
}
