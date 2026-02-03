package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/ripta/hotpod/internal/server"
)

// HealthHandlers provides health check endpoint handlers.
type HealthHandlers struct {
	lifecycle *server.Lifecycle
}

// NewHealthHandlers creates handlers for health endpoints.
func NewHealthHandlers(lc *server.Lifecycle) *HealthHandlers {
	return &HealthHandlers{lifecycle: lc}
}

// Register adds health routes to the mux.
func (h *HealthHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("GET /readyz", h.Readyz)
	mux.HandleFunc("GET /startupz", h.Startupz)
}

// HealthResponse is the JSON response for health endpoints.
type HealthResponse struct {
	// Status is "ok", "not_ready", or "starting"
	Status string `json:"status"`
	// Reason explains why the server is not ready (omitted when ok)
	Reason string `json:"reason,omitempty"`
	// Remaining is the time until startup completes (only for /startupz)
	Remaining string `json:"remaining,omitempty"`
}

func (h *HealthHandlers) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(HealthResponse{Status: "ok"}); err != nil {
		slog.Warn("failed to encode healthz response", "error", err)
	}
}

func (h *HealthHandlers) Readyz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var resp HealthResponse
	var status int

	switch h.lifecycle.State() {
	case server.StateStarting:
		status = http.StatusServiceUnavailable
		resp = HealthResponse{Status: "not_ready", Reason: "server is starting"}
	case server.StateShuttingDown:
		status = http.StatusServiceUnavailable
		resp = HealthResponse{Status: "not_ready", Reason: "server is shutting down"}
	case server.StateReady:
		status = http.StatusOK
		resp = HealthResponse{Status: "ok"}
	default:
		status = http.StatusInternalServerError
		resp = HealthResponse{Status: "error", Reason: "unknown server state"}
	}

	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode readyz response", "error", err)
	}
}

func (h *HealthHandlers) Startupz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.lifecycle.State() == server.StateStarting {
		remaining := h.lifecycle.StartupRemaining()
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(HealthResponse{
			Status:    "starting",
			Reason:    "startup in progress",
			Remaining: remaining.String(),
		}); err != nil {
			slog.Warn("failed to encode startupz response", "error", err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(HealthResponse{Status: "ok"}); err != nil {
		slog.Warn("failed to encode startupz response", "error", err)
	}
}
