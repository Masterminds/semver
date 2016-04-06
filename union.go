package semver

import "strings"

type unionConstraint []realConstraint

func (uc unionConstraint) Matches(v *Version) error {
	var err error
	for _, c := range uc {
		if err = c.Matches(v); err == nil {
			return nil
		}
	}

	// FIXME lollol, returning the last error is just laughably wrong
	return err
}

func (uc unionConstraint) Intersect(c2 Constraint) Constraint {
	var other []realConstraint

	switch tc2 := c2.(type) {
	case none:
		return None()
	case any:
		return uc
	case *Version:
		return c2
	case rangeConstraint:
		other = append(other, tc2)
	case unionConstraint:
		other = c2.(unionConstraint)
	default:
		panic("unknown type")
	}

	var newc []Constraint
	// TODO dart has a smarter loop, i guess, but i don't grok it yet, so for
	// now just do NxN
	for _, c := range uc {
		for _, oc := range other {
			i := c.Intersect(oc)
			if !IsNone(i) {
				newc = append(newc, i)
			}
		}
	}

	return Union(newc...)
}

func (uc unionConstraint) MatchesAny(c Constraint) bool {
	for _, ic := range uc {
		if ic.MatchesAny(c) {
			return true
		}
	}
	return false
}

func (uc unionConstraint) Union(c Constraint) Constraint {
	return Union(uc, c)
}

func (uc unionConstraint) String() string {
	var pieces []string
	for _, c := range uc {
		pieces = append(pieces, c.String())
	}

	return strings.Join(pieces, " || ")
}
func (unionConstraint) _private() {}

type constraintList []realConstraint

func (cl constraintList) Len() int {
	return len(cl)
}

func (cl constraintList) Swap(i, j int) {
	cl[i], cl[j] = cl[j], cl[i]
}

func (cl constraintList) Less(i, j int) bool {
	ic, jc := cl[i], cl[j]

	switch tic := ic.(type) {
	case *Version:
		switch tjc := jc.(type) {
		case *Version:
			return tic.LessThan(tjc)
		case rangeConstraint:
			if tjc.min == nil {
				return false
			}
			return tic.LessThan(tjc.min)
		}
	case rangeConstraint:
		switch tjc := jc.(type) {
		case *Version:
			if tic.min == nil {
				return true
			}
			return tic.min.LessThan(tjc)
		case rangeConstraint:
			if tic.min == nil {
				return true
			}
			if tjc.min == nil {
				return false
			}
			return tic.min.LessThan(tjc.min)
		}
	}

	panic("unreachable")
}
