package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFaultCrashDisabled(t *testing.T) {
	h := NewFaultHandlers(false)

	req := httptest.NewRequest("POST", "/fault/crash", nil)
	rec := httptest.NewRecorder()

	h.Crash(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestFaultCrashInvalidExitCode(t *testing.T) {
	h := NewFaultHandlers(true)

	testCases := []string{"-1", "256", "abc"}
	for _, exitCode := range testCases {
		req := httptest.NewRequest("POST", "/fault/crash?exit_code="+exitCode, nil)
		rec := httptest.NewRecorder()

		h.Crash(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("exit_code=%s: status = %d, want %d", exitCode, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestFaultCrashInvalidDelay(t *testing.T) {
	h := NewFaultHandlers(true)

	req := httptest.NewRequest("POST", "/fault/crash?delay=invalid", nil)
	rec := httptest.NewRecorder()

	h.Crash(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestFaultHangDisabled(t *testing.T) {
	h := NewFaultHandlers(false)

	req := httptest.NewRequest("POST", "/fault/hang", nil)
	rec := httptest.NewRecorder()

	h.Hang(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestFaultHangInvalidDuration(t *testing.T) {
	h := NewFaultHandlers(true)

	req := httptest.NewRequest("POST", "/fault/hang?duration=invalid", nil)
	rec := httptest.NewRecorder()

	h.Hang(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestFaultHangShortDuration(t *testing.T) {
	h := NewFaultHandlers(true)

	req := httptest.NewRequest("POST", "/fault/hang?duration=10ms", nil)
	rec := httptest.NewRecorder()

	h.Hang(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp HangResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Duration != "10ms" {
		t.Errorf("duration = %q, want \"10ms\"", resp.Duration)
	}
	if resp.Cancelled {
		t.Error("expected not cancelled for completed hang")
	}
}

func TestFaultOOMDisabled(t *testing.T) {
	h := NewFaultHandlers(false)

	req := httptest.NewRequest("POST", "/fault/oom", nil)
	rec := httptest.NewRecorder()

	h.OOM(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestFaultOOMInvalidRate(t *testing.T) {
	h := NewFaultHandlers(true)

	testCases := []string{"invalid", "-1", "0"}
	for _, rate := range testCases {
		req := httptest.NewRequest("POST", "/fault/oom?rate="+rate, nil)
		rec := httptest.NewRecorder()

		h.OOM(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("rate=%s: status = %d, want %d", rate, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestFaultErrorDisabled(t *testing.T) {
	h := NewFaultHandlers(false)

	req := httptest.NewRequest("GET", "/fault/error", nil)
	rec := httptest.NewRecorder()

	h.Error(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestFaultErrorInvalidRate(t *testing.T) {
	h := NewFaultHandlers(true)

	testCases := []string{"invalid", "-0.1", "1.5"}
	for _, rate := range testCases {
		req := httptest.NewRequest("GET", "/fault/error?rate="+rate, nil)
		rec := httptest.NewRecorder()

		h.Error(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("rate=%s: status = %d, want %d", rate, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestFaultErrorInvalidStatus(t *testing.T) {
	h := NewFaultHandlers(true)

	testCases := []string{"invalid", "200", "399", "600"}
	for _, status := range testCases {
		req := httptest.NewRequest("GET", "/fault/error?status="+status, nil)
		rec := httptest.NewRecorder()

		h.Error(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status=%s: got status = %d, want %d", status, rec.Code, http.StatusBadRequest)
		}
	}
}

func TestFaultErrorAlwaysInject(t *testing.T) {
	h := NewFaultHandlers(true)

	req := httptest.NewRequest("GET", "/fault/error?rate=1&status=503", nil)
	rec := httptest.NewRecorder()

	h.Error(rec, req)

	if rec.Code != 503 {
		t.Errorf("status = %d, want 503", rec.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if !resp.Injected {
		t.Error("expected Injected = true")
	}
	if resp.Status != 503 {
		t.Errorf("response.Status = %d, want 503", resp.Status)
	}
}

func TestFaultErrorNeverInject(t *testing.T) {
	h := NewFaultHandlers(true)

	req := httptest.NewRequest("GET", "/fault/error?rate=0", nil)
	rec := httptest.NewRecorder()

	h.Error(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Injected {
		t.Error("expected Injected = false")
	}
}

func TestFaultRegister(t *testing.T) {
	h := NewFaultHandlers(false)

	mux := http.NewServeMux()
	h.Register(mux)

	// Test that routes are registered (will return 403 since disabled)
	endpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/fault/crash"},
		{"POST", "/fault/hang"},
		{"POST", "/fault/oom"},
		{"GET", "/fault/error"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want %d (route should be registered)", ep.method, ep.path, rec.Code, http.StatusForbidden)
		}
	}
}
