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

func TestWorkDefault(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewWorkHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/work", nil)
	rec := httptest.NewRecorder()

	h.Work(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp WorkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Profile != "web" {
		t.Errorf("response.Profile = %q, want \"web\"", resp.Profile)
	}
	if resp.CPUIterations == 0 {
		t.Error("response.CPUIterations = 0, want > 0")
	}
	if resp.MemorySize == 0 {
		t.Error("response.MemorySize = 0, want > 0")
	}
}

func TestWorkProfiles(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewWorkHandlers(tracker, testConfig())

	profiles := []string{"web", "api", "worker", "heavy"}
	for _, profile := range profiles {
		req := httptest.NewRequest("GET", "/work?profile="+profile, nil)
		rec := httptest.NewRecorder()

		h.Work(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("profile=%s: status = %d, want %d", profile, rec.Code, http.StatusOK)
		}

		var resp WorkResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("profile=%s: failed to parse response: %v", profile, err)
		}
		if resp.Profile != profile {
			t.Errorf("profile=%s: response.Profile = %q", profile, resp.Profile)
		}
	}
}

func TestWorkWithVariance(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewWorkHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/work?profile=web&variance=0.5", nil)
	rec := httptest.NewRecorder()

	h.Work(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp WorkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Variance != 0.5 {
		t.Errorf("response.Variance = %f, want 0.5", resp.Variance)
	}
}

func TestWorkInvalidProfile(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewWorkHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/work?profile=invalid", nil)
	rec := httptest.NewRecorder()

	h.Work(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWorkInvalidVariance(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewWorkHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/work?variance=invalid", nil)
	rec := httptest.NewRecorder()

	h.Work(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWorkVarianceOutOfRange(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewWorkHandlers(tracker, testConfig())

	testCases := []string{"-0.1", "1.5"}
	for _, variance := range testCases {
		req := httptest.NewRequest("GET", "/work?variance="+variance, nil)
		rec := httptest.NewRecorder()

		h.Work(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("variance=%s: status = %d, want %d", variance, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestWorkTooManyOps(t *testing.T) {
	tracker := load.NewTracker(1)
	h := NewWorkHandlers(tracker, testConfig())

	release, _ := tracker.Acquire(load.OpTypeWork)
	defer release()

	req := httptest.NewRequest("GET", "/work", nil)
	rec := httptest.NewRecorder()

	h.Work(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestWorkCancellation(t *testing.T) {
	tracker := load.NewTracker(100)
	cfg := testConfig()
	cfg.MaxCPUDuration = 10 * time.Second
	h := NewWorkHandlers(tracker, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/work?profile=heavy", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.Work(rec, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("handler did not return after cancellation")
	}

	var resp WorkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Cancelled {
		t.Error("response.Cancelled = false, want true")
	}
}

func TestWorkLimitsApplied(t *testing.T) {
	tracker := load.NewTracker(100)
	cfg := testConfig()
	cfg.MaxCPUDuration = 1 * time.Millisecond
	cfg.MaxMemorySize = 1 << 10 // 1KB
	h := NewWorkHandlers(tracker, cfg)

	req := httptest.NewRequest("GET", "/work?profile=heavy", nil)
	rec := httptest.NewRecorder()

	h.Work(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp WorkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.LimitsApplied {
		t.Error("response.LimitsApplied = false, want true")
	}
}

func TestWorkRegister(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewWorkHandlers(tracker, testConfig())

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/work", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestApplyVariance(t *testing.T) {
	d := 100 * time.Millisecond

	result := applyVariance(d, 0)
	if result != d {
		t.Errorf("applyVariance with 0 variance: got %v, want %v", result, d)
	}

	for range 10 {
		result = applyVariance(d, 0.5)
		minExpected := time.Duration(float64(d) * 0.5)
		maxExpected := time.Duration(float64(d) * 1.5)
		if result < minExpected || result > maxExpected {
			t.Errorf("applyVariance with 0.5 variance: got %v, expected between %v and %v", result, minExpected, maxExpected)
		}
	}
}

func TestApplyVarianceInt64(t *testing.T) {
	n := int64(1000)

	result := applyVarianceInt64(n, 0)
	if result != n {
		t.Errorf("applyVarianceInt64 with 0 variance: got %d, want %d", result, n)
	}

	for range 10 {
		result = applyVarianceInt64(n, 0.5)
		minExpected := int64(float64(n) * 0.5)
		maxExpected := int64(float64(n) * 1.5)
		if result < minExpected || result > maxExpected {
			t.Errorf("applyVarianceInt64 with 0.5 variance: got %d, expected between %d and %d", result, minExpected, maxExpected)
		}
	}
}
