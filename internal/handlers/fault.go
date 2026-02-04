package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"

	"github.com/ripta/hotpod/internal/fault"
)

// FaultHandlers provides chaos engineering endpoint handlers.
type FaultHandlers struct {
	enabled bool
}

// NewFaultHandlers creates handlers for chaos engineering endpoints.
func NewFaultHandlers(enabled bool) *FaultHandlers {
	return &FaultHandlers{
		enabled: enabled,
	}
}

// Register adds fault routes to the mux.
func (h *FaultHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /fault/crash", h.Crash)
	mux.HandleFunc("POST /fault/hang", h.Hang)
	mux.HandleFunc("POST /fault/oom", h.OOM)
	mux.HandleFunc("GET /fault/error", h.Error)
}

// CrashResponse is the JSON response for /fault/crash (sent before crashing).
type CrashResponse struct {
	Message   string `json:"message"`
	Delay     string `json:"delay"`
	ExitCode  int    `json:"exit_code"`
	Scheduled bool   `json:"scheduled"`
}

func (h *FaultHandlers) Crash(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		writeError(w, http.StatusForbidden, "CHAOS_DISABLED", "chaos endpoints are disabled")
		return
	}

	delay, err := parseDuration(r, "delay", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}

	exitCodeStr := r.URL.Query().Get("exit_code")
	exitCode := 1
	if exitCodeStr != "" {
		exitCode, err = strconv.Atoi(exitCodeStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "exit_code must be an integer")
			return
		}
		if exitCode < 0 || exitCode > 255 {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "exit_code must be between 0 and 255")
			return
		}
	}

	resp := CrashResponse{
		Message:   "crash scheduled",
		Delay:     delay.String(),
		ExitCode:  exitCode,
		Scheduled: true,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode crash response", "error", err)
	}

	// Flush the response before crashing
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	go fault.Crash(delay, exitCode)
}

// HangResponse is the JSON response for /fault/hang.
type HangResponse struct {
	Message   string `json:"message"`
	Duration  string `json:"duration"`
	Cancelled bool   `json:"cancelled,omitempty"`
}

func (h *FaultHandlers) Hang(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		writeError(w, http.StatusForbidden, "CHAOS_DISABLED", "chaos endpoints are disabled")
		return
	}

	duration, err := parseDuration(r, "duration", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}

	partial := r.URL.Query().Get("partial") == "true"

	if partial {
		// Send headers and partial body, then hang
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"message":"hanging`)); err != nil {
			slog.Warn("failed to write partial response", "error", err)
			return
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		cancelled := fault.Hang(r.Context(), duration)

		// Try to complete the response (client may have disconnected)
		suffix := `","cancelled":false}`
		if cancelled {
			suffix = `","cancelled":true}`
		}
		if _, err := w.Write([]byte(suffix)); err != nil {
			slog.Debug("failed to complete partial response after hang", "error", err)
		}
		return
	}

	// Normal mode: hang first, then respond
	cancelled := fault.Hang(r.Context(), duration)

	resp := HangResponse{
		Message:   "hang completed",
		Duration:  duration.String(),
		Cancelled: cancelled,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode hang response", "error", err)
	}
}

// OOMResponse is the JSON response for /fault/oom (sent before OOM starts).
type OOMResponse struct {
	Message string `json:"message"`
	Rate    string `json:"rate"`
	Started bool   `json:"started"`
}

func (h *FaultHandlers) OOM(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		writeError(w, http.StatusForbidden, "CHAOS_DISABLED", "chaos endpoints are disabled")
		return
	}

	rate, err := parseSize(r, "rate", 100<<20) // Default 100MB/s
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}
	if rate <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "rate must be positive")
		return
	}

	resp := OOMResponse{
		Message: "OOM simulation started",
		Rate:    formatSize(rate) + "/s",
		Started: true,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode oom response", "error", err)
	}

	// Flush the response before starting OOM
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Run OOM in background goroutine - use Background context so it survives
	// request cancellation and continues allocating until the process is killed
	go fault.OOM(context.Background(), rate)
}

// ErrorResponse is the JSON response for /fault/error.
type ErrorResponse struct {
	Injected bool   `json:"injected"`
	Status   int    `json:"status,omitempty"`
	Message  string `json:"message"`
}

func (h *FaultHandlers) Error(w http.ResponseWriter, r *http.Request) {
	if !h.enabled {
		writeError(w, http.StatusForbidden, "CHAOS_DISABLED", "chaos endpoints are disabled")
		return
	}

	rateStr := r.URL.Query().Get("rate")
	rate := 0.5 // Default 50%
	if rateStr != "" {
		var err error
		rate, err = strconv.ParseFloat(rateStr, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "rate must be a number")
			return
		}
		if rate < 0 || rate > 1 {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "rate must be between 0 and 1")
			return
		}
	}

	statusStr := r.URL.Query().Get("status")
	status := 500
	if statusStr != "" {
		var err error
		status, err = strconv.Atoi(statusStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "status must be an integer")
			return
		}
		if status < 400 || status > 599 {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "status must be between 400 and 599")
			return
		}
	}

	// Decide whether to inject error based on rate
	if rand.Float64() < rate {
		resp := ErrorResponse{
			Injected: true,
			Status:   status,
			Message:  "injected error",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			slog.Warn("failed to encode error response", "error", err)
		}
		return
	}

	resp := ErrorResponse{
		Injected: false,
		Message:  "no error injected",
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode error response", "error", err)
	}
}
