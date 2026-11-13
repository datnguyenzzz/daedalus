package internal

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_node4_str_insertAndRemoveChildren(t *testing.T) {
	type param struct {
		desc                 string
		actions              []NodeAction[string]
		expectedKeys         [Node4KeysMax]*nodeKey
		expectedChildren     [Node4PointersLen]*INode[string]
		expectedChildrenLen  uint8
		expectedAscChildren  []*INode[string]
		expectedDescChildren []*INode[string]
		expectedGetChild     map[*nodeKey]*INode[string] // expectedGetChild in ASC sorted order by key
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
			expectedKeys: [Node4KeysMax]*nodeKey{
				nil, ToNodeKey(1), ToNodeKey(2), ToNodeKey(3),
			},
			expectedChildren: [Node4PointersLen]*INode[string]{
				nil,
				&sampleLeaves[0],
				&sampleLeaves[1],
				&sampleLeaves[2],
			},
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
			expectedKeys: [Node4KeysMax]*nodeKey{
				nil, nil, nil, ToNodeKey(1),
			},
			expectedChildren: [Node4PointersLen]*INode[string]{
				nil,
				nil,
				nil,
				&sampleLeaves[3],
			},
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
			expectedKeys: [Node4KeysMax]*nodeKey{nil, nil, nil, nil},
			expectedChildren: [Node4PointersLen]*INode[string]{
				nil,
				nil,
				nil,
				nil,
			},
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
			expectedKeys: [Node4KeysMax]*nodeKey{
				nil, ToNodeKey(1), ToNodeKey(3), ToNodeKey(4),
			},
			expectedChildren: [Node4PointersLen]*INode[string]{
				nil,
				&sampleLeaves[0],
				&sampleLeaves[2],
				&sampleLeaves[3],
			},
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
			expectedKeys: [Node4KeysMax]*nodeKey{
				nil, ToNodeKey(1), ToNodeKey(3), ToNodeKey(4),
			},
			expectedChildren: [Node4PointersLen]*INode[string]{
				nil,
				&sampleLeaves[2],
				&sampleLeaves[0],
				&sampleLeaves[3],
			},
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
			expectedKeys: [Node4KeysMax]*nodeKey{
				ToNodeKey(1), ToNodeKey(2), ToNodeKey(3), ToNodeKey(4),
			},
			expectedChildren: [Node4PointersLen]*INode[string]{
				&sampleLeaves[2],
				&sampleLeaves[3],
				&sampleLeaves[0],
				&sampleLeaves[3],
			},
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
			expectedKeys: [Node4KeysMax]*nodeKey{
				nil, ToNodeKey(1), ToNodeKey(3), ToNodeKey(4),
			},
			expectedChildren: [Node4PointersLen]*INode[string]{
				nil,
				&sampleLeaves[2],
				&sampleLeaves[0],
				&sampleLeaves[3],
			},
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
			expectedKeys: [Node4KeysMax]*nodeKey{
				nil, NullNodeKey(), ToNodeKey(0), ToNodeKey(1),
			},
			expectedChildren: [Node4PointersLen]*INode[string]{
				nil,
				&sampleLeaves[3],
				&sampleLeaves[2],
				&sampleLeaves[1],
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
			n4 := NewNode[string](KindNode4)
			ctx := context.Background()
			// perform actions
			for _, action := range tc.actions {
				key := NullNodeKey()
				if !action.IsNull {
					key = ToNodeKey(action.Key)
				}
				if action.Kind == InsertAction {
					err := n4.addChild(ctx, key, action.Child)
					assert.NoError(t, err)
				} else {
					err := n4.removeChild(ctx, key)
					assert.NoError(t, err)
				}
			}
			// verify output
			n4o, ok := n4.(*Node4[string])
			assert.True(t, ok, "can not cast to Node4[string]")
			for i := range len(n4o.keys) {
				if n4o.keys[i] == nil {
					assert.Nil(t, tc.expectedKeys[i])
					continue
				}
				assert.Zero(t, tc.expectedKeys[i].Compare(n4o.keys[i]), "node keys is different")
			}
			assert.Equal(t, tc.expectedChildren, n4o.children, "node children is different")
			assert.Equal(t, tc.expectedChildrenLen, n4o.getChildrenLen(ctx), "node children length is different")
			assert.Equal(t, tc.expectedAscChildren, n4o.getAllChildren(ctx, AscOrder), "node children in ASC is different")
			assert.Equal(t, tc.expectedDescChildren, n4o.getAllChildren(ctx, DescOrder), "node children in DESC is different")
			for k, expectedChild := range tc.expectedGetChild {
				child, err := n4o.getChild(ctx, k)
				assert.NoError(t, err)
				assert.Equal(t, expectedChild, child)
			}
			for i := range tc.expectedChildrenLen {
				key, child, err := n4o.getChildByIndex(ctx, i)
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedAscChildren[i], child)
				// verify key matches expected key from expectedKeys array
				// expectedKeys contains nil for empty slots, non-nil keys are at the end
				expectedKey := tc.expectedKeys[Node4KeysMax-tc.expectedChildrenLen+i]
				assert.Zero(t, expectedKey.Compare(key))
			}
			_, _, err := n4o.getChildByIndex(ctx, tc.expectedChildrenLen)
			assert.ErrorIs(t, err, childNodeNotFound)
		})
	}
}

