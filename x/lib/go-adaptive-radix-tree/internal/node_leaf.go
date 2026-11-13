package internal

import (
	"context"

	go_context_aware_lock "github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock"
)

// NodeLeaf aka Single-value leaf: The values are stored
// in an additional leaf node type, instead of in the inner node.
// It's also an implication that the prefix on the leaf node is equal to the Key itself.
type NodeLeaf[V any] struct {
	nodeHeader
	locker go_context_aware_lock.IOptRWMutex
	value  V
}

func (n *NodeLeaf[V]) getValue(ctx context.Context) V {
	return n.value
}

func (n *NodeLeaf[V]) setValue(ctx context.Context, v V) { //nolint:unused
	n.value = v
}

func (n *NodeLeaf[V]) GetKind(ctx context.Context) Kind {
	return KindNodeLeaf
}

func (n *NodeLeaf[V]) addChild(ctx context.Context, key *nodeKey, child *INode[V]) error { //nolint:unused
	return nil // node leaf doesn't support this function
}

func (n *NodeLeaf[V]) removeChild(ctx context.Context, key *nodeKey) error { //nolint:unused
	return nil // node leaf doesn't support this function
}

func (n *NodeLeaf[V]) getChild(ctx context.Context, key *nodeKey) (*INode[V], error) { //nolint:unused
	return nil, nil // node leaf doesn't support this function
}

func (n *NodeLeaf[V]) getAllChildren(ctx context.Context, order Order) []*INode[V] { //nolint:unused
	return []*INode[V]{}
}

func (n *NodeLeaf[V]) getChildByIndex(ctx context.Context, idx uint8) (*nodeKey, *INode[V], error) { //nolint:unused
	return nil, nil, nil // node leaf doesn't support this function
}

func (n *NodeLeaf[V]) grow(ctx context.Context) (*INode[V], error) { //nolint:unused
	return nil, nil // node leaf doesn't support this function
}

func (n *NodeLeaf[V]) shrink(ctx context.Context) (*INode[V], error) { //nolint:unused
	return nil, nil // node leaf doesn't support this function
}

func (n *NodeLeaf[V]) hasEnoughSpace(ctx context.Context) bool { //nolint:unused
	return false // node leaf doesn't support this function
}

func (n *NodeLeaf[V]) isShrinkable(ctx context.Context) bool { //nolint:unused
	return false
}

func (n *NodeLeaf[V]) GetLocker() go_context_aware_lock.IOptRWMutex {
	return n.locker
}

func (n *NodeLeaf[V]) setLocker(locker go_context_aware_lock.IOptRWMutex) {
	n.locker = locker
}

func (n *NodeLeaf[V]) clone() INode[V] { //nolint:unused
	nn := &NodeLeaf[V]{}
	nn.nodeHeader = n.nodeHeader
	nn.locker = n.locker
	nn.value = n.value
	return nn
}

var _ INode[any] = (*NodeLeaf[any])(nil)
