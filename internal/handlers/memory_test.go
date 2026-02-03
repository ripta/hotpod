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

func TestMemoryDefault(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewMemoryHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/memory?duration=100ms", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	h.Memory(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if elapsed < 100*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 100ms", elapsed)
	}

	var resp MemoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.RequestedSize != 10<<20 {
		t.Errorf("response.RequestedSize = %d, want %d (10MB)", resp.RequestedSize, 10<<20)
	}
	if resp.Pattern != "random" {
		t.Errorf("response.Pattern = %q, want \"random\"", resp.Pattern)
	}
}

func TestMemoryCustomParams(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewMemoryHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/memory?size=1MB&duration=50ms&pattern=sequential", nil)
	rec := httptest.NewRecorder()

	h.Memory(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp MemoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.RequestedSize != 1<<20 {
		t.Errorf("response.RequestedSize = %d, want %d (1MB)", resp.RequestedSize, 1<<20)
	}
	if resp.Pattern != "sequential" {
		t.Errorf("response.Pattern = %q, want \"sequential\"", resp.Pattern)
	}
}

func TestMemoryPatterns(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewMemoryHandlers(tracker, testConfig())

	patterns := []string{"zero", "random", "sequential"}
	for _, pattern := range patterns {
		req := httptest.NewRequest("GET", "/memory?size=1KB&duration=1ms&pattern="+pattern, nil)
		rec := httptest.NewRecorder()

		h.Memory(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("pattern=%s: status = %d, want %d", pattern, rec.Code, http.StatusOK)
		}

		var resp MemoryResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("pattern=%s: failed to parse response: %v", pattern, err)
		}
		if resp.Pattern != pattern {
			t.Errorf("pattern=%s: response.Pattern = %q", pattern, resp.Pattern)
		}
	}
}

func TestMemoryInvalidSize(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewMemoryHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/memory?size=invalid", nil)
	rec := httptest.NewRecorder()

	h.Memory(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestMemoryInvalidDuration(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewMemoryHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/memory?duration=invalid", nil)
	rec := httptest.NewRecorder()

	h.Memory(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestMemoryInvalidPattern(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewMemoryHandlers(tracker, testConfig())

	req := httptest.NewRequest("GET", "/memory?pattern=invalid", nil)
	rec := httptest.NewRecorder()

	h.Memory(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestMemoryTooManyOps(t *testing.T) {
	tracker := load.NewTracker(1)
	h := NewMemoryHandlers(tracker, testConfig())

	release, _ := tracker.Acquire(load.OpTypeMemory)
	defer release()

	req := httptest.NewRequest("GET", "/memory?size=1KB&duration=1ms", nil)
	rec := httptest.NewRecorder()

	h.Memory(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestMemoryCancellation(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewMemoryHandlers(tracker, testConfig())

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/memory?size=1KB&duration=10s", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.Memory(rec, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("handler did not return after cancellation")
	}

	var resp MemoryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Cancelled {
		t.Error("response.Cancelled = false, want true")
	}
}

func TestMemoryMaxSizeLimit(t *testing.T) {
	tracker := load.NewTracker(100)
	cfg := &config.Config{
		MaxCPUDuration: 60 * time.Second,
		MaxMemorySize:  1 << 10, // 1KB limit
	}
	h := NewMemoryHandlers(tracker, cfg)

	req := httptest.NewRequest("GET", "/memory?size=1GB&duration=1ms", nil)
	rec := httptest.NewRecorder()

	h.Memory(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp MemoryResponse
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

func TestMemoryRegister(t *testing.T) {
	tracker := load.NewTracker(100)
	h := NewMemoryHandlers(tracker, testConfig())

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/memory?size=1KB&duration=1ms", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

type formatSizeTest struct {
	bytes int64
	want  string
}

var formatSizeTests = []formatSizeTest{
	{0, "0B"},
	{512, "512B"},
	{1024, "1.0KB"},
	{1536, "1.5KB"},
	{1048576, "1.0MB"},
	{1073741824, "1.0GB"},
	{1099511627776, "1.0TB"},
}

func TestFormatSize(t *testing.T) {
	for _, tt := range formatSizeTests {
		got := formatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestFillMemory(t *testing.T) {
	data := make([]byte, 256)

	fillMemory(data, "zero")
	for i, b := range data {
		if b != 0 {
			t.Errorf("zero pattern: data[%d] = %d, want 0", i, b)
			break
		}
	}

	fillMemory(data, "sequential")
	for i, b := range data {
		if b != byte(i) {
			t.Errorf("sequential pattern: data[%d] = %d, want %d", i, b, byte(i))
			break
		}
	}

	fillMemory(data, "random")
	allZero := true
	for _, b := range data {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("random pattern: all bytes are zero, expected random data")
	}
}
