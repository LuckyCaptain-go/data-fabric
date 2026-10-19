package service

import (
	"context"
	"sync"
)

// sessionContinuationLock serializes state changes for one query session while
// allowing a waiting request to stop when its context is cancelled.
type sessionContinuationLock struct {
	once sync.Once
	gate chan struct{}
}

func (lock *sessionContinuationLock) Lock(ctx context.Context) error {
	lock.once.Do(func() {
		lock.gate = make(chan struct{}, 1)
	})
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case lock.gate <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (lock *sessionContinuationLock) Unlock() {
	lock.once.Do(func() {
		lock.gate = make(chan struct{}, 1)
	})
	<-lock.gate
}

func validateContinuationToken(sessionSlug, sessionToken, slug, token string) error {
	if sessionSlug != slug || sessionToken != token {
		return ErrInvalidQuerySession
	}
	return nil
}
