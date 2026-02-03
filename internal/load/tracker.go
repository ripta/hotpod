package load

import (
	"fmt"
	"sync/atomic"
)

// OpType represents the type of load operation.
type OpType string

const (
	OpTypeCPU     OpType = "cpu"
	OpTypeMemory  OpType = "memory"
	OpTypeIO      OpType = "io"
	OpTypeLatency OpType = "latency"
	OpTypeWork    OpType = "work"
)

// Tracker tracks concurrent operations and enforces limits.
type Tracker struct {
	// maxOps is the maximum concurrent operations per type (<=0 means unlimited)
	maxOps int
	// counts tracks current operation counts per type
	counts map[OpType]*atomic.Int64
}

// NewTracker creates a new operation tracker.
func NewTracker(maxOps int) *Tracker {
	return &Tracker{
		maxOps: maxOps,
		counts: map[OpType]*atomic.Int64{
			OpTypeCPU:     {},
			OpTypeMemory:  {},
			OpTypeIO:      {},
			OpTypeLatency: {},
			OpTypeWork:    {},
		},
	}
}

// ErrTooManyOps is returned when the concurrent operation limit is exceeded.
var ErrTooManyOps = fmt.Errorf("too many concurrent operations")

// Acquire attempts to start an operation of the given type.
// Returns a release function on success, or ErrTooManyOps if limit exceeded.
func (t *Tracker) Acquire(op OpType) (release func(), err error) {
	counter := t.counts[op]

	for {
		current := counter.Load()
		if t.maxOps > 0 && current >= int64(t.maxOps) {
			return nil, ErrTooManyOps
		}

		if counter.CompareAndSwap(current, current+1) {
			return func() { counter.Add(-1) }, nil
		}
	}
}

// Count returns the current operation count for the given type.
func (t *Tracker) Count(op OpType) int64 {
	if counter := t.counts[op]; counter != nil {
		return counter.Load()
	}
	return 0
}

// Counts returns all current operation counts.
func (t *Tracker) Counts() map[OpType]int64 {
	result := make(map[OpType]int64, len(t.counts))
	for op, counter := range t.counts {
		result[op] = counter.Load()
	}
	return result
}
