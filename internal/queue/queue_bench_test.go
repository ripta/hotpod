package queue

import (
	"runtime"
	"testing"
	"time"
)

// BenchmarkQueueContentionProfileOverhead measures the throughput cost of the
// block and mutex profilers while many goroutines contend on the queue's mutex.
//
// The queue's Enqueue/Dequeue path is the shared hot spot the worker pool hits
// on every item, and it is guarded by a single sync.Mutex — exactly the kind of
// contention the block profiler (goroutine parking) and mutex profiler (lock
// contention) instrument. Each op is one Enqueue + one Dequeue, so queue depth
// stays near zero and the measurement reflects lock traffic rather than growth.
//
// profile=off leaves both profilers disabled; profile=on enables both at their
// most aggressive sampling (record every event), which is the worst-case cost
// of turning them on. The drop in ops/s is the overhead.
//
//	benchstat -col /profile benchmarks/pprof-overhead.txt
func BenchmarkQueueContentionProfileOverhead(b *testing.B) {
	b.Run("profile=off", func(b *testing.B) {
		runQueueContentionBench(b, false)
	})
	b.Run("profile=on", func(b *testing.B) {
		runQueueContentionBench(b, true)
	})
}

func runQueueContentionBench(b *testing.B, profile bool) {
	if profile {
		runtime.SetBlockProfileRate(1)     // record every blocking event
		runtime.SetMutexProfileFraction(1) // record every mutex contention event
		defer func() {
			runtime.SetBlockProfileRate(0)
			runtime.SetMutexProfileFraction(0)
		}()
	}

	// Large enough that a balanced enqueue/dequeue workload never trips the
	// depth limit, even with all goroutines mid-enqueue.
	q := New(1 << 20)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Per-goroutine item: Enqueue may rewrite item.Priority, so sharing one
		// across goroutines would be a data race.
		item := &Item{Priority: PriorityNormal, ProcessingTime: time.Microsecond}
		for pb.Next() {
			if err := q.Enqueue(item); err != nil {
				b.Errorf("enqueue: %v", err)
				return
			}
			q.Dequeue()
		}
	})
	b.StopTimer()

	if secs := b.Elapsed().Seconds(); secs > 0 {
		b.ReportMetric(float64(b.N)/secs, "ops/s")
	}
}
