package server

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/jonboulle/clockwork"
)

// State represents the server lifecycle state.
type State int32

const (
	StateStarting State = iota
	StateReady
	StateShuttingDown
)

func (s State) String() string {
	switch s {
	case StateStarting:
		return "starting"
	case StateReady:
		return "ready"
	case StateShuttingDown:
		return "shutting_down"
	default:
		return "unknown"
	}
}

// Lifecycle manages server startup and shutdown states.
type Lifecycle struct {
	// clock provides time operations (real or fake for testing)
	clock clockwork.Clock
	// state holds the current lifecycle state (StateStarting, StateReady, StateShuttingDown)
	state atomic.Int32
	// inFlight tracks the number of requests currently being processed
	inFlight atomic.Int64
	// startTime is when the lifecycle was created
	startTime time.Time
	// readyTime is when the server became ready (for future metrics/observability)
	readyTime time.Time
	// startupDuration is the actual delay (including jitter) before becoming ready
	startupDuration time.Duration

	// drainImmediately rejects new requests immediately when shutting down
	drainImmediately bool
	// shutdownDelay is the pre-stop delay before starting graceful shutdown
	shutdownDelay time.Duration
	// shutdownTimeout is the max time to wait for in-flight requests to complete
	shutdownTimeout time.Duration
}

// NewLifecycle creates a new lifecycle manager.
func NewLifecycle(startupDelay, startupJitter, shutdownDelay, shutdownTimeout time.Duration, drainImmediately bool) *Lifecycle {
	return NewLifecycleWithClock(clockwork.NewRealClock(), startupDelay, startupJitter, shutdownDelay, shutdownTimeout, drainImmediately)
}

// NewLifecycleWithClock creates a lifecycle manager with a custom clock for testing.
func NewLifecycleWithClock(clock clockwork.Clock, startupDelay, startupJitter, shutdownDelay, shutdownTimeout time.Duration, drainImmediately bool) *Lifecycle {
	actualDelay := startupDelay
	if startupJitter > 0 {
		actualDelay += time.Duration(rand.Int64N(int64(startupJitter)))
	}

	lc := &Lifecycle{
		clock:            clock,
		startTime:        clock.Now(),
		startupDuration:  actualDelay,
		drainImmediately: drainImmediately,
		shutdownDelay:    shutdownDelay,
		shutdownTimeout:  shutdownTimeout,
	}
	lc.state.Store(int32(StateStarting))

	if actualDelay > 0 {
		slog.Info("startup delay configured", "delay", actualDelay)
		go lc.waitForStartup()
	} else {
		lc.becomeReady()
	}

	return lc
}

func (lc *Lifecycle) waitForStartup() {
	lc.clock.Sleep(lc.startupDuration)
	lc.becomeReady()
}

func (lc *Lifecycle) becomeReady() {
	lc.readyTime = lc.clock.Now()
	lc.state.Store(int32(StateReady))
	slog.Info("server is ready")
}

// State returns the current lifecycle state.
func (lc *Lifecycle) State() State {
	return State(lc.state.Load())
}

// IsReady returns true if the server is ready to accept traffic.
func (lc *Lifecycle) IsReady() bool {
	return lc.State() == StateReady
}

// IsShuttingDown returns true if the server is shutting down.
func (lc *Lifecycle) IsShuttingDown() bool {
	return lc.State() == StateShuttingDown
}

// StartupRemaining returns the remaining startup delay, or 0 if ready.
func (lc *Lifecycle) StartupRemaining() time.Duration {
	if lc.State() != StateStarting {
		return 0
	}
	elapsed := lc.clock.Since(lc.startTime)
	remaining := lc.startupDuration - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// InFlightRequests returns the number of requests currently being processed.
func (lc *Lifecycle) InFlightRequests() int64 {
	return lc.inFlight.Load()
}

// TrackRequest increments the in-flight counter and returns a function to decrement it.
func (lc *Lifecycle) TrackRequest() func() {
	lc.inFlight.Add(1)
	return func() {
		lc.inFlight.Add(-1)
	}
}

// ShouldRejectRequest returns true if new requests should be rejected.
func (lc *Lifecycle) ShouldRejectRequest() bool {
	return lc.drainImmediately && lc.IsShuttingDown()
}

// ReadyTime returns when the server became ready, or zero if not yet ready.
func (lc *Lifecycle) ReadyTime() time.Time {
	return lc.readyTime
}

// Shutdown initiates graceful shutdown and returns when complete or context is cancelled.
func (lc *Lifecycle) Shutdown(ctx context.Context) error {
	lc.state.Store(int32(StateShuttingDown))
	slog.Info("shutdown initiated")

	if lc.shutdownDelay > 0 {
		slog.Info("pre-stop delay", "delay", lc.shutdownDelay)
		select {
		case <-lc.clock.After(lc.shutdownDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	deadline := lc.clock.Now().Add(lc.shutdownTimeout)
	for lc.inFlight.Load() > 0 {
		if lc.clock.Now().After(deadline) {
			slog.Warn("shutdown timeout exceeded", "in_flight", lc.inFlight.Load())
			break
		}
		select {
		case <-lc.clock.After(100 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	slog.Info("shutdown complete", "in_flight", lc.inFlight.Load())
	return nil
}
