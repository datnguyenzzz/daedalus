package db

import (
	"sync"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

// Inspired by PebbleDB
// The commit pipeline is the component which manages the steps in
// committing write batches, e.g writing the batch to the WAL and
// applying its contents to the memtable.
//
// If concurrent performance was not a concern, the commit pipeline could
// simply be a mutex which synchnously writes to the WAL and application
// of the batch records to the memtable. But we want to challenge ourself here :)
//
// The primary challenge in concurrent performance in the commit pipeline
// is maintaining 2 invariants:
// 1. Batches need to be written to the WAL in sequence number order.
// 2. Batches need to be made visible for reads in sequence number
//    order. This invariant arises from the use of a single sequence
//    number which indicates which mutations are visible.
//
// So we have four steps in committing a write batch:
// 1. Write the batch to the WAL
// 2. Apply the mutations in the batch to the memtable
// 3. Bump the visible sequence number
// 4. Sync the WAL
//
// Writing the batch to the WAL is actually very fast as it is just a
// memory copy. Applying the mutations in the batch to the memtable is by
// far the most CPU intensive part of the commit pipeline. Syncing the
// WAL is the most expensive from a wall clock perspective.
//
// The commit pipeline looks like:
// 1. Lock commit mutex
//   * Add batch to commit queue
//   * Assign batch sequence number
//   * Write batch to the WAL
// 2. Unlock commit mutex
// 3. Apply batch to memtable (concurrently)
// 4. Publish batch sequence number

type commit struct {
	mu sync.Mutex

	// The next sequence number to give to a batch.
	nextSeqNum nogodb_common.SeqNum
	// The visible sequence number at which reads should be performed. Ratcheted
	// upwards atomically as batches are applied to the memtable.
	visibleSeqNum nogodb_common.SeqNum
}

func (c *commit) Commit(b *Batch) error {
	// Implement a slow way for now to build the e2e flow first
	c.mu.Lock()
	defer c.mu.Unlock()

	b.SetSeqNumToHeader(uint64(c.nextSeqNum))
	b.SetCountToHeader()

	c.nextSeqNum += nogodb_common.SeqNum(b.Count())

	if err := c.writeToWal(b); err != nil {
		return err
	}

	// apply the mutations from batch to the memtable
	if err := c.applyToMem(b); err != nil {
		return err
	}

	// publish batch sequence number
	b.applied.Store(true)
	// because we synchnously writes to the WAL then apply
	// the batch records to the memtable. So the request batch
	// is considered as finished wholly already here
	newSeqNum := b.SeqNum() + nogodb_common.SeqNum(b.Count())
	c.visibleSeqNum = newSeqNum

	return nil
}

func (c *commit) applyToMem(b *Batch) error {
	// the current mutable memtable has been prepared for applying
	// such as prevent it being flushed, etc. in the writeToWal() step
	mem := b.db.mu.mem.mutable

	if err := mem.apply(b, b.SeqNum()); err != nil {
		return err
	}

	if mem.writerUnref() {
		// we don't want to flush a memTable
		// when it is still being ref-ed
		b.db.mu.Lock()
		b.db.maybeScheduleFlush()
		b.db.mu.Unlock()
	}

	return nil
}

func (c *commit) writeToWal(b *Batch) error {
	mem := b.db.mu.mem.mutable
	mem.prepare(b)

	_, err := b.db.mu.log.writer.Write(b.buf)
	if err != nil {
		return err
	}

	return nil
}
