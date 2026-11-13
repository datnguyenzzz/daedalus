//go:build race
// +build race

package optimisticrwmutex

import (
	"sync"
)

// OptRWMutex is an implementation of the optimistic read write mutex.
// https://15721.courses.cs.cmu.edu/spring2017/papers/08-oltpindexes2/leis-damon2016.pdf
// Each node header stores a 64 bit version field that is read and written atomically.
// The 2 least significant bits indicate if the node is obsolete or if the node is locked.
// The remaining bits store the update counter.
//
// golang race detector won't be able to recognize correctness of the optimistic locking
// and will report races if tests are executed with --race flag
type OptRWMutex struct {
	mu sync.Mutex
}

func (l *OptRWMutex) RLock() (version uint64, obsolete bool) {
	l.mu.Lock()
	return 0, false
}

func (l *OptRWMutex) RUnlock(version uint64) (obsolete bool) {
	l.mu.Unlock()
	return false
}

func (l *OptRWMutex) Lock() {
	l.mu.Lock()
}

func (l *OptRWMutex) Unlock() {
	l.mu.Unlock()
}

// Upgrade promotes rlock to wlock
func (l *OptRWMutex) Upgrade(version uint64) (upgraded bool) {
	return true
}

func (l *OptRWMutex) Check(version uint64) (obsolete bool) {
	return false
}
