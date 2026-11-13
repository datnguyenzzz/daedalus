//go:build functional_tests

package functional

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"time"

	"github.com/datnguyenzzz/nogodb/lib/common"
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
	"github.com/go-faker/faker/v4"
)

const (
	suffixLen = 5
)

type mvccComparer struct {
	nogodb_common.DefaultComparer
}

func NewMvccComparer() *mvccComparer {
	return &mvccComparer{
		DefaultComparer: *nogodb_common.NewComparer(),
	}
}

func (c *mvccComparer) Compare(a, b []byte) int {
	prefixA, prefixB := c.Split(a), c.Split(b)
	if cp := c.DefaultComparer.Compare(a[:prefixA], b[:prefixB]); cp != 0 {
		return cp
	}

	return c.DefaultComparer.Compare(a[prefixA:], b[prefixB:])
}

func (c *mvccComparer) Separator(a, b []byte) []byte {
	return a
}

func (c *mvccComparer) Successor(b []byte) []byte {
	return b
}

func (c *mvccComparer) Split(b []byte) int {
	return len(b) - suffixLen
}

var _ common.IComparer = (*mvccComparer)(nil)

type kvType struct {
	key   []byte
	value []byte
}

func generateBytes(lo, hi []byte) []byte {
	if len(lo) == 0 {
		return []byte{0}
	}
	res := make([]byte, len(lo))
	copy(res, lo)

	for i := len(res) - 1; i >= 0; i-- {
		if res[i] < byte(255) {
			res[i] += 1
			for j := i + 1; j < len(res); j++ {
				res[j] = 0
			}
			break
		}
	}
	return res
}

// generateKV Generate list of kvType in an increasing order of key
// with suffix in an increasing order
func generateKVWithSuffix(size int, isUnique bool) []kvType {
	res := generateKV(size, isUnique)

	for i := 0; i < len(res); {
		j := i
		for ; j < len(res) && bytes.Equal(res[i].key, res[j].key); j++ {
		}
		for k := i; k < j; k++ {
			suffix := make([]byte, suffixLen)
			binary.BigEndian.PutUint32(suffix, uint32(k-i))
			res[k].key = slices.Concat(res[k].key, suffix)
		}

		i = j
	}

	return res
}

// generateKV Generate list of kvType in an increasing order of key
func generateKV(size int, isUnique bool) []kvType {
	res := make([]kvType, 0, size)

	// generate a list of key–value pairs such that adjacent keys share some common bytes.
	for i := range size {
		res = append(res, kvType{[]byte(randomQuote()), []byte(randomQuote())})
		if i == 0 {
			continue
		}

		if !isUnique {
			res[i].key = generateKeyFromAnotherKey(res[i-1].key)
		}
	}

	sort.Slice(res, func(i, j int) bool {
		return bytes.Compare(res[i].key, res[j].key) < 0
	})

	return res
}

// generateKeyFromAnotherKey generate a random key that share some first bytes of the given key
func generateKeyFromAnotherKey(key []byte) []byte {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	mutualLength := r.Intn(len(key)-1) + 1

	randomBytes := []byte(randomQuote())

	newKey := make([]byte, mutualLength+len(randomBytes))
	copy(newKey, key)
	copy(newKey[mutualLength:], randomBytes)

	return newKey
}

func randomQuote() string {
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
