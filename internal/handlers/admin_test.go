package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/ripta/hotpod/internal/config"
	"github.com/ripta/hotpod/internal/fault"
	"github.com/ripta/hotpod/internal/queue"
	"github.com/ripta/hotpod/internal/server"
)

type adminEndpoint struct {
	method string
	path   string
}

var adminEndpoints = []adminEndpoint{
	{"POST", "/admin/ready"},
	{"POST", "/admin/gc"},
	{"GET", "/admin/config"},
	{"POST", "/admin/reset"},
	{"POST", "/admin/error-rate"},
	{"POST", "/admin/queue/pause"},
	{"POST", "/admin/queue/resume"},
}

func newTestLifecycle() *server.Lifecycle {
	clock := clockwork.NewFakeClock()
	return server.NewLifecycleWithClock(clock, 0, 0, 0, 30*time.Second, false)
}

func newTestConfig() *config.Config {
	return &config.Config{
		Port:             8080,
		LogLevel:         "info",
		MaxCPUDuration:   60 * time.Second,
		MaxMemorySize:    1 << 30,
		MaxIOSize:        1 << 30,
		MaxConcurrentOps: 100,
		RequestTimeout:   5 * time.Minute,
		Mode:             "app",
	}
}

func newTestAdminHandlers(token string) (*AdminHandlers, *queue.Queue, *queue.WorkerPool) {
	lc := newTestLifecycle()
	inj := fault.NewInjector()
	cfg := newTestConfig()
	q := queue.New(100)
	wp := queue.NewWorkerPool(q)
	h := NewAdminHandlers(token, lc, inj, cfg, q, wp)
	return h, q, wp
}

func TestAdminRegister(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	mux := http.NewServeMux()
	h.Register(mux)

	for _, ep := range adminEndpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		// Endpoints should respond (not 404/405)
		if rec.Code == http.StatusNotFound || rec.Code == http.StatusMethodNotAllowed {
			t.Errorf("%s %s: got %d, expected route to be registered", ep.method, ep.path, rec.Code)
		}
	}
}

func TestAdminAuthNoToken(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/ready?state=true", nil)
	rec := httptest.NewRecorder()

	h.Ready(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (no token = open access)", rec.Code, http.StatusOK)
	}
}

