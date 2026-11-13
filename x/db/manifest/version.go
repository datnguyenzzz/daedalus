package manifest

import (
	"sync"
	"sync/atomic"

	nogodb_common "github.com/datnguyenzzz/nogodb/lib/common"
)

// NumLevels is the number of levels a Version contains.
const NumLevels = 7

// Version is a collection of SStable metadata for on-disk tables at various
// levels. Memtables are written to level-0 tables, and compactions migrate
// data from level N to level N+1.
type Version struct {
	Cmp  nogodb_common.IComparer
	refs atomic.Uint32
	// levels contains metadata for all of the tables within a level of the LSM.
	Levels     [NumLevels]*levelMetadata
	list       *VersionList
	prev, next *Version
}

func NewVersion(
	comparer nogodb_common.IComparer,
) *Version {
	v := &Version{
		Cmp: comparer,
	}
	for i := range NumLevels {
		v.Levels[i] = NewLevelMetadata(i)
	}
	return v
}

// The versions are ordered from oldest to newest.
type VersionList struct {
	mu   *sync.Mutex
	root Version
}

func (l *VersionList) Init(mu *sync.Mutex) {
	l.mu = mu
	l.root.next = &l.root
	l.root.prev = &l.root
}

// pushBack adds a _new_ version to the back of the list
func (l *VersionList) PushBack(v *Version) {
	if v.refs.Load() > 0 {
		panic("VersionSet tries appending a referenced version")
	}
	if v.list != nil || v.next != nil || v.prev != nil {
		panic("versionList.pushBack tried adding an old version ")
	}

	tmp := l.root.prev
	l.root.prev = v
	tmp.next = v
	v.next = &l.root
	v.prev = tmp
	v.list = l
}

func (l *VersionList) Back() *Version {
	return l.root.prev
}
