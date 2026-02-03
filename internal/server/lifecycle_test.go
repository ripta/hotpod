package server

import (
	"context"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

type stateStringTest struct {
	state State
	want  string
}

var stateStringTests = []stateStringTest{
	{StateStarting, "starting"},
	{StateReady, "ready"},
	{StateShuttingDown, "shutting_down"},
	{State(99), "unknown"},
}

func TestLifecycleNoDelay(t *testing.T) {
	clock := clockwork.NewFakeClock()
	lc := NewLifecycleWithClock(clock, 0, 0, 0, 30*time.Second, false)

	if !lc.IsReady() {
		t.Error("expected lifecycle to be ready with no delay")
	}
	if lc.State() != StateReady {
		t.Errorf("State() = %v, want StateReady", lc.State())
	}
}

func TestLifecycleWithDelay(t *testing.T) {
	clock := clockwork.NewFakeClock()
	lc := NewLifecycleWithClock(clock, 100*time.Millisecond, 0, 0, 30*time.Second, false)

	if lc.State() != StateStarting {
		t.Errorf("initial State() = %v, want StateStarting", lc.State())
	}
	if lc.IsReady() {
		t.Error("expected lifecycle to not be ready during startup delay")
	}

	remaining := lc.StartupRemaining()
	if remaining != 100*time.Millisecond {
		t.Errorf("StartupRemaining() = %v, want 100ms", remaining)
	}

	if err := clock.BlockUntilContext(context.Background(), 1); err != nil {
		t.Fatalf("BlockUntilContext: %v", err)
	}
	clock.Advance(100 * time.Millisecond)

	time.Sleep(10 * time.Millisecond)

	if !lc.IsReady() {
		t.Error("expected lifecycle to be ready after delay")
	}
	if lc.StartupRemaining() != 0 {
		t.Errorf("StartupRemaining() = %v after ready, want 0", lc.StartupRemaining())
	}
}

func TestLifecycleTrackRequest(t *testing.T) {
	clock := clockwork.NewFakeClock()
	lc := NewLifecycleWithClock(clock, 0, 0, 0, 30*time.Second, false)

	if lc.InFlightRequests() != 0 {
		t.Errorf("initial InFlightRequests() = %d, want 0", lc.InFlightRequests())
	}

	done1 := lc.TrackRequest()
	if lc.InFlightRequests() != 1 {
		t.Errorf("InFlightRequests() after first = %d, want 1", lc.InFlightRequests())
	}

	done2 := lc.TrackRequest()
	if lc.InFlightRequests() != 2 {
		t.Errorf("InFlightRequests() after second = %d, want 2", lc.InFlightRequests())
	}

	done1()
	if lc.InFlightRequests() != 1 {
		t.Errorf("InFlightRequests() after first done = %d, want 1", lc.InFlightRequests())
	}

	done2()
	if lc.InFlightRequests() != 0 {
		t.Errorf("InFlightRequests() after all done = %d, want 0", lc.InFlightRequests())
	}
}

func TestLifecycleShutdown(t *testing.T) {
	clock := clockwork.NewFakeClock()
	lc := NewLifecycleWithClock(clock, 0, 0, 0, 1*time.Second, false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := lc.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}

	if !lc.IsShuttingDown() {
		t.Error("expected IsShuttingDown() to be true after shutdown")
	}
}

func TestLifecycleShutdownWithDelay(t *testing.T) {
	clock := clockwork.NewFakeClock()
	lc := NewLifecycleWithClock(clock, 0, 0, 100*time.Millisecond, 1*time.Second, false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- lc.Shutdown(ctx)
	}()

	if err := clock.BlockUntilContext(context.Background(), 1); err != nil {
		t.Fatalf("BlockUntilContext: %v", err)
	}
	clock.Advance(100 * time.Millisecond)

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Shutdown() did not complete")
	}
}

func TestLifecycleShutdownWaitsForInFlight(t *testing.T) {
	clock := clockwork.NewFakeClock()
	lc := NewLifecycleWithClock(clock, 0, 0, 0, 5*time.Second, false)

	done := lc.TrackRequest()

	shutdownComplete := make(chan struct{})
	go func() {
		_ = lc.Shutdown(context.Background())
		close(shutdownComplete)
	}()

	if err := clock.BlockUntilContext(context.Background(), 1); err != nil {
		t.Fatalf("BlockUntilContext: %v", err)
	}

	done()

	clock.Advance(100 * time.Millisecond)

	select {
	case <-shutdownComplete:
	case <-time.After(1 * time.Second):
		t.Error("Shutdown() did not complete after in-flight request finished")
	}
}

func TestLifecycleDrainImmediately(t *testing.T) {
	clock := clockwork.NewFakeClock()
	lc := NewLifecycleWithClock(clock, 0, 0, 0, 30*time.Second, true)

	if lc.ShouldRejectRequest() {
		t.Error("ShouldRejectRequest() = true before shutdown")
	}

	go func() { _ = lc.Shutdown(context.Background()) }()

	// Give goroutine a moment to start
	time.Sleep(10 * time.Millisecond)

	if !lc.ShouldRejectRequest() {
		t.Error("ShouldRejectRequest() = false after shutdown with drain_immediately=true")
	}
}

func TestLifecycleNoDrainImmediately(t *testing.T) {
	clock := clockwork.NewFakeClock()
	lc := NewLifecycleWithClock(clock, 0, 0, 0, 30*time.Second, false)

	go func() { _ = lc.Shutdown(context.Background()) }()

	// Give goroutine a moment to start
	time.Sleep(10 * time.Millisecond)

	if lc.ShouldRejectRequest() {
		t.Error("ShouldRejectRequest() = true during shutdown with drain_immediately=false")
	}
}

func TestStateString(t *testing.T) {
	for _, tt := range stateStringTests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
