package queue

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ripta/hotpod/internal/metrics"
)

// WorkerPool manages background workers that process queue items.
type WorkerPool struct {
	queue *Queue

	mu            sync.Mutex
	activeWorkers atomic.Int32
	cancel        context.CancelFunc
	wg            sync.WaitGroup

	// Per-item resource consumption (immutable after Start, no lock needed for reads)
	cpuPerItem    atomic.Int64
	memoryPerItem atomic.Int64
}

// NewWorkerPool creates a new worker pool for the given queue.
func NewWorkerPool(q *Queue) *WorkerPool {
	return &WorkerPool{
		queue: q,
	}
}

// Start launches workers to process queue items.
// If workers are already running, this stops them first.
// The provided context controls worker lifetime - workers stop when it's cancelled.
func (wp *WorkerPool) Start(ctx context.Context, workerCount int, cpuPerItem time.Duration, memoryPerItem int64) {
	// Stop existing workers first (outside the lock to avoid deadlock)
	wp.Stop()

	wp.mu.Lock()
	defer wp.mu.Unlock()

	// Store config atomically for safe concurrent reads by workers
	wp.cpuPerItem.Store(int64(cpuPerItem))
	wp.memoryPerItem.Store(memoryPerItem)

	workerCtx, cancel := context.WithCancel(ctx)
	wp.cancel = cancel

	for i := range workerCount {
		wp.wg.Add(1)
		go wp.worker(workerCtx, i)
	}

	slog.Info("worker pool started", "workers", workerCount, "cpu_per_item", cpuPerItem, "memory_per_item", memoryPerItem)
}

// Stop gracefully stops all workers.
func (wp *WorkerPool) Stop() {
	wp.mu.Lock()
	if wp.cancel != nil {
		wp.cancel()
		wp.cancel = nil
	}
	wp.mu.Unlock()

	wp.wg.Wait()
	slog.Info("worker pool stopped")
}

// ActiveWorkers returns the number of currently active workers.
func (wp *WorkerPool) ActiveWorkers() int {
	return int(wp.activeWorkers.Load())
}

func (wp *WorkerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()

	slog.Debug("worker started", "worker_id", id)

	for {
		select {
		case <-ctx.Done():
			slog.Debug("worker stopping", "worker_id", id)
			return
		default:
		}

		item := wp.queue.Dequeue()
		if item == nil {
			// Queue is empty or paused, wait a bit
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}

		wp.activeWorkers.Add(1)
		metrics.QueueActiveWorkers.Set(float64(wp.activeWorkers.Load()))

		wp.processItem(ctx, item)

		wp.activeWorkers.Add(-1)
		metrics.QueueActiveWorkers.Set(float64(wp.activeWorkers.Load()))
	}
}

func (wp *WorkerPool) processItem(ctx context.Context, item *Item) {
	start := time.Now()

	// Simulate processing time
	processingTime := item.ProcessingTime
	if processingTime <= 0 {
		processingTime = 100 * time.Millisecond
	}

	// Load config atomically (safe for concurrent reads)
	memoryPerItem := wp.memoryPerItem.Load()
	cpuPerItem := time.Duration(wp.cpuPerItem.Load())

	// Allocate memory if configured
	var memSink []byte
	if memoryPerItem > 0 {
		memSink = make([]byte, memoryPerItem)
		for i := range memoryPerItem {
			memSink[i] = byte(i)
		}
	}

	// Simulate CPU work if configured
	if cpuPerItem > 0 {
		cpuEnd := time.Now().Add(cpuPerItem)
		for time.Now().Before(cpuEnd) {
			select {
			case <-ctx.Done():
				wp.queue.MarkFailed()
				return
			default:
				// Busy loop for CPU consumption
				for i := 0; i < 1000; i++ {
					_ = i * i
				}
			}
		}
	}

	// Wait for remaining processing time
	elapsed := time.Since(start)
	remaining := processingTime - elapsed
	if remaining > 0 {
		select {
		case <-ctx.Done():
			wp.queue.MarkFailed()
			return
		case <-time.After(remaining):
		}
	}

	// Keep memory alive until processing is done
	_ = memSink

	wp.queue.MarkProcessed()
	metrics.QueueProcessingSeconds.Observe(time.Since(start).Seconds())

	slog.Debug("item processed",
		"item_id", item.ID,
		"priority", item.Priority,
		"duration", time.Since(start),
		"wait_time", start.Sub(item.EnqueuedAt),
	)
}
