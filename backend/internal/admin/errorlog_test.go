package admin

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecentErrors_MissingFileReturnsEmptyNotError(t *testing.T) {
	svc := &Service{errorLogPath: filepath.Join(t.TempDir(), "does-not-exist.jsonl")}
	entries, err := svc.RecentErrors(10)
	if err != nil {
		t.Fatalf("RecentErrors() error = %v, want nil for a missing file", err)
	}
	if entries != nil {
		t.Errorf("entries = %+v, want nil", entries)
	}
}

func TestRecentErrors_ReturnsMostRecentFirstAndRespectsLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "error-log.jsonl")
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	lines := ""
	for i := 0; i < 5; i++ {
		lines += `{"app":"cashflow","container":"cashflow-api-1","timestamp":"` +
			now.Add(time.Duration(i)*time.Minute).Format(time.RFC3339) +
			`","message":"error ` + string(rune('A'+i)) + `"}` + "\n"
	}
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatalf("write test log: %v", err)
	}

	svc := &Service{errorLogPath: path}
	entries, err := svc.RecentErrors(3)
	if err != nil {
		t.Fatalf("RecentErrors() error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}
	if entries[0].Message != "error E" {
		t.Errorf("entries[0].Message = %q, want the most recent entry first", entries[0].Message)
	}
}
