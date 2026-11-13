//go:build !race

package optimisticrwmutex

import (
	"runtime"
	"sync/atomic"
)

// OptRWMutex is an implementation of the optimistic read write mutex.
// https://15721.courses.cs.cmu.edu/spring2017/papers/08-oltpindexes2/leis-damon2016.pdf
// Each node header stores a 64 bit version field that is read and written atomically.
// The 2 least significant bits indicate if the node is obsolete or if the node is locked.
// The remaining bits store the update counter.
type OptRWMutex struct {
	version uint64
}

func (l *OptRWMutex) RLock() (version uint64, obsolete bool) {
	version = l.waitUnlocked()
	obsolete = isObsolete(version)
	return version, obsolete
}

func (l *OptRWMutex) RUnlock(version uint64) (obsolete bool) {
	obsolete = atomic.LoadUint64(&l.version) != version
	return obsolete
}

func (l *OptRWMutex) Lock() {
	for {
		version, obsolete := l.RLock()
		if obsolete {
			continue
		}
		if l.Upgrade(version) {
			return
		}
	}
}

func (l *OptRWMutex) Unlock() {
	if !isLocked(atomic.LoadUint64(&l.version)) {
		panic("unlock of unlocked mutex")
	}

	atomic.AddUint64(&l.version, 0b10)
}

// Upgrade promotes rlock to wlock
func (l *OptRWMutex) Upgrade(version uint64) (upgraded bool) {
	upgraded = atomic.CompareAndSwapUint64(&l.version, version, setLocked(version))
	return upgraded
}

func (l *OptRWMutex) Check(version uint64) (obsolete bool) {
	return atomic.LoadUint64(&l.version) != version
}

func (l *OptRWMutex) waitUnlocked() (version uint64) {
	for {
		version = atomic.LoadUint64(&l.version)
		if version&0b10 != 0b10 {
			return version
		}
		runtime.Gosched()
	}
}

func isLocked(version uint64) bool {
	return version&0b10 == 0b10
}

func isObsolete(version uint64) bool {
	return version&0b01 == 1
}

func setLocked(version uint64) uint64 {
	return version | 0b10
}
