package semver

import (
	"errors"
	"fmt"
)

var noneErr = errors.New("The 'None' constraint admits no versions.")

type Constraint interface {
	// Admits checks that a version satisfies the constraint. If it does not,
	// an error is returned indcating the problem; if it does, the error is nil.
	Admits(v *Version) error

	// Intersect computes the intersection between the receiving Constraint and
	// passed Constraint, and returns a new Constraint representing the result.
	Intersect(Constraint) Constraint

	// AdmitsAny returns a bool indicating whether there exists any version that
	// can satisfy the Constraint.
	AdmitsAny() bool

	// IsMagic indicates if the constraint is 'magic' - e.g., is either the empty
	// set, or the set of all versions.
	IsMagic() bool
}

// Any is a constraint that is satisfied by any valid semantic version.
type any struct{}

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

// AdmitsAny indicates whether there exists any version that can satisfy the
// constraint. As all versions satisfy Any, this is always true.
func (any) AdmitsAny() bool {
	return true
}

func (any) IsMagic() bool {
	return true
}

// None is an unsatisfiable constraint - it represents the empty set.
type none struct{}

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
	return none{}
}

// AdmitsAny indicates whether there exists any version that can satisfy the
// constraint. As no versions satisfy None, this is always false.
func (none) AdmitsAny() bool {
	return false
}

func (none) IsMagic() bool {
	return true
}

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

func (rc rangeConstraint) Intersect(c Constraint) Constraint {
	switch oc := c.(type) {
	case any:
		return rc
	case none:
		return none{}
	case unionConstraint:
		return oc.Intersect(rc)
	case *Version, rangeConstraint:
		break
	default:
		// this duplicates what's above, but doing it this way allows a slightly
		// faster path for internal operations while still respecting the
		// interface contract
		if c.IsMagic() {
			if c.AdmitsAny() {
				return rc
			} else {
				return none{}
			}
		}
		panic("unknown type")
	}

	panic("not implemented")
}

func (rc rangeConstraint) AdmitsAny() bool {
	return true
}

func (rc rangeConstraint) IsMagic() bool {
	return false
}

type unionConstraint []Constraint

func (uc unionConstraint) Admits(v *Version) error {
	panic("not implemented")
}

func (uc unionConstraint) Intersect(Constraint) Constraint {
	panic("not implemented - this is really the one annoying bit")
}

func (uc unionConstraint) AdmitsAny() bool {
	return true
}

func (uc unionConstraint) IsMagic() bool {
	return false
}

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
		// Zero members means unconstrained, so return any
		return any{}
	case 1:
		// Just one member means that's our final constraint
		return cg[0]
	}

	// Do a preliminary first pass to see if we have any constraints that
	// supercede everything else, making it easy
	for _, c := range cg {
		switch c.(type) {
		case none, *Version:
			return c
		}
	}

	// Now we know there's no easy wins, so step through and intersect each with
	// the previous
	head, tail := cg[0], cg[1:]
	for _, c := range tail {
		head = head.Intersect(c)
	}

	return head
}
