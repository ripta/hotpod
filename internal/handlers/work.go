package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ripta/hotpod/internal/config"
	"github.com/ripta/hotpod/internal/load"
)

// workProfile defines the parameters for a composite workload.
type workProfile struct {
	cpuDuration time.Duration
	cpuCores    int
	intensity   string
	memorySize  int64
	latency     time.Duration
}

var workProfiles = map[string]workProfile{
	"web": {
		cpuDuration: 20 * time.Millisecond,
		cpuCores:    1,
		intensity:   intensityMedium,
		memorySize:  5 << 20, // 5MB
		latency:     50 * time.Millisecond,
	},
	"api": {
		cpuDuration: 50 * time.Millisecond,
		cpuCores:    1,
		intensity:   intensityMedium,
		memorySize:  2 << 20, // 2MB
		latency:     20 * time.Millisecond,
	},
	"worker": {
		cpuDuration: 200 * time.Millisecond,
		cpuCores:    2,
		intensity:   intensityHigh,
		memorySize:  50 << 20, // 50MB
		latency:     100 * time.Millisecond,
	},
	"heavy": {
		cpuDuration: 500 * time.Millisecond,
		cpuCores:    4,
		intensity:   intensityHigh,
		memorySize:  100 << 20, // 100MB
		latency:     10 * time.Millisecond,
	},
}

// WorkHandlers provides the /work endpoint handler.
type WorkHandlers struct {
	tracker       *load.Tracker
	maxCPUDur     time.Duration
	maxMemorySize int64
}

// NewWorkHandlers creates handlers for composite work endpoints.
func NewWorkHandlers(tracker *load.Tracker, cfg *config.Config) *WorkHandlers {
	return &WorkHandlers{
		tracker:       tracker,
		maxCPUDur:     cfg.MaxCPUDuration,
		maxMemorySize: cfg.MaxMemorySize,
	}
}

// Register adds work routes to the mux.
func (h *WorkHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /work", h.Work)
}

// WorkResponse is the JSON response for /work.
type WorkResponse struct {
	// Profile is the workload profile used
	Profile string `json:"profile"`
	// Variance is the variance multiplier applied
	Variance float64 `json:"variance"`
	// ActualDuration is the total time for the composite workload
	ActualDuration string `json:"actual_duration"`
	// CPUDuration is how long CPU work ran
	CPUDuration string `json:"cpu_duration"`
	// CPUIterations is the number of CPU work iterations
	CPUIterations int64 `json:"cpu_iterations"`
	// MemorySize is the amount of memory allocated
	MemorySize int64 `json:"memory_size"`
	// MemorySizeHuman is the human-readable memory size
	MemorySizeHuman string `json:"memory_size_human"`
	// Latency is the simulated latency duration
	Latency string `json:"latency"`
	// Cancelled indicates if the operation was cancelled
	Cancelled bool `json:"cancelled,omitempty"`
	// LimitsApplied indicates if any limits were applied
	LimitsApplied bool `json:"limits_applied,omitempty"`
}

func (h *WorkHandlers) Work(w http.ResponseWriter, r *http.Request) {
	profileName := r.URL.Query().Get("profile")
	if profileName == "" {
		profileName = "web"
	}

	profile, ok := workProfiles[profileName]
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "profile must be web, api, worker, or heavy")
		return
	}

	varianceStr := r.URL.Query().Get("variance")
	variance := 0.0
	if varianceStr != "" {
		var err error
		variance, err = strconv.ParseFloat(varianceStr, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "variance must be a number")
			return
		}
		if variance < 0 || variance > 1 {
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "variance must be between 0 and 1")
			return
		}
	}

	cpuDuration := applyVariance(profile.cpuDuration, variance)
	memorySize := applyVarianceInt64(profile.memorySize, variance)
	latency := applyVariance(profile.latency, variance)

	limitsApplied := false
	if h.maxCPUDur > 0 && cpuDuration > h.maxCPUDur {
		cpuDuration = h.maxCPUDur
		limitsApplied = true
	}
	if h.maxMemorySize > 0 && memorySize > h.maxMemorySize {
		memorySize = h.maxMemorySize
		limitsApplied = true
	}

	release, err := h.tracker.Acquire(load.OpTypeWork)
	if err != nil {
		writeError(w, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "concurrent operation limit exceeded")
		return
	}
	defer release()

	start := time.Now()
	cpuIterations, cancelled := h.runWorkload(r.Context(), cpuDuration, profile.cpuCores, profile.intensity, memorySize, latency)
	elapsed := time.Since(start)

	resp := WorkResponse{
		Profile:         profileName,
		Variance:        variance,
		ActualDuration:  elapsed.String(),
		CPUDuration:     cpuDuration.String(),
		CPUIterations:   cpuIterations,
		MemorySize:      memorySize,
		MemorySizeHuman: formatSize(memorySize),
		Latency:         latency.String(),
		Cancelled:       cancelled,
		LimitsApplied:   limitsApplied,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode work response", "error", err)
	}
}

func (h *WorkHandlers) runWorkload(ctx context.Context, cpuDuration time.Duration, cpuCores int, intensity string, memorySize int64, latency time.Duration) (cpuIterations int64, cancelled bool) {
	var wg sync.WaitGroup
	var cpuCancelled, memCancelled, sleepCancelled bool

	wg.Add(3)

	go func() {
		defer wg.Done()
		cpuIterations, cpuCancelled = burnCPU(ctx, cpuDuration, cpuCores, intensity)
	}()

	go func() {
		defer wg.Done()
		memCancelled = holdMemory(ctx, memorySize, cpuDuration, patternRandom)
	}()

	go func() {
		defer wg.Done()
		sleepCancelled = sleep(ctx, latency)
	}()

	wg.Wait()

	cancelled = cpuCancelled || memCancelled || sleepCancelled
	return cpuIterations, cancelled
}

// applyVariance applies a random variance multiplier to a duration.
// Variance of 0.2 means the result will be in the range [0.8*d, 1.2*d].
func applyVariance(d time.Duration, variance float64) time.Duration {
	if variance == 0 {
		return d
	}

	mult := 1.0 + (rand.Float64()*2-1)*variance
	return time.Duration(float64(d) * mult)
}

// applyVarianceInt64 applies a random variance multiplier to an int64.
func applyVarianceInt64(n int64, variance float64) int64 {
	if variance == 0 {
		return n
	}

	mult := 1.0 + (rand.Float64()*2-1)*variance
	return int64(float64(n) * mult)
}
