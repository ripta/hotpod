package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ripta/hotpod/internal/config"
	"github.com/ripta/hotpod/internal/load"
)

const (
	intensityLow    = "low"
	intensityMedium = "medium"
	intensityHigh   = "high"
)

// CPUHandlers provides the /cpu endpoint handler.
type CPUHandlers struct {
	tracker     *load.Tracker
	maxDuration time.Duration
}

// NewCPUHandlers creates handlers for CPU load endpoints.
func NewCPUHandlers(tracker *load.Tracker, cfg *config.Config) *CPUHandlers {
	return &CPUHandlers{
		tracker:     tracker,
		maxDuration: cfg.MaxCPUDuration,
	}
}

// Register adds CPU load routes to the mux.
func (h *CPUHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /cpu", h.CPU)
}

// CPUResponse is the JSON response for /cpu.
type CPUResponse struct {
	// RequestedDuration is the duration parameter value
	RequestedDuration string `json:"requested_duration"`
	// ActualDuration is how long the operation actually took
	ActualDuration string `json:"actual_duration"`
	// Cores is the number of goroutines used for CPU work
	Cores int `json:"cores"`
	// Intensity is the intensity level used
	Intensity string `json:"intensity"`
	// Iterations is the total number of work iterations completed
	Iterations int64 `json:"iterations"`
	// Cancelled indicates if the operation was cancelled
	Cancelled bool `json:"cancelled,omitempty"`
	// LimitApplied indicates if the duration was capped by the safety limit
	LimitApplied bool `json:"limit_applied,omitempty"`
}

func (h *CPUHandlers) CPU(w http.ResponseWriter, r *http.Request) {
	duration, err := parseDuration(r, "duration", 1*time.Second)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}
	if duration < 0 {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "duration must be non-negative")
		return
	}

	cores, err := parseInt(r, "cores", 1)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}
	if cores < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "cores must be at least 1")
		return
	}

	intensity := r.URL.Query().Get("intensity")
	if intensity == "" {
		intensity = intensityMedium
	}
	if intensity != intensityLow && intensity != intensityMedium && intensity != intensityHigh {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "intensity must be low, medium, or high")
		return
	}

	limitApplied := false
	if h.maxDuration > 0 && duration > h.maxDuration {
		duration = h.maxDuration
		limitApplied = true
	}

	release, err := h.tracker.Acquire(load.OpTypeCPU)
	if err != nil {
		writeError(w, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "concurrent operation limit exceeded")
		return
	}
	defer release()

	start := time.Now()
	iterations, cancelled := burnCPU(r.Context(), duration, cores, intensity)
	elapsed := time.Since(start)

	resp := CPUResponse{
		RequestedDuration: duration.String(),
		ActualDuration:    elapsed.String(),
		Cores:             cores,
		Intensity:         intensity,
		Iterations:        iterations,
		Cancelled:         cancelled,
		LimitApplied:      limitApplied,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode cpu response", "error", err)
	}
}

// burnCPU performs CPU-intensive work across multiple goroutines.
// Returns the total iterations completed and whether the operation was cancelled.
func burnCPU(ctx context.Context, duration time.Duration, cores int, intensity string) (int64, bool) {
	var totalIterations atomic.Int64
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	for range cores {
		wg.Add(1)
		go func() {
			defer wg.Done()
			iterations := cpuWork(ctx, intensity)
			totalIterations.Add(iterations)
		}()
	}

	wg.Wait()

	cancelled := errors.Is(ctx.Err(), context.Canceled)
	return totalIterations.Load(), cancelled
}

// cpuWork performs CPU-intensive work until context is done.
// Returns the number of iterations completed.
func cpuWork(ctx context.Context, intensity string) int64 {
	var iterations int64

	switch intensity {
	case intensityLow:
		for {
			select {
			case <-ctx.Done():
				return iterations
			default:
				for j := range 100 {
					_ = math.Sqrt(float64(j * j))
				}
				iterations++
				runtime.Gosched()
			}
		}
	case intensityMedium:
		for {
			select {
			case <-ctx.Done():
				return iterations
			default:
				x := 1.0
				for range 1000 {
					x = math.Sin(x) + math.Cos(x)
					x = math.Sqrt(math.Abs(x) + 1)
				}
				iterations++
			}
		}
	case intensityHigh:
		data := make([]byte, 1024)
		for {
			select {
			case <-ctx.Done():
				return iterations
			default:
				hash := sha256.Sum256(data)
				copy(data[:32], hash[:])
				iterations++
			}
		}
	}

	return iterations
}
