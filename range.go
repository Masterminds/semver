package semver

import (
	"fmt"
	"sort"
	"strings"
)

type rangeConstraint struct {
	min, max               *Version
	includeMin, includeMax bool
	excl                   []*Version
}

func (rc rangeConstraint) Admits(v *Version) error {
	var fail bool
	var emsg string
	if rc.min != nil {
		// TODO ensure sane handling of prerelease versions (which are strictly
		// less than the normal version, but should be admitted in a geq range)
		cmp := rc.min.Compare(v)
		if rc.includeMin {
			emsg = "%s is less than %s"
			fail = cmp == 1
		} else {
			emsg = "%s is less than or equal to %s"
			fail = cmp != -1
		}

		if fail {
			return fmt.Errorf(emsg, v, rc.min.String())
		}
	}

	if rc.max != nil {
		// TODO ensure sane handling of prerelease versions (which are strictly
		// less than the normal version, but should be admitted in a geq range)
		cmp := rc.max.Compare(v)
		if rc.includeMax {
			emsg = "%s is greater than %s"
			fail = cmp == -1
		} else {
			emsg = "%s is greater than or equal to %s"
			fail = cmp != 1
		}

		if fail {
			return fmt.Errorf(emsg, v, rc.max.String())
		}
	}

	for _, excl := range rc.excl {
		if excl.Equal(v) {
			return fmt.Errorf("Version %s is specifically disallowed.", v.String())
		}
	}

	return nil
}

func (rc rangeConstraint) dup() rangeConstraint {
	var excl []*Version

	if len(rc.excl) > 0 {
		excl = make([]*Version, len(rc.excl))
		copy(excl, rc.excl)
	}

	return rangeConstraint{
		min:        rc.min,
		max:        rc.max,
		includeMin: rc.includeMin,
		includeMax: rc.includeMax,
		excl:       excl,
	}
}

func (rc rangeConstraint) Intersect(c Constraint) Constraint {
	switch oc := c.(type) {
	case any:
		return rc
	case none:
		return None()
	case unionConstraint:
		return oc.Intersect(rc)
	case *Version:
		if err := rc.Admits; err != nil {
			return None()
		} else {
			return c
		}
	case rangeConstraint:
		nr := rangeConstraint{
			min:        rc.min,
			max:        rc.max,
			includeMin: rc.includeMin,
			includeMax: rc.includeMax,
		}

		if oc.min != nil {
			if nr.min == nil || nr.min.LessThan(oc.min) {
				nr.min = oc.min
				nr.includeMin = oc.includeMin
			} else if oc.min.Equal(nr.min) && !oc.includeMin {
				// intersection means we must follow the least inclusive
				nr.includeMin = false
			}
		}

		if oc.max != nil {
			if nr.max == nil || nr.max.GreaterThan(oc.max) {
				nr.max = oc.max
				nr.includeMax = oc.includeMax
			} else if oc.max.Equal(nr.max) && !oc.includeMax {
				// intersection means we must follow the least inclusive
				nr.includeMax = false
			}
		}

		// Ensure any applicable excls from oc are included in nc
		for _, e := range append(rc.excl, oc.excl...) {
			if nr.Admits(e) == nil {
				nr.excl = append(nr.excl, e)
			}
		}

		if nr.min == nil && nr.max == nil {
			return nr
		}

		// TODO could still have nils?
		if nr.min.Equal(nr.max) {
			// min and max are equal. if range is inclusive, return that
			// version; otherwise, none
			if nr.includeMin && nr.includeMax {
				return nr.min
			}
			return None()
		}

		if nr.min != nil && nr.max != nil && nr.min.GreaterThan(nr.max) {
			// min is greater than max - not possible, so we return none
			return None()
		}

		// range now fully validated, return what we have
		return nr

	default:
		panic("unknown type")
	}
}

