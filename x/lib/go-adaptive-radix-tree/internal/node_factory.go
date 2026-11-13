package internal

import (
	"fmt"

	go_context_aware_lock "github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock"
)

func NewNode[V any](k Kind) INode[V] {
	switch k {
	case KindNode4:
		n := new(Node4[V])
		n.setLocker(go_context_aware_lock.NewOptimisticLock())
		return n
	case KindNode16:
		n := new(Node16[V])
		n.setLocker(go_context_aware_lock.NewOptimisticLock())
		return n
	case KindNode48:
		n := new(Node48[V])
		n.setLocker(go_context_aware_lock.NewOptimisticLock())
		return n
	case KindNode256:
		n := new(Node256[V])
		n.setLocker(go_context_aware_lock.NewOptimisticLock())
		return n
	case KindNodeLeaf:
		n := new(NodeLeaf[V])
		n.setLocker(go_context_aware_lock.NewOptimisticLock())
		return n
	default:
		panic(fmt.Sprintf("%v node is unsupported", k))
	}
}
