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
		result, err := parseConstraintGroup(v)
		if err != nil {
			return nil, err
		}

		for _, pc := range result {
			// If one of the constraints has a prerelease record this.
			// This information is used when checking all in an "and"
			// group to ensure they all check for prereleases.
			if pc.con.pre != "" {
				hasPre[k] = true
				break
			}
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
			if !c.ok(v, cs.IncludePrerelease || cs.containsPre[i]) {
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
	var buf strings.Builder

	for k, v := range cs.constraints {
		// Separate the OR groups
		if k > 0 {
			buf.WriteString(" || ")
		}

		for kk, c := range v {
			// Space separate the AND conditions
			if kk > 0 {
				buf.WriteString(" ")
			}

			buf.WriteString(c.origfunc)
			buf.WriteString(c.orig)
		}
	}

	return buf.String()
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

// Used to rewrite a hyphenated range into a pair of comparisons
var constraintRangeRegex *regexp.Regexp

// constraintSegChars is a lookup table for the characters allowed in the
// numeric segments of a constraint version. Along with the digits it holds the
// wildcards and, for compatibility with the regular expression that this
// scanner replaced, a literal |. Segments that are not a lone wildcard are
// checked for being numeric when the version is built.
var constraintSegChars [256]bool

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

	for _, ch := range []byte("0123456789xX*|") {
		constraintSegChars[ch] = true
	}

	constraintRangeRegex = regexp.MustCompile(fmt.Sprintf(
		`\s*(%s)\s+-\s+(%s)\s*`,
		cvRegex, cvRegex))
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

	// The function that performs the check, resolved from origfunc when the
	// constraint is parsed so that checking does not need a map lookup.
	cf cfunc

	// When an x is used as part of the version (e.g., 1.x)
	minorDirty bool
	dirty      bool
	patchDirty bool
}

// Check if a version meets the constraint. The error explains why the check
// failed. Callers that ignore the error should use ok instead so that nothing
// is formatted.
func (c *constraint) check(v *Version, includePre bool) (bool, error) {
	res, r := c.cf(v, c, includePre)
	if res {
		return true, nil
	}
	return false, r.err(v, c)
}

// ok reports if a version meets the constraint. Unlike check it does not build
// the message explaining a failure, which is the bulk of the cost of a failing
// check.
func (c *constraint) ok(v *Version, includePre bool) bool {
	res, _ := c.cf(v, c, includePre)
	return res
}

type cfunc func(v *Version, c *constraint, includePre bool) (bool, failReason)

// failReason identifies why a constraint check failed. Constraint functions
// return a reason rather than an error so that the message is only built when
// a caller asks for it. Check discards the reason, Validate turns it into an
// error.
type failReason uint8

const (
	reasonNone failReason = iota
	reasonPrerelease
	reasonEqual
	reasonNotEqual
	reasonLessThan
	reasonLessThanEqual
	reasonGreaterThan
	reasonGreaterThanEqual
	reasonMajor
	reasonMajorMinor
	reasonCaretMinor
	reasonCaretMinorZero
	reasonCaretPatch
)

// reasonFormats holds the message for each failReason. Every message takes the
// version and the original constraint text, except reasonPrerelease which
// takes the version alone.
var reasonFormats = [...]string{
	reasonNone:             "%q does not satisfy %q",
	reasonPrerelease:       "%q is a prerelease version and the constraint is only looking for release versions",
	reasonEqual:            "%q is equal to %q",
	reasonNotEqual:         "%q is not equal to %q",
	reasonLessThan:         "%q is less than %q",
	reasonLessThanEqual:    "%q is less than or equal to %q",
	reasonGreaterThan:      "%q is greater than %q",
	reasonGreaterThanEqual: "%q is greater than or equal to %q",
	reasonMajor:            "%q does not have same major version as %q",
	reasonMajorMinor:       "%q does not have same major and minor version as %q",
	reasonCaretMinor:       "%q does not have same minor version as %q. Expected minor versions to match when constraint major version is 0",
	reasonCaretMinorZero:   "%q does not have same minor version as %q",
	reasonCaretPatch:       "%q does not equal %q. Expect version and constraint to equal when major and minor versions are 0",
}

// err builds the error describing a failed check.
func (r failReason) err(v *Version, c *constraint) error {
	if r == reasonPrerelease {
		return fmt.Errorf(reasonFormats[reasonPrerelease], v)
	}
	return fmt.Errorf(reasonFormats[r], v, c.orig)
}

// errConstraintParser is returned when a constraint has the shape of one but
// holds a version that cannot be built.
var errConstraintParser = errors.New("constraint parser error")

// isConstraintSpace reports if b is one of the characters that separate the
// parts of a constraint string. This is the set \s matched in the regular
// expressions the constraint scanner replaced.
func isConstraintSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\f' || b == '\r'
}

