package common

import "math"

// IFlushDecider is used to decide when to flush a block
type IFlushDecider interface {
	// ShouldFlush returns true if we should flush the current block of sizeBefore
	// instead of adding another K/V that would increase the block to sizeAfter.
	ShouldFlush(sizeBefore, sizeAfter int) bool
}

type decider struct {
	lowWatermark  int
	highWatermark int
}

// ShouldFlush Criteria:
//
//	We flush right before the block would exceed targetBlockSize.
//	If block size is smaller than blockSizeThreshold percent of the target
//	we flush right after the target block size is exceeded.
//
// TODO(low) - Room for optimisation
//
//	We should flush as nearest the boundaries [lowWatermark, highWatermark) as possible to
//	minimize wasted memory space in the block cache
func (d decider) ShouldFlush(sizeBefore, sizeAfter int) bool {
	// We always add another K/V to a block if its initial size is below
	// lowWatermark, even if the block is very large after adding the KV. This is
	// a safeguard to avoid very small blocks in the presence of large KVs.
	if sizeBefore < d.lowWatermark {
		return false
	}

	// We never add another K/V to a block if its existing size exceeds
	// highWatermark (unless its initial size is < lowWatermark).
	if sizeAfter > d.highWatermark {
		return true
	}

	return false
}

// NewFlushDecider create a controller to decide when a block should be flushed
func NewFlushDecider(targetBlockSize int, blockSizeThreshold float32) IFlushDecider {
	return &decider{
		lowWatermark:  int(math.Ceil(float64(float32(targetBlockSize) * blockSizeThreshold))),
		highWatermark: targetBlockSize,
	}
}

var _ IFlushDecider = (*decider)(nil)
