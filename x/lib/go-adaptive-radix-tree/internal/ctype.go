package internal

import (
	"context"
	"fmt"

	go_context_aware_lock "github.com/datnguyenzzz/nogodb/lib/go-context-aware-lock"
)

// errors
var (
	// tree level errors
	failedToAddChild    error = fmt.Errorf("failed to add Child")
	failedToRemoveChild error = fmt.Errorf("failed to remove Child")
	failedToGrowNode    error = fmt.Errorf("failed to grow node")
	failedToShrinkNode  error = fmt.Errorf("failed to shrink node")
	childNodeNotFound   error = fmt.Errorf("Child node not found")
	NoSuchKey           error = fmt.Errorf("not such Key")
	internalError       error = fmt.Errorf("internal error")
)

type Callback[V any] func(ctx context.Context, k []byte, v V)

type Kind int8

const (
	KindNodeNoop Kind = iota
	KindNodeLeaf
	KindNode4
	KindNode16
	KindNode48
	KindNode256
)

type Order int8

const (
	AscOrder Order = iota
	DescOrder
)

type iNodeHeader interface {
	// checkPrefix compares the compressed path of a node with the Key and returns the number of equal bytes
	checkPrefix(ctx context.Context, key []byte, offset uint8) uint8
	getPrefix(ctx context.Context) []byte
	getPrefixLen(ctx context.Context) uint8
	setPrefix(ctx context.Context, prefix []byte)
	getChildrenLen(ctx context.Context) uint8
	setChildrenLen(ctx context.Context, childrenLen uint8)
	cleanup(ctx context.Context)
	isDeleted(ctx context.Context) bool
}

// iNodeSizeManager to control the size of the node itself
type iNodeSizeManager[V any] interface {
	grow(ctx context.Context) (*INode[V], error)
	hasEnoughSpace(ctx context.Context) bool
	shrink(ctx context.Context) (*INode[V], error)
	isShrinkable(ctx context.Context) bool
}

// iNodeChildrenManager control the node's children
type iNodeChildrenManager[V any] interface {
	addChild(ctx context.Context, key *nodeKey, child *INode[V]) error
	removeChild(ctx context.Context, key *nodeKey) error
	getChild(ctx context.Context, key *nodeKey) (*INode[V], error)
	getAllChildren(ctx context.Context, order Order) []*INode[V]
	getChildByIndex(ctx context.Context, idx uint8) (*nodeKey, *INode[V], error)
}

type INode[V any] interface {
	iNodeHeader
	iNodeSizeManager[V]
	iNodeChildrenManager[V]

	GetKind(ctx context.Context) Kind
	getValue(ctx context.Context) V
	setValue(ctx context.Context, v V)
	GetLocker() go_context_aware_lock.IOptRWMutex
	setLocker(locker go_context_aware_lock.IOptRWMutex)
	clone() INode[V]
}
