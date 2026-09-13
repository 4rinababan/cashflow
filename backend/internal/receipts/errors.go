package receipts

import "errors"

var (
	// ErrNotConfigured means no AI_API_KEY is set — distinct from a
	// scan failure so the handler can return 503 (server not set up for this
	// feature yet) instead of 422 (this particular scan failed).
	ErrNotConfigured = errors.New("AI receipt scanning is not configured on this server")
	// ErrRateLimited means the caller has hit their daily scan limit.
	ErrRateLimited = errors.New("daily receipt scan limit reached; try again tomorrow or enter it manually")
	// ErrUnsupportedImage covers a missing/empty file or an unsupported content type.
	ErrUnsupportedImage = errors.New("unsupported image type; use JPEG, PNG, or WebP")
	// ErrImageTooLarge covers a file over the size cap.
	ErrImageTooLarge = errors.New("image is too large")
	// ErrScanFailed is returned to the caller for any provider/parsing
	// failure — deliberately generic so internals (API error bodies, etc.)
	// never reach the client; the real cause is logged server-side only.
	ErrScanFailed = errors.New("could not read the receipt; please enter it manually")
)
