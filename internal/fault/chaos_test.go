package fault

import (
	"context"
	"testing"
	"time"
)

func TestHangWithDuration(t *testing.T) {
	ctx := context.Background()

	start := time.Now()
	cancelled := Hang(ctx, 50*time.Millisecond)
	elapsed := time.Since(start)

	if cancelled {
		t.Error("expected not cancelled for completed hang")
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("elapsed = %v, want >= 50ms", elapsed)
	}
}

func TestHangContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	start := time.Now()
	cancelled := Hang(ctx, time.Hour)
	elapsed := time.Since(start)

	if !cancelled {
		t.Error("expected hang to be cancelled")
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("elapsed = %v, want < 100ms (should cancel quickly)", elapsed)
	}
}

func TestHangIndefiniteWithCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	start := time.Now()
	cancelled := Hang(ctx, 0) // 0 duration means indefinite
	elapsed := time.Since(start)

	if !cancelled {
		t.Error("expected indefinite hang to be cancelled")
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("elapsed = %v, want < 100ms (should cancel quickly)", elapsed)
	}
}
