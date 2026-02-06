package sidecar

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/ripta/hotpod/internal/metrics"
)

func gaugeValue(g prometheus.Gauge) float64 {
	var m dto.Metric
	_ = g.Write(&m)
	return m.GetGauge().GetValue()
}

func counterValue(c prometheus.Counter) float64 {
	var m dto.Metric
	_ = c.Write(&m)
	return m.GetCounter().GetValue()
}

func TestStartAllocatesMemory(t *testing.T) {
	r := New(0, 0, 1024)
	ctx, cancel := context.WithCancel(context.Background())

	go r.Start(ctx)
	// Give the runner time to allocate and enter the loop.
	time.Sleep(100 * time.Millisecond)

	held := gaugeValue(metrics.SidecarMemoryHeldBytes)
	if held != 1024 {
		t.Errorf("SidecarMemoryHeldBytes = %v, want 1024", held)
	}

	r.mu.Lock()
	memLen := len(r.memory)
	r.mu.Unlock()
	if memLen != 1024 {
		t.Errorf("memory len = %d, want 1024", memLen)
	}

	cancel()
	r.Stop()
}

func TestStopReleasesMemory(t *testing.T) {
	r := New(0, 0, 2048)
	ctx, cancel := context.WithCancel(context.Background())

	go r.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	cancel()
	r.Stop()

	held := gaugeValue(metrics.SidecarMemoryHeldBytes)
	if held != 0 {
		t.Errorf("SidecarMemoryHeldBytes after stop = %v, want 0", held)
	}

	r.mu.Lock()
	memNil := r.memory == nil
	r.mu.Unlock()
	if !memNil {
		t.Error("memory should be nil after Stop")
	}
}

func TestCPULoopIncrementsBurnCounter(t *testing.T) {
	before := counterValue(metrics.SidecarCPUBurnSecondsTotal)

	// Use a small baseline so the test completes quickly.
	r := New(10*time.Millisecond, 0, 0)
	ctx, cancel := context.WithCancel(context.Background())

	go r.Start(ctx)
	// Wait for at least one tick cycle.
	time.Sleep(1500 * time.Millisecond)

	cancel()
	r.Stop()

	after := counterValue(metrics.SidecarCPUBurnSecondsTotal)
	if after <= before {
		t.Errorf("SidecarCPUBurnSecondsTotal did not increase: before=%v, after=%v", before, after)
	}
}

func TestContextCancellationStopsRunner(t *testing.T) {
	r := New(10*time.Millisecond, 0, 512)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		r.Start(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Start returned as expected.
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

func TestZeroMemoryBaseline(t *testing.T) {
	r := New(0, 0, 0)
	ctx, cancel := context.WithCancel(context.Background())

	go r.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	r.mu.Lock()
	memNil := r.memory == nil
	r.mu.Unlock()
	if !memNil {
		t.Error("memory should be nil with zero baseline")
	}

	cancel()
	r.Stop()
}
