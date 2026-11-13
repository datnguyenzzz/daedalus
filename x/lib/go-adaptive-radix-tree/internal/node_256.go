package internal

import (
	"context"
	"fmt"

	go_context_aware_lock "github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock"
)

const (
	Node256PointersMin uint8  = Node48PointersMax + 1
	Node256PointersMax uint16 = 257
)

// Node256 The largest node type is simply an array of 256
// pointers and is used for storing between 49 and 256 entries.
// With this representation, the next node can be found very
// efficiently using a single lookup of the Key byte in that array.
// No additional indirection is necessary. If most entries are not
// null, this representation is also very space efficient because
// only pointers need to be stored.
type Node256[V any] struct {
	nodeHeader
	locker go_context_aware_lock.IOptRWMutex
	// pointers to children node
	children [Node256PointersMax]*INode[V]
}

func (n *Node256[V]) getValue(ctx context.Context) V { //nolint:unused
	panic("node 256 doesn't hold any value")
}

func (n *Node256[V]) setValue(ctx context.Context, v V) { //nolint:unused
	panic("node 256 doesn't hold any value")
}

func (n *Node256[V]) GetKind(ctx context.Context) Kind {
	return KindNode256
}

func (n *Node256[V]) getIdx(key *nodeKey) int {
	if key.IsNull() {
		return 0
	}

	return int(key.b + 1)
}

func (n *Node256[V]) getKeyFromIdx(idx int) *nodeKey {
	if idx == 0 {
		return NullNodeKey()
	}

	return ToNodeKey(byte(idx - 1))
}

func (n *Node256[V]) addChild(ctx context.Context, key *nodeKey, child *INode[V]) error { //nolint:unused
	currChildrenLen := n.getChildrenLen(ctx)
	if uint16(currChildrenLen) >= Node256PointersMax {
		return fmt.Errorf("node256 is maxed out and don't have enough room for a new Key")
	}

	n.children[n.getIdx(key)] = child
	n.setChildrenLen(ctx, currChildrenLen+1)
	return nil
}

func (n *Node256[V]) removeChild(ctx context.Context, key *nodeKey) error { //nolint:unused
	currChildrenLen := n.getChildrenLen(ctx)
	n.children[n.getIdx(key)] = nil
	n.setChildrenLen(ctx, currChildrenLen-1)
	return nil
}

func (n *Node256[V]) getChild(ctx context.Context, key *nodeKey) (*INode[V], error) {
	child := n.children[n.getIdx(key)]
	if child == nil {
		return nil, childNodeNotFound
	}
	return child, nil
}

func (n *Node256[V]) getAllChildren(ctx context.Context, order Order) []*INode[V] {
	switch order {
	case AscOrder:
		res := make([]*INode[V], n.getChildrenLen(ctx))
		cnt := 0
		for k := range int(Node256PointersMax) {
			child := n.children[k]
			if child == nil {
				continue
			}
			res[cnt] = child
			cnt += 1
		}
		return res
	case DescOrder:
		res := make([]*INode[V], n.getChildrenLen(ctx))
		cnt := 0
		for k := int(Node256PointersMax) - 1; k >= 0; k-- {
			child := n.children[k]
			if child == nil {
				continue
			}
			res[cnt] = child
			cnt += 1
		}
		return res
	default:
		// shouldn't go into that block
		return make([]*INode[V], n.getChildrenLen(ctx))
	}
}

func (n *Node256[V]) getChildByIndex(ctx context.Context, idx uint8) (*nodeKey, *INode[V], error) {
	currLen := n.getChildrenLen(ctx)
	if idx == currLen {
		return nil, nil, childNodeNotFound
	}

	cnt := 0
	for k := range int(Node256PointersMax) {
		child := n.children[k]
		if child == nil {
			continue
		}
		if cnt == int(idx) {
			return n.getKeyFromIdx(k), child, nil
		}
		cnt += 1
	}
	return nil, nil, childNodeNotFound
}

func (n *Node256[V]) grow(ctx context.Context) (*INode[V], error) { //nolint:unused
	return nil, fmt.Errorf("node256 can not grow anymore")
}

// shrink to node48
func (n *Node256[V]) shrink(ctx context.Context) (*INode[V], error) { //nolint:unused
	if !n.isShrinkable(ctx) {
		return nil, fmt.Errorf("node256 is still too big for shrinking")
	}

	currChildrenLen := n.getChildrenLen(ctx)
	if currChildrenLen == 0 {
		return nil, fmt.Errorf("node256 has 0 children, which is unexpected")
	}

	n48 := NewNode[V](KindNode48)
	n48.setPrefix(ctx, n.getPrefix(ctx))

	for k := range int(Node256PointersMax) {
		child := n.children[k]
		if child == nil {
			continue
		}
		if err := n48.addChild(ctx, n.getKeyFromIdx(k), child); err != nil {
			return nil, err
		}
	}

	return &n48, nil
}

func (n *Node256[V]) hasEnoughSpace(ctx context.Context) bool { //nolint:unused
	// node256 is the biggest node so it always (supposed to be) has enough space
	return true
}

func (n *Node256[V]) isShrinkable(ctx context.Context) bool { //nolint:unused
	return n.getChildrenLen(ctx) < Node256PointersMin
}

func (n *Node256[V]) GetLocker() go_context_aware_lock.IOptRWMutex {
	return n.locker
}

func (n *Node256[V]) setLocker(locker go_context_aware_lock.IOptRWMutex) {
	n.locker = locker
}

func (n *Node256[V]) clone() INode[V] { //nolint:unused
	nn := &Node256[V]{}
	nn.nodeHeader = n.nodeHeader
	nn.locker = n.locker
	copy(nn.children[:], n.children[:])
	return nn
}

var _ INode[any] = (*Node256[any])(nil)
