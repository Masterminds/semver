package semver

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Constraints is one or more constraint that a semantic version can be
// checked against.
type Constraints struct {
	constraints [][]*constraint
	containsPre []bool

	// IncludePrerelease specifies if pre-releases should be included in
	// the results. Note, if a constraint range has a prerelease than
	// prereleases will be included for that AND group even if this is
	// set to false.
	IncludePrerelease bool
}

// MaxConstraintLen is the maximum allowed length of a constraint string.
const MaxConstraintLen = 512

// MaxConstraintGroups is the maximum number of OR groups allowed in a
// constraint string.
const MaxConstraintGroups = 32

// ErrConstraintTooLong is returned when a constraint string exceeds the
// maximum allowed length.
var ErrConstraintTooLong = fmt.Errorf("constraint string is too long (max %d bytes)", MaxConstraintLen)

// ErrTooManyConstraintGroups is returned when a constraint string contains
// too many OR groups.
var ErrTooManyConstraintGroups = fmt.Errorf("too many constraint groups (max %d)", MaxConstraintGroups)

// NewConstraint returns a Constraints instance that a Version instance can
// be checked against. If there is a parse error it will be returned.
func NewConstraint(c string) (*Constraints, error) {

	if len(c) > MaxConstraintLen {
		return nil, ErrConstraintTooLong
	}

	// Rewrite - ranges into a comparison operation.
	c = rewriteRange(c)

	ors := strings.Split(c, "||")
	if len(ors) > MaxConstraintGroups {
		return nil, ErrTooManyConstraintGroups
	}
	lenors := len(ors)
	or := make([][]*constraint, lenors)
	hasPre := make([]bool, lenors)
	for k, v := range ors {
		cs, ok := handSplitGroup(v)
		if !ok {
			return nil, fmt.Errorf("improper constraint: %q", v)
		}
		result := make([]*constraint, len(cs))
		for i, s := range cs {
			pc, err := parseConstraint(s)
			if err != nil {
				return nil, err
			}

			// If one of the constraints has a prerelease record this.
			// This information is used when checking all in an "and"
			// group to ensure they all check for prereleases.
			if pc.con.pre != "" {
				hasPre[k] = true
			}

			result[i] = pc
		}
		or[k] = result
	}

	o := &Constraints{
		constraints: or,
		containsPre: hasPre,
	}
	return o, nil
}

