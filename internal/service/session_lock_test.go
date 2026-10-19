package service

import (
	"context"
	"errors"
	"testing"
)

func TestSessionContinuationLockHonorsCancellation(t *testing.T) {
	var lock sessionContinuationLock
	if err := lock.Lock(context.Background()); err != nil {
		t.Fatalf("Lock() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := lock.Lock(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Lock(cancelled context) error = %v, want context.Canceled", err)
	}
	lock.Unlock()
}
