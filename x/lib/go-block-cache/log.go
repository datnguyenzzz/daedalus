package go_block_cache

import "go.uber.org/zap"

type clockType byte

const (
	unknown clockType = iota
	hot
	cold
	test
)

type log struct {
	n          *kv
	prev, next *log
	clockType  clockType
}

func (l *log) remove() {
	if l.prev == nil || l.next == nil {
		msg := "evict a zombie node"
		zap.L().Error(msg)
		panic(msg)
	}
	l.prev.next = l.next
	l.next.prev = l.prev
	l.prev = nil
	l.next = nil
}

// insert linkage: l <--> another <--> l.next
func (l *log) insert(another *log) {
	tmp := l.next
	l.next = another
	another.prev = l
	another.next = tmp
	tmp.prev = another
}
