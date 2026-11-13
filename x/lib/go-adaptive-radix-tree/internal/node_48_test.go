package internal

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_node48_str_insertAndRemoveChildren(t *testing.T) {
	type param struct {
		desc                 string
		actions              []NodeAction[string]
		expectedKeys         []byte
		expectedChildrenLen  uint8
		expectedAscChildren  []*INode[string]
		expectedDescChildren []*INode[string]
		expectedGetChild     map[*nodeKey]*INode[string]
	}

	sampleLeaves := generateStringLeaves(4)

	testList := []param{
		{
			desc: "Happy case: #1",
			actions: []NodeAction[string]{
				{
					Kind:  InsertAction,
					Key:   1,
					Child: &sampleLeaves[0],
				},
				{
					Kind:  InsertAction,
					Key:   2,
					Child: &sampleLeaves[1],
				},
				{
					Kind:  InsertAction,
					Key:   3,
					Child: &sampleLeaves[2],
				},
			},
			expectedKeys:        []byte{2, 3, 4},
			expectedChildrenLen: 3,
			expectedAscChildren: []*INode[string]{
				&sampleLeaves[0],
				&sampleLeaves[1],
				&sampleLeaves[2],
			},
			expectedDescChildren: []*INode[string]{
				&sampleLeaves[2],
				&sampleLeaves[1],
				&sampleLeaves[0],
			},
			expectedGetChild: map[*nodeKey]*INode[string]{
				ToNodeKey(1): &sampleLeaves[0],
				ToNodeKey(2): &sampleLeaves[1],
				ToNodeKey(3): &sampleLeaves[2],
			},
		},
		{
			desc: "Happy case: #2",
			actions: []NodeAction[string]{
				{
					Kind:  InsertAction,
					Key:   1,
					Child: &sampleLeaves[0],
				},
				{
					Kind: RemoveAction,
					Key:  1,
				},
				{
					Kind:  InsertAction,
					Key:   1,
					Child: &sampleLeaves[3],
				},
			},
			expectedKeys:        []byte{2},
			expectedChildrenLen: 1,
			expectedAscChildren: []*INode[string]{
				&sampleLeaves[3],
			},
			expectedDescChildren: []*INode[string]{
				&sampleLeaves[3],
			},
			expectedGetChild: map[*nodeKey]*INode[string]{
				ToNodeKey(1): &sampleLeaves[3],
			},
		},
		{
			desc: "Happy case: #3",
			actions: []NodeAction[string]{
				{
					Kind:  InsertAction,
					Key:   1,
					Child: &sampleLeaves[0],
				},
				{
					Kind:  InsertAction,
					Key:   2,
					Child: &sampleLeaves[1],
				},
				{
					Kind: RemoveAction,
					Key:  1,
				},
				{
					Kind: RemoveAction,
					Key:  2,
				},
			},
			expectedKeys:         []byte{},
			expectedChildrenLen:  0,
			expectedAscChildren:  []*INode[string]{},
			expectedDescChildren: []*INode[string]{},
			expectedGetChild:     map[*nodeKey]*INode[string]{},
		},
		{
			desc: "Happy case: #4",
			actions: []NodeAction[string]{
				{
					Kind:  InsertAction,
					Key:   1,
					Child: &sampleLeaves[0],
				},
				{
					Kind:  InsertAction,
					Key:   2,
					Child: &sampleLeaves[1],
				},
				{
					Kind:  InsertAction,
					Key:   3,
					Child: &sampleLeaves[2],
				},
				{
					Kind: RemoveAction,
					Key:  2,
				},
				{
					Kind:  InsertAction,
					Key:   4,
					Child: &sampleLeaves[3],
				},
			},
			expectedKeys:        []byte{2, 4, 5},
			expectedChildrenLen: 3,
			expectedAscChildren: []*INode[string]{
				&sampleLeaves[0],
				&sampleLeaves[2],
				&sampleLeaves[3],
			},
			expectedDescChildren: []*INode[string]{
				&sampleLeaves[3],
				&sampleLeaves[2],
				&sampleLeaves[0],
			},
			expectedGetChild: map[*nodeKey]*INode[string]{
				ToNodeKey(1): &sampleLeaves[0],
				ToNodeKey(3): &sampleLeaves[2],
				ToNodeKey(4): &sampleLeaves[3],
			},
		},
		{
			desc: "Happy case: #5",
			actions: []NodeAction[string]{
				{
					Kind:  InsertAction,
					Key:   3,
					Child: &sampleLeaves[0],
				},
				{
					Kind:  InsertAction,
					Key:   2,
					Child: &sampleLeaves[1],
				},
				{
					Kind:  InsertAction,
					Key:   1,
					Child: &sampleLeaves[2],
				},
				{
					Kind: RemoveAction,
					Key:  2,
				},
				{
					Kind:  InsertAction,
					Key:   4,
					Child: &sampleLeaves[3],
				},
			},
			expectedKeys:        []byte{2, 4, 5},
			expectedChildrenLen: 3,
			expectedAscChildren: []*INode[string]{
				&sampleLeaves[2],
				&sampleLeaves[0],
				&sampleLeaves[3],
			},
			expectedDescChildren: []*INode[string]{
				&sampleLeaves[3],
				&sampleLeaves[0],
				&sampleLeaves[2],
			},
			expectedGetChild: map[*nodeKey]*INode[string]{
				ToNodeKey(1): &sampleLeaves[2],
				ToNodeKey(3): &sampleLeaves[0],
				ToNodeKey(4): &sampleLeaves[3],
			},
		},
		{
			desc: "Happy case: #6",
			actions: []NodeAction[string]{
				{
					Kind:  InsertAction,
					Key:   3,
					Child: &sampleLeaves[0],
				},
				{
					Kind:  InsertAction,
					Key:   2,
					Child: &sampleLeaves[1],
				},
				{
					Kind:  InsertAction,
					Key:   4,
					Child: &sampleLeaves[1],
				},
				{
					Kind:  InsertAction,
					Key:   1,
					Child: &sampleLeaves[2],
				},
				{
					Kind: RemoveAction,
					Key:  2,
				},
				{
					Kind: RemoveAction,
					Key:  4,
				},
				{
					Kind:  InsertAction,
					Key:   2,
					Child: &sampleLeaves[3],
				},
				{
					Kind:  InsertAction,
					Key:   4,
					Child: &sampleLeaves[3],
				},
			},
			expectedKeys:        []byte{2, 3, 4, 5},
			expectedChildrenLen: 4,
			expectedAscChildren: []*INode[string]{
				&sampleLeaves[2],
				&sampleLeaves[3],
				&sampleLeaves[0],
				&sampleLeaves[3],
			},
			expectedDescChildren: []*INode[string]{
				&sampleLeaves[3],
				&sampleLeaves[0],
				&sampleLeaves[3],
				&sampleLeaves[2],
			},
			expectedGetChild: map[*nodeKey]*INode[string]{
				ToNodeKey(1): &sampleLeaves[2],
				ToNodeKey(2): &sampleLeaves[3],
				ToNodeKey(3): &sampleLeaves[0],
				ToNodeKey(4): &sampleLeaves[3],
			},
		},
		{
			desc: "Happy case: #7",
			actions: []NodeAction[string]{
				{
					Kind:  InsertAction,
					Key:   3,
					Child: &sampleLeaves[0],
				},
				{
					Kind:  InsertAction,
					Key:   2,
					Child: &sampleLeaves[1],
				},
				{
					Kind: RemoveAction,
					Key:  2,
				},
				{
					Kind:  InsertAction,
					Key:   4,
					Child: &sampleLeaves[3],
				},
				{
					Kind:  InsertAction,
					Key:   1,
					Child: &sampleLeaves[2],
				},
			},
			expectedKeys:        []byte{2, 4, 5},
			expectedChildrenLen: 3,
			expectedAscChildren: []*INode[string]{
				&sampleLeaves[2],
				&sampleLeaves[0],
				&sampleLeaves[3],
			},
			expectedDescChildren: []*INode[string]{
				&sampleLeaves[3],
				&sampleLeaves[0],
				&sampleLeaves[2],
			},
			expectedGetChild: map[*nodeKey]*INode[string]{
				ToNodeKey(1): &sampleLeaves[2],
				ToNodeKey(3): &sampleLeaves[0],
				ToNodeKey(4): &sampleLeaves[3],
			},
		},
		{
			desc: "Happy case, with null nodes",
			actions: []NodeAction[string]{
				{
					Kind:  InsertAction,
					Key:   0,
					Child: &sampleLeaves[0],
				},
				{
					Kind:  InsertAction,
					Key:   1,
					Child: &sampleLeaves[1],
				},
				{
					Kind:   InsertAction,
					IsNull: true,
					Child:  &sampleLeaves[2],
				},
				{
					Kind:   RemoveAction,
					IsNull: true,
				},
				{
					Kind:   InsertAction,
					IsNull: true,
					Child:  &sampleLeaves[3],
				},
				{
					Kind: RemoveAction,
					Key:  0,
				},
				{
					Kind:  InsertAction,
					Key:   0,
					Child: &sampleLeaves[2],
				},
			},
			expectedKeys: []byte{
				0, 1, 2,
			},
			expectedChildrenLen: 3,
			expectedAscChildren: []*INode[string]{
				&sampleLeaves[3],
				&sampleLeaves[2],
				&sampleLeaves[1],
			},
			expectedDescChildren: []*INode[string]{
				&sampleLeaves[1],
				&sampleLeaves[2],
				&sampleLeaves[3],
			},
			expectedGetChild: map[*nodeKey]*INode[string]{
				NullNodeKey(): &sampleLeaves[3],
				ToNodeKey(0):  &sampleLeaves[2],
				ToNodeKey(1):  &sampleLeaves[1],
			},
		},
	}

	for _, tc := range testList {
		t.Run(tc.desc, func(t *testing.T) {
			n48 := NewNode[string](KindNode48)
			ctx := context.Background()
			// perform actions
			for _, action := range tc.actions {
				key := NullNodeKey()
				if !action.IsNull {
					key = ToNodeKey(action.Key)
				}
				if action.Kind == InsertAction {
					err := n48.addChild(ctx, key, action.Child)
					assert.NoError(t, err)
				} else {
					err := n48.removeChild(ctx, key)
					assert.NoError(t, err)
				}
			}
			// verify output
			n48o, ok := n48.(*Node48[string])
			assert.True(t, ok, "can not cast to Node4[string]")
			//// fill expectedKeys with 0
			var expectedKeys [Node48KeysLen]byte
			for idx, k := range tc.expectedKeys {
				expectedKeys[k] = byte(idx + 1)
			}
			assert.Equal(t, n48o.keys, expectedKeys, "node keys is different")
			// fill expectedChildren with nil pointer
			var expectedChildren [Node48PointersMax]*INode[string]
			copy(expectedChildren[:tc.expectedChildrenLen], tc.expectedAscChildren)

			assert.Equal(t, expectedChildren, n48o.children, "node children is different")
			assert.Equal(t, tc.expectedChildrenLen, n48o.getChildrenLen(ctx), "node children length is different")
			assert.Equal(t, tc.expectedAscChildren, n48o.getAllChildren(ctx, AscOrder), "node children in ASC is different")
			assert.Equal(t, tc.expectedDescChildren, n48o.getAllChildren(ctx, DescOrder), "node children in DESC is different")
			for k, expectedChild := range tc.expectedGetChild {
				child, err := n48o.getChild(ctx, k)
				assert.NoError(t, err)
				assert.Equal(t, expectedChild, child)
			}
			for i := range tc.expectedChildrenLen {
				key, child, err := n48o.getChildByIndex(ctx, i)
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedAscChildren[i], child)
				// verify key matches expected key
				expectedChildFromMap, err := n48o.getChild(ctx, key)
				assert.NoError(t, err)
				assert.Equal(t, expectedChildFromMap, child)
			}
			_, _, err := n48o.getChildByIndex(ctx, tc.expectedChildrenLen)
			assert.ErrorIs(t, err, childNodeNotFound)
		})
	}
}

