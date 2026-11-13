package internal

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_node16_str_insertAndRemoveChildren(t *testing.T) {
	type param struct {
		desc                 string
		actions              []NodeAction[string]
		expectedKeys         [Node16KeysMax]*nodeKey
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
			expectedKeys: [Node16KeysMax]*nodeKey{
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, ToNodeKey(1), ToNodeKey(2), ToNodeKey(3),
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
			expectedKeys: [Node16KeysMax]*nodeKey{
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, ToNodeKey(1),
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
			expectedKeys:         [Node16KeysMax]*nodeKey{},
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
			expectedKeys: [Node16KeysMax]*nodeKey{
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, ToNodeKey(1), ToNodeKey(3), ToNodeKey(4),
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
			expectedKeys: [Node16KeysMax]*nodeKey{
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, ToNodeKey(1), ToNodeKey(3), ToNodeKey(4),
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
			expectedKeys: [Node16KeysMax]*nodeKey{
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, ToNodeKey(1), ToNodeKey(2), ToNodeKey(3), ToNodeKey(4),
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
			expectedKeys: [Node16KeysMax]*nodeKey{
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, ToNodeKey(1), ToNodeKey(3), ToNodeKey(4),
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
			desc: "Happy case #8, with null nodes",
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
			expectedKeys: [Node16KeysMax]*nodeKey{
				nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, NullNodeKey(), ToNodeKey(0), ToNodeKey(1),
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
			n16 := NewNode[string](KindNode16)
			ctx := context.Background()
			// perform actions
			for _, action := range tc.actions {
				key := NullNodeKey()
				if !action.IsNull {
					key = ToNodeKey(action.Key)
				}
				if action.Kind == InsertAction {
					err := n16.addChild(ctx, key, action.Child)
					assert.NoError(t, err)
				} else {
					err := n16.removeChild(ctx, key)
					assert.NoError(t, err)
				}
			}
			// verify output
			n16o, ok := n16.(*Node16[string])
			assert.True(t, ok, "can not cast to Node16[string]")
			for i := range len(n16o.keys) {
				if n16o.keys[i] == nil {
					assert.Nil(t, tc.expectedKeys[i])
					continue
				}
				assert.Zero(t, tc.expectedKeys[i].Compare(n16o.keys[i]), "node keys is different")
			}
			// fill expectedChildren with nil pointer
			var expectedChildren [Node16PointersLen]*INode[string]
			copy(expectedChildren[Node16KeysMax-tc.expectedChildrenLen:], tc.expectedAscChildren)
			assert.Equal(t, expectedChildren, n16o.children, "node children is different")
			assert.Equal(t, tc.expectedChildrenLen, n16o.getChildrenLen(ctx), "node children length is different")
			assert.Equal(t, tc.expectedAscChildren, n16o.getAllChildren(ctx, AscOrder), "node children in ASC is different")
			assert.Equal(t, tc.expectedDescChildren, n16o.getAllChildren(ctx, DescOrder), "node children in DESC is different")
			for k, expectedChild := range tc.expectedGetChild {
				child, err := n16o.getChild(ctx, k)
				assert.NoError(t, err)
				assert.Equal(t, expectedChild, child)
			}
			for i := range tc.expectedChildrenLen {
				key, child, err := n16o.getChildByIndex(ctx, i)
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedAscChildren[i], child)
				// verify key matches expected key from expectedKeys array
				// expectedKeys contains nil for empty slots, non-nil keys are at the end
				expectedKey := tc.expectedKeys[Node16KeysMax-tc.expectedChildrenLen+i]
				assert.Zero(t, expectedKey.Compare(key))
			}
			_, _, err := n16o.getChildByIndex(ctx, tc.expectedChildrenLen)
			assert.ErrorIs(t, err, childNodeNotFound)
		})
	}
}

func Test_node16_str_grow(t *testing.T) {
	ctx := context.Background()
	n16 := NewNode[string](KindNode16)

	samplePrefix := RandomBytes(5)
	n16.setPrefix(ctx, samplePrefix)

	sampleLeaves := generateStringLeaves(int(Node16KeysMax))
	// Add children to the node until it reaches its space capacity
	var keys []*nodeKey
	var children []*INode[string]
	for idx := range Node16KeysMax {
		leaf := sampleLeaves[idx]
		keys = append(keys, ToNodeKey(idx))
		children = append(children, &leaf)
		err := n16.addChild(ctx, ToNodeKey(idx), &leaf)
		assert.NoError(t, err, fmt.Sprintf("shouldn't fail to add new Child with Key - %v", idx))
	}

	// grow to bigger node
	nn, err := n16.grow(ctx)
	assert.NoError(t, err, "shouldn't fail to grow")
	// verify output
	n48 := *nn
	n48o, ok := n48.(*Node48[string])
	assert.True(t, ok, "can not cast to Node48[string]")
	assert.Equal(t, samplePrefix, n48o.getPrefix(ctx))
	assert.Equal(t, KindNode48, n48o.GetKind(ctx))

	var expectedKeys [Node48KeysLen]byte
	for i, key := range keys {
		expectedKeys[n48o.getIdx(key)] = byte(i + 1)
	}
	assert.Equal(t, expectedKeys, n48o.keys, "node keys is different")
	// fill expectedChildren with nil pointer
	var expectedChildren [Node48PointersMax]*INode[string]
	copy(expectedChildren[:Node16KeysMax], children)
	assert.Equal(t, expectedChildren, n48o.children, "node children is different")
}

func Test_node16_str_shrink(t *testing.T) {
	ctx := context.Background()
	n16 := NewNode[string](KindNode16)

	samplePrefix := RandomBytes(5)
	n16.setPrefix(ctx, samplePrefix)

	sampleLeaves := generateStringLeaves(int(Node16KeysMin - 1))
	// Add children to the node which is lower than the minimum required capacity
	var keys []*nodeKey
	var children []*INode[string]
	for idx := range Node16KeysMin - 1 {
		leaf := sampleLeaves[idx]
		keys = append(keys, ToNodeKey(idx))
		children = append(children, &leaf)
		err := n16.addChild(ctx, ToNodeKey(idx), &leaf)
		assert.NoError(t, err, fmt.Sprintf("shouldn't fail to add new Child with Key - %v", idx))
	}

	// shrink to smaller node
	nn, err := n16.shrink(ctx)
	assert.NoError(t, err, "shouldn't fail to shrink")
	// verify output
	n4 := *nn
	n4o, ok := n4.(*Node4[string])
	assert.True(t, ok, "can not cast to Node4[string]")
	assert.Equal(t, samplePrefix, n4o.getPrefix(ctx))
	assert.Equal(t, KindNode4, n4o.GetKind(ctx))

	// fill expectedKeys with nil
	var expectedKeys [Node4KeysMax]*nodeKey
	for i, key := range keys {
		expectedKeys[Node4KeysMax-uint8(len(keys))+uint8(i)] = key
	}
	for i := range len(n4o.keys) {
		if expectedKeys[i] == nil {
			assert.Nil(t, n4o.keys[i])
			continue
		}
		assert.Zero(t, expectedKeys[i].Compare(n4o.keys[i]), "node keys is different")
	}

	// fill expectedChildren with nil pointer
	var expectedChildren [Node4PointersLen]*INode[string]
	copy(expectedChildren[Node4KeysMax-uint8(len(keys)):], children)
	assert.Equal(t, expectedChildren, n4o.children, "node children is different")
}
