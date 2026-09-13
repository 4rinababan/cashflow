package ledger

import "errors"

var (
	// ErrNotFound covers "doesn't exist" and "belongs to someone else" —
	// deliberately the same error so a lookup never confirms another user's
	// row exists (docs/business-rules.md: 404, not 403).
	ErrNotFound             = errors.New("not found")
	ErrInvalidName          = errors.New("name is required")
	ErrInvalidCurrency      = errors.New("currency is required")
	ErrInvalidDirection     = errors.New("direction must be income or expense")
	ErrInvalidAmount        = errors.New("amount must be greater than zero")
	ErrCategoryKindMismatch = errors.New("transaction direction does not match the category's kind")
	ErrAccountArchived      = errors.New("account is archived")
	ErrCategoryArchived     = errors.New("category is archived")
	ErrInUse                = errors.New("still referenced by transactions; archive it instead of deleting")
)
