package fault

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Crash terminates the process after an optional delay.
func Crash(delay time.Duration, exitCode int) {
	if delay > 0 {
		slog.Warn("crash scheduled", "delay", delay, "exit_code", exitCode)
		time.Sleep(delay)
	}
	slog.Error("crashing process", "exit_code", exitCode)
	os.Exit(exitCode)
}

// Hang blocks the current goroutine for the specified duration.
// If duration is 0 or negative, blocks indefinitely.
// Returns true if the hang was interrupted by context cancellation.
func Hang(ctx context.Context, duration time.Duration) bool {
	slog.Warn("hang initiated", "duration", duration)

	if duration <= 0 {
		// Block indefinitely until context is cancelled
		<-ctx.Done()
		return true
	}

	select {
	case <-time.After(duration):
		return false
	case <-ctx.Done():
		return true
	}
}

// oomMu protects oomSink from concurrent access.
var oomMu sync.Mutex

// oomSink prevents the garbage collector from collecting OOM allocations.
// This is intentionally a package-level variable to ensure allocations persist.
var oomSink [][]byte

// OOM allocates memory at the specified rate until the process is killed.
// Rate is in bytes per second. Returns only if context is cancelled.
// Note: Only one OOM simulation should run at a time per process.
func OOM(ctx context.Context, rate int64) {
	oomMu.Lock()
	defer oomMu.Unlock()

	slog.Warn("OOM simulation started", "rate_bytes_per_sec", rate)

	// Use a slice of slices to prevent garbage collection
	oomSink = nil // Reset from any previous run

	// Calculate allocation size per tick (10 ticks per second)
	tickInterval := 100 * time.Millisecond
	allocSize := max(rate/10, 1024) // Minimum 1KB per tick

	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	totalAllocated := int64(0)
	for {
		select {
		case <-ctx.Done():
			slog.Info("OOM simulation cancelled", "total_allocated", totalAllocated)
			return
		case <-ticker.C:
			// Allocate memory and touch it to ensure it's actually allocated
			buf := make([]byte, allocSize)
			for i := range buf {
				buf[i] = byte(i)
			}
			oomSink = append(oomSink, buf)
			totalAllocated += allocSize

			if totalAllocated%(100<<20) == 0 { // Log every 100MB
				slog.Info("OOM progress", "allocated_mb", totalAllocated>>20)
			}
		}
	}
}
