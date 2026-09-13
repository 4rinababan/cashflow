package workspaces

import "errors"

var (
	// ErrNotFound covers "doesn't exist" and "caller isn't a member" —
	// deliberately the same error so a lookup never confirms a workspace a
	// stranger isn't part of even exists (same anti-enumeration discipline
	// as internal/ledger).
	ErrNotFound = errors.New("not found")
	// ErrForbidden means the caller is a member but lacks the role
	// required for this action (owner-only actions attempted by a member).
	ErrForbidden         = errors.New("only the workspace owner can do this")
	ErrInvalidName       = errors.New("name is required")
	ErrAlreadyAMember    = errors.New("that person is already a member of this workspace")
	ErrCannotRemoveOwner = errors.New("the workspace owner cannot be removed")
	ErrInviteInvalid     = errors.New("this invite is invalid, expired, or already used")
)
