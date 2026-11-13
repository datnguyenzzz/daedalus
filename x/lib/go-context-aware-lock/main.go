package go_context_aware_lock

import (
	"github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock/local_lock"
	optimisticrwmutex "github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock/optimistic_rw_mutex"
)

func NewLocalLock() ICtxLock {
	return local_lock.NewLock()
}

func NewOptimisticLock() IOptRWMutex {
	return &optimisticrwmutex.OptRWMutex{}
}
