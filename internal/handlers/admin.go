package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ripta/hotpod/internal/config"
	"github.com/ripta/hotpod/internal/fault"
	"github.com/ripta/hotpod/internal/queue"
	"github.com/ripta/hotpod/internal/server"
)

// AdminHandlers provides admin endpoint handlers for runtime configuration.
type AdminHandlers struct {
	// token is the authentication token (empty = open access)
	token string
	// lifecycle is the server lifecycle manager
	lifecycle *server.Lifecycle
	// injector is the fault injection manager
	injector *fault.Injector
	// cfg is the server configuration
	cfg *config.Config
	// queue is the work queue (nil in sidecar mode)
	queue *queue.Queue
	// workerPool is the queue worker pool (nil in sidecar mode)
	workerPool *queue.WorkerPool
}

// NewAdminHandlers creates handlers for admin endpoints.
func NewAdminHandlers(token string, lc *server.Lifecycle, injector *fault.Injector, cfg *config.Config, q *queue.Queue, wp *queue.WorkerPool) *AdminHandlers {
	return &AdminHandlers{
		token:      token,
		lifecycle:  lc,
		injector:   injector,
		cfg:        cfg,
		queue:      q,
		workerPool: wp,
	}
}

// Register adds admin routes to the mux.
func (h *AdminHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /admin/ready", h.Ready)
	mux.HandleFunc("POST /admin/gc", h.GC)
	mux.HandleFunc("GET /admin/config", h.Config)
	mux.HandleFunc("POST /admin/reset", h.Reset)
	mux.HandleFunc("POST /admin/error-rate", h.ErrorRate)
	mux.HandleFunc("POST /admin/queue/pause", h.QueuePause)
	mux.HandleFunc("POST /admin/queue/resume", h.QueueResume)
}

func (h *AdminHandlers) authenticate(w http.ResponseWriter, r *http.Request) bool {
	if h.token == "" {
		return true
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Admin-Token")), []byte(h.token)) == 1 {
		return true
	}
	writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or missing admin token")
	return false
}

// AdminReadyResponse is the JSON response for POST /admin/ready.
type AdminReadyResponse struct {
	Ready    bool   `json:"ready"`
	Override *bool  `json:"override"`
	State    string `json:"state"`
}

func (h *AdminHandlers) Ready(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(w, r) {
		return
	}

	stateParam := r.URL.Query().Get("state")

	switch stateParam {
	case "true":
		v := true
		h.lifecycle.SetReadyOverride(&v)
	case "false":
		v := false
		h.lifecycle.SetReadyOverride(&v)
	case "":
		// Toggle: if override exists, clear it; otherwise force not-ready
		if h.lifecycle.ReadyOverride() != nil {
			h.lifecycle.SetReadyOverride(nil)
		} else {
			v := false
			h.lifecycle.SetReadyOverride(&v)
		}
	default:
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "state must be true, false, or empty")
		return
	}

	resp := AdminReadyResponse{
		Ready:    h.lifecycle.IsReady(),
		Override: h.lifecycle.ReadyOverride(),
		State:    h.lifecycle.State().String(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode admin ready response", "error", err)
	}
}

// AdminGCMemStats holds memory stats for the GC response.
type AdminGCMemStats struct {
	Alloc uint64 `json:"alloc"`
	Sys   uint64 `json:"sys"`
	NumGC uint32 `json:"num_gc"`
}

// AdminGCResponse is the JSON response for POST /admin/gc.
type AdminGCResponse struct {
	Before AdminGCMemStats `json:"before"`
	After  AdminGCMemStats `json:"after"`
}

