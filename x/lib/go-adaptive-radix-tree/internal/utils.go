package internal

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

func findLCP(key1 []byte, key2 []byte, offset uint8) uint8 {
	i := offset
	for ; int(i) < min(len(key1), len(key2)); i++ {
		if key1[i] != key2[i] {
			break
		}
	}

	return i - offset
}

func isExactMatch(key1 []byte, key2 []byte) bool {
	return len(key1) == len(key2) && bytes.Equal(key1, key2)
}

func Goid() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}
