package worker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitUntilReadyRetriesUntilOperationSucceeds(t *testing.T) {
	attempts := 0
	err := WaitUntilReady(t.Context(), time.Millisecond, func(context.Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("redis unavailable")
		}
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("WaitUntilReady() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestWaitUntilReadyStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := WaitUntilReady(ctx, time.Hour, func(context.Context) error {
		return errors.New("redis unavailable")
	}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitUntilReady() error = %v, want context canceled", err)
	}
}