func Test_node48_str_grow(t *testing.T) {
	ctx := context.Background()
	n48 := NewNode[string](KindNode48)

	samplePrefix := RandomBytes(5)
	n48.setPrefix(ctx, samplePrefix)

	sampleLeaves := generateStringLeaves(int(Node48PointersMax))
	// Add children to the node until it reaches its space capacity
	var children []*INode[string]
	for idx := range Node48PointersMax {
		leaf := sampleLeaves[idx]
		children = append(children, &leaf)
		err := n48.addChild(ctx, ToNodeKey(idx), &leaf)
		assert.NoError(t, err, fmt.Sprintf("shouldn't fail to add new Child with Key - %v", idx))
	}

	// grow to bigger node
	nn, err := n48.grow(ctx)
	assert.NoError(t, err, "shouldn't fail to grow")
	// verify output
	n256 := *nn
	n256o, ok := n256.(*Node256[string])
	assert.True(t, ok, "can not cast to Node256[string]")
	assert.Equal(t, samplePrefix, n256o.getPrefix(ctx))
	assert.Equal(t, KindNode256, n256o.GetKind(ctx))
	// fill expectedChildren with nil pointer
	var expectedChildren [Node256PointersMax]*INode[string]
	for idx, child := range children {
		expectedChildren[idx+1] = child // index 0 is for NullNodeKey
	}
	assert.Equal(t, expectedChildren, n256o.children, "node children is different")
}