func Test_node4_str_grow(t *testing.T) {
	ctx := context.Background()
	n4 := NewNode[string](KindNode4)

	samplePrefix := RandomBytes(5)
	n4.setPrefix(ctx, samplePrefix)

	sampleLeaves := generateStringLeaves(int(Node4KeysMax))
	// Add children to the node until it reaches its space capacity
	var keys []*nodeKey
	var children []*INode[string]
	for idx := range Node4KeysMax {
		leaf := sampleLeaves[idx]
		keys = append(keys, ToNodeKey(idx))
		children = append(children, &leaf)
		err := n4.addChild(ctx, ToNodeKey(idx), &leaf)
		assert.NoError(t, err, fmt.Sprintf("shouldn't fail to add new Child with Key - %v", idx))
	}

	// grow to bigger node
	nn, err := n4.grow(ctx)
	assert.NoError(t, err, "shouldn't fail to grow")
	// verify output
	n16 := *nn
	n16o, ok := n16.(*Node16[string])
	assert.True(t, ok, "can not cast to Node16[string]")
	assert.Equal(t, n16o.getPrefix(ctx), samplePrefix)
	assert.Equal(t, n16o.GetKind(ctx), KindNode16)
	// fill expectedKeys with 0
	var expectedKeys [Node16KeysMax]*nodeKey
	copy(expectedKeys[Node16KeysMax-Node4KeysMax:], keys)
	for i := range expectedKeys {
		if expectedKeys[i] == nil {
			assert.Nil(t, n16o.keys[i])
			continue
		}
		assert.Zero(t, expectedKeys[i].Compare(n16o.keys[i]))
	}
	// fill expectedChildren with nil pointer
	var expectedChildren [Node16PointersLen]*INode[string]
	copy(expectedChildren[Node16KeysMax-Node4KeysMax:], children)
	assert.Equal(t, n16o.children, expectedChildren, "node children is different")
}

func Test_node4_str_shrink(t *testing.T) {
	ctx := context.Background()
	n4 := NewNode[string](KindNode4)

	sampleLeaves := generateStringLeaves(1)
	// Add children to the node which is lower than its minimum required capacity
	var children []*INode[string]
	leaf := sampleLeaves[0]
	children = append(children, &leaf)
	err := n4.addChild(ctx, ToNodeKey(0), &leaf)
	assert.NoError(t, err)

	// shrink to smaller node
	nn, err := n4.shrink(ctx)
	assert.NoError(t, err, "shouldn't fail to shrink")
	// verify output
	nl := *nn
	nlo, ok := nl.(*NodeLeaf[string])
	assert.True(t, ok, "can not cast to NodeLeaf[string]")
	assert.Equal(t, nlo.GetKind(ctx), KindNodeLeaf)
	assert.Equal(t, nlo.getPrefix(ctx), leaf.getPrefix(ctx), "node prefix is different")
	// fill expectedChildren with nil pointer
	var expectedChildren [Node16PointersLen]*INode[string]
	copy(expectedChildren[Node16KeysMax-Node4KeysMax:], children)
	assert.Equal(t, nlo.getValue(ctx), leaf.getValue(ctx), "node value is different")
}
