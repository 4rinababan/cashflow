package receipts

import (
	"context"
	"time"
)

// memoryScanStore is an in-memory ScanStore fake for Service tests, and a
// spyScanner records whether it was ever called — used to prove the rate
// limit is enforced BEFORE the (costly) model call, not after.

type memoryScanStore struct {
	records []ScanRecord
	times   []time.Time
}

func (s *memoryScanStore) CountSince(_ context.Context, userID string, since time.Time) (int, error) {
	count := 0
	for i, r := range s.records {
		if r.UserID == userID && !s.times[i].Before(since) {
			count++
		}
	}
	return count, nil
}

func (s *memoryScanStore) Record(_ context.Context, r ScanRecord) error {
	s.records = append(s.records, r)
	s.times = append(s.times, time.Now())
	return nil
}

type fakeScanner struct {
	draft  Draft
	err    error
	called bool
}

func (f *fakeScanner) Scan(context.Context, []byte, string) (Draft, error) {
	f.called = true
	return f.draft, f.err
}
