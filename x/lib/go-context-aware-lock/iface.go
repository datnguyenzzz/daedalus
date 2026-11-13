package go_context_aware_lock

import (
	"context"
)

type ICtxLock interface {
	AcquireCtx(ctx context.Context) error
	ReleaseCtx(ctx context.Context) error
}

type IOptRWMutex interface {
	RLock() (version uint64, outdated bool)
	RUnlock(version uint64) bool
	Lock()
	Unlock()
	// Upgrade promotes rlock to wlock
	Upgrade(version uint64) bool
	Check(version uint64) bool
}
