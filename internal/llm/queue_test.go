package llm

import (
	"context"
	"testing"
)

func TestLimiterQueuesUntilContextIsCancelled(t *testing.T) {
	queue := newLimiter(1)
	if err := queue.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() first slot error = %v", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := queue.Acquire(cancelled); err == nil {
		t.Fatal("Acquire() while queue is full error = nil, want cancellation")
	}

	queue.Release()
	if err := queue.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire() after release error = %v", err)
	}
	queue.Release()
}
