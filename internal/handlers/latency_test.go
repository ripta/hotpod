package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ripta/hotpod/internal/load"
)

func TestLatencyDefault(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewLatencyHandlers(tracker)

	req := httptest.NewRequest("GET", "/latency", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	h.Latency(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if elapsed < 100*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 100ms (default duration)", elapsed)
	}

	var resp LatencyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Errorf("response status = %d, want %d", resp.Status, http.StatusOK)
	}
}

func TestLatencyCustomDuration(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewLatencyHandlers(tracker)

	req := httptest.NewRequest("GET", "/latency?duration=50ms", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	h.Latency(rec, req)
	elapsed := time.Since(start)

	if elapsed < 50*time.Millisecond || elapsed > 100*time.Millisecond {
		t.Errorf("elapsed = %v, want ~50ms", elapsed)
	}
}

func TestLatencyCustomStatus(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewLatencyHandlers(tracker)

	req := httptest.NewRequest("GET", "/latency?duration=1ms&status=503", nil)
	rec := httptest.NewRecorder()

	h.Latency(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp LatencyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Status != 503 {
		t.Errorf("response status = %d, want 503", resp.Status)
	}
}

func TestLatencyInvalidDuration(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewLatencyHandlers(tracker)

	req := httptest.NewRequest("GET", "/latency?duration=invalid", nil)
	rec := httptest.NewRecorder()

	h.Latency(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLatencyInvalidStatus(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewLatencyHandlers(tracker)

	req := httptest.NewRequest("GET", "/latency?duration=1ms&status=999", nil)
	rec := httptest.NewRecorder()

	h.Latency(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLatencyTooManyOps(t *testing.T) {
	tracker := load.NewTracker(1)
	h := NewLatencyHandlers(tracker)

	release, _ := tracker.Acquire(load.OpTypeLatency)
	defer release()

	req := httptest.NewRequest("GET", "/latency?duration=1ms", nil)
	rec := httptest.NewRecorder()

	h.Latency(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestLatencyCancellation(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewLatencyHandlers(tracker)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/latency?duration=10s", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.Latency(rec, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("handler did not return after cancellation")
	}

	var resp LatencyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Cancelled {
		t.Error("response.Cancelled = false, want true")
	}
}

func TestLatencyJitter(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewLatencyHandlers(tracker)

	req := httptest.NewRequest("GET", "/latency?duration=10ms&jitter=20ms", nil)
	rec := httptest.NewRecorder()

	h.Latency(rec, req)

	var resp LatencyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Jitter != "20ms" {
		t.Errorf("response.Jitter = %q, want \"20ms\"", resp.Jitter)
	}
}

func TestLatencyRegister(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewLatencyHandlers(tracker)

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/latency?duration=1ms", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