// skipConstraintSpace returns s without its leading whitespace.
func skipConstraintSpace(s string) string {
	i := 0
	for i < len(s) && isConstraintSpace(s[i]) {
		i++
	}
	return s[i:]
}

// parseConstraintGroup scans an AND group of constraints, such as
// ">=2.1.x, <3.1.0". Constraints within a group are separated by whitespace, a
// comma, or a comma surrounded by whitespace.
func parseConstraintGroup(g string) ([]*constraint, error) {
	s := skipConstraintSpace(g)
	if s == "" {
		return nil, fmt.Errorf("improper constraint: %q", g)
	}

	// Most groups hold one or two constraints.
	result := make([]*constraint, 0, 2)
	for {
		c, rest, err := scanConstraint(s)
		if err != nil {
			return nil, err
		}
		result = append(result, c)

		trimmed := skipConstraintSpace(rest)
		if trimmed == "" {
			return result, nil
		}

		// Constraints must be separated. A comma may stand on its own, while
		// whitespace is recognised by rest having been shortened.
		switch {
		case trimmed[0] == ',':
			trimmed = skipConstraintSpace(trimmed[1:])
		case len(trimmed) == len(rest):
			return nil, fmt.Errorf("improper constraint: %q", g)
		}

		if trimmed == "" {
			return nil, fmt.Errorf("improper constraint: %q", g)
		}
		s = trimmed
	}
}

// scanConstraint reads a single constraint from the front of s, which must not
// have leading whitespace, and returns it along with the unread remainder.
func scanConstraint(s string) (*constraint, string, error) {
	op, rest := scanConstraintOp(s)
	rest = skipConstraintSpace(rest)

	segs, n, pre, metadata, after, ok := scanConstraintVersion(rest)
	if !ok {
		return nil, "", fmt.Errorf("improper constraint: %q", s)
	}

	c, err := newConstraint(op, rest[:len(rest)-len(after)], segs, n, pre, metadata)
	if err != nil {
		return nil, "", err
	}
	return c, after, nil
}

// scanConstraintOp reads the operator from the front of s. An absent operator
// is an empty string, which is the same as =.
func scanConstraintOp(s string) (op, rest string) {
	if len(s) >= 2 {
		switch s[:2] {
		case "!=", ">=", "=>", "<=", "=<", "~>":
			return s[:2], s[2:]
		}
	}
	if len(s) >= 1 {
		switch s[0] {
		case '=', '>', '<', '~', '^':
			return s[:1], s[1:]
		}
	}
	return "", s
}

// scanConstraintVersion reads the version portion of a constraint from the
// front of s: an optional v, one to three segments of digits or a wildcard,
// then an optional prerelease and metadata. Anything that is not part of the
// version, such as a trailing dot, is left in rest for the caller to reject.
func scanConstraintVersion(s string) (segs [3]string, n int, pre, metadata, rest string, ok bool) {
	i := 0
	if i < len(s) && s[i] == 'v' {
		i++
	}

	for {
		start := i
		for i < len(s) && constraintSegChars[s[i]] {
			i++
		}
		if i == start {
			return segs, 0, "", "", "", false
		}
		segs[n] = s[start:i]
		n++

		// Only the first three segments belong to the version, and a dot is
		// only a separator when a segment follows it.
		if n == 3 || i+1 >= len(s) || s[i] != '.' || !constraintSegChars[s[i+1]] {
			break
		}
		i++
	}

	if i < len(s) && s[i] == '-' {
		if end := scanIdentifiers(s, i+1); end > i+1 {
			pre = s[i+1 : end]
			i = end
		}
	}

	if i < len(s) && s[i] == '+' {
		if end := scanIdentifiers(s, i+1); end > i+1 {
			metadata = s[i+1 : end]
			i = end
		}
	}

	return segs, n, pre, metadata, s[i:], true
}

