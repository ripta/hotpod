package queue

import (
	"testing"
	"time"
)

func TestEnqueueDequeue(t *testing.T) {
	q := New(100)

	item := &Item{
		ID:             "test-1",
		Priority:       PriorityNormal,
		ProcessingTime: time.Second,
		EnqueuedAt:     time.Now(),
	}

	if err := q.Enqueue(item); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	if q.Depth() != 1 {
		t.Errorf("depth = %d, want 1", q.Depth())
	}

	got := q.Dequeue()
	if got == nil {
		t.Fatal("dequeue returned nil")
	}
	if got.ID != "test-1" {
		t.Errorf("got ID = %q, want \"test-1\"", got.ID)
	}

	if q.Depth() != 0 {
		t.Errorf("depth = %d, want 0 after dequeue", q.Depth())
	}
}

func TestPriorityOrder(t *testing.T) {
	q := New(100)

	// Enqueue in reverse priority order
	items := []*Item{
		{ID: "low", Priority: PriorityLow, EnqueuedAt: time.Now()},
		{ID: "normal", Priority: PriorityNormal, EnqueuedAt: time.Now()},
		{ID: "high", Priority: PriorityHigh, EnqueuedAt: time.Now()},
	}

	for _, item := range items {
		if err := q.Enqueue(item); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}
	}

	// Should dequeue in priority order: high, normal, low
	expectedOrder := []string{"high", "normal", "low"}
	for _, expected := range expectedOrder {
		got := q.Dequeue()
		if got == nil {
			t.Fatalf("dequeue returned nil, expected %q", expected)
		}
		if got.ID != expected {
			t.Errorf("got ID = %q, want %q", got.ID, expected)
		}
	}
}

func TestMaxDepth(t *testing.T) {
	q := New(3)

	for i := range 3 {
		item := &Item{ID: string(rune('a' + i)), Priority: PriorityNormal, EnqueuedAt: time.Now()}
		if err := q.Enqueue(item); err != nil {
			t.Fatalf("enqueue %d failed: %v", i, err)
		}
	}

	// Fourth item should fail
	item := &Item{ID: "d", Priority: PriorityNormal, EnqueuedAt: time.Now()}
	if err := q.Enqueue(item); err != ErrQueueFull {
		t.Errorf("expected ErrQueueFull, got %v", err)
	}
}

func TestPauseResume(t *testing.T) {
	q := New(100)

	item := &Item{ID: "test", Priority: PriorityNormal, EnqueuedAt: time.Now()}
	if err := q.Enqueue(item); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	q.Pause()

	if !q.IsPaused() {
		t.Error("expected queue to be paused")
	}

	// Dequeue should return nil when paused
	if got := q.Dequeue(); got != nil {
		t.Error("expected nil dequeue when paused")
	}

	q.Resume()

	if q.IsPaused() {
		t.Error("expected queue to be resumed")
	}

	// Dequeue should work after resume
	if got := q.Dequeue(); got == nil {
		t.Error("expected item after resume")
	}
}

func TestClear(t *testing.T) {
	q := New(100)

	for i := range 5 {
		item := &Item{ID: string(rune('a' + i)), Priority: PriorityNormal, EnqueuedAt: time.Now()}
		if err := q.Enqueue(item); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}
	}

	cleared := q.Clear()
	if cleared != 5 {
		t.Errorf("cleared = %d, want 5", cleared)
	}

	if q.Depth() != 0 {
		t.Errorf("depth = %d, want 0 after clear", q.Depth())
	}
}

func TestStats(t *testing.T) {
	q := New(100)

	// Enqueue items of different priorities
	items := []*Item{
		{ID: "h1", Priority: PriorityHigh, EnqueuedAt: time.Now()},
		{ID: "n1", Priority: PriorityNormal, EnqueuedAt: time.Now()},
		{ID: "n2", Priority: PriorityNormal, EnqueuedAt: time.Now()},
		{ID: "l1", Priority: PriorityLow, EnqueuedAt: time.Now()},
	}

	for _, item := range items {
		if err := q.Enqueue(item); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}
	}

	stats := q.Stats()

	if stats.Depth != 4 {
		t.Errorf("Depth = %d, want 4", stats.Depth)
	}
	if stats.HighDepth != 1 {
		t.Errorf("HighDepth = %d, want 1", stats.HighDepth)
	}
	if stats.NormalDepth != 2 {
		t.Errorf("NormalDepth = %d, want 2", stats.NormalDepth)
	}
	if stats.LowDepth != 1 {
		t.Errorf("LowDepth = %d, want 1", stats.LowDepth)
	}
	if stats.EnqueuedTotal != 4 {
		t.Errorf("EnqueuedTotal = %d, want 4", stats.EnqueuedTotal)
	}
}

func TestDefaultPriority(t *testing.T) {
	q := New(100)

	// Item with empty priority should default to normal
	item := &Item{ID: "test", Priority: "", EnqueuedAt: time.Now()}
	if err := q.Enqueue(item); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	got := q.Dequeue()
	if got.Priority != PriorityNormal {
		t.Errorf("priority = %q, want %q", got.Priority, PriorityNormal)
	}
}

func TestDepthByPriority(t *testing.T) {
	q := New(100)

	items := []*Item{
		{ID: "h1", Priority: PriorityHigh, EnqueuedAt: time.Now()},
		{ID: "h2", Priority: PriorityHigh, EnqueuedAt: time.Now()},
		{ID: "n1", Priority: PriorityNormal, EnqueuedAt: time.Now()},
		{ID: "l1", Priority: PriorityLow, EnqueuedAt: time.Now()},
		{ID: "l2", Priority: PriorityLow, EnqueuedAt: time.Now()},
		{ID: "l3", Priority: PriorityLow, EnqueuedAt: time.Now()},
	}

	for _, item := range items {
		if err := q.Enqueue(item); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}
	}

	high, normal, low := q.DepthByPriority()
	if high != 2 {
		t.Errorf("high = %d, want 2", high)
	}
	if normal != 1 {
		t.Errorf("normal = %d, want 1", normal)
	}
	if low != 3 {
		t.Errorf("low = %d, want 3", low)
	}
}