func Test_node48_str_shrink(t *testing.T) {
	ctx := context.Background()
	n48 := NewNode[string](KindNode48)

	samplePrefix := RandomBytes(5)
	n48.setPrefix(ctx, samplePrefix)

	sampleLeaves := generateStringLeaves(int(Node48PointersMin - 1))
	// Add children to the node which is lower than the minimum required capacity
	var keys []byte
	var children []*INode[string]
	for idx := range Node48PointersMin - 1 {
		leaf := sampleLeaves[idx]
		keys = append(keys, idx)
		children = append(children, &leaf)
		err := n48.addChild(ctx, ToNodeKey(idx), &leaf)
		assert.NoError(t, err, fmt.Sprintf("shouldn't fail to add new Child with Key - %v", idx))
	}

	// shrink to smaller node
	nn, err := n48.shrink(ctx)
	assert.NoError(t, err, "shouldn't fail to shrink")
	// verify output
	n16 := *nn
	n16o, ok := n16.(*Node16[string])
	assert.True(t, ok, "can not cast to Node16[string]")
	assert.Equal(t, samplePrefix, n16o.getPrefix(ctx))
	assert.Equal(t, KindNode16, n16o.GetKind(ctx))

	// fill expectedKeys with nil
	var expectedKeys [Node16KeysMax]*nodeKey
	for i, key := range keys {
		expectedKeys[Node16KeysMax-uint8(len(keys))+uint8(i)] = ToNodeKey(key)
	}
	for i := range len(n16o.keys) {
		assert.Zero(t, expectedKeys[i].Compare(n16o.keys[i]), "node keys is different")
	}

	// fill expectedChildren with nil pointer
	var expectedChildren [Node16PointersLen]*INode[string]
	copy(expectedChildren[Node16KeysMax-uint8(len(keys)):], children)
	assert.Equal(t, expectedChildren, n16o.children, "node children is different")
}