// Check tests if a version satisfies the constraints.
func (cs Constraints) Check(v *Version) bool {
	// TODO(mattfarina): For v4 of this library consolidate the Check and Validate
	// functions as the underlying functions make that possible now.
	// loop over the ORs and check the inner ANDs
	for i, o := range cs.constraints {
		joy := true
		for _, c := range o {
			// The predicate form returns an errID instead of a formatted
			// error, so a failing check costs no formatting or allocation.
			if ok, _ := c.pf(v, c, cs.IncludePrerelease || cs.containsPre[i]); !ok {
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

// Validate checks if a version satisfies a constraint. If not a slice of
// reasons for the failure are returned in addition to a bool.
func (cs Constraints) Validate(v *Version) (bool, []error) {
	// loop over the ORs and check the inner ANDs
	var e []error

	// Capture the prerelease message only once. When it happens the first time
	// this var is marked
	var prerelesase bool
	for i, o := range cs.constraints {
		joy := true
		for _, c := range o {
			// Before running the check handle the case there the version is
			// a prerelease and the check is not searching for prereleases.
			if !cs.IncludePrerelease && !cs.containsPre[i] && v.pre != "" {
				if !prerelesase {
					em := fmt.Errorf("%q is a prerelease version and the constraint is only looking for release versions", v)
					e = append(e, em)
					prerelesase = true
				}
				joy = false

			} else {

				if _, err := c.check(v, (cs.IncludePrerelease || cs.containsPre[i])); err != nil {
					e = append(e, err)
					joy = false
				}
			}
		}

		if joy {
			return true, []error{}
		}
	}

	return false, e
}

func (cs Constraints) String() string {
	buf := make([]string, len(cs.constraints))
	var tmp bytes.Buffer

	for k, v := range cs.constraints {
		tmp.Reset()
		vlen := len(v)
		for kk, c := range v {
			tmp.WriteString(c.string())

			// Space separate the AND conditions
			if vlen > 1 && kk < vlen-1 {
				tmp.WriteString(" ")
			}
		}
		buf[k] = tmp.String()
	}

	return strings.Join(buf, " || ")
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (cs *Constraints) UnmarshalText(text []byte) error {
	temp, err := NewConstraint(string(text))
	if err != nil {
		return err
	}

	*cs = *temp

	return nil
}

// MarshalText implements the encoding.TextMarshaler interface.
func (cs Constraints) MarshalText() ([]byte, error) {
	return []byte(cs.String()), nil
}

var constraintOps map[string]cfunc
var constraintPreds map[string]pfunc
var constraintRegex *regexp.Regexp
var constraintRangeRegex *regexp.Regexp

// Used to find individual constraints within a multi-constraint string
var findConstraintRegex *regexp.Regexp

// Used to validate an segment of ANDs is valid
var validConstraintRegex *regexp.Regexp

const cvRegex string = `v?([0-9|x|X|\*]+)(\.[0-9|x|X|\*]+)?(\.[0-9|x|X|\*]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?`

func init() {
	constraintOps = map[string]cfunc{
		"":   constraintTildeOrEqual,
		"=":  constraintTildeOrEqual,
		"!=": constraintNotEqual,
		">":  constraintGreaterThan,
		"<":  constraintLessThan,
		">=": constraintGreaterThanEqual,
		"=>": constraintGreaterThanEqual,
		"<=": constraintLessThanEqual,
		"=<": constraintLessThanEqual,
		"~":  constraintTilde,
		"~>": constraintTilde,
		"^":  constraintCaret,
	}

	constraintPreds = map[string]pfunc{
		"":   tildeOrEqualPred,
		"=":  tildeOrEqualPred,
		"!=": notEqualPred,
		">":  greaterThanPred,
		"<":  lessThanPred,
		">=": greaterThanEqualPred,
		"=>": greaterThanEqualPred,
		"<=": lessThanEqualPred,
		"=<": lessThanEqualPred,
		"~":  tildePred,
		"~>": tildePred,
		"^":  caretPred,
	}

	ops := `=||!=|>|<|>=|=>|<=|=<|~|~>|\^`

	constraintRegex = regexp.MustCompile(fmt.Sprintf(
		`^\s*(%s)\s*(%s)\s*$`,
		ops,
		cvRegex))

	constraintRangeRegex = regexp.MustCompile(fmt.Sprintf(
		`\s*(%s)\s+-\s+(%s)\s*`,
		cvRegex, cvRegex))

	findConstraintRegex = regexp.MustCompile(fmt.Sprintf(
		`(%s)\s*(%s)`,
		ops,
		cvRegex))

	// The first time a constraint shows up will look slightly different from
	// future times it shows up due to a leading space or comma in a given
	// string.
	validConstraintRegex = regexp.MustCompile(fmt.Sprintf(
		`^(\s*(%s)\s*(%s)\s*)((?:\s+|,\s*)(%s)\s*(%s)\s*)*$`,
		ops,
		cvRegex,
		ops,
		cvRegex))
}

// An individual constraint
type constraint struct {
	// The version used in the constraint check. For example, if a constraint
	// is '<= 2.0.0' the con a version instance representing 2.0.0.
	con *Version

	// The original parsed version (e.g., 4.x from != 4.x)
	orig string

	// The original operator for the constraint
	origfunc string

	// When an x is used as part of the version (e.g., 1.x)
	minorDirty bool
	dirty      bool
	patchDirty bool

	// cf is the constraint function resolved at parse time so that Check does
	// not pay a map lookup per check.
	cf cfunc

	// pf is the predicate form of cf that reports the failure as an errID
	// instead of an error, so Check performs no error formatting.
	pf pfunc
}

// Check if a version meets the constraint
func (c *constraint) check(v *Version, includePre bool) (bool, error) {
	return c.cf(v, c, includePre)
}

// String prints an individual constraint into a string
func (c *constraint) string() string {
	return c.origfunc + c.orig
}

type cfunc func(v *Version, c *constraint, includePre bool) (bool, error)

// pfunc is the fast predicate form of cfunc. It reports the failure reason as
// an errID so Check can skip all error formatting and allocation.
type pfunc func(v *Version, c *constraint, includePre bool) (bool, errID)

// Wrappers over the predicates that format the error for Validate.
func constraintNotEqual(v *Version, c *constraint, includePre bool) (bool, error) {
	if ok, id := notEqualPred(v, c, includePre); !ok {
		return false, cerrError(id, v, c)
	}
	return true, nil
}

func constraintGreaterThan(v *Version, c *constraint, includePre bool) (bool, error) {
	if ok, id := greaterThanPred(v, c, includePre); !ok {
		return false, cerrError(id, v, c)
	}
	return true, nil
}

func constraintLessThan(v *Version, c *constraint, includePre bool) (bool, error) {
	if ok, id := lessThanPred(v, c, includePre); !ok {
		return false, cerrError(id, v, c)
	}
	return true, nil
}

func constraintGreaterThanEqual(v *Version, c *constraint, includePre bool) (bool, error) {
	if ok, id := greaterThanEqualPred(v, c, includePre); !ok {
		return false, cerrError(id, v, c)
	}
	return true, nil
}

func constraintLessThanEqual(v *Version, c *constraint, includePre bool) (bool, error) {
	if ok, id := lessThanEqualPred(v, c, includePre); !ok {
		return false, cerrError(id, v, c)
	}
	return true, nil
}

func constraintTilde(v *Version, c *constraint, includePre bool) (bool, error) {
	if ok, id := tildePred(v, c, includePre); !ok {
		return false, cerrError(id, v, c)
	}
	return true, nil
}

func constraintTildeOrEqual(v *Version, c *constraint, includePre bool) (bool, error) {
	if ok, id := tildeOrEqualPred(v, c, includePre); !ok {
		return false, cerrError(id, v, c)
	}
	return true, nil
}

func constraintCaret(v *Version, c *constraint, includePre bool) (bool, error) {
	if ok, id := caretPred(v, c, includePre); !ok {
		return false, cerrError(id, v, c)
	}
	return true, nil
}

// errID identifies the reason a constraint check failed. The predicates used
// by Check return an errID instead of a formatted error so that a Check that
// discards the error performs no message formatting and no allocation. The
// wrapper cfuncs convert the errID into an error for Validate.
type errID int

const (
	errNone           = errID(iota)
	errPre            // "%q is a prerelease version and the constraint is only looking for release versions"
	errEqual          // "%q is equal to %q"
	errLTE            // "%q is less than or equal to %q"
	errGTE            // "%q is greater than or equal to %q"
	errLT             // "%q is less than %q"
	errGT             // "%q is greater than %q"
	errSameMajor      // "%q does not have same major version as %q"
	errSameMajorMinor // "%q does not have same major and minor version as %q"
	errNotEqual       // "%q is not equal to %q"
	errMinorMatch     // "%q does not have same minor version as %q. Expected minor versions to match when constraint major version is 0"
	errMinor          // "%q does not have same minor version as %q"
	errZeroZero       // "%q does not equal %q. Expect version and constraint to equal when major and minor versions are 0"
)

// cerrError formats the error for a failed constraint check.
func cerrError(id errID, v *Version, c *constraint) error {
	switch id {
	case errPre:
		return fmt.Errorf("%q is a prerelease version and the constraint is only looking for release versions", v)
	case errEqual:
		return fmt.Errorf("%q is equal to %q", v, c.orig)
	case errLTE:
		return fmt.Errorf("%q is less than or equal to %q", v, c.orig)
	case errGTE:
		return fmt.Errorf("%q is greater than or equal to %q", v, c.orig)
	case errLT:
		return fmt.Errorf("%q is less than %q", v, c.orig)
	case errGT:
		return fmt.Errorf("%q is greater than %q", v, c.orig)
	case errSameMajor:
		return fmt.Errorf("%q does not have same major version as %q", v, c.orig)
	case errSameMajorMinor:
		return fmt.Errorf("%q does not have same major and minor version as %q", v, c.orig)
	case errNotEqual:
		return fmt.Errorf("%q is not equal to %q", v, c.orig)
	case errMinorMatch:
		return fmt.Errorf("%q does not have same minor version as %q. Expected minor versions to match when constraint major version is 0", v, c.orig)
	case errMinor:
		return fmt.Errorf("%q does not have same minor version as %q", v, c.orig)
	case errZeroZero:
		return fmt.Errorf("%q does not equal %q. Expect version and constraint to equal when major and minor versions are 0", v, c.orig)
	}
	return fmt.Errorf("unknown check error")
}

func parseConstraint(c string) (*constraint, error) {
	if len(c) > 0 {
		m, ok := parseAtomGroups(c)
		if !ok {
			return nil, fmt.Errorf("improper constraint: %q", c)
		}

		cs := &constraint{
			orig:     m[2],
			origfunc: m[1],
		}

		ver := m[2]
		minorDirty := false
		patchDirty := false
		dirty := false
		if isX(m[3]) || m[3] == "" {
			ver = fmt.Sprintf("0.0.0%s", m[6])
			dirty = true
		} else if isX(strings.TrimPrefix(m[4], ".")) || m[4] == "" {
			minorDirty = true
			dirty = true
			ver = fmt.Sprintf("%s.0.0%s", m[3], m[6])
		} else if isX(strings.TrimPrefix(m[5], ".")) || m[5] == "" {
			dirty = true
			patchDirty = true
			ver = fmt.Sprintf("%s%s.0%s", m[3], m[4], m[6])
		}

		con, err := NewVersion(ver)
		if err != nil {

			// The constraintRegex should catch any regex parsing errors. So,
			// we should never get here.
			return nil, errors.New("constraint parser error")
		}

		cs.con = con
		cs.minorDirty = minorDirty
		cs.patchDirty = patchDirty
		cs.dirty = dirty
		cs.cf = constraintOps[m[1]]
		cs.pf = constraintPreds[m[1]]

		return cs, nil
	}

	// The rest is the special case where an empty string was passed in which
	// is equivalent to * or >=0.0.0
	con, err := StrictNewVersion("0.0.0")
	if err != nil {

		// The constraintRegex should catch any regex parsing errors. So,
		// we should never get here.
		return nil, errors.New("constraint parser error")
	}

	cs := &constraint{
		con:        con,
		orig:       c,
		origfunc:   "",
		minorDirty: false,
		patchDirty: false,
		dirty:      true,
		cf:         constraintOps[""],
		pf:         constraintPreds[""],
	}
	return cs, nil
}

// Constraint functions
func notEqualPred(v *Version, c *constraint, includePre bool) (bool, errID) {
	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, errPre
	}

	if c.dirty {
		if c.con.Major() != v.Major() {
			return true, errNone
		}
		if c.con.Minor() != v.Minor() && !c.minorDirty {
			return true, errNone
		} else if c.minorDirty {
			return false, errEqual
		} else if c.con.Patch() != v.Patch() && !c.patchDirty {
			return true, errNone
		} else if c.patchDirty {
			// Need to handle prereleases if present
			if v.Prerelease() != "" || c.con.Prerelease() != "" {
				eq := comparePrerelease(v.Prerelease(), c.con.Prerelease()) != 0
				if eq {
					return true, errNone
				}
				return false, errEqual
			}
			return false, errEqual
		}
	}

	eq := v.Equal(c.con)
	if eq {
		return false, errEqual
	}

	return true, errNone
}

func greaterThanPred(v *Version, c *constraint, includePre bool) (bool, errID) {

	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, errPre
	}

	var eq bool

	if !c.dirty {
		eq = v.Compare(c.con) == 1
		if eq {
			return true, errNone
		}
		return false, errLTE
	}

	if v.Major() > c.con.Major() {
		return true, errNone
	} else if v.Major() < c.con.Major() {
		return false, errLTE
	} else if c.minorDirty {
		// This is a range case such as >11. When the version is something like
		// 11.1.0 is it not > 11. For that we would need 12 or higher
		return false, errLTE
	} else if c.patchDirty {
		// This is for ranges such as >11.1. A version of 11.1.1 is not greater
		// which one of 11.2.1 is greater
		eq = v.Minor() > c.con.Minor()
		if eq {
			return true, errNone
		}
		return false, errLTE
	}

	// If we have gotten here we are not comparing pre-preleases and can use the
	// Compare function to accomplish that.
	eq = v.Compare(c.con) == 1
	if eq {
		return true, errNone
	}
	return false, errLTE
}

func lessThanPred(v *Version, c *constraint, includePre bool) (bool, errID) {
	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, errPre
	}

	eq := v.Compare(c.con) < 0
	if eq {
		return true, errNone
	}
	return false, errGTE
}

func greaterThanEqualPred(v *Version, c *constraint, includePre bool) (bool, errID) {

	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, errPre
	}

	eq := v.Compare(c.con) >= 0
	if eq {
		return true, errNone
	}
	return false, errLT
}

func lessThanEqualPred(v *Version, c *constraint, includePre bool) (bool, errID) {
	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, errPre
	}

	var eq bool

	if !c.dirty {
		eq = v.Compare(c.con) <= 0
		if eq {
			return true, errNone
		}
		return false, errGT
	}

	if v.Major() > c.con.Major() {
		return false, errGT
	} else if v.Major() == c.con.Major() && v.Minor() > c.con.Minor() && !c.minorDirty {
		return false, errGT
	}

	return true, errNone
}