// scanIdentifiers returns the index just past the dot separated identifiers
// starting at index i in s. Identifiers hold the characters [0-9A-Za-z-] and
// must not be empty. When there is no identifier at i the returned index is i.
func scanIdentifiers(s string, i int) int {
	end := i
	for {
		start := end
		for end < len(s) && allowedChars[s[end]] {
			end++
		}
		if end == start {
			return start
		}
		if end+1 < len(s) && s[end] == '.' && allowedChars[s[end+1]] {
			end++
			continue
		}
		return end
	}
}

// newConstraint builds a constraint from the scanned pieces of one. A wildcard
// segment makes the constraint dirty and drops the segments that follow it,
// which is how a constraint such as 1.x comes to hold the version 1.0.0.
func newConstraint(op, orig string, segs [3]string, n int, pre, metadata string) (*constraint, error) {
	cf, found := constraintOps[op]
	if !found {
		// scanConstraintOp only returns the operators in constraintOps, so we
		// should never get here.
		return nil, fmt.Errorf("improper constraint: %q", orig)
	}

	c := &constraint{
		orig:     orig,
		origfunc: op,
		cf:       cf,
	}

	// The version a constraint holds is never handed back to a caller, so the
	// text the constraint was scanned from stands in as the original.
	con := &Version{
		pre:      pre,
		original: orig,
	}

	// The length of the version that would be built, so that the limit
	// NewVersion applies is applied here too.
	verLen := len("0.0.0")
	if pre != "" {
		verLen += 1 + len(pre)
	}

	var err error
	switch {
	case isX(segs[0]):
		// A wildcard major version matches everything.
		c.dirty = true
	case n < 2 || isX(segs[1]):
		c.dirty = true
		c.minorDirty = true
		verLen += len(segs[0]) - 1
		if con.major, err = constraintSegment(segs[0]); err != nil {
			return nil, err
		}
	case n < 3 || isX(segs[2]):
		c.dirty = true
		c.patchDirty = true
		verLen += len(segs[0]) + len(segs[1]) - 2
		if con.major, err = constraintSegment(segs[0]); err != nil {
			return nil, err
		}
		if con.minor, err = constraintSegment(segs[1]); err != nil {
			return nil, err
		}
	default:
		// Metadata is only kept when the version is fully specified.
		con.metadata = metadata
		verLen = len(orig)
		if con.major, err = constraintSegment(segs[0]); err != nil {
			return nil, err
		}
		if con.minor, err = constraintSegment(segs[1]); err != nil {
			return nil, err
		}
		if con.patch, err = constraintSegment(segs[2]); err != nil {
			return nil, err
		}
	}

	if verLen > MaxVersionLen {
		return nil, errConstraintParser
	}

	// The characters in the prerelease are known to be valid. This catches a
	// numeric identifier with a leading 0, which is not a valid version.
	if pre != "" {
		if err = validatePrerelease(pre); err != nil {
			return nil, errConstraintParser
		}
	}

	c.con = con
	return c, nil
}

// constraintSegment parses a numeric segment of a constraint version under the
// same rules NewVersion applies to one.
func constraintSegment(s string) (uint64, error) {
	if !containsOnlyNum(s) {
		return 0, errConstraintParser
	}

	// A leading 0 is only valid in a version when NewVersion coerces it.
	if !CoerceNewVersion && len(s) > 1 && s[0] == '0' {
		return 0, errConstraintParser
	}

	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, errConstraintParser
	}
	return v, nil
}

