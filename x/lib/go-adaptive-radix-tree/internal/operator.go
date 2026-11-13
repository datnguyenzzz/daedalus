package internal

import (
	"context"
	"errors"
	"fmt"
	"slices"

	go_context_aware_lock "github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock"
)

// InsertNode returns the previous value and an error indicating if any was set.
//
//	nodePtr: Pointer to the current node
//	Key, v: The target Key and value
//	offset: The number of bytes of the "Key" that have been processed
func InsertNode[V any](
	ctx context.Context,
	nodePtr *INode[V],
	parentPtr *INode[V],
	parentVersion uint64,
	key []byte,
	v V,
	offset uint8,
) (V, bool, error) {
	if *nodePtr == nil || (*nodePtr).isDeleted(ctx) {
		if !(*parentPtr).GetLocker().Upgrade(parentVersion) {
			return *new(V), true, nil
		}
		newLeaf := NewLeafWithKV(ctx, key, v)
		*nodePtr = newLeaf
		(*parentPtr).GetLocker().Unlock()
		return *new(V), false, nil
	}

	version, obsolete := (*nodePtr).GetLocker().RLock()
	if obsolete {
		return *new(V), true, nil
	}

	if (*nodePtr).GetKind(ctx) == KindNodeLeaf {
		if !(*parentPtr).GetLocker().Upgrade(parentVersion) {
			_ = (*nodePtr).GetLocker().RUnlock(version)
			return *new(V), true, nil
		}

		if !(*nodePtr).GetLocker().Upgrade(version) {
			(*parentPtr).GetLocker().Unlock()
			return *new(V), true, nil
		}

		currLeafKey := (*nodePtr).getPrefix(ctx)

		if isExactMatch(key, currLeafKey) {
			oldValue := (*nodePtr).getValue(ctx)
			(*nodePtr).setValue(ctx, v)
			(*nodePtr).GetLocker().Unlock()
			(*parentPtr).GetLocker().Unlock()
			return oldValue, false, nil
		}

		lcp := findLCP(key, currLeafKey, offset)
		nn := NewNode[V](KindNode4)
		nn.setPrefix(ctx, key[offset:offset+lcp])
		nn.setLocker((*nodePtr).GetLocker())
		offset += lcp

		nodeCopy := (*nodePtr).clone()
		nodeCopy.setLocker(go_context_aware_lock.NewOptimisticLock())

		// add current leaf node to the new inner node
		// but it needs a new locker
		if err := nn.addChild(ctx, GetNodeKey(currLeafKey, int(offset)), &nodeCopy); err != nil {
			(*nodePtr).GetLocker().Unlock()
			(*parentPtr).GetLocker().Unlock()
			return *new(V), false, fmt.Errorf("%w: %v", failedToAddChild, err)
		}

		// create new leaf node for the targeting Key, and add it to the inner node
		// TODO(high): Only need to store fraction of the [key]
		newLeaf := NewLeafWithKV(ctx, key, v)
		if err := nn.addChild(ctx, GetNodeKey(key, int(offset)), &newLeaf); err != nil {
			(*nodePtr).GetLocker().Unlock()
			(*parentPtr).GetLocker().Unlock()
			return *new(V), false, fmt.Errorf("%w: %v", failedToAddChild, err)
		}
		// replace the current node with the newly created inner node
		*nodePtr = nn

		(*nodePtr).GetLocker().Unlock()
		(*parentPtr).GetLocker().Unlock()
		return *new(V), false, nil
	}

	matchedPrefixLen := (*nodePtr).checkPrefix(ctx, key, offset)
	if matchedPrefixLen != (*nodePtr).getPrefixLen(ctx) {
		if !(*parentPtr).GetLocker().Upgrade(parentVersion) {
			_ = (*nodePtr).GetLocker().RUnlock(version)
			return *new(V), true, nil
		}

		if !(*nodePtr).GetLocker().Upgrade(version) {
			(*parentPtr).GetLocker().Unlock()
			return *new(V), true, nil
		}

		currNodePrefix := (*nodePtr).getPrefix(ctx)
		nn := NewNode[V](KindNode4)
		nn.setPrefix(ctx, currNodePrefix[:matchedPrefixLen])
		nn.setLocker((*nodePtr).GetLocker())

		newLeaf := NewLeafWithKV(ctx, key, v)
		if err := nn.addChild(ctx, GetNodeKey(key, int(offset+matchedPrefixLen)), &newLeaf); err != nil {
			(*nodePtr).GetLocker().Unlock()
			(*parentPtr).GetLocker().Unlock()
			return *new(V), false, fmt.Errorf("%w: %v", failedToAddChild, err)
		}

		// adjust the current node compressed prefix accordingly
		// 1 character is in edge
		(*nodePtr).setPrefix(ctx, currNodePrefix[matchedPrefixLen+1:])

		// deep copy the current node to a new node nodeCopy
		nodeCopy := (*nodePtr).clone()
		nodeCopy.setLocker(go_context_aware_lock.NewOptimisticLock())

		if err := nn.addChild(ctx, GetNodeKey(currNodePrefix, int(matchedPrefixLen)), &nodeCopy); err != nil {
			(*nodePtr).GetLocker().Unlock()
			(*parentPtr).GetLocker().Unlock()
			return *new(V), false, fmt.Errorf("%w: %v", failedToAddChild, err)
		}

		// replace the current node with the newly created inner node
		*nodePtr = nn

		(*parentPtr).GetLocker().Unlock()
		(*nodePtr).GetLocker().Unlock()
		return *new(V), false, nil
	}

	offset += (*nodePtr).getPrefixLen(ctx)
	nextNodePtr, err := (*nodePtr).getChild(ctx, GetNodeKey(key, int(offset)))
	if errors.Is(err, childNodeNotFound) {
		if !(*parentPtr).GetLocker().Upgrade(parentVersion) {
			_ = (*nodePtr).GetLocker().RUnlock(version)
			return *new(V), true, nil
		}

		if !(*nodePtr).GetLocker().Upgrade(version) {
			(*parentPtr).GetLocker().Unlock()
			return *new(V), true, nil
		}

		newLeaf := NewLeafWithKV(ctx, key, v)
		// grow to a bigger node if don't have enough space
		shouldGrow := !(*nodePtr).hasEnoughSpace(ctx)
		if shouldGrow {
			biggerNodePtr, err := (*nodePtr).grow(ctx)
			if err != nil {
				(*parentPtr).GetLocker().Unlock()
				(*nodePtr).GetLocker().Unlock()
				return *new(V), false, fmt.Errorf("current node type - %v, %w: %v", (*nodePtr).GetKind(ctx), failedToGrowNode, err)
			}
			currLocker := (*nodePtr).GetLocker()
			*nodePtr = *biggerNodePtr
			(*nodePtr).setLocker(currLocker)
		}

		if err := (*nodePtr).addChild(ctx, GetNodeKey(key, int(offset)), &newLeaf); err != nil {
			(*parentPtr).GetLocker().Unlock()
			(*nodePtr).GetLocker().Unlock()
			return *new(V), false, fmt.Errorf("%w: %v", failedToAddChild, err)
		}

		(*nodePtr).GetLocker().Unlock()
		(*parentPtr).GetLocker().Unlock()
		return *new(V), false, nil
	}

	if (*nodePtr).GetLocker().Check(version) {
		return *new(V), true, nil
	}

	if (*parentPtr).GetLocker().RUnlock(parentVersion) {
		return *new(V), true, nil
	}

	v, obsolete, err = InsertNode(ctx, nextNodePtr, nodePtr, version, key, v, offset+1)
	if obsolete {
		return *new(V), true, nil
	}

	return v, false, err
}