// ~*, ~>* --> >= 0.0.0 (any)
// ~2, ~2.x, ~2.x.x, ~>2, ~>2.x ~>2.x.x --> >=2.0.0, <3.0.0
// ~2.0, ~2.0.x, ~>2.0, ~>2.0.x --> >=2.0.0, <2.1.0
// ~1.2, ~1.2.x, ~>1.2, ~>1.2.x --> >=1.2.0, <1.3.0
// ~1.2.3, ~>1.2.3 --> >=1.2.3, <1.3.0
// ~1.2.0, ~>1.2.0 --> >=1.2.0, <1.3.0
func tildePred(v *Version, c *constraint, includePre bool) (bool, errID) {
	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, errPre
	}

	if v.LessThan(c.con) {
		return false, errLT
	}

	// ~0.0.0 is a special case where all constraints are accepted. It's
	// equivalent to >= 0.0.0.
	if c.con.Major() == 0 && c.con.Minor() == 0 && c.con.Patch() == 0 &&
		!c.minorDirty && !c.patchDirty {
		return true, errNone
	}

	if v.Major() != c.con.Major() {
		return false, errSameMajor
	}

	if v.Minor() != c.con.Minor() && !c.minorDirty {
		return false, errSameMajorMinor
	}

	return true, errNone
}

