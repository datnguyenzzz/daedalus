package internal

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
)

type ActionType uint8

const (
	InsertAction ActionType = iota
	RemoveAction
)

type NodeAction[V any] struct {
	Kind   ActionType
	Key    byte
	Child  *INode[V]
	IsNull bool
}

type KV[V any] struct {
	Key   []byte
	Value V
}

type TreeAction[V any] struct {
	Kind ActionType
	KV[V]
}

func randomByte() byte {
	randomByte := make([]byte, 1)

	// Read random data into the byte slice
	_, err := rand.Read(randomByte)
	if err != nil {
		fmt.Println("Error generating random byte:", err)
		return 0
	}

	return randomByte[0]
}

func RandomBytes(num uint8) []byte {
	res := make([]byte, num)
	for i := range num {
		res[i] = randomByte()
	}
	return res
}

func RandomQuote() string {
	quote := struct {
		Sentence string `faker:"sentence"`
	}{}

	err := faker.FakeData(&quote)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	return quote.Sentence
}

func generateStringLeaves(sz int) []INode[string] {
	res := make([]INode[string], sz)

	for i := range sz {
		res[i] = NewLeafWithKV(context.Background(), RandomBytes(5), RandomQuote())
	}

	return res
}

func SeedMapKVString(sz int) []KV[string] {
	res := make([]KV[string], sz)
	for i := range sz {
		res[i] = KV[string]{
			Key:   fmt.Appendf(nil, "%d__%v", i, RandomQuote()),
			Value: RandomQuote(),
		}
	}
	return res
}

// SeedNode4 generate a node4 with prefix for testing purposes
func SeedNode4[V any](ctx context.Context, prefix []byte) INode[V] {
	n := &Node4[V]{}
	n.setPrefix(ctx, prefix)
	return n
}

// SeedNodeLeaf generate a node leaf with prefix and value for testing purposes
func SeedNodeLeaf[V any](ctx context.Context, prefix []byte, value V) INode[V] {
	return NewLeafWithKV(ctx, prefix, value)
}

// PreorderTraverseAndValidate traverse through the tree by the pre-order and perform the validation
// return number of children
func PreorderTraverseAndValidate[V any](
	t *testing.T, ctx context.Context,
	node INode[V],
	expectedPreorder []INode[V], idx, depth int8,
) int {
	if node == nil || node.isDeleted(ctx) {
		// node is not exist
		assert.Nil(t, expectedPreorder[idx])
		return 0
	}
	// fmt.Printf("prefix: %v, idx: %v \n", string(node.getPrefix(ctx)), idx)
	assert.Equal(t, node.GetKind(ctx), expectedPreorder[idx].GetKind(ctx), fmt.Sprintf("kind is not match at index - %v", idx))
	assert.Equal(t, node.getPrefix(ctx), expectedPreorder[idx].getPrefix(ctx), fmt.Sprintf("prefix is not match at index -  %v", idx))

	if node.GetKind(ctx) == KindNodeLeaf {
		assert.Equal(t, node.getValue(ctx), expectedPreorder[idx].getValue(ctx), fmt.Sprintf("value is not match at index - %v", idx))
		return 1
	}

	totalChildrenLen := 0
	for _, child := range node.getAllChildren(ctx, AscOrder) {
		subtreeSize := PreorderTraverseAndValidate(
			t, ctx, *child, expectedPreorder,
			int8(totalChildrenLen)+idx+1, depth+1,
		)
		totalChildrenLen += subtreeSize
	}

	return totalChildrenLen + 1
}