func parseConstraint(c string) (*constraint, error) {
	if len(c) == 0 {
		// The special case where an empty string was passed in, which is
		// equivalent to * or >=0.0.0
		con, err := StrictNewVersion("0.0.0")
		if err != nil {

			// The version is a constant, so we should never get here.
			return nil, errConstraintParser
		}

		return &constraint{
			con:        con,
			orig:       c,
			origfunc:   "",
			cf:         constraintOps[""],
			minorDirty: false,
			patchDirty: false,
			dirty:      true,
		}, nil
	}

	s := skipConstraintSpace(c)
	if s == "" {
		return nil, fmt.Errorf("improper constraint: %q", c)
	}

	cs, rest, err := scanConstraint(s)
	if err != nil {
		return nil, err
	}
	if skipConstraintSpace(rest) != "" {
		return nil, fmt.Errorf("improper constraint: %q", c)
	}

	return cs, nil
}

// Constraint functions
func constraintNotEqual(v *Version, c *constraint, includePre bool) (bool, failReason) {
	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, reasonPrerelease
	}

	if c.dirty {
		if c.con.Major() != v.Major() {
			return true, reasonNone
		}
		if c.con.Minor() != v.Minor() && !c.minorDirty {
			return true, reasonNone
		} else if c.minorDirty {
			return false, reasonEqual
		} else if c.con.Patch() != v.Patch() && !c.patchDirty {
			return true, reasonNone
		} else if c.patchDirty {
			// Need to handle prereleases if present
			if v.Prerelease() != "" || c.con.Prerelease() != "" {
				eq := comparePrerelease(v.Prerelease(), c.con.Prerelease()) != 0
				if eq {
					return true, reasonNone
				}
				return false, reasonEqual
			}
			return false, reasonEqual
		}
	}

	eq := v.Equal(c.con)
	if eq {
		return false, reasonEqual
	}

	return true, reasonNone
}

func constraintGreaterThan(v *Version, c *constraint, includePre bool) (bool, failReason) {

	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, reasonPrerelease
	}

	var eq bool

	if !c.dirty {
		eq = v.Compare(c.con) == 1
		if eq {
			return true, reasonNone
		}
		return false, reasonLessThanEqual
	}

	if v.Major() > c.con.Major() {
		return true, reasonNone
	} else if v.Major() < c.con.Major() {
		return false, reasonLessThanEqual
	} else if c.minorDirty {
		// This is a range case such as >11. When the version is something like
		// 11.1.0 is it not > 11. For that we would need 12 or higher
		return false, reasonLessThanEqual
	} else if c.patchDirty {
		// This is for ranges such as >11.1. A version of 11.1.1 is not greater
		// which one of 11.2.1 is greater
		eq = v.Minor() > c.con.Minor()
		if eq {
			return true, reasonNone
		}
		return false, reasonLessThanEqual
	}

	// If we have gotten here we are not comparing pre-preleases and can use the
	// Compare function to accomplish that.
	eq = v.Compare(c.con) == 1
	if eq {
		return true, reasonNone
	}
	return false, reasonLessThanEqual
}

func constraintLessThan(v *Version, c *constraint, includePre bool) (bool, failReason) {
	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, reasonPrerelease
	}

	eq := v.Compare(c.con) < 0
	if eq {
		return true, reasonNone
	}
	return false, reasonGreaterThanEqual
}

func constraintGreaterThanEqual(v *Version, c *constraint, includePre bool) (bool, failReason) {

	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, reasonPrerelease
	}

	eq := v.Compare(c.con) >= 0
	if eq {
		return true, reasonNone
	}
	return false, reasonLessThan
}

func constraintLessThanEqual(v *Version, c *constraint, includePre bool) (bool, failReason) {
	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, reasonPrerelease
	}

	var eq bool

	if !c.dirty {
		eq = v.Compare(c.con) <= 0
		if eq {
			return true, reasonNone
		}
		return false, reasonGreaterThan
	}

	if v.Major() > c.con.Major() {
		return false, reasonGreaterThan
	} else if v.Major() == c.con.Major() && v.Minor() > c.con.Minor() && !c.minorDirty {
		return false, reasonGreaterThan
	}

	return true, reasonNone
}