func (rc rangeConstraint) Union(c Constraint) Constraint {
	switch oc := c.(type) {
	case any:
		return Any()
	case none:
		return rc
	case unionConstraint:
		return oc.Union(rc)
	case *Version:
		if err := rc.Admits(oc); err == nil {
			return rc
		} else if len(rc.excl) > 0 { // TODO (re)checking like this is wasteful
			// ensure we don't have an excl-specific mismatch; if we do, remove
			// it and return that
			for k, e := range rc.excl {
				if e.Equal(oc) {
					excl := make([]*Version, len(rc.excl)-1)

					if k == len(rc.excl)-1 {
						copy(excl, rc.excl[:k])
					} else {
						copy(excl, append(rc.excl[:k], rc.excl[k+1:]...))
					}

					return rangeConstraint{
						min:        rc.min,
						max:        rc.max,
						includeMin: true,
						includeMax: rc.includeMax,
						excl:       excl,
					}
				}
			}
		}

		if oc.Equal(rc.min) {
			ret := rc.dup()
			ret.includeMin = true
			return ret
		}
	case rangeConstraint:
		if areAdjacent(rc, oc) {
			// Receiver adjoins the input from below
			nc := rc.dup()

			nc.max = oc.max
			nc.includeMax = oc.includeMax
			nc.excl = append(nc.excl, oc.excl...)

			return nc
		} else if areAdjacent(oc, rc) {
			// Input adjoins the receiver from below
			nc := oc.dup()

			nc.max = rc.max
			nc.includeMax = rc.includeMax
			nc.excl = append(nc.excl, rc.excl...)

			return nc

		} else if rc.AdmitsAny(oc) {
			// Receiver and input overlap; form a new range accordingly.
			nc := rangeConstraint{}

			// For efficiency, we simultaneously determine if either of the
			// ranges are supersets of the other, while also selecting the min
			// and max of the new range
			var info uint8

			const (
				lminlt uint8             = 1 << iota // left (rc) min less than right
				rminlt                               // right (oc) min less than left
				lmaxgt                               // left max greater than right
				rmaxgt                               // right max greater than left
				lsupr  = lminlt | lmaxgt             // left is superset of right
				rsupl  = rminlt | rmaxgt             // right is superset of left
			)

			// Pick the min
			if rc.min != nil {
				if oc.min == nil || rc.min.GreaterThan(oc.min) || (rc.min.Equal(oc.min) && !rc.includeMin && oc.includeMin) {
					info |= rminlt
					nc.min = oc.min
					nc.includeMin = oc.includeMin
				} else {
					info |= lminlt
					nc.min = rc.min
					nc.includeMin = rc.includeMin
				}
			} else if oc.min != nil {
				info |= lminlt
				nc.min = rc.min
				nc.includeMin = rc.includeMin
			}

			// Pick the max
			if rc.max != nil {
				if oc.max == nil || rc.max.LessThan(oc.max) || (rc.max.Equal(oc.max) && !rc.includeMax && oc.includeMax) {
					info |= rmaxgt
					nc.max = oc.max
					nc.includeMax = oc.includeMax
				} else {
					info |= lmaxgt
					nc.max = rc.max
					nc.includeMax = rc.includeMax
				}
			} else if oc.max != nil {
				info |= lmaxgt
				nc.max = rc.max
				nc.includeMax = rc.includeMax
			}

			// Reincorporate any excluded versions
			if info&lsupr != lsupr {
				// rc is not superset of oc, so must walk oc.excl
				for _, e := range oc.excl {
					if rc.Admits(e) != nil {
						nc.excl = append(nc.excl, e)
					}
				}
			}

			if info&rsupl != rsupl {
				// oc is not superset of rc, so must walk rc.excl
				for _, e := range rc.excl {
					if oc.Admits(e) != nil {
						nc.excl = append(nc.excl, e)
					}
				}
			}

			return nc
		} else {
			// Don't call Union() here b/c it would duplicate work
			uc := constraintList{rc, oc}
			sort.Sort(uc)
			return unionConstraint(uc)
		}
	}

	panic("unknown type")
}

func (rc rangeConstraint) isSupersetOf(rc2 rangeConstraint) bool {
	if rc.min != nil {
		if rc2.min == nil || rc.min.GreaterThan(rc2.min) || (rc.min.Equal(rc2.min) && !rc.includeMin && rc2.includeMin) {
			return false
		}
	}

	if rc.max != nil {
		if rc2.max == nil || rc.max.LessThan(rc2.max) || (rc.max.Equal(rc2.max) && !rc.includeMax && rc2.includeMax) {
			return false
		}
	}

	return true
}

func (rc rangeConstraint) String() string {
	// TODO express using caret or tilde, where applicable
	var pieces []string
	if rc.min != nil {
		if rc.includeMin {
			pieces = append(pieces, fmt.Sprintf(">= %s", rc.min))
		} else {
			pieces = append(pieces, fmt.Sprintf("> %s", rc.min))
		}
	}

	if rc.max != nil {
		if rc.includeMax {
			pieces = append(pieces, fmt.Sprintf("<= %s", rc.max))
		} else {
			pieces = append(pieces, fmt.Sprintf("< %s", rc.max))
		}
	}

	for _, e := range rc.excl {
		pieces = append(pieces, fmt.Sprintf("!=%s", e))
	}

	return strings.Join(pieces, ", ")
}

// areAdjacent tests two constraints to determine if they are adjacent,
// but non-overlapping.
//
// If either constraint is not a range, returns false. We still allow it at the
// type level, however, to make the check convenient elsewhere.
//
// Assumes the first range is less than the second; it is incumbent on the
// caller to arrange the inputs appropriately.
func areAdjacent(c1, c2 Constraint) bool {
	var rc1, rc2 rangeConstraint
	var ok bool
	if rc1, ok = c1.(rangeConstraint); !ok {
		return false
	}
	if rc2, ok = c2.(rangeConstraint); !ok {
		return false
	}

	if !areEq(rc1.max, rc2.min) {
		return false
	}

	return (rc1.includeMax && !rc2.includeMin) ||
		(!rc1.includeMax && rc2.includeMin)
}

func (rc rangeConstraint) AdmitsAny(c Constraint) bool {
	if _, ok := rc.Intersect(c).(none); ok {
		return false
	}
	return true
}

func (rangeConstraint) _private() {}
func (rangeConstraint) _real()    {}
