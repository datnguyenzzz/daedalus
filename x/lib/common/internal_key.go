package common

import (
	"cmp"
	"encoding/binary"
)

// KeyKind enumerates the kind of key: a deletion tombstone, a set
// value, a merged value, etc.
type KeyKind byte

const (
	KeyKindUnknown KeyKind = iota
	KeyKindDelete
	KeyKindSet
	KeyKindMerge
	KeyKindSeparator
	KeyKindMetaIndex
)

// SeqNum is a sequence number defining precedence among identical keys. A key
// with a higher sequence number takes precedence over a key with an equal user
// key of a lower sequence number. Basically this is an auto increment number
// will be assigned to every user key when it is written to the DB
//
// See db/batch.go about the batches, and db/commit.go to understand how batches
// and SeqNum are committed to the log
type SeqNum uint64

const (
	// SeqNumMax is the largest valid sequence number.
	SeqNumMax SeqNum = 1<<56 - 1
)

// InternalKeyTrailer encodes a [SeqNum (7) + InternalKeyKind (1)].
type InternalKeyTrailer uint64

const InternalKeyTrailerLen = 8

// InternalKey or internal key. Due to the LSM structure, keys are never updated in place, but overwritten with new
// versions. An Internal InternalKey is composed of the user specified key, a sequence number (7 bytes) and a kind (1 byte).
//
//	+-------------+------------+----------+
//	| UserKey (N) | SeqNum (7) | Kind (1) |
//	+-------------+------------+----------+
//
// In the MVCC model, a same key can have multiple versions.
// So the UserKey is composed by "[prefix - actual user key][suffix - MVCC id]"
// To find the prefix of a key, we should implement the comparer.Split()
type InternalKey struct {
	UserKey []byte
	Trailer InternalKeyTrailer
}

func (k *InternalKey) Size() int {
	return len(k.UserKey) + InternalKeyTrailerLen
}

// SerializeTo serialise an internal key into give buffer.
// Caller must ensure buf has enough size to hold
func (k *InternalKey) SerializeTo(buf []byte) {
	i := copy(buf, k.UserKey)
	binary.LittleEndian.PutUint64(buf[i:], uint64(k.Trailer))
}

func (k *InternalKey) Separator(comparer IComparer, other *InternalKey) *InternalKey {
	sep := comparer.Separator(k.UserKey, other.UserKey)
	if len(sep) <= len(k.UserKey) && comparer.Compare(k.UserKey, sep) < 0 {
		nk := MakeKey(sep, SeqNumMax, KeyKindSeparator)
		return &nk
	}
	return k
}

func (k *InternalKey) Compare(comparer IComparer, other *InternalKey) int {
	if c := comparer.Compare(k.UserKey, other.UserKey); c != 0 {
		return c
	}
	// A key with a higher sequence number takes precedence over a key
	// with an equal user key of a lower sequence number.
	return cmp.Compare(other.Trailer, k.Trailer)
}

func (k *InternalKey) Successor(comparer IComparer) *InternalKey {
	succ := comparer.Successor(k.UserKey)
	if len(succ) <= len(k.UserKey) && comparer.Compare(k.UserKey, succ) < 0 {
		nk := MakeKey(succ, SeqNumMax, KeyKindSeparator)
		return &nk
	}
	return k
}

func DeserializeKey(key []byte) *InternalKey {
	n := len(key) - InternalKeyTrailerLen
	if n >= 0 {
		return &InternalKey{
			UserKey: key[:n:n],
			Trailer: InternalKeyTrailer(binary.LittleEndian.Uint64(key[n:])),
		}
	}

	return &InternalKey{
		Trailer: InternalKeyTrailer(KeyKindUnknown),
	}
}

func MakeKey(userKey []byte, num SeqNum, kind KeyKind) InternalKey {
	trailer := InternalKeyTrailer((uint64(num) << 8) | uint64(kind))
	return InternalKey{
		UserKey: userKey,
		Trailer: trailer,
	}
}

func MakeMetaIndexKey(blkKind BlockKind) InternalKey {
	return InternalKey{
		UserKey: []byte{byte(blkKind)},
		Trailer: InternalKeyTrailer(blkKind),
	}
}

func (k *InternalKey) ReadMetaIndexKey() BlockKind {
	return BlockKind(k.Trailer)
}

func (k *InternalKey) SeqNum() SeqNum {
	return SeqNum(k.Trailer >> 8)
}

func (k *InternalKey) KeyKind() KeyKind {
	return KeyKind(k.Trailer & 0xFF) // trailer & (2^8 - 1)
}