func TestAdminAuthCorrectToken(t *testing.T) {
	h, _, _ := newTestAdminHandlers("secret123")

	req := httptest.NewRequest("POST", "/admin/ready?state=true", nil)
	req.Header.Set("X-Admin-Token", "secret123")
	rec := httptest.NewRecorder()

	h.Ready(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAdminAuthWrongToken(t *testing.T) {
	h, _, _ := newTestAdminHandlers("secret123")

	req := httptest.NewRequest("POST", "/admin/ready?state=true", nil)
	req.Header.Set("X-Admin-Token", "wrongtoken")
	rec := httptest.NewRecorder()

	h.Ready(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAdminAuthMissingToken(t *testing.T) {
	h, _, _ := newTestAdminHandlers("secret123")

	req := httptest.NewRequest("POST", "/admin/ready?state=true", nil)
	rec := httptest.NewRecorder()

	h.Ready(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAdminReadyForceTrue(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/ready?state=true", nil)
	rec := httptest.NewRecorder()

	h.Ready(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminReadyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if !resp.Ready {
		t.Error("expected ready = true")
	}
	if resp.Override == nil || !*resp.Override {
		t.Error("expected override = true")
	}
}

func TestAdminReadyForceFalse(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/ready?state=false", nil)
	rec := httptest.NewRecorder()

	h.Ready(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminReadyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Ready {
		t.Error("expected ready = false")
	}
	if resp.Override == nil || *resp.Override {
		t.Error("expected override = false")
	}
}

func TestAdminReadyToggle(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	// First call with no state: should toggle from no-override to force-not-ready
	req := httptest.NewRequest("POST", "/admin/ready", nil)
	rec := httptest.NewRecorder()
	h.Ready(rec, req)

	var resp AdminReadyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Ready {
		t.Error("first toggle: expected ready = false")
	}

	// Second call: should clear override
	req = httptest.NewRequest("POST", "/admin/ready", nil)
	rec = httptest.NewRecorder()
	h.Ready(rec, req)

	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Ready {
		t.Error("second toggle: expected ready = true (override cleared)")
	}
	if resp.Override != nil {
		t.Error("second toggle: expected override = nil")
	}
}

func TestAdminReadyInvalidState(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/ready?state=invalid", nil)
	rec := httptest.NewRecorder()

	h.Ready(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminGC(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/gc", nil)
	rec := httptest.NewRecorder()

	h.GC(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminGCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Before.Sys == 0 {
		t.Error("expected before.sys > 0")
	}
	if resp.After.Sys == 0 {
		t.Error("expected after.sys > 0")
	}
}

func TestAdminConfig(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("GET", "/admin/config", nil)
	rec := httptest.NewRecorder()

	h.Config(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Mode != "app" {
		t.Errorf("mode = %q, want %q", resp.Mode, "app")
	}
	if resp.Limits.MaxConcurrentOps != 100 {
		t.Errorf("max_concurrent_ops = %d, want 100", resp.Limits.MaxConcurrentOps)
	}
	if !resp.Queue.Available {
		t.Error("expected queue.available = true")
	}
	if resp.Sidecar.Active {
		t.Error("expected sidecar.active = false")
	}
}

func TestAdminConfigWithFaultInjection(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	h.injector.SetGlobalConfig(&fault.ErrorConfig{Rate: 0.5, Codes: []int{500, 503}})
	h.injector.SetEndpointConfig("/cpu", &fault.ErrorConfig{Rate: 0.8, Codes: []int{429}})

	req := httptest.NewRequest("GET", "/admin/config", nil)
	rec := httptest.NewRecorder()

	h.Config(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Fault.Global == nil {
		t.Fatal("expected fault.global to be set")
	}
	if resp.Fault.Global.Rate != 0.5 {
		t.Errorf("fault.global.rate = %f, want 0.5", resp.Fault.Global.Rate)
	}
	if len(resp.Fault.Endpoints) != 1 {
		t.Errorf("fault.endpoints length = %d, want 1", len(resp.Fault.Endpoints))
	}
}

func TestAdminReset(t *testing.T) {
	h, q, _ := newTestAdminHandlers("")

	// Set some state
	h.injector.SetGlobalConfig(&fault.ErrorConfig{Rate: 0.5, Codes: []int{500}})
	for i := range 3 {
		item := &queue.Item{ID: string(rune('a' + i)), Priority: queue.PriorityNormal}
		if err := q.Enqueue(item); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}
	}
	ready := false
	h.lifecycle.SetReadyOverride(&ready)

	req := httptest.NewRequest("POST", "/admin/reset", nil)
	rec := httptest.NewRecorder()

	h.Reset(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminResetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if !resp.FaultReset {
		t.Error("expected fault_reset = true")
	}
	if resp.QueueCleared != 3 {
		t.Errorf("queue_cleared = %d, want 3", resp.QueueCleared)
	}
	if !resp.ReadyOverrideCleared {
		t.Error("expected ready_override_cleared = true")
	}

	// Verify state was actually reset
	if h.injector.GetGlobalConfig() != nil {
		t.Error("expected global config to be nil after reset")
	}
	if q.Depth() != 0 {
		t.Errorf("queue depth = %d, want 0 after reset", q.Depth())
	}
	if h.lifecycle.ReadyOverride() != nil {
		t.Error("expected ready override to be nil after reset")
	}
}

func TestAdminErrorRateGlobal(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/error-rate?rate=0.5&codes=500,503", nil)
	rec := httptest.NewRecorder()

	h.ErrorRate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminErrorRateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Rate != 0.5 {
		t.Errorf("rate = %f, want 0.5", resp.Rate)
	}
	if len(resp.Codes) != 2 {
		t.Errorf("codes length = %d, want 2", len(resp.Codes))
	}
	if resp.Endpoint != "" {
		t.Errorf("endpoint = %q, want empty", resp.Endpoint)
	}
}

func TestAdminErrorRateEndpoint(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/error-rate?endpoint=/cpu&rate=0.8&codes=429", nil)
	rec := httptest.NewRecorder()

	h.ErrorRate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminErrorRateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Endpoint != "/cpu" {
		t.Errorf("endpoint = %q, want /cpu", resp.Endpoint)
	}
	if resp.Rate != 0.8 {
		t.Errorf("rate = %f, want 0.8", resp.Rate)
	}
}

func TestAdminErrorRateWithDuration(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/error-rate?rate=0.5&duration=5m", nil)
	rec := httptest.NewRecorder()

	h.ErrorRate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminErrorRateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Duration != "5m" {
		t.Errorf("duration = %q, want 5m", resp.Duration)
	}
}

func TestAdminErrorRateMissingRate(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/error-rate", nil)
	rec := httptest.NewRecorder()

	h.ErrorRate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminErrorRateInvalidRate(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	testCases := []string{"abc", "-0.1", "1.5"}
	for _, rate := range testCases {
		req := httptest.NewRequest("POST", "/admin/error-rate?rate="+rate, nil)
		rec := httptest.NewRecorder()

		h.ErrorRate(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("rate=%s: status = %d, want %d", rate, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestAdminErrorRateInvalidCodes(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/error-rate?rate=0.5&codes=abc", nil)
	rec := httptest.NewRecorder()

	h.ErrorRate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminErrorRateInvalidDuration(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/error-rate?rate=0.5&duration=invalid", nil)
	rec := httptest.NewRecorder()

	h.ErrorRate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminQueuePause(t *testing.T) {
	h, q, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/queue/pause", nil)
	rec := httptest.NewRecorder()

	h.QueuePause(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if !q.IsPaused() {
		t.Error("expected queue to be paused")
	}
}

func TestAdminQueueResume(t *testing.T) {
	h, q, _ := newTestAdminHandlers("")

	q.Pause()

	req := httptest.NewRequest("POST", "/admin/queue/resume", nil)
	rec := httptest.NewRecorder()

	h.QueueResume(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if q.IsPaused() {
		t.Error("expected queue to not be paused")
	}
}

func TestAdminQueuePauseNilQueue(t *testing.T) {
	lc := newTestLifecycle()
	inj := fault.NewInjector()
	cfg := newTestConfig()
	h := NewAdminHandlers("", lc, inj, cfg, nil, nil)

	req := httptest.NewRequest("POST", "/admin/queue/pause", nil)
	rec := httptest.NewRecorder()

	h.QueuePause(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAdminQueueResumeNilQueue(t *testing.T) {
	lc := newTestLifecycle()
	inj := fault.NewInjector()
	cfg := newTestConfig()
	h := NewAdminHandlers("", lc, inj, cfg, nil, nil)

	req := httptest.NewRequest("POST", "/admin/queue/resume", nil)
	rec := httptest.NewRecorder()

	h.QueueResume(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestAdminResetNilQueue(t *testing.T) {
	lc := newTestLifecycle()
	inj := fault.NewInjector()
	cfg := newTestConfig()
	h := NewAdminHandlers("", lc, inj, cfg, nil, nil)

	req := httptest.NewRequest("POST", "/admin/reset", nil)
	rec := httptest.NewRecorder()

	h.Reset(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminResetResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.QueueCleared != 0 {
		t.Errorf("queue_cleared = %d, want 0", resp.QueueCleared)
	}
	if resp.WorkersStopped {
		t.Error("expected workers_stopped = false with nil worker pool")
	}
}

func TestAdminErrorRateDefaultCodes(t *testing.T) {
	h, _, _ := newTestAdminHandlers("")

	req := httptest.NewRequest("POST", "/admin/error-rate?rate=0.5", nil)
	rec := httptest.NewRecorder()

	h.ErrorRate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp AdminErrorRateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Codes) != 1 || resp.Codes[0] != 500 {
		t.Errorf("codes = %v, want [500]", resp.Codes)
	}
}
