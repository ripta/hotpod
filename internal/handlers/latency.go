package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"github.com/ripta/hotpod/internal/load"
)

// LatencyHandlers provides the /latency endpoint handler.
type LatencyHandlers struct {
	tracker *load.Tracker
}

// NewLatencyHandlers creates handlers for latency endpoints.
func NewLatencyHandlers(tracker *load.Tracker) *LatencyHandlers {
	return &LatencyHandlers{tracker: tracker}
}

// Register adds latency routes to the mux.
func (h *LatencyHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /latency", h.Latency)
}

// LatencyResponse is the JSON response for /latency.
type LatencyResponse struct {
	// RequestedDuration is the duration parameter value
	RequestedDuration string `json:"requested_duration"`
	// ActualDuration is how long the operation actually took
	ActualDuration string `json:"actual_duration"`
	// Jitter is the jitter parameter value
	Jitter string `json:"jitter,omitempty"`
	// Status is the HTTP status code returned
	Status int `json:"status"`
	// Cancelled indicates if the operation was cancelled
	Cancelled bool `json:"cancelled,omitempty"`
}

func (h *LatencyHandlers) Latency(w http.ResponseWriter, r *http.Request) {
	duration, err := parseDuration(r, "duration", 100*time.Millisecond)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}

	jitter, err := parseDuration(r, "jitter", 0)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}

	status, err := parseInt(r, "status", http.StatusOK)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}
	if status < 100 || status > 599 {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "status must be between 100 and 599")
		return
	}

	release, err := h.tracker.Acquire(load.OpTypeLatency)
	if err != nil {
		writeError(w, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "concurrent operation limit exceeded")
		return
	}
	defer release()

	actualDuration := duration
	if jitter > 0 {
		actualDuration += time.Duration(rand.Int64N(int64(jitter)))
	}

	start := time.Now()
	cancelled := sleep(r.Context(), actualDuration)
	elapsed := time.Since(start)

	resp := LatencyResponse{
		RequestedDuration: duration.String(),
		ActualDuration:    elapsed.String(),
		Status:            status,
		Cancelled:         cancelled,
	}
	if jitter > 0 {
		resp.Jitter = jitter.String()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode latency response", "error", err)
	}
}

func sleep(ctx context.Context, d time.Duration) (cancelled bool) {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return false
	case <-ctx.Done():
		return true
	}
}

func parseDuration(r *http.Request, key string, defaultVal time.Duration) (time.Duration, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal, nil
	}
	return time.ParseDuration(v)
}

func parseInt(r *http.Request, key string, defaultVal int) (int, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal, nil
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return i, nil
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]string{"error": message, "code": code}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode error response", "error", err)
	}
}