// When there is a .x (dirty) status it automatically opts in to ~. Otherwise
// it's a straight =
func tildeOrEqualPred(v *Version, c *constraint, includePre bool) (bool, errID) {
	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, errPre
	}

	if c.dirty {
		return tildePred(v, c, includePre)
	}

	eq := v.Equal(c.con)
	if eq {
		return true, errNone
	}

	return false, errNotEqual
}

// ^*      -->  (any)
// ^1.2.3  -->  >=1.2.3 <2.0.0
// ^1.2    -->  >=1.2.0 <2.0.0
// ^1      -->  >=1.0.0 <2.0.0
// ^0.2.3  -->  >=0.2.3 <0.3.0
// ^0.2    -->  >=0.2.0 <0.3.0
// ^0.0.3  -->  >=0.0.3 <0.0.4
// ^0.0    -->  >=0.0.0 <0.1.0
// ^0      -->  >=0.0.0 <1.0.0
func caretPred(v *Version, c *constraint, includePre bool) (bool, errID) {
	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, errPre
	}

	// This less than handles prereleases
	if v.LessThan(c.con) {
		return false, errLT
	}

	var eq bool

	// ^ when the major > 0 is >=x.y.z < x+1
	if c.con.Major() > 0 || c.minorDirty {

		// ^ has to be within a major range for > 0. Everything less than was
		// filtered out with the LessThan call above. This filters out those
		// that greater but not within the same major range.
		eq = v.Major() == c.con.Major()
		if eq {
			return true, errNone
		}
		return false, errSameMajor
	}

	// ^ when the major is 0 and minor > 0 is >=0.y.z < 0.y+1
	if c.con.Major() == 0 && v.Major() > 0 {
		return false, errSameMajor
	}
	// If the con Minor is > 0 it is not dirty
	if c.con.Minor() > 0 || c.patchDirty {
		eq = v.Minor() == c.con.Minor()
		if eq {
			return true, errNone
		}
		return false, errMinorMatch
	}
	// ^ when the minor is 0 and minor > 0 is =0.0.z
	if c.con.Minor() == 0 && v.Minor() > 0 {
		return false, errMinor
	}

	// At this point the major is 0 and the minor is 0 and not dirty. The patch
	// is not dirty so we need to check if they are equal. If they are not equal
	eq = c.con.Patch() == v.Patch()
	if eq {
		return true, errNone
	}
	return false, errZeroZero
}

