package queue

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ripta/hotpod/internal/metrics"
)

// Priority levels for queue items.
const (
	PriorityHigh   = "high"
	PriorityNormal = "normal"
	PriorityLow    = "low"
)

// ErrQueueFull is returned when the queue has reached its maximum depth.
var ErrQueueFull = errors.New("queue is full")

// Item represents a work item in the queue.
type Item struct {
	// ID is a unique identifier for the item
	ID string
	// Priority is the item priority (high, normal, low)
	Priority string
	// ProcessingTime is how long processing should take
	ProcessingTime time.Duration
	// EnqueuedAt is when the item was added to the queue
	EnqueuedAt time.Time
}

// Queue is a thread-safe priority queue.
type Queue struct {
	mu       sync.Mutex
	maxDepth int

	// Separate queues for each priority level
	high   []*Item
	normal []*Item
	low    []*Item

	// Counters
	enqueuedTotal  atomic.Int64
	processedTotal atomic.Int64
	failedTotal    atomic.Int64

	// State
	paused atomic.Bool
}

// New creates a new queue with the given maximum depth.
func New(maxDepth int) *Queue {
	return &Queue{
		maxDepth: maxDepth,
		high:     make([]*Item, 0),
		normal:   make([]*Item, 0),
		low:      make([]*Item, 0),
	}
}

// Enqueue adds an item to the queue.
func (q *Queue) Enqueue(item *Item) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.depth() >= q.maxDepth {
		return ErrQueueFull
	}

	switch item.Priority {
	case PriorityHigh:
		q.high = append(q.high, item)
	case PriorityLow:
		q.low = append(q.low, item)
	default:
		item.Priority = PriorityNormal
		q.normal = append(q.normal, item)
	}

	q.enqueuedTotal.Add(1)
	metrics.QueueItemsEnqueuedTotal.Inc()
	q.updateMetrics()
	return nil
}

// Dequeue removes and returns the highest priority item.
// Returns nil if the queue is empty or paused.
func (q *Queue) Dequeue() *Item {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.paused.Load() {
		return nil
	}

	var item *Item

	if len(q.high) > 0 {
		item = q.high[0]
		q.high = q.high[1:]
	} else if len(q.normal) > 0 {
		item = q.normal[0]
		q.normal = q.normal[1:]
	} else if len(q.low) > 0 {
		item = q.low[0]
		q.low = q.low[1:]
	}

	if item != nil {
		q.updateMetrics()
	}

	return item
}

// MarkProcessed increments the processed counter.
func (q *Queue) MarkProcessed() {
	q.processedTotal.Add(1)
	metrics.QueueItemsProcessedTotal.Inc()
}

// MarkFailed increments the failed counter.
func (q *Queue) MarkFailed() {
	q.failedTotal.Add(1)
	metrics.QueueItemsFailedTotal.Inc()
}

// Depth returns the current queue depth.
func (q *Queue) Depth() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.depth()
}

// depth returns the queue depth (must hold lock).
func (q *Queue) depth() int {
	return len(q.high) + len(q.normal) + len(q.low)
}

// DepthByPriority returns the depth for each priority level.
func (q *Queue) DepthByPriority() (high, normal, low int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.high), len(q.normal), len(q.low)
}

// Stats returns queue statistics.
type Stats struct {
	Depth          int
	HighDepth      int
	NormalDepth    int
	LowDepth       int
	EnqueuedTotal  int64
	ProcessedTotal int64
	FailedTotal    int64
	Paused         bool
	OldestItemAge  time.Duration
}

// Stats returns current queue statistics.
func (q *Queue) Stats() Stats {
	q.mu.Lock()
	defer q.mu.Unlock()

	stats := Stats{
		Depth:          q.depth(),
		HighDepth:      len(q.high),
		NormalDepth:    len(q.normal),
		LowDepth:       len(q.low),
		EnqueuedTotal:  q.enqueuedTotal.Load(),
		ProcessedTotal: q.processedTotal.Load(),
		FailedTotal:    q.failedTotal.Load(),
		Paused:         q.paused.Load(),
	}

	// Find oldest item
	var oldest time.Time
	if len(q.high) > 0 && (oldest.IsZero() || q.high[0].EnqueuedAt.Before(oldest)) {
		oldest = q.high[0].EnqueuedAt
	}
	if len(q.normal) > 0 && (oldest.IsZero() || q.normal[0].EnqueuedAt.Before(oldest)) {
		oldest = q.normal[0].EnqueuedAt
	}
	if len(q.low) > 0 && (oldest.IsZero() || q.low[0].EnqueuedAt.Before(oldest)) {
		oldest = q.low[0].EnqueuedAt
	}

	if !oldest.IsZero() {
		stats.OldestItemAge = time.Since(oldest)
	}

	return stats
}

// Clear removes all items from the queue.
func (q *Queue) Clear() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	count := q.depth()
	q.high = make([]*Item, 0)
	q.normal = make([]*Item, 0)
	q.low = make([]*Item, 0)

	q.updateMetrics()
	return count
}

// Pause stops dequeue operations.
func (q *Queue) Pause() {
	q.paused.Store(true)
}

// Resume allows dequeue operations.
func (q *Queue) Resume() {
	q.paused.Store(false)
}

// IsPaused returns whether the queue is paused.
func (q *Queue) IsPaused() bool {
	return q.paused.Load()
}

// updateMetrics updates Prometheus metrics (must hold lock).
func (q *Queue) updateMetrics() {
	depth := q.depth()
	metrics.QueueDepth.Set(float64(depth))
	metrics.QueueDepthByPriority.WithLabelValues(PriorityHigh).Set(float64(len(q.high)))
	metrics.QueueDepthByPriority.WithLabelValues(PriorityNormal).Set(float64(len(q.normal)))
	metrics.QueueDepthByPriority.WithLabelValues(PriorityLow).Set(float64(len(q.low)))

	// Update oldest item age
	var oldest time.Time
	if len(q.high) > 0 {
		oldest = q.high[0].EnqueuedAt
	}
	if len(q.normal) > 0 && (oldest.IsZero() || q.normal[0].EnqueuedAt.Before(oldest)) {
		oldest = q.normal[0].EnqueuedAt
	}
	if len(q.low) > 0 && (oldest.IsZero() || q.low[0].EnqueuedAt.Before(oldest)) {
		oldest = q.low[0].EnqueuedAt
	}
	if !oldest.IsZero() {
		metrics.QueueOldestItemAgeSeconds.Set(time.Since(oldest).Seconds())
	} else {
		metrics.QueueOldestItemAgeSeconds.Set(0)
	}
}
