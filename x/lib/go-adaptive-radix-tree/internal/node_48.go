package internal

import (
	"context"
	"fmt"

	go_context_aware_lock "github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock"
)

const (
	Node48KeysLen     uint16 = 256 + 1
	Node48PointersMax uint8  = 48
	Node48PointersMin uint8  = Node16KeysMax + 1 // node48 needs at least 17 children, else it can be shrunk to node16
)

// Node48 As the number of entries in a node increases,
// searching the Key array becomes expensive. Therefore, nodes
// with more than 16 pointers do not store the keys explicitly.
// Instead, a 256-element array is used, which can be indexed
// with Key bytes directly. If a node has between 17 and 48 Child
// pointers, this array stores indexes into a second array which
// contains up to 48 pointers.
type Node48[V any] struct {
	nodeHeader
	locker go_context_aware_lock.IOptRWMutex
	// At position i-th, keys[i] = [position in the pointers array] + 1,
	// if keys[i] = 0 means the Key i-th haven't had a Child yet
	// pointers[i] = pointer to Child for the Key = i-th
	// keys is an array of length 256 for a 1-byte Key.
	// The pointers array is sorted in ascending order (by the value of keys)
	keys [Node48KeysLen]byte
	// pointers to children node. children[i] is a pointer to a Child node for a Key = i
	children [Node48PointersMax]*INode[V]
}

func (n *Node48[V]) getIdx(key *nodeKey) int {
	if key.IsNull() {
		return 0
	}

	return int(key.b + 1)
}

func (n *Node48[V]) getKeyFromIdx(idx int) *nodeKey {
	if idx == 0 {
		return NullNodeKey()
	}

	return ToNodeKey(byte(idx - 1))
}

func (n *Node48[V]) getValue(ctx context.Context) V { //nolint:unused
	panic("node 48 doesn't hold any value")
}

func (n *Node48[V]) setValue(ctx context.Context, v V) { //nolint:unused
	panic("node 48 doesn't hold any value")
}

func (n *Node48[V]) GetKind(ctx context.Context) Kind {
	return KindNode48
}

func (n *Node48[V]) addChild(ctx context.Context, key *nodeKey, child *INode[V]) error { //nolint:unused
	currChildrenLen := n.getChildrenLen(ctx)
	if currChildrenLen >= Node48PointersMax {
		return fmt.Errorf("node_48 is maxed out and don't have enough room for a new Key")
	}

	if n.keys[n.getIdx(key)] > 0 {
		return fmt.Errorf("Key: %v already exists", key)
	}

	// shift all keys[Key+1:] 1 step to the right to make room
	for k := int(Node48KeysLen) - 1; k > n.getIdx(key); k-- {
		if n.keys[k] == 0 {
			// Key k-th is not exist yet
			continue
		}
		childPos := n.keys[k] - 1
		if childPos+1 >= Node48PointersMax {
			return fmt.Errorf("node_48 is maxed out and don't have enough room for a new Key")
		}
		n.children[childPos+1] = n.children[childPos]
		n.children[childPos] = nil
		n.keys[k] += 1
	}
	pos := uint8(0)
	for k := range n.getIdx(key) {
		if n.keys[k] == 0 {
			// Key k-th is not exist yet
			continue
		}
		pos = n.keys[k]
	}
	// children[pos] should be free now
	if n.children[pos] != nil {
		return fmt.Errorf("pos: %v in n.children is not freed yet", pos)
	}
	n.keys[n.getIdx(key)] = pos + 1
	n.children[pos] = child
	n.setChildrenLen(ctx, currChildrenLen+1)

	return nil
}

func (n *Node48[V]) removeChild(ctx context.Context, key *nodeKey) error { //nolint:unused
	currChildrenLen := n.getChildrenLen(ctx)
	if currChildrenLen == 0 {
		return fmt.Errorf("node is empty")
	}
	if n.keys[n.getIdx(key)] == 0 {
		return childNodeNotFound
	}

	// remove n.keys[Key]
	n.children[n.keys[n.getIdx(key)]-1] = nil
	n.keys[n.getIdx(key)] = 0

	// shift all keys[Key+1:] 1 step to the left
	for k := n.getIdx(key) + 1; k < int(Node48KeysLen); k++ {
		if n.keys[k] == 0 {
			// Key k-th is not exist yet
			continue
		}
		childPos := n.keys[k] - 1
		n.children[childPos-1] = n.children[childPos]
		n.children[childPos] = nil
		n.keys[k] -= 1
	}
	n.setChildrenLen(ctx, currChildrenLen-1)

	return nil
}

