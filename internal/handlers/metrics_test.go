package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsEndpoint(t *testing.T) {
	h := NewMetricsHandlers()

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()

	// Check for expected hotpod metrics (non-vector gauges are always present)
	expectedMetrics := []string{
		"hotpod_in_flight_requests",
		"hotpod_cpu_seconds_total",
		"hotpod_memory_allocated_bytes",
		"hotpod_active_cpu_operations",
		"hotpod_active_memory_allocations",
		"hotpod_active_io_operations",
		"hotpod_startup_complete",
		"hotpod_startup_duration_seconds",
		"hotpod_shutdown_in_progress",
		"hotpod_shutdown_started_timestamp_seconds",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(body, metric) {
			t.Errorf("metrics output missing %q", metric)
		}
	}
}

func TestMetricsContentType(t *testing.T) {
	h := NewMetricsHandlers()

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain*", contentType)
	}
}
