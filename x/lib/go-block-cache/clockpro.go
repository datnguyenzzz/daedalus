package go_block_cache

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

type clockPro struct {
	mu sync.RWMutex

	maxSize int64 // max (hot + cold) memory that the shard can hold
	maxCold int64 // max cold memory that the shard can hold

	sizeHot  int64
	sizeCold int64
	sizeTest int64

	handHot  *log
	handCold *log
	handTest *log
}

// GetInUsed returns the current in-use size of the cache,
func (c *clockPro) GetInUsed() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	inUsed := c.sizeHot + c.sizeCold

	return inUsed
}

// Promote promotes the given node in the cache
// diffSize is the size difference between the new value and the old value of the node
func (c *clockPro) Promote(node *kv, diffSize int64, o op) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch o {
	case opGet:
		node.usedBit = true
	case opSet:
		switch {
		case node.log == nil:
			// new node that is not yet in the cache,
			// move it to the cold node
			node.usedBit = false
			if !c.logAdd(node, cold) {
				return false
			}
			c.sizeCold += diffSize

		case (*log)(atomic.LoadPointer(&node.log)).clockType == test:
			// belongs to the test nodes
			// revive it from test to hot node
			if !c.logDel(node) {
				return false
			}

			node.usedBit = false
			if c.maxCold < c.maxSize {
				c.maxCold += diffSize
			}
			c.sizeTest -= diffSize

			if !c.logAdd(node, hot) {
				return false
			}
			c.sizeHot += diffSize

		default:
			// belongs to the hot or cold nodes
			node.usedBit = true
			l := (*log)(node.log)
			if l.clockType == hot {
				c.sizeHot += diffSize
			} else {
				c.sizeCold += diffSize
			}

			c.evict()
		}
	default:
		panic("unsupported operation")
	}

	return true
}

func (c *clockPro) Evict(node *kv) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node.log == nil {
		return
	}
	clockType := (*log)(atomic.LoadPointer(&node.log)).clockType
	switch clockType {
	case hot:
		c.sizeHot -= node.size
	case cold:
		c.sizeCold -= node.size
	case test:
		c.sizeTest -= node.size
	default:
		panic("unsupported clock type")
	}
	c.logDel(node)
}

func (c *clockPro) SetCapacity(capacity int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.maxSize = capacity
	c.evict()
}

// logAdd adds new log after the handHot
// and evicts data if needed
func (c *clockPro) logAdd(node *kv, ct clockType) bool {
	c.evict()
	l := &log{n: node, clockType: ct}
	l.next = l
	l.prev = l
	if c.handHot == nil {
		c.handHot = l
		c.handCold = l
		c.handTest = l
	} else {
		c.handHot.insert(l)
	}

	if c.handCold == c.handHot {
		c.handCold = c.handCold.prev
	}

	node.log = unsafe.Pointer(l)

	return true
}

func (c *clockPro) logDel(node *kv) bool {
	l := (*log)(node.log)
	node.log = nil

	switch l {
	case c.handHot:
		c.handHot = c.handHot.prev
	case c.handCold:
		c.handCold = c.handCold.prev
	case c.handTest:
		c.handTest = c.handTest.prev
	}

	l.remove()
	if c.handHot.next == nil || c.handHot.prev == nil {
		c.handHot = nil
		c.handCold = nil
		c.handTest = nil
	}

	return true
}

func (c *clockPro) evict() {
	for c.handCold != nil && c.sizeHot+c.sizeCold > c.maxSize {
		c.runHandCold()
	}
}

func (c *clockPro) runHandCold() {
	if c.handCold.clockType == cold {
		n := c.handCold.n
		ref := n.usedBit
		c.sizeCold -= n.size

		if ref {
			// move to hot
			n.usedBit = false
			c.handCold.clockType = hot
			c.sizeHot += n.size
		} else {
			// move to test and de-allocate the node's value
			// but still keep its meta
			//
			// Hacky: Keep the current node's size, so when the test node
			// get removed, we can still calculate the size correctly
			n.SetValue(nil, n.size)
			c.handCold.clockType = test
			c.sizeTest += n.size
			for c.handTest != nil && c.sizeTest > c.maxSize {
				c.runHandTest()
			}
		}
	}

	c.handCold = c.handCold.next

	for c.handHot != nil && c.maxCold+c.sizeHot > c.maxSize {
		c.runHandHot()
	}
}

func (c *clockPro) runHandHot() {
	if c.handHot == c.handTest && c.handTest != nil {
		c.runHandTest()
		if c.handHot == nil {
			return
		}
	}

	n := c.handHot.n
	if c.handHot.clockType == hot {
		ref := n.usedBit
		if ref {
			n.usedBit = false
		} else {
			// move to cold
			c.handHot.clockType = cold
			c.sizeHot -= n.size
			c.sizeCold += n.size
		}
	}

	c.handHot = c.handHot.next
}

func (c *clockPro) runHandTest() {
	if c.handTest == c.handCold && c.handCold != nil {
		c.runHandCold()
		if c.handTest == nil {
			return
		}
	}

	n := c.handTest.n
	if c.handTest.clockType == test {
		// remove the node entirely
		c.maxCold -= n.size
		c.maxCold = max(c.maxCold, 0)

		c.mu.Unlock()
		n.s.mu.Lock()
		n.s.evict(n)
		n.s.mu.Unlock()
		c.mu.Lock()
	}

	c.handTest = c.handTest.next
}

func NewClockPro(maxSize int64) *clockPro {
	c := &clockPro{}
	c.SetCapacity(maxSize)
	return c
}

var _ ICacher = (*clockPro)(nil)
