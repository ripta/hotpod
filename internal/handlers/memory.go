package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/ripta/hotpod/internal/config"
	"github.com/ripta/hotpod/internal/load"
)

const (
	patternZero       = "zero"
	patternRandom     = "random"
	patternSequential = "sequential"
)

// MemoryHandlers provides the /memory endpoint handler.
type MemoryHandlers struct {
	tracker *load.Tracker
	maxSize int64
}

// NewMemoryHandlers creates handlers for memory load endpoints.
func NewMemoryHandlers(tracker *load.Tracker, cfg *config.Config) *MemoryHandlers {
	return &MemoryHandlers{
		tracker: tracker,
		maxSize: cfg.MaxMemorySize,
	}
}

// Register adds memory load routes to the mux.
func (h *MemoryHandlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /memory", h.Memory)
}

// MemoryResponse is the JSON response for /memory.
type MemoryResponse struct {
	// RequestedSize is the size parameter value in bytes
	RequestedSize int64 `json:"requested_size"`
	// RequestedSizeHuman is the human-readable size
	RequestedSizeHuman string `json:"requested_size_human"`
	// Duration is how long the memory was held
	Duration string `json:"duration"`
	// Pattern is the fill pattern used
	Pattern string `json:"pattern"`
	// Cancelled indicates if the operation was cancelled
	Cancelled bool `json:"cancelled,omitempty"`
	// LimitApplied indicates if the size was capped by the safety limit
	LimitApplied bool `json:"limit_applied,omitempty"`
}

func (h *MemoryHandlers) Memory(w http.ResponseWriter, r *http.Request) {
	size, err := parseSize(r, "size", 10<<20) // Default 10MB
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}
	if size < 0 {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "size must be non-negative")
		return
	}

	duration, err := parseDuration(r, "duration", 10*time.Second)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", err.Error())
		return
	}
	if duration < 0 {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "duration must be non-negative")
		return
	}

	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		pattern = patternRandom
	}
	if pattern != patternZero && pattern != patternRandom && pattern != patternSequential {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "pattern must be zero, random, or sequential")
		return
	}

	limitApplied := false
	if h.maxSize > 0 && size > h.maxSize {
		size = h.maxSize
		limitApplied = true
	}

	release, err := h.tracker.Acquire(load.OpTypeMemory)
	if err != nil {
		writeError(w, http.StatusTooManyRequests, "TOO_MANY_REQUESTS", "concurrent operation limit exceeded")
		return
	}
	defer release()

	cancelled := holdMemory(r.Context(), size, duration, pattern)

	resp := MemoryResponse{
		RequestedSize:      size,
		RequestedSizeHuman: formatSize(size),
		Duration:           duration.String(),
		Pattern:            pattern,
		Cancelled:          cancelled,
		LimitApplied:       limitApplied,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Warn("failed to encode memory response", "error", err)
	}
}

// holdMemory allocates and fills memory, holding it for the specified duration.
// Returns true if the operation was cancelled before completion.
func holdMemory(ctx context.Context, size int64, duration time.Duration, pattern string) bool {
	// Allocate the memory
	data := make([]byte, size)

	// Fill according to pattern
	fillMemory(data, pattern)

	// Hold the memory for the duration
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return false
	case <-ctx.Done():
		return true
	}
}

// fillMemory fills the byte slice according to the specified pattern.
func fillMemory(data []byte, pattern string) {
	switch pattern {
	case patternZero:
		// Already zero-initialized by Go
	case patternRandom:
		for i := 0; i+8 <= len(data); i += 8 {
			v := rand.Uint64()
			data[i] = byte(v)
			data[i+1] = byte(v >> 8)
			data[i+2] = byte(v >> 16)
			data[i+3] = byte(v >> 24)
			data[i+4] = byte(v >> 32)
			data[i+5] = byte(v >> 40)
			data[i+6] = byte(v >> 48)
			data[i+7] = byte(v >> 56)
		}
		for i := len(data) &^ 7; i < len(data); i++ {
			data[i] = byte(rand.Uint32())
		}
	case patternSequential:
		for i := range data {
			data[i] = byte(i)
		}
	}
}

// parseSize parses a size parameter from the request.
func parseSize(r *http.Request, key string, defaultVal int64) (int64, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return defaultVal, nil
	}
	return config.ParseSize(v)
}

// formatSize formats bytes as a human-readable string.
func formatSize(bytes int64) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
		TB = 1 << 40
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1fTB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
