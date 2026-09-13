package server

import "testing"

func TestMetricsRoute_CollapsesUUIDSegments(t *testing.T) {
	got := metricsRoute("/accounts/550e8400-e29b-41d4-a716-446655440000")
	want := "/accounts/{id}"
	if got != want {
		t.Errorf("metricsRoute() = %q, want %q", got, want)
	}
}

func TestMetricsRoute_LeavesNonUUIDSegmentsAlone(t *testing.T) {
	got := metricsRoute("/reports/monthly-trend")
	if got != "/reports/monthly-trend" {
		t.Errorf("metricsRoute() = %q, want unchanged", got)
	}
}

func TestMetricsRoute_CollapsesOnlyTheUUIDSegmentInAMixedPath(t *testing.T) {
	got := metricsRoute("/accounts/550e8400-e29b-41d4-a716-446655440000/transactions")
	want := "/accounts/{id}/transactions"
	if got != want {
		t.Errorf("metricsRoute() = %q, want %q", got, want)
	}
}