func (h *AdminHandlers) GC(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(w, r) {
		return
	}

	var beforeStats runtime.MemStats
	runtime.ReadMemStats(&beforeStats)

	runtime.GC()

	var afterStats runtime.MemStats
	runtime.ReadMemStats(&afterStats)

	resp := AdminGCResponse{
		Before: AdminGCMemStats{
			Alloc: beforeStats.Alloc,
			Sys:   beforeStats.Sys,
			NumGC: beforeStats.NumGC,
		},
		After: AdminGCMemStats{
			Alloc: afterStats.Alloc,
			Sys:   afterStats.Sys,
			NumGC: afterStats.NumGC,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode admin gc response", "error", err)
	}
}

// AdminConfigFaultEndpoint holds per-endpoint fault injection config.
type AdminConfigFaultEndpoint struct {
	Rate      float64 `json:"rate"`
	Codes     []int   `json:"codes"`
	ExpiresAt string  `json:"expires_at,omitempty"`
}

// AdminConfigFault holds fault injection state.
type AdminConfigFault struct {
	Global    *AdminConfigFaultEndpoint            `json:"global"`
	Endpoints map[string]*AdminConfigFaultEndpoint `json:"endpoints,omitempty"`
}

// AdminConfigQueue holds queue state for the config response.
type AdminConfigQueue struct {
	Available bool `json:"available"`
	Depth     int  `json:"depth,omitempty"`
	Paused    bool `json:"paused,omitempty"`
	Workers   int  `json:"workers,omitempty"`
}

// AdminConfigLimits holds configuration limits.
type AdminConfigLimits struct {
	MaxCPUDuration   string `json:"max_cpu_duration"`
	MaxMemorySize    string `json:"max_memory_size"`
	MaxIOSize        string `json:"max_io_size"`
	MaxConcurrentOps int    `json:"max_concurrent_ops"`
	RequestTimeout   string `json:"request_timeout"`
}

// AdminConfigSidecar holds sidecar configuration.
type AdminConfigSidecar struct {
	Active          bool   `json:"active"`
	CPUBaseline     string `json:"cpu_baseline,omitempty"`
	CPUJitter       string `json:"cpu_jitter,omitempty"`
	MemoryBaseline  string `json:"memory_baseline,omitempty"`
	RequestOverhead string `json:"request_overhead,omitempty"`
}

// AdminConfigResponse is the JSON response for GET /admin/config.
type AdminConfigResponse struct {
	Mode    string             `json:"mode"`
	Limits  AdminConfigLimits  `json:"limits"`
	Fault   AdminConfigFault   `json:"fault"`
	Queue   AdminConfigQueue   `json:"queue"`
	Sidecar AdminConfigSidecar `json:"sidecar"`
}

func (h *AdminHandlers) Config(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(w, r) {
		return
	}

	faultState := AdminConfigFault{}
	if gc := h.injector.GetGlobalConfig(); gc != nil {
		faultState.Global = &AdminConfigFaultEndpoint{
			Rate:  gc.Rate,
			Codes: gc.Codes,
		}
		if !gc.ExpiresAt.IsZero() {
			faultState.Global.ExpiresAt = gc.ExpiresAt.Format(time.RFC3339)
		}
	}

	epConfigs := h.injector.GetEndpointConfigs()
	if len(epConfigs) > 0 {
		faultState.Endpoints = make(map[string]*AdminConfigFaultEndpoint, len(epConfigs))
		for ep, ec := range epConfigs {
			entry := &AdminConfigFaultEndpoint{
				Rate:  ec.Rate,
				Codes: ec.Codes,
			}
			if !ec.ExpiresAt.IsZero() {
				entry.ExpiresAt = ec.ExpiresAt.Format(time.RFC3339)
			}
			faultState.Endpoints[ep] = entry
		}
	}

	queueState := AdminConfigQueue{
		Available: h.queue != nil,
	}
	if h.queue != nil {
		queueState.Depth = h.queue.Depth()
		queueState.Paused = h.queue.IsPaused()
	}
	if h.workerPool != nil {
		queueState.Workers = h.workerPool.ActiveWorkers()
	}

	sidecarState := AdminConfigSidecar{
		Active: h.cfg.Mode == "sidecar",
	}
	if h.cfg.Mode == "sidecar" {
		sidecarState.CPUBaseline = h.cfg.SidecarCPUBaseline.String()
		sidecarState.CPUJitter = h.cfg.SidecarCPUJitter.String()
		sidecarState.MemoryBaseline = formatSize(h.cfg.SidecarMemoryBaseline)
		sidecarState.RequestOverhead = h.cfg.SidecarRequestOverhead.String()
	}

	resp := AdminConfigResponse{
		Mode: h.cfg.Mode,
		Limits: AdminConfigLimits{
			MaxCPUDuration:   h.cfg.MaxCPUDuration.String(),
			MaxMemorySize:    formatSize(h.cfg.MaxMemorySize),
			MaxIOSize:        formatSize(h.cfg.MaxIOSize),
			MaxConcurrentOps: h.cfg.MaxConcurrentOps,
			RequestTimeout:   h.cfg.RequestTimeout.String(),
		},
		Fault:   faultState,
		Queue:   queueState,
		Sidecar: sidecarState,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode admin config response", "error", err)
	}
}

// AdminResetResponse is the JSON response for POST /admin/reset.
type AdminResetResponse struct {
	FaultReset           bool `json:"fault_reset"`
	QueueCleared         int  `json:"queue_cleared"`
	WorkersStopped       bool `json:"workers_stopped"`
	ReadyOverrideCleared bool `json:"ready_override_cleared"`
}

func (h *AdminHandlers) Reset(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(w, r) {
		return
	}

	h.injector.Reset()

	resp := AdminResetResponse{
		FaultReset:           true,
		ReadyOverrideCleared: true,
	}

	if h.queue != nil {
		resp.QueueCleared = h.queue.Clear()
	}
	if h.workerPool != nil {
		h.workerPool.Stop()
		resp.WorkersStopped = true
	}

	h.lifecycle.SetReadyOverride(nil)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode admin reset response", "error", err)
	}
}

