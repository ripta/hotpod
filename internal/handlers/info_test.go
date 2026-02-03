package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ripta/hotpod/internal/config"
	"github.com/ripta/hotpod/internal/server"
)

func TestInfoEndpoint(t *testing.T) {
	cfg := &config.Config{
		Port:             8080,
		LogLevel:         "info",
		MaxCPUDuration:   60 * time.Second,
		MaxMemorySize:    1 << 30,
		MaxIOSize:        1 << 30,
		IODirName:        "hotpod",
		MaxConcurrentOps: 100,
		RequestTimeout:   5 * time.Minute,
		ShutdownTimeout:  30 * time.Second,
	}

	lc := server.NewLifecycle(0, 0, 0, 30*time.Second, false)
	// Wait a bit for lifecycle to become ready
	time.Sleep(10 * time.Millisecond)

	h := NewInfoHandlers("test-version", lc, cfg)

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/info", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp InfoResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Version != "test-version" {
		t.Errorf("response.Version = %q, want \"test-version\"", resp.Version)
	}

	if resp.Lifecycle.State != "ready" {
		t.Errorf("response.Lifecycle.State = %q, want \"ready\"", resp.Lifecycle.State)
	}

	if !resp.Lifecycle.StartupComplete {
		t.Error("response.Lifecycle.StartupComplete = false, want true")
	}

	if resp.Lifecycle.ShuttingDown {
		t.Error("response.Lifecycle.ShuttingDown = true, want false")
	}

	if resp.Resources.CPUCores == 0 {
		t.Error("response.Resources.CPUCores = 0, want > 0")
	}

	if resp.Resources.Goroutines == 0 {
		t.Error("response.Resources.Goroutines = 0, want > 0")
	}

	if resp.Config.Port != 8080 {
		t.Errorf("response.Config.Port = %d, want 8080", resp.Config.Port)
	}

	if resp.Config.MaxConcurrentOps != 100 {
		t.Errorf("response.Config.MaxConcurrentOps = %d, want 100", resp.Config.MaxConcurrentOps)
	}
}

func TestInfoDuringStartup(t *testing.T) {
	cfg := &config.Config{
		Port:             8080,
		LogLevel:         "info",
		MaxCPUDuration:   60 * time.Second,
		MaxMemorySize:    1 << 30,
		MaxIOSize:        1 << 30,
		IODirName:        "hotpod",
		MaxConcurrentOps: 100,
		RequestTimeout:   5 * time.Minute,
		ShutdownTimeout:  30 * time.Second,
	}

	// Create lifecycle with startup delay
	lc := server.NewLifecycle(1*time.Second, 0, 0, 30*time.Second, false)

	h := NewInfoHandlers("test-version", lc, cfg)

	req := httptest.NewRequest("GET", "/info", nil)
	rec := httptest.NewRecorder()
	h.Info(rec, req)

	var resp InfoResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Lifecycle.State != "starting" {
		t.Errorf("response.Lifecycle.State = %q, want \"starting\"", resp.Lifecycle.State)
	}

	if resp.Lifecycle.StartupComplete {
		t.Error("response.Lifecycle.StartupComplete = true, want false")
	}

	if resp.Lifecycle.ReadyAt != "" {
		t.Errorf("response.Lifecycle.ReadyAt = %q, want empty", resp.Lifecycle.ReadyAt)
	}
}

func TestInfoContentType(t *testing.T) {
	cfg := &config.Config{
		Port:             8080,
		LogLevel:         "info",
		MaxCPUDuration:   60 * time.Second,
		MaxMemorySize:    1 << 30,
		MaxIOSize:        1 << 30,
		IODirName:        "hotpod",
		MaxConcurrentOps: 100,
		RequestTimeout:   5 * time.Minute,
		ShutdownTimeout:  30 * time.Second,
	}

	lc := server.NewLifecycle(0, 0, 0, 30*time.Second, false)
	h := NewInfoHandlers("test-version", lc, cfg)

	req := httptest.NewRequest("GET", "/info", nil)
	rec := httptest.NewRecorder()
	h.Info(rec, req)

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want \"application/json\"", contentType)
	}
}
