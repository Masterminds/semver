package semver

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Constraints is one or more constraint that a semantic version can be
// checked against.
type Constraints struct {
	constraints [][]*constraint
}

// NewConstraint returns a Constraints instance that a Version instance can
// be checked against. If there is a parse error it will be returned.
func NewConstraint(c string) (*Constraints, error) {

	// Rewrite the constraint string to convert things like ranges
	// into something the checks can handle.
	for _, rwf := range rewriteFuncs {
		c = rwf(c)
	}

	ors := strings.Split(c, "||")
	or := make([][]*constraint, len(ors))
	for k, v := range ors {
		cs := strings.Split(v, ",")
		result := make([]*constraint, len(cs))
		for i, s := range cs {
			pc, err := parseConstraint(s)
			if err != nil {
				return nil, err
			}

			result[i] = pc
		}
		or[k] = result
	}

	o := &Constraints{constraints: or}
	return o, nil
}

// Check tests if a version satisfies the constraints.
func (cs Constraints) Check(v *Version) bool {
	// loop over the ORs and check the inner ANDs
	for _, o := range cs.constraints {
		joy := true
		for _, c := range o {
			if !c.check(v) {
				joy = false
				break
			}
		}

		if joy {
			return true
		}
	}

	return false
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

	constraintRangeRegex = regexp.MustCompile(fmt.Sprintf(
		`\s*(%s)\s*-\s*(%s)\s*`,
		SemVerRegex, SemVerRegex))

	constraintCaretRegex = regexp.MustCompile(`\^` + cvRegex)
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

type rwfunc func(i string) string

var constraintRangeRegex *regexp.Regexp
var rewriteFuncs = []rwfunc{
	rewriteRange,
	rewriteCarets,
}

const cvRegex string = `v?([0-9|x|X|\*]+)(\.[0-9|x|X|\*]+)?(\.[0-9|x|X|\*]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?`

func isX(x string) bool {
	l := strings.ToLower(x)
	return l == "x" || l == "*"
}

func rewriteRange(i string) string {
	m := constraintRangeRegex.FindAllStringSubmatch(i, -1)
	if m == nil {
		return i
	}
	o := i
	for _, v := range m {
		t := fmt.Sprintf(">= %s, <= %s", v[1], v[11])
		o = strings.Replace(o, v[0], t, 1)
	}

	return o
}

// ^ --> * (any)
// ^2, ^2.x, ^2.x.x --> >=2.0.0 <3.0.0
// ^2.0, ^2.0.x --> >=2.0.0 <3.0.0
// ^1.2, ^1.2.x --> >=1.2.0 <2.0.0
// ^1.2.3 --> >=1.2.3 <2.0.0
// ^1.2.0 --> >=1.2.0 <2.0.0
var constraintCaretRegex *regexp.Regexp

func rewriteCarets(i string) string {
	m := constraintCaretRegex.FindAllStringSubmatch(i, -1)
	if m == nil {
		return i
	}
	o := i
	for _, v := range m {
		if isX(v[1]) {
			o = strings.Replace(o, v[0], ">=0.0.0", 1)
		} else if isX(strings.TrimPrefix(v[2], ".")) {
			ii, err := strconv.ParseInt(v[1], 10, 32)

			// The regular expression and isX checking should already make this
			// safe so something is broken in the lib.
			if err != nil {
				panic("Error converting string to Int. Should not occur.")
			}
			t := fmt.Sprintf(">= %s.0%s, < %d", v[1], v[4], ii+1)
			o = strings.Replace(o, v[0], t, 1)
		} else if isX(strings.TrimPrefix(v[3], ".")) {
			ii, err := strconv.ParseInt(v[1], 10, 32)

			// The regular expression and isX checking should already make this
			// safe so something is broken in the lib.
			if err != nil {
				panic("Error converting string to Int. Should not occur.")
			}
			t := fmt.Sprintf(">= %s%s.0%s, < %d", v[1], v[2], v[4], ii+1)
			o = strings.Replace(o, v[0], t, 1)
		} else {
			ii, err := strconv.ParseInt(v[1], 10, 32)
			// The regular expression and isX checking should already make this
			// safe so something is broken in the lib.
			if err != nil {
				panic("Error converting string to Int. Should not occur.")
			}

			t := fmt.Sprintf(">= %s%s%s%s, < %d", v[1], v[2], v[3], v[4], ii+1)
			o = strings.Replace(o, v[0], t, 1)
		}
	}

	return o
}
