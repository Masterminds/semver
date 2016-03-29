package semver

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var noneErr = errors.New("The 'None' constraint admits no versions.")

type Constraint interface {
	// Constraints compose the fmt.Stringer interface. Printing a constraint
	// will yield a string that, if passed to NewConstraint(), will produce the
	// original constraint. (Bidirectional serialization)
	fmt.Stringer

	// Admits checks that a version satisfies the constraint. If it does not,
	// an error is returned indcating the problem; if it does, the error is nil.
	Admits(v *Version) error

	// Intersect computes the intersection between the receiving Constraint and
	// passed Constraint, and returns a new Constraint representing the result.
	Intersect(Constraint) Constraint

	// Union computes the union between the receiving Constraint and the passed
	// Constraint, and returns a new Constraint representing the result.
	Union(Constraint) Constraint

	// AdmitsAny returns a bool indicating whether there exists any version that
	// satisfies both the receiver constraint, and the passed Constraint.
	//
	// In other words, this reports whether an intersection would be non-empty.
	AdmitsAny(Constraint) bool

	// Restrict implementation of this interface to this package. We need the
	// flexibility of an interface, but we cover all possibilities here; closing
	// off the interface to external implementation lets us safely do tricks
	// with types for magic types (none and any)
	_private()
}

// realConstraint is used internally to differentiate between any, none, and
// unionConstraints, vs. Version and rangeConstraints.
type realConstraint interface {
	Constraint
	_real()
}

// Any is a constraint that is satisfied by any valid semantic version.
type any struct{}

// Any creates a constraint that will match any version.
func Any() Constraint {
	return any{}
}

func (any) String() string {
	return "*"
}

// Admits checks that a version satisfies the constraint. As all versions
// satisfy Any, this always returns nil.
func (any) Admits(v *Version) error {
	return nil
}

// Intersect computes the intersection between two constraints.
//
// As Any is the set of all possible versions, any intersection with that
// infinite set will necessarily be the entirety of the second set. Thus, this
// simply returns the passed constraint.
func (any) Intersect(c Constraint) Constraint {
	return c
}

// AdmitsAny indicates whether there exists any version that can satisfy both
// this constraint, and the passed constraint. As all versions
// satisfy Any, this is always true - unless none is passed.
func (any) AdmitsAny(c Constraint) bool {
	if _, ok := c.(none); ok {
		return false
	}
	return true
}

func (any) Union(c Constraint) Constraint {
	return Any()
}

func (any) _private() {}

// None is an unsatisfiable constraint - it represents the empty set.
type none struct{}

// None creates a constraint that matches no versions (the empty set).
func None() Constraint {
	return none{}
}

func (none) String() string {
	return ""
}

// Admits checks that a version satisfies the constraint. As no version can
// satisfy None, this always fails (returns an error).
func (none) Admits(v *Version) error {
	return noneErr
}

// Intersect computes the intersection between two constraints.
//
// None is the empty set of versions, and any intersection with the empty set is
// necessarily the empty set. Thus, this always returns None.
func (none) Intersect(Constraint) Constraint {
	return None()
}

func (none) Union(c Constraint) Constraint {
	return c
}

// AdmitsAny indicates whether there exists any version that can satisfy the
// constraint. As no versions satisfy None, this is always false.
func (none) AdmitsAny(c Constraint) bool {
	return false
}