func RemoveNode[V any](
	ctx context.Context,
	nodePtr *INode[V],
	parentPtr *INode[V],
	parentVersion uint64,
	key []byte,
	offset uint8,
) (V, bool, error) {
	if nodePtr == nil || *nodePtr == nil {
		if (*parentPtr).GetLocker().RUnlock(parentVersion) {
			return *new(V), true, nil
		}

		return *new(V), false, NoSuchKey
	}

	if (*nodePtr).isDeleted(ctx) {
		if (*parentPtr).GetLocker().RUnlock(parentVersion) {
			return *new(V), true, nil
		}

		return *new(V), false, NoSuchKey
	}

	version, obsolete := (*nodePtr).GetLocker().RLock()
	if obsolete {
		return *new(V), true, nil
	}

	if (*nodePtr).GetKind(ctx) == KindNodeLeaf {
		// special case when a tree has only 1 node which is a leaf
		if !(*parentPtr).GetLocker().Upgrade(parentVersion) {
			_ = (*nodePtr).GetLocker().RUnlock(version)
			return *new(V), true, nil
		}

		if !(*nodePtr).GetLocker().Upgrade(version) {
			(*parentPtr).GetLocker().Unlock()
			return *new(V), true, nil
		}

		v, err := tryToRemoveLeafNode(ctx, key, nodePtr)
		if err == nil {
			(*nodePtr).cleanup(ctx)
		}
		(*nodePtr).GetLocker().Unlock()
		(*parentPtr).GetLocker().Unlock()
		return v, false, err
	}

	matchedPrefixLen := (*nodePtr).checkPrefix(ctx, key, offset)
	if matchedPrefixLen != (*nodePtr).getPrefixLen(ctx) {
		if (*nodePtr).GetLocker().RUnlock(version) {
			return *new(V), true, nil
		}

		if (*parentPtr).GetLocker().RUnlock(parentVersion) {
			return *new(V), true, nil
		}
		return *new(V), false, NoSuchKey
	}

	offset += (*nodePtr).getPrefixLen(ctx)
	if offset >= uint8(len(key)) {
		if (*nodePtr).GetLocker().RUnlock(version) {
			return *new(V), true, nil
		}

		if (*parentPtr).GetLocker().RUnlock(parentVersion) {
			return *new(V), true, nil
		}

		return *new(V), false, fmt.Errorf("%w: offset overflow", internalError)
	}

	nextNodePtr, err := (*nodePtr).getChild(ctx, GetNodeKey(key, int(offset)))
	if err != nil || nextNodePtr == nil || (*nextNodePtr).isDeleted(ctx) {
		if (*nodePtr).GetLocker().RUnlock(version) {
			return *new(V), true, nil
		}

		if (*parentPtr).GetLocker().RUnlock(parentVersion) {
			return *new(V), true, nil
		}
		return *new(V), false, NoSuchKey
	}

	if (*nextNodePtr).GetKind(ctx) == KindNodeLeaf {
		if !(*parentPtr).GetLocker().Upgrade(parentVersion) {
			_ = (*nodePtr).GetLocker().RUnlock(version)
			return *new(V), true, nil
		}

		if !(*nodePtr).GetLocker().Upgrade(version) {
			(*parentPtr).GetLocker().Unlock()
			return *new(V), true, nil
		}

		v, err := tryToRemoveLeafNode(ctx, key, nextNodePtr)
		if err != nil {
			(*nodePtr).GetLocker().Unlock()
			(*parentPtr).GetLocker().Unlock()
			return v, false, err
		}
		// attempt to remove the leaf node and unify the inner node if needed
		if err := (*nodePtr).removeChild(ctx, GetNodeKey(key, int(offset))); err != nil {
			(*nodePtr).GetLocker().Unlock()
			(*parentPtr).GetLocker().Unlock()
			return *new(V), false, fmt.Errorf("fail to remove child, current node type - %v, %w: %v", (*nodePtr).GetKind(ctx), failedToRemoveChild, err)
		}
		(*nextNodePtr).cleanup(ctx)

		switch (*nodePtr).getChildrenLen(ctx) {
		case 0:
			prevOffset := offset - (*nodePtr).getPrefixLen(ctx) - 1
			if err = (*parentPtr).removeChild(ctx, ToNodeKey(prevOffset)); err != nil {
				(*nodePtr).GetLocker().Unlock()
				(*parentPtr).GetLocker().Unlock()
				return *new(V), false, fmt.Errorf("fail to remove child, current node type - %v, %w: %v", (*parentPtr).GetKind(ctx), failedToRemoveChild, err)
			}

			(*nodePtr).cleanup(ctx)
		case 1:
			onlyChildK, onlyChildPtr, err := (*nodePtr).getChildByIndex(ctx, 0)
			if err != nil {
				(*nodePtr).GetLocker().Unlock()
				(*parentPtr).GetLocker().Unlock()
				return *new(V), false, err
			}

			var newPrefix []byte
			if (*onlyChildPtr).GetKind(ctx) == KindNodeLeaf {
				newPrefix = (*onlyChildPtr).getPrefix(ctx)
			} else {
				if !onlyChildK.IsNull() {
					newPrefix = slices.Concat((*nodePtr).getPrefix(ctx), []byte{onlyChildK.b}, (*onlyChildPtr).getPrefix(ctx))
				} else {
					newPrefix = slices.Concat((*nodePtr).getPrefix(ctx), (*onlyChildPtr).getPrefix(ctx))
				}
			}

			nodeCopy := (*onlyChildPtr).clone()
			nodeCopy.setLocker((*nodePtr).GetLocker())
			nodeCopy.setPrefix(ctx, newPrefix)

			*nodePtr = nodeCopy
		}

		if (*nodePtr).isShrinkable(ctx) {
			smallerNodePtr, err := (*nodePtr).shrink(ctx)
			if err != nil {
				return *new(V), false, fmt.Errorf("%w: %v", failedToShrinkNode, err)
			}

			currLocker := (*nodePtr).GetLocker()
			*nodePtr = *smallerNodePtr
			(*nodePtr).setLocker(currLocker)
		}

		(*nodePtr).GetLocker().Unlock()
		(*parentPtr).GetLocker().Unlock()
		return v, false, nil
	}

	if (*nodePtr).GetLocker().Check(version) {
		return *new(V), true, nil
	}

	if (*parentPtr).GetLocker().RUnlock(parentVersion) {
		return *new(V), true, nil
	}

	v, obsolete, err := RemoveNode(ctx, nextNodePtr, nodePtr, version, key, offset+1)
	if obsolete {
		return *new(V), true, nil
	}

	return v, false, err
}

