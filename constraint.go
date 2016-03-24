package semver

import "errors"

type Constraint interface {
	// Admits checks that a version satisfies the constraint. If it does not,
	// an error is returned indcating the problem; if it does, the error is nil.
	Admits(v Version) error

	// Intersect computes the intersection between the receiving Constraint and
	// passed Constraint, and returns a new Constraint representing the result.
	Intersect(Constraint) Constraint

	// AdmitsAny returns a bool indicating whether there exists any version that
	// can satisfy the Constraint.
	AdmitsAny() bool
}

// Any is a constraint that is satisfied by any valid semantic version.
type Any struct{}

// Admits checks that a version satisfies the constraint. As all versions
// satisfy Any, this always returns nil.
func (Any) Admits(v Version) error {
	return nil
}

// Intersect computes the intersection between two constraints.
//
// As Any is the set of all possible versions, any intersection with that
// infinite set will necessarily be the entirety of the second set. Thus, this
// simply returns (a copy of) the passed constraint.
func (Any) Intersect(c Constraint) Constraint {
	c2 := &c
	return *c2
}

// AdmitsAny indicates whether there exists any version that can satisfy the
// constraint. As all versions satisfy Any, this is always true.
func (Any) AdmitsAny() bool {
	return true
}

// None is an unsatisfiable constraint - it represents the empty set.
type None struct{}

func (None) Admits(v Version) error {
	return errors.New("The 'None' constraint admits no versions.")
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

type rangeConstraint struct {
	min, max               *Version
	includeMin, includeMax bool
	excl                   []*Version
}

func (rangeConstraint) Admits(v Version) error {
	panic("not implemented")
}

func (rangeConstraint) Intersect(Constraint) Constraint {
	panic("not implemented")
}

func (rangeConstraint) AdmitsAny() bool {
	panic("not implemented")
}
