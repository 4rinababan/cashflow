// Package receipts turns a photographed receipt into a draft transaction via
// an AI model, never saving anything to the ledger itself —
// the caller (the existing POST /transactions endpoint) does that once the
// user reviews and confirms. See docs/adr/0002-ai-receipt-scanning.md.
package receipts

import (
	"context"
	"time"
)

// LineItem is one line the model read off the receipt.
type LineItem struct {
	Description string
	AmountMinor int64
}

// Draft is what a scan returns: never a database row, only a suggestion for
// the app to pre-fill into the manual "new transaction" form (Phase 3) —
// the user must still press Save.
type Draft struct {
	Merchant      string
	OccurredAt    *time.Time
	Currency      string
	TotalMinor    int64
	CategoryGuess string
	LineItems     []LineItem
	// Confidence is the model's own 0.0-1.0 estimate that the extraction is
	// correct. The server never rejects a low-confidence result outright —
	// docs/adr/0002 requires falling back gracefully, not blocking the user —
	// it is passed through so the app can flag it for extra scrutiny.
	Confidence float64
}

// ScanStatus is what got recorded about one scan attempt, for rate-limiting
// and abuse visibility only — never the extracted content itself.
type ScanStatus string

const (
	ScanStatusOK     ScanStatus = "ok"
	ScanStatusFailed ScanStatus = "failed"
)

// ScanRecord is one audit-log row.
type ScanRecord struct {
	UserID    string
	Status    ScanStatus
	ErrorCode string
}

// ScanStore persists the audit log used to enforce the per-user daily limit.
type ScanStore interface {
	CountSince(ctx context.Context, userID string, since time.Time) (int, error)
	Record(ctx context.Context, r ScanRecord) error
}

// Scanner calls out to the AI model. OpenAIScanner is the real
// implementation; tests use a fake so they never need a live API key.
type Scanner interface {
	Scan(ctx context.Context, image []byte, mimeType string) (Draft, error)
}