// AdminErrorRateResponse is the JSON response for POST /admin/error-rate.
type AdminErrorRateResponse struct {
	Endpoint string  `json:"endpoint"`
	Rate     float64 `json:"rate"`
	Codes    []int   `json:"codes"`
	Duration string  `json:"duration,omitempty"`
}

func (h *AdminHandlers) ErrorRate(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(w, r) {
		return
	}

	endpoint := r.URL.Query().Get("endpoint")

	rateStr := r.URL.Query().Get("rate")
	if rateStr == "" {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "rate is required")
		return
	}
	rate, err := strconv.ParseFloat(rateStr, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "rate must be a number")
		return
	}
	if rate < 0 || rate > 1 {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "rate must be between 0 and 1")
		return
	}

	codesStr := r.URL.Query().Get("codes")
	var codes []int
	if codesStr != "" {
		for _, s := range strings.Split(codesStr, ",") {
			s = strings.TrimSpace(s)
			code, err := strconv.Atoi(s)
			if err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "codes must be comma-separated integers")
				return
			}
			if code < 100 || code > 599 {
				writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "codes must be valid HTTP status codes (100-599)")
				return
			}
			codes = append(codes, code)
		}
	} else {
		codes = []int{500}
	}

	cfg := &fault.ErrorConfig{
		Rate:  rate,
		Codes: codes,
	}

	durationStr := r.URL.Query().Get("duration")
	if durationStr != "" {
		d, err := time.ParseDuration(durationStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "invalid duration")
			return
		}
		cfg.ExpiresAt = time.Now().Add(d)
	}

	if endpoint == "" {
		h.injector.SetGlobalConfig(cfg)
	} else {
		h.injector.SetEndpointConfig(endpoint, cfg)
	}

	resp := AdminErrorRateResponse{
		Endpoint: endpoint,
		Rate:     rate,
		Codes:    codes,
	}
	if durationStr != "" {
		resp.Duration = durationStr
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode admin error-rate response", "error", err)
	}
}

// AdminQueuePauseResponse is the JSON response for POST /admin/queue/pause.
type AdminQueuePauseResponse struct {
	Paused bool `json:"paused"`
}

func (h *AdminHandlers) QueuePause(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(w, r) {
		return
	}

	if h.queue == nil {
		writeError(w, http.StatusNotFound, "QUEUE_NOT_AVAILABLE", "queue is not available in this mode")
		return
	}

	h.queue.Pause()

	resp := AdminQueuePauseResponse{Paused: true}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode admin queue pause response", "error", err)
	}
}

// AdminQueueResumeResponse is the JSON response for POST /admin/queue/resume.
type AdminQueueResumeResponse struct {
	Paused bool `json:"paused"`
}

func (h *AdminHandlers) QueueResume(w http.ResponseWriter, r *http.Request) {
	if !h.authenticate(w, r) {
		return
	}

	if h.queue == nil {
		writeError(w, http.StatusNotFound, "QUEUE_NOT_AVAILABLE", "queue is not available in this mode")
		return
	}

	h.queue.Resume()

	resp := AdminQueueResumeResponse{Paused: false}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode admin queue resume response", "error", err)
	}
}