// scanVersionEnd scans the constraint version part starting at s[start] and
// returns the exclusive end offset, mirroring the segments/prerelease/metadata
// part of cvRegex. Returns ok=false if the start is not the beginning of a
// valid version segment.
func scanVersionEnd(s string, start int) (int, bool) {
	i := start
	if i < len(s) && s[i] == 'v' {
		i++
	}
	if i >= len(s) || !segTable[s[i]] {
		return 0, false
	}
	for i < len(s) && segTable[s[i]] {
		i++
	}
	if i < len(s) && s[i] == '.' {
		if i+1 >= len(s) || !segTable[s[i+1]] {
			return 0, false
		}
		i++
		for i < len(s) && segTable[s[i]] {
			i++
		}
	}
	if i < len(s) && s[i] == '.' {
		if i+1 >= len(s) || !segTable[s[i+1]] {
			return 0, false
		}
		i++
		for i < len(s) && segTable[s[i]] {
			i++
		}
	}
	if i < len(s) && s[i] == '-' {
		if inner, j := scanIdent(s, i+1); inner != "" {
			i = j
		} else {
			return 0, false
		}
	}
	if i < len(s) && s[i] == '+' {
		if _, j := scanIdent(s, i+1); j > i+1 {
			i = j
		} else {
			return 0, false
		}
	}
	return i, true
}

