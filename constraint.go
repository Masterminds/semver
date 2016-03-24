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
type Any struct{}

// Admits checks that a version satisfies the constraint. As all versions
// satisfy Any, this always returns nil.
func (Any) Admits(v *Version) error {
	return nil
}

// Intersect computes the intersection between two constraints.
//
// As Any is the set of all possible versions, any intersection with that
// infinite set will necessarily be the entirety of the second set. Thus, this
// simply returns the passed constraint.
func (Any) Intersect(c Constraint) Constraint {
	return c
}

// AdmitsAny indicates whether there exists any version that can satisfy the
// constraint. As all versions satisfy Any, this is always true.
func (Any) AdmitsAny() bool {
	return true
}

func (Any) IsMagic() bool {
	return true
}

// None is an unsatisfiable constraint - it represents the empty set.
type None struct{}

// Admits checks that a version satisfies the constraint. As no version can
// satisfy None, this always fails (returns an error).
func (None) Admits(v *Version) error {
	return noneErr
}

// Intersect computes the intersection between two constraints.
//
// None is the empty set of versions, and any intersection with the empty set is
// necessarily the empty set. Thus, this always returns None.
func (None) Intersect(Constraint) Constraint {
	return None{}
}

// AdmitsAny indicates whether there exists any version that can satisfy the
// constraint. As no versions satisfy None, this is always false.
func (None) AdmitsAny() bool {
	return false
}

func (None) IsMagic() bool {
	return true
}

type rangeConstraint struct {
	min, max *constraint
	excl     []*constraint
}

func (rc rangeConstraint) Admits(v *Version) error {
	if rc.min != nil {
		if !rc.min.check(v) {
			return fmt.Errorf(rc.min.msg, v, rc.min.orig)
		}
	}

	if rc.max != nil {
		if !rc.min.check(v) {
			return fmt.Errorf(rc.max.msg, v, rc.max.orig)
		}
	}

	for _, excl := range rc.excl {
		if excl.con.Equal(v) {
			return fmt.Errorf("Version %s is specifically disallowed.", v.String())
		}
	}

	return nil
}

func (rc rangeConstraint) Intersect(c Constraint) Constraint {
	switch oc := c.(type) {
	case Any:
		return rc
	case None:
		return None{}
	case unionConstraint:
		return oc.Intersect(rc)
	case *Version, rangeConstraint:
		panic("not implemented")
	default:
		// this duplicates what's above, but doing it this way allows a slightly
		// faster path for internal operations while still respecting the
		// interface contract
		if c.IsMagic() {
			if c.AdmitsAny() {
				return rc
			} else {
				return None{}
			}
		}
		panic("unknown type")
	}
}

func (rc rangeConstraint) AdmitsAny() bool {
	return true
}

func (rc rangeConstraint) IsMagic() bool {
	return false
}

type unionConstraint struct {
	constraints []Constraint
}

func (unionConstraint) Admits(v *Version) error {
	panic("not implemented")
}

func (unionConstraint) Intersect(Constraint) Constraint {
	panic("not implemented")
}

func (unionConstraint) AdmitsAny() bool {
	return true
}

func (unionConstraint) IsMagic() bool {
	return false
}
