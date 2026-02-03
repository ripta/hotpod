package load

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTrackerAcquireRelease(t *testing.T) {
	tracker := NewTracker(100)

	if tracker.Count(OpTypeLatency) != 0 {
		t.Errorf("initial count = %d, want 0", tracker.Count(OpTypeLatency))
	}

	release, err := tracker.Acquire(OpTypeLatency)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	if tracker.Count(OpTypeLatency) != 1 {
		t.Errorf("count after acquire = %d, want 1", tracker.Count(OpTypeLatency))
	}

	release()

	if tracker.Count(OpTypeLatency) != 0 {
		t.Errorf("count after release = %d, want 0", tracker.Count(OpTypeLatency))
	}
}

func TestTrackerLimitEnforced(t *testing.T) {
	tracker := NewTracker(2)

	release1, err := tracker.Acquire(OpTypeCPU)
	if err != nil {
		t.Fatalf("Acquire 1 error = %v", err)
	}

	release2, err := tracker.Acquire(OpTypeCPU)
	if err != nil {
		t.Fatalf("Acquire 2 error = %v", err)
	}

	_, err = tracker.Acquire(OpTypeCPU)
	if err != ErrTooManyOps {
		t.Errorf("Acquire 3 error = %v, want ErrTooManyOps", err)
	}

	release1()

	release3, err := tracker.Acquire(OpTypeCPU)
	if err != nil {
		t.Errorf("Acquire after release error = %v", err)
	}

	release2()
	release3()
}

func TestTrackerUnlimitedWhenZero(t *testing.T) {
	tracker := NewTracker(0)

	var releases []func()
	for i := range 1000 {
		release, err := tracker.Acquire(OpTypeMemory)
		if err != nil {
			t.Fatalf("Acquire %d error = %v", i, err)
		}
		releases = append(releases, release)
	}

	for _, release := range releases {
		release()
	}
}

func TestTrackerUnlimitedWhenNegative(t *testing.T) {
	tracker := NewTracker(-1)

	release, err := tracker.Acquire(OpTypeIO)
	if err != nil {
		t.Fatalf("Acquire error = %v", err)
	}
	release()
}

func TestTrackerTypesIndependent(t *testing.T) {
	tracker := NewTracker(1)

	releaseCPU, err := tracker.Acquire(OpTypeCPU)
	if err != nil {
		t.Fatalf("Acquire CPU error = %v", err)
	}

	releaseMemory, err := tracker.Acquire(OpTypeMemory)
	if err != nil {
		t.Fatalf("Acquire Memory error = %v (types should be independent)", err)
	}

	releaseCPU()
	releaseMemory()
}

func TestTrackerCounts(t *testing.T) {
	tracker := NewTracker(100)

	_, _ = tracker.Acquire(OpTypeCPU)
	_, _ = tracker.Acquire(OpTypeCPU)
	_, _ = tracker.Acquire(OpTypeMemory)

	counts := tracker.Counts()

	if counts[OpTypeCPU] != 2 {
		t.Errorf("CPU count = %d, want 2", counts[OpTypeCPU])
	}
	if counts[OpTypeMemory] != 1 {
		t.Errorf("Memory count = %d, want 1", counts[OpTypeMemory])
	}
	if counts[OpTypeLatency] != 0 {
		t.Errorf("Latency count = %d, want 0", counts[OpTypeLatency])
	}
}

func TestTrackerConcurrent(t *testing.T) {
	tracker := NewTracker(100)
	var wg sync.WaitGroup

	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := tracker.Acquire(OpTypeLatency)
			if err != nil {
				t.Errorf("Acquire error = %v", err)
				return
			}
			release()
		}()
	}

	wg.Wait()

	if tracker.Count(OpTypeLatency) != 0 {
		t.Errorf("final count = %d, want 0", tracker.Count(OpTypeLatency))
	}
}

func TestTrackerLimitUnderConcurrency(t *testing.T) {
	limit := 10
	tracker := NewTracker(limit)

	var wg sync.WaitGroup
	var maxConcurrent atomic.Int64

	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := tracker.Acquire(OpTypeLatency)
			if err == ErrTooManyOps {
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			current := tracker.Count(OpTypeLatency)
			for {
				max := maxConcurrent.Load()
				if current <= max || maxConcurrent.CompareAndSwap(max, current) {
					break
				}
			}

			time.Sleep(10 * time.Millisecond)
			release()
		}()
	}

	wg.Wait()

	if maxConcurrent.Load() > int64(limit) {
		t.Errorf("max concurrent = %d, exceeded limit %d", maxConcurrent.Load(), limit)
	}

	if tracker.Count(OpTypeLatency) != 0 {
		t.Errorf("leaked operations: count = %d", tracker.Count(OpTypeLatency))
	}
}
