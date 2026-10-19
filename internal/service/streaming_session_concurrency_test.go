package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestConcurrentContinuationTokenReuseAdvancesOnce(t *testing.T) {
	manager := &StreamingSessionManager{sessions: make(map[string]*StreamingSession)}
	manager.Store(&StreamingSession{
		QueryID:   "query-1",
		Slug:      "slug-1",
		Token:     "token-1",
		ExpiresAt: time.Now().Add(time.Minute),
	})

	first, err := manager.getAndLock(context.Background(), "query-1")
	if err != nil {
		t.Fatalf("first getAndLock() error = %v", err)
	}
	if err := first.validateToken("slug-1", "token-1"); err != nil {
		first.continuationLock.Unlock()
		t.Fatalf("first token validation error = %v", err)
	}

	secondStarted := make(chan struct{})
	secondResult := make(chan error, 1)
	go func() {
		close(secondStarted)
		second, err := manager.getAndLock(context.Background(), "query-1")
		if err != nil {
			secondResult <- err
			return
		}
		defer second.continuationLock.Unlock()
		secondResult <- second.validateToken("slug-1", "token-1")
	}()
	<-secondStarted

	// Simulate the first continuation consuming a page and rotating its token.
	first.Token = "token-2"
	first.continuationLock.Unlock()

	select {
	case err := <-secondResult:
		if !errors.Is(err, ErrInvalidQuerySession) {
			t.Fatalf("second continuation error = %v, want stale-token rejection", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second continuation did not finish after the first released its session")
	}

	current, err := manager.Get("query-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if current.Token != "token-2" {
		t.Fatalf("stored token = %q, want token-2", current.Token)
	}
}
