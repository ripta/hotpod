package sidecar

import (
	"context"
	"log/slog"
	"math"
	"math/rand/v2"
	"runtime"
	"sync"
	"time"

	"github.com/ripta/hotpod/internal/metrics"
)

// Runner maintains steady CPU and memory consumption to simulate a sidecar
// container (e.g., service mesh proxy) for ContainerResource HPA testing.
type Runner struct {
	cpuBaseline    time.Duration
	cpuJitter      time.Duration
	memoryBaseline int64

	mu       sync.Mutex
	memory   []byte
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
}

// New creates a Runner with the given resource baselines.
func New(cpuBaseline, cpuJitter time.Duration, memoryBaseline int64) *Runner {
	return &Runner{
		cpuBaseline:    cpuBaseline,
		cpuJitter:      cpuJitter,
		memoryBaseline: memoryBaseline,
	}
}

// Start allocates baseline memory and begins the CPU burn loop. It blocks
// until the provided context is cancelled.
func (r *Runner) Start(ctx context.Context) {
	ctx, r.cancel = context.WithCancel(ctx)
	r.done = make(chan struct{})

	r.mu.Lock()
	if r.memoryBaseline > 0 {
		r.memory = make([]byte, r.memoryBaseline)
		// Touch every page to ensure the memory is actually allocated by the OS.
		for i := range r.memory {
			r.memory[i] = 1
		}
		metrics.SidecarMemoryHeldBytes.Set(float64(r.memoryBaseline))
	}
	r.mu.Unlock()

	slog.Info("sidecar runner started",
		"cpu_baseline", r.cpuBaseline,
		"cpu_jitter", r.cpuJitter,
		"memory_baseline", r.memoryBaseline,
	)

	r.cpuLoop(ctx)
	close(r.done)
}

// Stop releases held memory and signals the CPU loop to exit. It is safe to
// call multiple times.
func (r *Runner) Stop() {
	r.stopOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		if r.done != nil {
			<-r.done
		}

		r.mu.Lock()
		r.memory = nil
		r.mu.Unlock()

		metrics.SidecarMemoryHeldBytes.Set(0)
		slog.Info("sidecar runner stopped")
	})
}

func (r *Runner) cpuLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			burnDuration := r.cpuBaseline
			if r.cpuJitter > 0 {
				jitter := time.Duration(rand.Int64N(int64(r.cpuJitter)*2+1)) - r.cpuJitter
				burnDuration += jitter
				if burnDuration < 0 {
					burnDuration = 0
				}
			}
			if burnDuration > 0 {
				cpuBurn(burnDuration)
				metrics.SidecarCPUBurnSecondsTotal.Add(burnDuration.Seconds())
			}
		}
	}
}

// cpuBurn performs a tight compute loop for the given duration.
func cpuBurn(d time.Duration) {
	deadline := time.Now().Add(d)
	x := 1.0
	for time.Now().Before(deadline) {
		for range 1000 {
			x = math.Sin(x) + math.Cos(x)
			x = math.Sqrt(math.Abs(x) + 1)
		}
	}
	runtime.KeepAlive(x)
}
