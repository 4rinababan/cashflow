package receipts

import (
	"context"
	"errors"
	"testing"
)

const validJPEG = "image/jpeg"

func someBytes() []byte { return []byte{0xFF, 0xD8, 0xFF, 0xE0} }

func TestService_Scan_RejectsAnUnsupportedMimeTypeBeforeCallingTheScanner(t *testing.T) {
	scanner := &fakeScanner{}
	svc := NewService(scanner, &memoryScanStore{}, 10)

	_, err := svc.Scan(context.Background(), "user-1", someBytes(), "application/pdf")
	if !errors.Is(err, ErrUnsupportedImage) {
		t.Errorf("error = %v, want ErrUnsupportedImage", err)
	}
	if scanner.called {
		t.Error("scanner was called for an unsupported image type — it must be rejected first")
	}
}

func TestService_Scan_RejectsAnEmptyImage(t *testing.T) {
	svc := NewService(&fakeScanner{}, &memoryScanStore{}, 10)
	if _, err := svc.Scan(context.Background(), "user-1", []byte{}, validJPEG); !errors.Is(err, ErrUnsupportedImage) {
		t.Errorf("error = %v, want ErrUnsupportedImage", err)
	}
}

func TestService_Scan_RejectsAnImageOverTheSizeCap(t *testing.T) {
	scanner := &fakeScanner{}
	svc := NewService(scanner, &memoryScanStore{}, 10)

	oversized := make([]byte, maxImageBytes+1)
	_, err := svc.Scan(context.Background(), "user-1", oversized, validJPEG)
	if !errors.Is(err, ErrImageTooLarge) {
		t.Errorf("error = %v, want ErrImageTooLarge", err)
	}
	if scanner.called {
		t.Error("scanner was called for an oversized image — it must be rejected first")
	}
}

func TestService_Scan_EnforcesTheDailyLimitBeforeCallingTheScanner(t *testing.T) {
	scanner := &fakeScanner{draft: Draft{Merchant: "Warung"}}
	store := &memoryScanStore{}
	svc := NewService(scanner, store, 1)
	ctx := context.Background()

	if _, err := svc.Scan(ctx, "user-1", someBytes(), validJPEG); err != nil {
		t.Fatalf("first scan (within limit) error = %v", err)
	}
	if !scanner.called {
		t.Fatal("scanner was not called for the first (allowed) scan")
	}

	scanner.called = false
	_, err := svc.Scan(ctx, "user-1", someBytes(), validJPEG)
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("second scan error = %v, want ErrRateLimited", err)
	}
	if scanner.called {
		t.Error("scanner was called after the daily limit was already reached")
	}
}

func TestService_Scan_DailyLimitIsPerUser(t *testing.T) {
	scanner := &fakeScanner{draft: Draft{Merchant: "Warung"}}
	store := &memoryScanStore{}
	svc := NewService(scanner, store, 1)
	ctx := context.Background()

	if _, err := svc.Scan(ctx, "user-a", someBytes(), validJPEG); err != nil {
		t.Fatalf("user-a scan error = %v", err)
	}
	if _, err := svc.Scan(ctx, "user-b", someBytes(), validJPEG); err != nil {
		t.Errorf("user-b scan error = %v, want nil (limit is per-user, not global)", err)
	}
}

func TestService_Scan_RecordsAFailureAndReturnsTheGenericErrorOnScannerFailure(t *testing.T) {
	scanner := &fakeScanner{err: errors.New("AI endpoint returned status 500")}
	store := &memoryScanStore{}
	svc := NewService(scanner, store, 10)

	_, err := svc.Scan(context.Background(), "user-1", someBytes(), validJPEG)
	if !errors.Is(err, ErrScanFailed) {
		t.Errorf("error = %v, want the generic ErrScanFailed (never leak the provider error to the caller)", err)
	}
	if len(store.records) != 1 || store.records[0].Status != ScanStatusFailed {
		t.Errorf("records = %+v, want exactly one failed record", store.records)
	}
}

func TestService_Scan_NotConfiguredIsReturnedAsIs(t *testing.T) {
	scanner := &fakeScanner{err: ErrNotConfigured}
	svc := NewService(scanner, &memoryScanStore{}, 10)

	_, err := svc.Scan(context.Background(), "user-1", someBytes(), validJPEG)
	if !errors.Is(err, ErrNotConfigured) {
		t.Errorf("error = %v, want ErrNotConfigured (distinct from a scan failure, so the handler can 503 instead of 422)", err)
	}
}

func TestService_Scan_RecordsSuccessAndReturnsTheDraft(t *testing.T) {
	want := Draft{Merchant: "Warung Bu Tini", TotalMinor: 5000000, Currency: "IDR", Confidence: 0.9}
	scanner := &fakeScanner{draft: want}
	store := &memoryScanStore{}
	svc := NewService(scanner, store, 10)

	got, err := svc.Scan(context.Background(), "user-1", someBytes(), validJPEG)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if got.Merchant != want.Merchant || got.TotalMinor != want.TotalMinor {
		t.Errorf("draft = %+v, want %+v", got, want)
	}
	if len(store.records) != 1 || store.records[0].Status != ScanStatusOK {
		t.Errorf("records = %+v, want exactly one ok record", store.records)
	}
}