func tryToRemoveLeafNode[V any](ctx context.Context, key []byte, nodePtr *INode[V]) (V, error) {
	if !isExactMatch(key, (*nodePtr).getPrefix(ctx)) {
		return *new(V), NoSuchKey
	}

	return (*nodePtr).getValue(ctx), nil
}

// Get is used to look up a specific Key, returning the value and if it was found
//
//	node: The current node
//	Key: The target Key
//	offset: The number of bytes of the "Key" that have been processed
func Get[V any](
	ctx context.Context,
	node INode[V],
	parent INode[V],
	parentVersion uint64,
	key []byte,
	offset uint8,
) (V, bool, error) {
	if node == nil || node.isDeleted(ctx) {
		return *new(V), false, NoSuchKey
	}

	version, obsolete := node.GetLocker().RLock()
	if obsolete || parent.GetLocker().RUnlock(parentVersion) {
		return *new(V), true, nil
	}

	// As we use the Single-value leaf, so the value must be always found in the leaf node,
	// not in the inner nodes
	if node.GetKind(ctx) == KindNodeLeaf {
		currLeafKey := node.getPrefix(ctx)
		if isExactMatch(key, currLeafKey) {
			if node.GetLocker().RUnlock(version) {
				return *new(V), true, nil
			}

			return node.getValue(ctx), false, nil
		}

		if node.GetLocker().RUnlock(version) {
			return *new(V), true, nil
		}

		return *new(V), false, NoSuchKey
	}

	matchedPrefixLen := node.checkPrefix(ctx, key, offset)
	if matchedPrefixLen != node.getPrefixLen(ctx) {
		if node.GetLocker().RUnlock(version) {
			return *new(V), true, nil
		}
		return *new(V), false, NoSuchKey
	}

	offset += node.getPrefixLen(ctx)
	childPtr, err := node.getChild(ctx, GetNodeKey(key, int(offset)))
	if err != nil {
		if node.GetLocker().RUnlock(version) {
			return *new(V), true, nil
		}
		return *new(V), false, NoSuchKey
	}

	if node.GetLocker().RUnlock(version) {
		return *new(V), true, nil
	}

	return Get(ctx, *childPtr, node, version, key, offset+1)
}

