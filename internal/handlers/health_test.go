package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ripta/hotpod/internal/server"
)

type healthHandlerTest struct {
	method string
	path   string
	want   int
}

var healthHandlerTests = []healthHandlerTest{
	{"GET", "/healthz", http.StatusOK},
	{"GET", "/readyz", http.StatusOK},
	{"GET", "/startupz", http.StatusOK},
}

func TestHealthz(t *testing.T) {
	lc := server.NewLifecycle(0, 0, 0, 30*time.Second, false)
	h := NewHealthHandlers(lc)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	h.Healthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Healthz status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("Healthz status = %q, want \"ok\"", resp.Status)
	}
}

func TestReadyzWhenReady(t *testing.T) {
	lc := server.NewLifecycle(0, 0, 0, 30*time.Second, false)
	// Give it a moment to become ready
	time.Sleep(10 * time.Millisecond)

	h := NewHealthHandlers(lc)

	req := httptest.NewRequest("GET", "/readyz", nil)
	rec := httptest.NewRecorder()

	h.Readyz(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Readyz status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("Readyz response status = %q, want \"ok\"", resp.Status)
	}
}

func TestReadyzDuringStartup(t *testing.T) {
	lc := server.NewLifecycle(1*time.Hour, 0, 0, 30*time.Second, false)
	h := NewHealthHandlers(lc)

	req := httptest.NewRequest("GET", "/readyz", nil)
	rec := httptest.NewRecorder()

	h.Readyz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Readyz status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Status != "not_ready" {
		t.Errorf("Readyz response status = %q, want \"not_ready\"", resp.Status)
	}
	if resp.Reason == "" {
		t.Error("Readyz response should include reason")
	}
}

func TestStartupzWhenReady(t *testing.T) {
	lc := server.NewLifecycle(0, 0, 0, 30*time.Second, false)
	// Give it a moment to become ready
	time.Sleep(10 * time.Millisecond)

	h := NewHealthHandlers(lc)

	req := httptest.NewRequest("GET", "/startupz", nil)
	rec := httptest.NewRecorder()

	h.Startupz(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Startupz status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestStartupzDuringStartup(t *testing.T) {
	lc := server.NewLifecycle(1*time.Hour, 0, 0, 30*time.Second, false)
	h := NewHealthHandlers(lc)

	req := httptest.NewRequest("GET", "/startupz", nil)
	rec := httptest.NewRecorder()

	h.Startupz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Startupz status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Status != "starting" {
		t.Errorf("Startupz response status = %q, want \"starting\"", resp.Status)
	}
	if resp.Remaining == "" {
		t.Error("Startupz response should include remaining time")
	}
}

func TestHealthHandlersRegister(t *testing.T) {
	lc := server.NewLifecycle(0, 0, 0, 30*time.Second, false)
	h := NewHealthHandlers(lc)

	mux := http.NewServeMux()
	h.Register(mux)

	time.Sleep(10 * time.Millisecond)

	for _, tt := range healthHandlerTests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != tt.want {
			t.Errorf("%s %s = %d, want %d", tt.method, tt.path, rec.Code, tt.want)
		}
	}
}
