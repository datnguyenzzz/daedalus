package common

import (
	"bytes"
)

// IComparer defines a total ordering over the space of []byte keys: a 'less than' relationship.
type IComparer interface {
	// Compare returns -1, 0, or +1 depending on whether a is 'less than', 'equal
	// to' or 'greater than' b.
	//
	// A key a is less than b if a's prefix is byte-wise less than b's prefix, or if
	// the prefixes are equal and a's suffix is less than b's suffix
	Compare(a, b []byte) int

	// Separator returns a sequence of bytes x such that a <= x && x < b,
	// where 'less than' is consistent with Compare.
	// Trivial implementation is just simply returns "a", however we try to return
	// a shorter "x" to reduce the SSTable size
	//
	// Use case: Use for creating an index key that separate 2 blocks
	Separator(a, b []byte) []byte

	// Successor returns a sequence of bytes x such that x >= b, where
	// 'less than' is consistent with Compare.
	// Trivial implementation is just simply return "b", however we try to return
	// a shorter "x" to reduce the SSTable size
	//
	// Use case: Use to create an index key of the last block in a SSTable
	Successor(b []byte) []byte

	// Split return the prefix of a given key
	//
	// Use case: Uses to separate the actual user key and MVCC id
	// in the internalKey.UserKey
	Split(b []byte) int

	// Name of the comparer will be persisted within the Manifest
	Name() string

	// AbbreviatedKey returns a fixed length prefix of a user key such that
	//
	//	AbbreviatedKey(a) < AbbreviatedKey(b) implies a < b, and
	//	AbbreviatedKey(a) > AbbreviatedKey(b) implies a > b.
	//
	// If AbbreviatedKey(a) == AbbreviatedKey(b), an additional comparison is
	// required to determine if the two keys are actually equal.
	//
	// This helps optimize indexed batch comparisons for cache locality. If a Split
	// function is specified, AbbreviatedKey usually returns the first eight bytes
	// of the user key prefix in the order that gives the correct ordering.
	AbbreviatedKey(key []byte) []byte
}

// TODO(high): Implement separated comparer for the MVVC key

type DefaultComparer struct{}

func (c DefaultComparer) Name() string {
	return "DefaultComparer"
}

func (c DefaultComparer) Separator(a, b []byte) []byte {
	var prefixLen int
	n := min(len(a), len(b))
	for prefixLen = 0; prefixLen < n && a[prefixLen] == b[prefixLen]; prefixLen++ {
	}
	if prefixLen >= n || a[prefixLen] >= b[prefixLen] {
		return a
	} else {
		// If the LCP == len(b)-1 --> a[LCP]+1 < b[LCP]
		// Else just require a[LCP] +1 <= b[LCP]
		isLess := (prefixLen == len(b)-1 && a[prefixLen]+1 < b[prefixLen]) ||
			(prefixLen < len(b)-1 && a[prefixLen]+1 <= b[prefixLen])
		if a[prefixLen] < 0xff && isLess {
			dst := make([]byte, prefixLen+1)
			copy(dst, a[:prefixLen+1])
			dst[len(dst)-1]++
			return dst
		}
		return a
	}
}

func (c DefaultComparer) Successor(b []byte) []byte {
	for i, v := range b {
		// get first byte i'th that < 255 --> append [b[0] ... b[i]+1] to dst
		if v < 0xff {
			dst := make([]byte, i+1)
			copy(dst, b[:i+1])
			dst[len(dst)-1]++
			return dst
		}
	}
	return b
}

func (c DefaultComparer) Compare(a, b []byte) int {
	return bytes.Compare(a, b)
}

func (c DefaultComparer) Split(b []byte) int {
	return len(b)
}

func (c DefaultComparer) AbbreviatedKey(key []byte) []byte {
	if len(key) >= 8 {
		return key[:8]
	}
	newKey := make([]byte, 8)
	copy(newKey, key)
	return newKey
}

func NewComparer() *DefaultComparer {
	return &DefaultComparer{}
}

var _ IComparer = (*DefaultComparer)(nil)
