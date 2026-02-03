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

func TestIODefault(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewIOHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/io?size=1KB", nil)
	rec := httptest.NewRecorder()

	h.IO(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp IOResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.RequestedSize != 1024 {
		t.Errorf("response.RequestedSize = %d, want 1024", resp.RequestedSize)
	}
	if resp.Operation != "write" {
		t.Errorf("response.Operation = %q, want \"write\"", resp.Operation)
	}
	if resp.BytesWritten == 0 {
		t.Error("response.BytesWritten = 0, want > 0")
	}
}

func TestIOOperations(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewIOHandlers(tracker, testConfig())

	// Use 128KB for mixed to ensure multiple blocks (block size is 64KB)
	type ioOpTest struct {
		operation string
		size      string
	}
	tests := []ioOpTest{
		{"write", "1KB"},
		{"read", "1KB"},
		{"mixed", "128KB"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest("GET", "/io?size="+tt.size+"&operation="+tt.operation, nil)
		rec := httptest.NewRecorder()

		h.IO(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("operation=%s: status = %d, want %d", tt.operation, rec.Code, http.StatusOK)
		}

		var resp IOResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("operation=%s: failed to parse response: %v", tt.operation, err)
		}
		if resp.Operation != tt.operation {
			t.Errorf("operation=%s: response.Operation = %q", tt.operation, resp.Operation)
		}
		if resp.BytesWritten == 0 {
			t.Errorf("operation=%s: response.BytesWritten = 0, want > 0", tt.operation)
		}
		if tt.operation == "read" || tt.operation == "mixed" {
			if resp.BytesRead == 0 {
				t.Errorf("operation=%s: response.BytesRead = 0, want > 0", tt.operation)
			}
		}
	}
}

func TestIOWithSync(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewIOHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/io?size=1KB&sync=true", nil)
	rec := httptest.NewRecorder()

	h.IO(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp IOResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Sync {
		t.Error("response.Sync = false, want true")
	}
}

func TestIOInvalidSize(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewIOHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/io?size=invalid", nil)
	rec := httptest.NewRecorder()

	h.IO(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestIONegativeSize(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewIOHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/io?size=-1", nil)
	rec := httptest.NewRecorder()

	h.IO(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestIOInvalidOperation(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewIOHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/io?operation=invalid", nil)
	rec := httptest.NewRecorder()

	h.IO(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestIOInvalidSync(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewIOHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/io?sync=maybe", nil)
	rec := httptest.NewRecorder()

	h.IO(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestIOTooManyOps(t *testing.T) {
	tracker := load.NewTracker(1)
	h := NewIOHandlers(tracker, testConfig())

	release, _ := tracker.Acquire(load.OpTypeIO)
	defer release()

	req := httptest.NewRequest("GET", "/io?size=1KB", nil)
	rec := httptest.NewRecorder()

	h.IO(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestIOCancellation(t *testing.T) {
	tracker := load.NewTracker(100)
	cfg := testConfig()
	cfg.MaxIOSize = 10 << 30 // Allow up to 10GB for this test
	h := NewIOHandlers(tracker, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	// Use 1GB to ensure operation takes long enough to be cancelled
	req := httptest.NewRequest("GET", "/io?size=1GB", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.IO(rec, req)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("handler did not return after cancellation")
	}

	var resp IOResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Cancelled {
		t.Error("response.Cancelled = false, want true")
	}
}

func TestIOMaxSizeLimit(t *testing.T) {
	tracker := load.NewTracker(100)
	cfg := testConfig()
	cfg.MaxIOSize = 1 << 10 // 1KB limit
	h := NewIOHandlers(tracker, cfg)

	req := httptest.NewRequest("GET", "/io?size=1GB", nil)
	rec := httptest.NewRecorder()

	h.IO(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp IOResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.RequestedSize != 1<<10 {
		t.Errorf("response.RequestedSize = %d, want %d (1KB, capped)", resp.RequestedSize, 1<<10)
	}
	if !resp.LimitApplied {
		t.Error("response.LimitApplied = false, want true")
	}
}

func TestIORegister(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewIOHandlers(tracker, testConfig())

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/io?size=1KB", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