// Walk iterates over all keys in the tree and trigger the callback function.
func Walk[V any](ctx context.Context, node INode[V], cb Callback[V], order Order) {
	for {
		if node == nil || node.isDeleted(ctx) {
			return
		}

		version, obsolete := node.GetLocker().RLock()
		if obsolete {
			continue
		}

		if node.GetKind(ctx) == KindNodeLeaf {
			cb(ctx, node.getPrefix(ctx), node.getValue(ctx))
			if node.GetLocker().RUnlock(version) {
				continue
			}

			return
		}

		children := node.getAllChildren(ctx, order)
		if node.GetLocker().RUnlock(version) {
			continue
		}

		for _, childPtr := range children {
			Walk(ctx, *childPtr, cb, order)
		}
		return
	}
}

func Visualize[V any](ctx context.Context, node INode[V]) {
	if node == nil {
		return
	}
	for _, childPtr := range node.getAllChildren(ctx, AscOrder) {
		fmt.Printf("%p --> %p\n", node.GetLocker(), (*childPtr).GetLocker())
	}
	for _, childPtr := range node.getAllChildren(ctx, AscOrder) {
		Visualize(ctx, *childPtr)
	}
}

func NewLeafWithKV[V any](ctx context.Context, key []byte, v V) INode[V] {
	newLeaf := NewNode[V](KindNodeLeaf)
	newLeaf.setPrefix(ctx, key)
	newLeaf.setValue(ctx, v)
	return newLeaf
}

func GetNodeKey(buf []byte, offset int) *nodeKey {
	if offset >= len(buf) {
		return NullNodeKey()
	}

	return ToNodeKey(buf[offset])
}
