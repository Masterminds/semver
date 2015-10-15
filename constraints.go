package semver

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Constraints is one or more constraint that a semantic version can be
// checked against.
type Constraints struct {
	constraints []*constraint
}

// NewConstraint returns a Constraints instance that a Version instance can
// be checked against. If there is a parse error it will be returned.
func NewConstraint(c string) (*Constraints, error) {
	cs := strings.Split(c, ",")
	result := make([]*constraint, len(cs))
	for i, s := range cs {
		pc, err := parseConstraint(s)
		if err != nil {
			return nil, err
		}

		result[i] = pc
	}

	o := &Constraints{constraints: result}
	return o, nil
}

// Check tests if a version satisfies the constraints.
func (cs Constraints) Check(v *Version) bool {
	for _, c := range cs.constraints {
		if !c.check(v) {
			return false
		}
	}

	return true
}

var constraintOps map[string]cfunc
var constraintRegex *regexp.Regexp

func init() {
	constraintOps = map[string]cfunc{
		"":   constraintEqual,
		"=":  constraintEqual,
		"!=": constraintNotEqual,
		">":  constraintGreaterThan,
		"<":  constraintLessThan,
		">=": constraintGreaterThanEqual,
		"=>": constraintGreaterThanEqual,
		"<=": constraintLessThanEqual,
		"=<": constraintLessThanEqual,
	}

	ops := make([]string, 0, len(constraintOps))
	for k := range constraintOps {
		ops = append(ops, regexp.QuoteMeta(k))
	}

	constraintRegex = regexp.MustCompile(fmt.Sprintf(
		`^\s*(%s)\s*(%s)\s*$`,
		strings.Join(ops, "|"),
		SemVerRegex))
}

// An individual constraint
type constraint struct {
	// The callback function for the restraint. It performs the logic for
	// the constraint.
	function cfunc

	// The version used in the constraint check. For example, if a constraint
	// is '<= 2.0.0' the con a version instance representing 2.0.0.
	con *Version
}

// Check if a version meets the constraint
func (c *constraint) check(v *Version) bool {
	return c.function(v, c.con)
}

type cfunc func(v, c *Version) bool

func parseConstraint(c string) (*constraint, error) {
	m := constraintRegex.FindStringSubmatch(c)
	if m == nil {
		return nil, fmt.Errorf("improper constraint: %s", c)
	}

	con, err := NewVersion(m[2])
	if err != nil {

		// The constraintRegex should catch any regex parsing errors. So,
		// we should never get here.
		return nil, errors.New("constraint Parser Error")
	}

	cs := &constraint{
		function: constraintOps[m[1]],
		con:      con,
	}
	return cs, nil
}

// Constraint functions
func constraintEqual(v, c *Version) bool {
	return v.Equal(c)
}

func constraintNotEqual(v, c *Version) bool {
	return !v.Equal(c)
}

func constraintGreaterThan(v, c *Version) bool {
	return v.Compare(c) == 1
}

func constraintLessThan(v, c *Version) bool {
	return v.Compare(c) == -1
}

func constraintGreaterThanEqual(v, c *Version) bool {
	return v.Compare(c) >= 0
}

func constraintLessThanEqual(v, c *Version) bool {
	return v.Compare(c) <= 0
}
