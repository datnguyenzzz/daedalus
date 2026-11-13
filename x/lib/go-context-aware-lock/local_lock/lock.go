package local_lock

import (
	"context"
	"fmt"
)

// CtxLock uses a Go channel to provide atomic locking and unlocking with context.Context cancellation
// support. This lock is resolved locally and does not require network calls.
type CtxLock struct {
	ch chan struct{}
}

func NewLock() *CtxLock {
	return &CtxLock{
		// buffered chanel with a size of 1,
		// so the sender will be blocked when the chanel is full
		ch: make(chan struct{}, 1),
	}
}

func (l *CtxLock) AcquireCtx(ctx context.Context) error {
	if l.ch == nil {
		return fmt.Errorf("failed to init lock")
	}

	select {
	case <-ctx.Done():
		// context is either timeout or cancelled
		return ctx.Err()
	case l.ch <- struct{}{}:
		return nil
	}
}

func (l *CtxLock) ReleaseCtx(ctx context.Context) error {
	if l.ch == nil {
		return fmt.Errorf("failed to init lock")
	}

	select {
	case <-ctx.Done():
		// context is either timeout or cancelled
		return ctx.Err()
	case <-l.ch:
		return nil
	}
}
