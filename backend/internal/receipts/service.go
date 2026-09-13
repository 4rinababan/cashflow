package receipts

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// maxImageBytes caps the uploaded photo — receipts are small, and this also
// bounds the size of the base64 payload sent to the AI endpoint.
const maxImageBytes = 8 * 1024 * 1024

var supportedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// Service enforces the daily rate limit, calls the Scanner, and records
// every attempt (success or failure) to ScanStore for that limit and for
// abuse visibility — never anything extracted from the receipt itself.
type Service struct {
	scanner    Scanner
	store      ScanStore
	dailyLimit int
	now        func() time.Time
}

func NewService(scanner Scanner, store ScanStore, dailyLimit int) *Service {
	return &Service{scanner: scanner, store: store, dailyLimit: dailyLimit, now: time.Now}
}

// Scan validates the upload, enforces the per-user daily limit, and — only
// if both pass — calls the model. It never touches the transactions ledger;
// the caller decides whether/how to save the draft (via the existing
// POST /transactions endpoint), per docs/adr/0002-ai-receipt-scanning.md.
func (s *Service) Scan(ctx context.Context, userID string, image []byte, mimeType string) (Draft, error) {
	if len(image) == 0 || !supportedMimeTypes[mimeType] {
		return Draft{}, ErrUnsupportedImage
	}
	if len(image) > maxImageBytes {
		return Draft{}, ErrImageTooLarge
	}

	since := s.now().Add(-24 * time.Hour)
	count, err := s.store.CountSince(ctx, userID, since)
	if err != nil {
		return Draft{}, err
	}
	if count >= s.dailyLimit {
		return Draft{}, ErrRateLimited
	}

	draft, scanErr := s.scanner.Scan(ctx, image, mimeType)
	if scanErr != nil {
		if !errors.Is(scanErr, ErrNotConfigured) {
			slog.Error("receipt scan failed", "user_id", userID, "error", scanErr)
		}
		_ = s.store.Record(ctx, ScanRecord{UserID: userID, Status: ScanStatusFailed, ErrorCode: classifyError(scanErr)})
		if errors.Is(scanErr, ErrNotConfigured) {
			return Draft{}, ErrNotConfigured
		}
		return Draft{}, ErrScanFailed
	}

	if err := s.store.Record(ctx, ScanRecord{UserID: userID, Status: ScanStatusOK}); err != nil {
		return Draft{}, err
	}

	return draft, nil
}

// classifyError reduces a scanner error to a short, non-sensitive code safe
// to store — the real error (which may include provider response bodies) is
// logged server-side by the caller, never persisted.
func classifyError(err error) string {
	if errors.Is(err, ErrNotConfigured) {
		return "not_configured"
	}
	return "provider_error"
}
