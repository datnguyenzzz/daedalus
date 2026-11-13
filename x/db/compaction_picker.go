package db

import (
	"math"

	"github.com/datnguyenzzz/nogodb/db/manifest"
	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

const (
	// BaseLevel is the Level that L0 will compact to
	BaseLevel         = 1        // TODO(high): PebbleDB dynamically determine this value. Learn from it
	LevelMultiplier   = 10       // TODO(low) Make this configurable
	BaseLevelMaxBytes = 64 << 20 // TODO(low) Make this configurable
)

type CompactionPicker struct {
	v *manifest.Version

	dbSize uint64
	// levelMaxBytes holds the dynamically adjusted max bytes
	// setting for each level.
	levelMaxBytes [manifest.NumLevels]int64
}

// Information about a candidate compaction level that has been identified by
// the compaction picker.
type candidateLevelInfo struct {
	// The fill factor of the level, calculated using file sizes and
	// A factor > 1 means that the level has more data than the ideal
	// size for that level.
	fillFactor float64
	// The score of the level, used to rank levels.
	// If the level doesn't require compaction, the score is 0.
	score float64
	level int
	// The level to compact to.
	outputLevel int
	// The files in level that will be compacted.
	tables *manifest.LevelIterator
}

func NewCompactionPicker(v *manifest.Version) *CompactionPicker {
	// TODO(high): Need to account for the in-progress compactions

	var dbSize uint64
	for l := 1; l < manifest.NumLevels; l++ {
		dbSize += v.Levels[l].AggregateSize()
	}

	if dbSize == 0 {
		// No levels for L1 and up contain any data.
		p := &CompactionPicker{
			v:      v,
			dbSize: dbSize + v.Levels[0].AggregateSize(),
		}
		for i := range manifest.NumLevels {
			p.levelMaxBytes[i] = math.MaxInt64
		}
		return p
	}

	dbSize += v.Levels[0].AggregateSize()

	return &CompactionPicker{
		v:             v,
		dbSize:        dbSize,
		levelMaxBytes: calculateLevelMaxBytes(dbSize),
	}
}

func (p *CompactionPicker) Pick() *compaction {
	scores := p.calculateLevelScores()
	// Check for a score-based compaction for each level, so if we find
	// a level which shouldn't be compacted, we can break early.
	for lvl, levelScore := range scores {
		if levelScore.score == 0 {
			break
		}

		if lvl == manifest.NumLevels-1 {
			break
		}

		// TODO(med): As the flush path from MemTable to L0 is more frequent
		// than lower levels, so we need a special treatment with L0 compaction
		// i.e sub compaction on L0, intra-compaction on L0, ...

		// Pick tables on the level for merging. In RocksDB, they  have a heuristic
		// method kMinOverlappingRatio in the level compaction, seeking to minimize
		// write amplification
		levelScore.tables = p.v.Levels[lvl].Iter(-1, -1)

		if c := p.Construct(&levelScore); c != nil {
			return c
		}
	}

	return nil
}

func (p *CompactionPicker) Construct(cand *candidateLevelInfo) *compaction {
	bound := nogodb_common.UserKeyBound{}
	// Ensure there is no ongoing compaction on the input tables
	for t := cand.tables.First(); t != nil; t = cand.tables.Next() {
		if t.CompactionState == manifest.CompactionStateCompacting {
			return nil
		}

		bound = bound.Union(p.v.Cmp, t.UserKeyBound())
	}

	// TODO(low): Add more levels into a current compaction job, as long as
	// it doesn't exceed the max allowed concurrency

	return newCompaction()
}

// calculateLevelScores calculates the candidateLevelInfo for all levels and
// returns them in decreasing score order. Tables with higher scores, they
// are more rewarding to be compacted to the next level
// TODO(high): Implement Tier-Size Compaction. Once a level is full, a whole
// tables inside that level are merged into the next level
// TODO(high): Need to account for the in-progress compactions
func (p *CompactionPicker) calculateLevelScores() [manifest.NumLevels]candidateLevelInfo {
	panic("implement me")
}

func calculateLevelMaxBytes(dbSize uint64) (levelMaxBytes [manifest.NumLevels]int64) {
	for l := range manifest.NumLevels {
		levelMaxBytes[l] = math.MaxInt64
	}

	lnSize := dbSize - dbSize/LevelMultiplier

	smoothedLevelMultiplier := math.Pow(
		float64(lnSize)/float64(BaseLevelMaxBytes),
		1.0/float64(manifest.NumLevels))

	lSize := float64(BaseLevelMaxBytes)

	for l := 1; l < manifest.NumLevels; l++ {
		roundedLevelSize := math.Round(lSize)
		if roundedLevelSize > float64(math.MaxInt64) {
			levelMaxBytes[l] = math.MaxInt64
		} else {
			levelMaxBytes[l] = int64(roundedLevelSize)
		}
		lSize *= smoothedLevelMultiplier
	}

	return levelMaxBytes
}