// matchAtomEnd returns the exclusive end offset of the constraint atom
// beginning at s[start] (skipping leading whitespace), or ok=false if there is
// no valid atom there. It tries the operator alternatives in order, as the
// original regex alternation does, and returns whichever lets the rest be a
// valid version.
func matchAtomEnd(s string, start int) (int, bool) {
	i := start
	for i < len(s) && isWS(s[i]) {
		i++
	}
	for _, op := range opAlternatives {
		if !strings.HasPrefix(s[i:], op) {
			continue
		}
		j := i + len(op)
		for j < len(s) && isWS(s[j]) {
			j++
		}
		if k, ok := scanVersionEnd(s, j); ok {
			return k, true
		}
	}
	return 0, false
}

// handSplitGroup validates an OR group and splits it into its atoms,
// reproducing validConstraintRegex + findConstraintRegex without a regex. A
// group is a sequence of atoms; consecutive atoms must be separated by
// whitespace and/or a single comma (matching the (?:\s+|,\s*) alternation),
// and every atom must be a valid operator+version.
func handSplitGroup(g string) ([]string, bool) {
	n := len(g)
	firstEnd, ok := matchAtomEnd(g, 0)
	if !ok {
		return nil, false
	}
	atoms := []string{g[:firstEnd]}
	rest := firstEnd
	for rest < n {
		q := rest
		for q < n && isWS(g[q]) {
			q++
		}
		if q == n {
			// Only trailing whitespace after the last atom.
			break
		}
		if g[q] == ',' {
			// Comma separator (,\s*); a valid atom must follow the comma.
			end, ok := matchAtomEnd(g, q+1)
			if !ok {
				return nil, false
			}
			atoms = append(atoms, g[q+1:end])
			rest = end
			continue
		}
		// Whitespace separator; require that whitespace actually separated the
		// atoms (q > rest), otherwise the atoms are directly adjacent and the
		// group is invalid.
		if q == rest {
			return nil, false
		}
		end, ok := matchAtomEnd(g, q)
		if !ok {
			return nil, false
		}
		atoms = append(atoms, g[q:end])
		rest = end
	}
	return atoms, true
}

func isX(x string) bool {
	switch x {
	case "x", "*", "X":
		return true
	default:
		return false
	}
}

// opAlternatives is the ordered list of operator alternatives of the original
// constraintRegex (`=||!=|>|<|>=|=>|<=|=<|~|~>|\^`). The order matters: the
// first alternative for which the remainder is a valid version is the one the
// regex would have chosen.
var opAlternatives = []string{"=", "", "!=", ">", "<", ">=", "=>", "<=", "=<", "~", "~>", "^"}

var segTable = [256]bool{}
var idTable = [256]bool{}
var wsTable = [256]bool{}

func init() {
	for c := '0'; c <= '9'; c++ {
		segTable[c] = true
		idTable[c] = true
	}
	segTable['x'], segTable['X'], segTable['*'], segTable['|'] = true, true, true, true
	for c := 'a'; c <= 'z'; c++ {
		idTable[c] = true
	}
	for c := 'A'; c <= 'Z'; c++ {
		idTable[c] = true
	}
	idTable['-'] = true
	// \s in Go's regexp (ASCII patterns) is [ \t\n\f\r].
	wsTable[' '] = true
	wsTable['\t'] = true
	wsTable['\n'] = true
	wsTable['\f'] = true
	wsTable['\r'] = true
}

func isWS(b byte) bool { return wsTable[b] }

