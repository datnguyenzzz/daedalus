package common

type BoundaryKind uint8

// The two possible values of BoundaryKind.
//
// Note that we prefer Exclusive to be the zero value, so that zero
// UserKeyBounds are not valid.
const (
	Exclusive BoundaryKind = iota
	Inclusive
)

// UserKeyBound is a user key interval with an inclusive start boundary and
// with an end boundary that can be either inclusive or exclusive.
type UserKeyBound struct {
	Start []byte
	End   UserKeyBoundary
}

// UserKeyBoundary represents the endpoint of a bound which can be exclusive or
// inclusive.
type UserKeyBoundary struct {
	Key  []byte
	Kind BoundaryKind
}

func (eb *UserKeyBoundary) Compare(cmp IComparer, other UserKeyBoundary) int {
	switch c := cmp.Compare(eb.Key, other.Key); {
	case c != 0:
		return c
	case eb.Kind == other.Kind:
		return 0
	case eb.Kind == Inclusive:
		// eb is inclusive, other is exclusive.
		return 1
	default:
		// eb is exclusive, other is inclusive.
		return -1
	}
}

func (u *UserKeyBound) Union(cmp IComparer, other UserKeyBound) UserKeyBound {
	if len(u.Start) == 0 || len(u.End.Key) == 0 {
		return other
	}

	union := *u
	if cmp.Compare(union.Start, other.Start) > 0 {
		union.Start = other.Start
	}

	if union.End.Compare(cmp, other.End) < 0 {
		union.End = other.End
	}

	return union
}
