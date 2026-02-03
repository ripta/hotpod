package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ripta/hotpod/internal/config"
	"github.com/ripta/hotpod/internal/load"
)

func testConfig() *config.Config {
	return &config.Config{
		MaxCPUDuration: 60 * time.Second,
		MaxMemorySize:  1 << 30,
	}
}

func TestCPUDefault(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewCPUHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/cpu", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	h.CPU(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if elapsed < 1*time.Second {
		t.Errorf("elapsed = %v, want >= 1s (default duration)", elapsed)
	}

	var resp CPUResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Cores != 1 {
		t.Errorf("response.Cores = %d, want 1", resp.Cores)
	}
	if resp.Intensity != "medium" {
		t.Errorf("response.Intensity = %q, want \"medium\"", resp.Intensity)
	}
	if resp.Iterations == 0 {
		t.Error("response.Iterations = 0, want > 0")
	}
}

func TestCPUCustomParams(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewCPUHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/cpu?duration=100ms&cores=2&intensity=high", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	h.CPU(rec, req)
	elapsed := time.Since(start)

	if elapsed < 100*time.Millisecond || elapsed > 300*time.Millisecond {
		t.Errorf("elapsed = %v, want ~100ms", elapsed)
	}

	var resp CPUResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Cores != 2 {
		t.Errorf("response.Cores = %d, want 2", resp.Cores)
	}
	if resp.Intensity != "high" {
		t.Errorf("response.Intensity = %q, want \"high\"", resp.Intensity)
	}
}

func TestCPUIntensityLevels(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewCPUHandlers(tracker, testConfig())

	levels := []string{"low", "medium", "high"}
	for _, level := range levels {
		req := httptest.NewRequest("GET", "/cpu?duration=50ms&intensity="+level, nil)
		rec := httptest.NewRecorder()

		h.CPU(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("intensity=%s: status = %d, want %d", level, rec.Code, http.StatusOK)
		}

		var resp CPUResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("intensity=%s: failed to parse response: %v", level, err)
		}
		if resp.Intensity != level {
			t.Errorf("intensity=%s: response.Intensity = %q", level, resp.Intensity)
		}
	}
}

func TestCPUInvalidDuration(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewCPUHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/cpu?duration=invalid", nil)
	rec := httptest.NewRecorder()

	h.CPU(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCPUInvalidCores(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewCPUHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/cpu?duration=1ms&cores=0", nil)
	rec := httptest.NewRecorder()

	h.CPU(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCPUInvalidIntensity(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewCPUHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/cpu?duration=1ms&intensity=extreme", nil)
	rec := httptest.NewRecorder()

	h.CPU(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCPUTooManyOps(t *testing.T) {
	tracker := load.NewTracker(1)
	h := NewCPUHandlers(tracker, testConfig())

	release, _ := tracker.Acquire(load.OpTypeCPU)
	defer release()

	req := httptest.NewRequest("GET", "/cpu?duration=1ms", nil)
	rec := httptest.NewRecorder()

	h.CPU(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestCPUCancellation(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewCPUHandlers(tracker, testConfig())

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/cpu?duration=10s", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.CPU(rec, req)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("handler did not return after cancellation")
	}

	var resp CPUResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Cancelled {
		t.Error("response.Cancelled = false, want true")
	}
}

func TestCPUMaxDurationLimit(t *testing.T) {
	tracker := load.NewTracker(100)
	cfg := &config.Config{
		MaxCPUDuration: 100 * time.Millisecond,
		MaxMemorySize:  1 << 30,
	}
	h := NewCPUHandlers(tracker, cfg)

	req := httptest.NewRequest("GET", "/cpu?duration=10s", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	h.CPU(rec, req)
	elapsed := time.Since(start)

	if elapsed > 300*time.Millisecond {
		t.Errorf("elapsed = %v, want <= 300ms (limit should cap at 100ms)", elapsed)
	}

	var resp CPUResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.LimitApplied {
		t.Error("response.LimitApplied = false, want true")
	}
}

func TestCPURegister(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewCPUHandlers(tracker, testConfig())

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/cpu?duration=1ms", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
