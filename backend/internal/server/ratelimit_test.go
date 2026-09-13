package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestIPRateLimiter_AllowsUpToBurstThenRejects(t *testing.T) {
	limiter := newIPRateLimiter(rate.Limit(1), 2)
	if !limiter.allow("1.2.3.4") {
		t.Error("first request should be allowed")
	}
	if !limiter.allow("1.2.3.4") {
		t.Error("second request (within burst) should be allowed")
	}
	if limiter.allow("1.2.3.4") {
		t.Error("third immediate request should be rejected (burst exhausted)")
	}
}

func TestIPRateLimiter_TracksEachIPIndependently(t *testing.T) {
	limiter := newIPRateLimiter(rate.Limit(1), 1)
	if !limiter.allow("1.1.1.1") {
		t.Fatal("first IP's first request should be allowed")
	}
	if !limiter.allow("2.2.2.2") {
		t.Error("a different IP must not share the first IP's bucket")
	}
}

func TestIPRateLimiter_RefillsOverTime(t *testing.T) {
	limiter := newIPRateLimiter(rate.Limit(50), 1) // fast refill so the test doesn't sleep long
	if !limiter.allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	if limiter.allow("1.2.3.4") {
		t.Fatal("immediate second request should be rejected")
	}
	time.Sleep(40 * time.Millisecond)
	if !limiter.allow("1.2.3.4") {
		t.Error("request after the refill interval should be allowed")
	}
}

func TestClientIP_ParsesHostFromRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:54321"
	if got := clientIP(req); got != "203.0.113.5" {
		t.Errorf("clientIP() = %q, want 203.0.113.5", got)
	}
}

func TestWithRateLimit_Returns429OnceLimitExceeded(t *testing.T) {
	limiter := newIPRateLimiter(rate.Limit(1), 1)
	handler := withRateLimit(limiter, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	newRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "9.9.9.9:1111"
		return req
	}

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, newRequest())
	if first.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", first.Code)
	}

	second := httptest.NewRecorder()
	handler.ServeHTTP(second, newRequest())
	if second.Code != http.StatusTooManyRequests {
		t.Errorf("second request status = %d, want 429", second.Code)
	}
}