// trimWSLeft trims leading whitespace bytes as \s in the original regex.
func trimWSLeft(s string) string {
	for len(s) > 0 && isWS(s[0]) {
		s = s[1:]
	}
	return s
}

// parseAtomGroups hand-parses a single constraint atom (operator + version),
// returning the capture groups the original constraintRegex would have
// produced: m[1]=operator, m[2]=whole version, m[3]=major, m[4]=.minor,
// m[5]=.patch, m[6]=-prerelease. It returns ok=false if no operator/version
// combination matches (i.e. the atom is improper).
func parseAtomGroups(c string) (m []string, ok bool) {
	s := trimWSLeft(c)
	m = make([]string, 7)
	m[0] = s
	for _, op := range opAlternatives {
		if !strings.HasPrefix(s, op) {
			continue
		}
		rest := trimWSLeft(s[len(op):])
		whole, major, minor, patch, pre, matched := parseVersionPart(rest)
		if !matched {
			continue
		}
		m[1] = op
		m[2] = whole
		m[3] = major
		m[4] = minor
		m[5] = patch
		m[6] = pre
		return m, true
	}
	return nil, false
}

// parseVersionPart parses the cvRegex version part of a constraint: an
// optional 'v', a required numeric-or-wildcard major segment, up to two more
// dot-separated numeric-or-wildcard segments, an optional prerelease (starting
// with '-'), and an optional build-metadata (starting with '+'). Only
// trailing whitespace may follow. It mirrors the original loose cvRegex,
// including its idiosyncratic segment character set (digits, x, X, *, '|').
func parseVersionPart(r string) (whole, major, minor, patch, pre string, ok bool) {
	i := 0
	if i < len(r) && r[i] == 'v' {
		i++
	}
	if i >= len(r) || !segTable[r[i]] {
		return "", "", "", "", "", false
	}
	j := i
	for j < len(r) && segTable[r[j]] {
		j++
	}
	major = r[i:j]
	i = j

	if i < len(r) && r[i] == '.' {
		if i+1 >= len(r) || !segTable[r[i+1]] {
			return "", "", "", "", "", false
		}
		j = i + 1
		for j < len(r) && segTable[r[j]] {
			j++
		}
		minor = r[i:j]
		i = j
	}

	if i < len(r) && r[i] == '.' {
		if i+1 >= len(r) || !segTable[r[i+1]] {
			return "", "", "", "", "", false
		}
		j = i + 1
		for j < len(r) && segTable[r[j]] {
			j++
		}
		patch = r[i:j]
		i = j
	}

	if i < len(r) && r[i] == '-' {
		if inner, k := scanIdent(r, i+1); inner != "" {
			pre = r[i:k]
			i = k
		} else {
			return "", "", "", "", "", false
		}
	}

	if i < len(r) && r[i] == '+' {
		if _, k := scanIdent(r, i+1); k > i+1 {
			i = k
		} else {
			return "", "", "", "", "", false
		}
	}

	for ; i < len(r); i++ {
		if !isWS(r[i]) {
			return "", "", "", "", "", false
		}
	}
	return r[:i], major, minor, patch, pre, true
}

// scanIdent scans one or more dot-separated identifiers made of [0-9A-Za-z-],
// as in the prerelease/metadata part of cvRegex, returning the matched
// identifier (without the leading '-' or '+') and its end offset. Returns ""
// if the first identifier is empty.
func scanIdent(s string, start int) (string, int) {
	if start >= len(s) || !idTable[s[start]] {
		return "", start
	}
	i := start
	for i < len(s) && idTable[s[i]] {
		i++
	}
	for i < len(s) && s[i] == '.' {
		k := i + 1
		for k < len(s) && idTable[s[k]] {
			k++
		}
		if k == i+1 {
			return s[start:i], i
		}
		i = k
	}
	return s[start:i], i
}

func rewriteRange(i string) string {
	// A range requires a hyphen (e.g. "2 - 3"). Skip the regex entirely when
	// there is none, which is the common case. A hyphen in a prerelease will
	// still fall through to the regex, so this is conservative.
	if !strings.Contains(i, "-") {
		return i
	}

	m := constraintRangeRegex.FindAllStringSubmatch(i, -1)
	if m == nil {
		return i
	}
	o := i
	for _, v := range m {
		t := fmt.Sprintf(">= %s, <= %s ", v[1], v[11])
		o = strings.Replace(o, v[0], t, 1)
	}

	return o
}
