package admin

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

// ErrorLogEntry mirrors one line scripts/log-watch.py appends to the
// error-log JSONL file it scrapes from both apps' container logs.
type ErrorLogEntry struct {
	App       string    `json:"app"`
	Container string    `json:"container"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

// RecentErrors returns up to limit entries, most recent first. A missing
// file (log-watch.py hasn't run yet, or isn't deployed) is not an error —
// it just means no data yet, same as an empty log.
func (s *Service) RecentErrors(limit int) ([]ErrorLogEntry, error) {
	f, err := os.Open(s.errorLogPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []ErrorLogEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry ErrorLogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // one malformed line must not take down the whole panel
		}
		all = append(all, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Most recent first, capped at limit.
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	if len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}
