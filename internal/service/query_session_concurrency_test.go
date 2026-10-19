package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestQuerySessionManagerRejectsConcurrentTokenReplay(t *testing.T) {
	manager := &SessionManager{sessions: make(map[string]*QuerySession)}
	manager.Store(&QuerySession{
		QueryID:   "query-1",
		Slug:      "slug-1",
		Token:     "token-1",
		ExpiresAt: time.Now().Add(time.Minute),
	})

	first, err := manager.getAndLock(context.Background(), "query-1")
	if err != nil {
		t.Fatalf("first getAndLock() error = %v", err)
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
		secondResult <- validateContinuationToken(second.Slug, second.Token, "slug-1", "token-1")
	}()
	<-secondStarted

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
}