func (n *Node48[V]) getChild(ctx context.Context, key *nodeKey) (*INode[V], error) {
	if n.keys[n.getIdx(key)] == 0 {
		return nil, childNodeNotFound
	}

	return n.children[n.keys[n.getIdx(key)]-1], nil
}

func (n *Node48[V]) getAllChildren(ctx context.Context, order Order) []*INode[V] {
	switch order {
	case AscOrder:
		res := make([]*INode[V], n.getChildrenLen(ctx))
		cnt := 0
		for k := range int(Node48KeysLen) {
			if n.keys[k] == 0 {
				// Key k-th is not exist yet
				continue
			}
			childPos := n.keys[k] - 1
			res[cnt] = n.children[childPos]
			cnt += 1
		}
		return res
	case DescOrder:
		res := make([]*INode[V], n.getChildrenLen(ctx))
		cnt := 0
		for k := int(Node48KeysLen) - 1; k >= 0; k-- {
			if n.keys[k] == 0 {
				// Key k-th is not exist yet
				continue
			}
			childPos := n.keys[k] - 1
			res[cnt] = n.children[childPos]
			cnt += 1
		}
		return res
	default:
		// shouldn't go into that block
		return make([]*INode[V], n.getChildrenLen(ctx))
	}
}

func (n *Node48[V]) getChildByIndex(ctx context.Context, idx uint8) (*nodeKey, *INode[V], error) {
	currLen := n.getChildrenLen(ctx)
	if idx == currLen {
		return nil, nil, childNodeNotFound
	}

	cnt := 0
	for k := range int(Node48KeysLen) {
		if n.keys[k] == 0 {
			// Key k-th is not exist yet
			continue
		}
		if cnt == int(idx) {
			return n.getKeyFromIdx(k), n.children[n.keys[k]-1], nil
		}
		cnt += 1
	}
	return nil, nil, childNodeNotFound
}

// grow to node256
func (n *Node48[V]) grow(ctx context.Context) (*INode[V], error) { //nolint:unused
	if n.getChildrenLen(ctx) != Node48PointersMax {
		return nil, fmt.Errorf("node48 is not maxed out yet, so don't have to grow to a bigger node")
	}

	n256 := NewNode[V](KindNode256)
	n256.setPrefix(ctx, n.getPrefix(ctx))

	for k := range int(Node48KeysLen) {
		if n.keys[k] == 0 {
			// Key k-th is not exist yet
			continue
		}
		childPos := n.keys[k] - 1
		child := n.children[childPos]
		if err := n256.addChild(ctx, n.getKeyFromIdx(k), child); err != nil {
			return nil, err
		}
	}

	return &n256, nil
}

// shrink to node16
func (n *Node48[V]) shrink(ctx context.Context) (*INode[V], error) { //nolint:unused
	if !n.isShrinkable(ctx) {
		return nil, fmt.Errorf("node48 is still too big for shrinking")
	}

	currChildrenLen := n.getChildrenLen(ctx)
	if currChildrenLen == 0 {
		return nil, fmt.Errorf("node48 has 0 children, which is unexpected")
	}

	n16 := NewNode[V](KindNode16)
	n16.setPrefix(ctx, n.getPrefix(ctx))

	for k := range int(Node48KeysLen) {
		if n.keys[k] == 0 {
			// Key k-th is not exist yet
			continue
		}
		childPos := n.keys[k] - 1
		child := n.children[childPos]
		if err := n16.addChild(ctx, n.getKeyFromIdx(k), child); err != nil {
			return nil, err
		}
	}

	return &n16, nil
}

func (n *Node48[V]) hasEnoughSpace(ctx context.Context) bool { //nolint:unused
	return n.getChildrenLen(ctx) < Node48PointersMax
}

func (n *Node48[V]) isShrinkable(ctx context.Context) bool { //nolint:unused
	return n.getChildrenLen(ctx) < Node48PointersMin
}

func (n *Node48[V]) GetLocker() go_context_aware_lock.IOptRWMutex {
	return n.locker
}

func (n *Node48[V]) setLocker(locker go_context_aware_lock.IOptRWMutex) {
	n.locker = locker
}

func (n *Node48[V]) clone() INode[V] { //nolint:unused
	nn := &Node48[V]{}
	nn.nodeHeader = n.nodeHeader
	nn.locker = n.locker
	copy(nn.keys[:], n.keys[:])
	copy(nn.children[:], n.children[:])
	return nn
}

var _ INode[any] = (*Node48[any])(nil)