// ~*, ~>* --> >= 0.0.0 (any)
// ~2, ~2.x, ~2.x.x, ~>2, ~>2.x ~>2.x.x --> >=2.0.0, <3.0.0
// ~2.0, ~2.0.x, ~>2.0, ~>2.0.x --> >=2.0.0, <2.1.0
// ~1.2, ~1.2.x, ~>1.2, ~>1.2.x --> >=1.2.0, <1.3.0
// ~1.2.3, ~>1.2.3 --> >=1.2.3, <1.3.0
// ~1.2.0, ~>1.2.0 --> >=1.2.0, <1.3.0
func constraintTilde(v *Version, c *constraint, includePre bool) (bool, failReason) {
	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, reasonPrerelease
	}

	if v.LessThan(c.con) {
		return false, reasonLessThan
	}

	// ~0.0.0 is a special case where all constraints are accepted. It's
	// equivalent to >= 0.0.0.
	if c.con.Major() == 0 && c.con.Minor() == 0 && c.con.Patch() == 0 &&
		!c.minorDirty && !c.patchDirty {
		return true, reasonNone
	}

	if v.Major() != c.con.Major() {
		return false, reasonMajor
	}

	if v.Minor() != c.con.Minor() && !c.minorDirty {
		return false, reasonMajorMinor
	}

	return true, reasonNone
}

// When there is a .x (dirty) status it automatically opts in to ~. Otherwise
// it's a straight =
func constraintTildeOrEqual(v *Version, c *constraint, includePre bool) (bool, failReason) {
	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, reasonPrerelease
	}

	if c.dirty {
		return constraintTilde(v, c, includePre)
	}

	eq := v.Equal(c.con)
	if eq {
		return true, reasonNone
	}

	return false, reasonNotEqual
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
func constraintCaret(v *Version, c *constraint, includePre bool) (bool, failReason) {
	// The existence of prereleases is checked at the group level and passed in.
	// Exit early if the version has a prerelease but those are to be ignored.
	if v.Prerelease() != "" && !includePre {
		return false, reasonPrerelease
	}

	// This less than handles prereleases
	if v.LessThan(c.con) {
		return false, reasonLessThan
	}

	var eq bool

	// ^ when the major > 0 is >=x.y.z < x+1
	if c.con.Major() > 0 || c.minorDirty {

		// ^ has to be within a major range for > 0. Everything less than was
		// filtered out with the LessThan call above. This filters out those
		// that greater but not within the same major range.
		eq = v.Major() == c.con.Major()
		if eq {
			return true, reasonNone
		}
		return false, reasonMajor
	}

	// ^ when the major is 0 and minor > 0 is >=0.y.z < 0.y+1
	if c.con.Major() == 0 && v.Major() > 0 {
		return false, reasonMajor
	}
	// If the con Minor is > 0 it is not dirty
	if c.con.Minor() > 0 || c.patchDirty {
		eq = v.Minor() == c.con.Minor()
		if eq {
			return true, reasonNone
		}
		return false, reasonCaretMinor
	}
	// ^ when the minor is 0 and minor > 0 is =0.0.z
	if c.con.Minor() == 0 && v.Minor() > 0 {
		return false, reasonCaretMinorZero
	}

	// At this point the major is 0 and the minor is 0 and not dirty. The patch
	// is not dirty so we need to check if they are equal. If they are not equal
	eq = c.con.Patch() == v.Patch()
	if eq {
		return true, reasonNone
	}
	return false, reasonCaretPatch
}

func isX(x string) bool {
	switch x {
	case "x", "*", "X":
		return true
	default:
		return false
	}
}

func rewriteRange(i string) string {
	// A range needs a hyphen between the two versions. Scanning for one is far
	// cheaper than running the regex over a constraint that cannot hold a
	// range. A hyphen within a prerelease still falls through to the regex.
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