func (none) _private() {}

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
		nr := rc.dup()

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
				} else {
					info |= lminlt
					nc.min = rc.min
				}
			} else if oc.min != nil {
				info |= lminlt
				nc.min = rc.min
			}

			// Pick the max
			if rc.max != nil {
				if oc.max == nil || rc.max.LessThan(oc.max) || (rc.max.Equal(oc.max) && !rc.includeMax && oc.includeMax) {
					info |= rmaxgt
					nc.max = oc.max
				} else {
					info |= lmaxgt
					nc.max = rc.max
				}
			} else if oc.max != nil {
				info |= lmaxgt
				nc.max = rc.max
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
			return unionConstraint{rc, oc}
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

func (rangeConstraint) _real() {}

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

func areEq(v1, v2 *Version) bool {
	if v1 == nil && v2 == nil {
		return true
	}

	if v1 != nil && v2 != nil {
		return v1.Equal(v2)
	}
	return false
}

func (rc rangeConstraint) AdmitsAny(c Constraint) bool {
	if _, ok := rc.Intersect(c).(none); ok {
		return false
	}
	return true
}

func (rangeConstraint) _private() {}

type unionConstraint []realConstraint

func (uc unionConstraint) Admits(v *Version) error {
	var err error
	for _, c := range uc {
		if err = c.Admits(v); err == nil {
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

func (uc unionConstraint) AdmitsAny(c Constraint) bool {
	for _, ic := range uc {
		if ic.AdmitsAny(c) {
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

// Intersection computes the intersection between N Constraints, returning as
// compact a representation of the intersection as possible.
//
// No error is indicated if all the sets are collectively disjoint; you must inspect the
// return value to see if the result is the empty set (indicated by both
// IsMagic() being true, and AdmitsAny() being false).
func Intersection(cg ...Constraint) Constraint {
	// If there's zero or one constraints in the group, we can quit fast
	switch len(cg) {
	case 0:
		// Zero members, only sane thing to do is return none
		return None()
	case 1:
		// Just one member means that's our final constraint
		return cg[0]
	}

	// Preliminary first pass to look for a none (that would supercede everything
	// else), and also construct a []realConstraint for everything else
	var real constraintList

	for _, c := range cg {
		switch tc := c.(type) {
		case any:
			continue
		case none:
			return c
		case *Version:
			real = append(real, tc)
		case rangeConstraint:
			real = append(real, tc)
		case unionConstraint:
			real = append(real, tc...)
		default:
			panic("unknown constraint type")
		}
	}

	sort.Sort(real)

	// Now we know there's no easy wins, so step through and intersect each with
	// the previous
	car, cdr := cg[0], cg[1:]
	for _, c := range cdr {
		car = car.Intersect(c)
		if IsNone(car) {
			return None()
		}
	}

	return car
}

// Union takes a variable number of constraints, and returns the most compact
// possible representation of those constraints.
//
// This effectively ORs together all the provided constraints. If any of the
// included constraints are the set of all versions (any), that supercedes
// everything else.
func Union(cg ...Constraint) Constraint {
	// If there's zero or one constraints in the group, we can quit fast
	switch len(cg) {
	case 0:
		// Zero members, only sane thing to do is return none
		return None()
	case 1:
		// One member, so the result will just be that
		return cg[0]
	}

	// Preliminary pass to look for 'any' in the current set (and bail out early
	// if found), but also construct a []realConstraint for everything else
	var real constraintList

	for _, c := range cg {
		switch tc := c.(type) {
		case any:
			return c
		case none:
			continue
		case *Version:
			real = append(real, tc)
		case rangeConstraint:
			real = append(real, tc)
		case unionConstraint:
			real = append(real, tc...)
		default:
			panic("unknown constraint type")
		}
	}

	// Sort both the versions and ranges into ascending order
	sort.Sort(real)

	// Iteratively merge the constraintList elements
	var nuc unionConstraint
	for _, c := range real {
		if len(nuc) == 0 {
			nuc = append(nuc, c)
			continue
		}

		last := nuc[len(nuc)-1]
		if last.AdmitsAny(c) || areAdjacent(last, c) {
			nuc[len(nuc)-1] = last.Union(c).(realConstraint)
		} else {
			nuc = append(nuc, c)
		}
	}

	if len(nuc) == 1 {
		return nuc[0]
	}
	return nuc
}

type ascendingRanges []rangeConstraint

func (rs ascendingRanges) Len() int {
	return len(rs)
}

func (rs ascendingRanges) Less(i, j int) bool {
	ir, jr := rs[i].max, rs[j].max
	inil, jnil := ir == nil, jr == nil

	if !inil && !jnil {
		if ir.LessThan(jr) {
			return true
		}
		if jr.LessThan(ir) {
			return false
		}

		// Last possible - if i is inclusive, but j isn't, then put i after j
		if !rs[j].includeMax && rs[i].includeMax {
			return false
		}

		// Or, if j inclusive, but i isn't...but actually, since we can't return
		// 0 on this comparator, this handles both that and the 'stable' case
		return true
	} else if inil || jnil {
		// ascending, so, if jnil, then j has no max but i does, so i should
		// come first. thus, return jnil
		return jnil
	}

	// neither have maxes, so now go by the lowest min
	ir, jr = rs[i].min, rs[j].min
	inil, jnil = ir == nil, jr == nil

	if !inil && !jnil {
		if ir.LessThan(jr) {
			return true
		}
		if jr.LessThan(ir) {
			return false
		}

		// Last possible - if j is inclusive, but i isn't, then put i after j
		if rs[j].includeMin && !rs[i].includeMin {
			return false
		}

		// Or, if i inclusive, but j isn't...but actually, since we can't return
		// 0 on this comparator, this handles both that and the 'stable' case
		return true
	} else if inil || jnil {
		// ascending, so, if inil, then i has no min but j does, so j should
		// come first. thus, return inil
		return inil
	}

	// Default to keeping i before j
	return true
}

func (rs ascendingRanges) Swap(i, j int) {
	rs[i], rs[j] = rs[j], rs[i]
}

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

// IsNone indicates if a constraint will match no versions - that is, the
// constraint represents the empty set.
func IsNone(c Constraint) bool {
	_, ok := c.(none)
	return ok
}

// IsAny indicates if a constraint will match any and all versions.
func IsAny(c Constraint) bool {
	_, ok := c.(none)
	return ok
}
