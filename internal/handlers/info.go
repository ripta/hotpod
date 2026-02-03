package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/ripta/hotpod/internal/config"
	"github.com/ripta/hotpod/internal/server"
)

// InfoHandlers provides the /info endpoint handler.
type InfoHandlers struct {
	version   string
	lifecycle *server.Lifecycle
	config    *config.Config
}

// NewInfoHandlers creates handlers for the info endpoint.
func NewInfoHandlers(version string, lifecycle *server.Lifecycle, cfg *config.Config) *InfoHandlers {
	return &InfoHandlers{
		version:   version,
		lifecycle: lifecycle,
		config:    cfg,
	}
}

// Register adds info routes to the mux.
func (h *InfoHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /info", h.Info)
}

// InfoResponse is the JSON response for /info.
type InfoResponse struct {
	Version   string        `json:"version"`
	Uptime    string        `json:"uptime"`
	Lifecycle InfoLifecycle `json:"lifecycle"`
	Resources InfoResources `json:"resources"`
	Config    InfoConfig    `json:"config"`
}

// InfoLifecycle contains lifecycle state information.
type InfoLifecycle struct {
	State            string `json:"state"`
	StartedAt        string `json:"started_at"`
	ReadyAt          string `json:"ready_at,omitempty"`
	StartupComplete  bool   `json:"startup_complete"`
	ShuttingDown     bool   `json:"shutting_down"`
	InFlightRequests int64  `json:"in_flight_requests"`
}

// InfoResources contains runtime resource information.
type InfoResources struct {
	CPUCores    int    `json:"cpu_cores"`
	MemoryTotal uint64 `json:"memory_total"`
	MemoryUsed  uint64 `json:"memory_used"`
	Goroutines  int    `json:"goroutines"`
}

// InfoConfig contains configuration information.
type InfoConfig struct {
	Port             int    `json:"port"`
	LogLevel         string `json:"log_level"`
	MaxCPUDuration   string `json:"max_cpu_duration"`
	MaxMemorySize    string `json:"max_memory_size"`
	MaxIOSize        string `json:"max_io_size"`
	IOPath           string `json:"io_path"`
	MaxConcurrentOps int    `json:"max_concurrent_ops"`
	RequestTimeout   string `json:"request_timeout"`
	StartupDelay     string `json:"startup_delay"`
	StartupJitter    string `json:"startup_jitter"`
	ShutdownDelay    string `json:"shutdown_delay"`
	ShutdownTimeout  string `json:"shutdown_timeout"`
	DrainImmediately bool   `json:"drain_immediately"`
}

func (h *InfoHandlers) Info(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	startedAt := h.lifecycle.StartTime()
	readyAt := h.lifecycle.ReadyTime()
	uptime := time.Since(startedAt)

	lifecycle := InfoLifecycle{
		State:            h.lifecycle.State().String(),
		StartedAt:        startedAt.Format(time.RFC3339),
		StartupComplete:  h.lifecycle.IsReady(),
		ShuttingDown:     h.lifecycle.IsShuttingDown(),
		InFlightRequests: h.lifecycle.InFlightRequests(),
	}
	if !readyAt.IsZero() {
		lifecycle.ReadyAt = readyAt.Format(time.RFC3339)
	}

	resp := InfoResponse{
		Version:   h.version,
		Uptime:    uptime.Round(time.Second).String(),
		Lifecycle: lifecycle,
		Resources: InfoResources{
			CPUCores:    runtime.NumCPU(),
			MemoryTotal: memStats.Sys,
			MemoryUsed:  memStats.Alloc,
			Goroutines:  runtime.NumGoroutine(),
		},
		Config: InfoConfig{
			Port:             h.config.Port,
			LogLevel:         h.config.LogLevel,
			MaxCPUDuration:   h.config.MaxCPUDuration.String(),
			MaxMemorySize:    formatSize(h.config.MaxMemorySize),
			MaxIOSize:        formatSize(h.config.MaxIOSize),
			IOPath:           h.config.IOPath(),
			MaxConcurrentOps: h.config.MaxConcurrentOps,
			RequestTimeout:   h.config.RequestTimeout.String(),
			StartupDelay:     h.config.StartupDelay.String(),
			StartupJitter:    h.config.StartupJitter.String(),
			ShutdownDelay:    h.config.ShutdownDelay.String(),
			ShutdownTimeout:  h.config.ShutdownTimeout.String(),
			DrainImmediately: h.config.DrainImmediately,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode info response", "error", err)
	}
}
