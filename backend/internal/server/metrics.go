package server

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cashflow_http_requests_total",
			Help: "Total HTTP requests, by method, route, and status code.",
		},
		[]string{"method", "route", "status"},
	)

	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cashflow_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds, by method and route.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
)

var uuidSegment = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// metricsRoute collapses a UUID path segment (every resource ID in this API
// is one — see docs/adr/0001-architecture.md) down to "{id}", so metrics
// stay low-cardinality instead of one time series per row ever fetched.
func metricsRoute(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if uuidSegment.MatchString(seg) {
			segments[i] = "{id}"
		}
	}
	return strings.Join(segments, "/")
}

// withMetrics records a Prometheus counter + histogram per request — Phase
// 7's basic observability bar, exposed at GET /metrics (see MetricsHandler).
func withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		route := metricsRoute(r.URL.Path)
		requestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(rec.status)).Inc()
		requestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

// MetricsHandler serves Prometheus-format metrics at GET /metrics. Not
// behind auth: standard practice for an endpoint meant to be scraped by
// infrastructure, not called by end users — it exposes only request
// counts/timings/labels, never user data.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
