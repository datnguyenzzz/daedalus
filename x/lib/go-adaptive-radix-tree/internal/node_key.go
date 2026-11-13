package internal

import "bytes"

type nodeKey struct {
	b   byte
	set bool
}

func ToNodeKey(b byte) *nodeKey {
	return &nodeKey{
		set: true,
		b:   b,
	}
}

func NullNodeKey() *nodeKey {
	return &nodeKey{}
}

func (k *nodeKey) IsNull() bool {
	return k == nil || !k.set
}

// Compare return -1,0,1 if a k is less than, equal, greater than k2, respectively
func (k *nodeKey) Compare(k2 *nodeKey) int {
	switch {
	case k.IsNull() && !k2.IsNull():
		return -1
	case !k.IsNull() && k2.IsNull():
		return 1
	case k.IsNull() && k2.IsNull():
		return 0
	default:
		return bytes.Compare([]byte{k.b}, []byte{k2.b})
	}
}
