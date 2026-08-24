package semver

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// This file keeps the regular expression based constraint parser that the hand
// written scanner replaced. It is only ever compiled into the tests.
// FuzzNewConstraintDifferential runs the two against each other so that the
// scanner stays behaviour preserving: the same inputs are accepted, the parsed
// constraints hold the same values, and checking a version against them gives
// the same answer.

const refCvRegex string = `v?([0-9|x|X|\*]+)(\.[0-9|x|X|\*]+)?(\.[0-9|x|X|\*]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?`

var (
	refConstraintRegex      *regexp.Regexp
	refConstraintRangeRegex *regexp.Regexp
	refFindConstraintRegex  *regexp.Regexp
	refValidConstraintRegex *regexp.Regexp
)

func init() {
	ops := `=||!=|>|<|>=|=>|<=|=<|~|~>|\^`

	refConstraintRegex = regexp.MustCompile(fmt.Sprintf(
		`^\s*(%s)\s*(%s)\s*$`,
		ops,
		refCvRegex))

	refConstraintRangeRegex = regexp.MustCompile(fmt.Sprintf(
		`\s*(%s)\s+-\s+(%s)\s*`,
		refCvRegex, refCvRegex))

	refFindConstraintRegex = regexp.MustCompile(fmt.Sprintf(
		`(%s)\s*(%s)`,
		ops,
		refCvRegex))

	refValidConstraintRegex = regexp.MustCompile(fmt.Sprintf(
		`^(\s*(%s)\s*(%s)\s*)((?:\s+|,\s*)(%s)\s*(%s)\s*)*$`,
		ops,
		refCvRegex,
		ops,
		refCvRegex))
}

func refNewConstraint(c string) (*Constraints, error) {
	if len(c) > MaxConstraintLen {
		return nil, ErrConstraintTooLong
	}

	// Rewrite - ranges into a comparison operation.
	c = refRewriteRange(c)

	ors := strings.Split(c, "||")
	if len(ors) > MaxConstraintGroups {
		return nil, ErrTooManyConstraintGroups
	}
	lenors := len(ors)
	or := make([][]*constraint, lenors)
	hasPre := make([]bool, lenors)
	for k, v := range ors {
		// Validate the segment
		if !refValidConstraintRegex.MatchString(v) {
			return nil, fmt.Errorf("improper constraint: %q", v)
		}

		cs := refFindConstraintRegex.FindAllString(v, -1)
		if cs == nil {
			cs = append(cs, v)
		}
		result := make([]*constraint, len(cs))
		for i, s := range cs {
			pc, err := refParseConstraint(s)
			if err != nil {
				return nil, err
			}

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

func refParseConstraint(c string) (*constraint, error) {
	if len(c) > 0 {
		m := refConstraintRegex.FindStringSubmatch(c)
		if m == nil {
			return nil, fmt.Errorf("improper constraint: %q", c)
		}

		cs := &constraint{
			orig:     m[2],
			origfunc: m[1],
			cf:       constraintOps[m[1]],
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
			return nil, errors.New("constraint parser error")
		}

		cs.con = con
		cs.minorDirty = minorDirty
		cs.patchDirty = patchDirty
		cs.dirty = dirty

		return cs, nil
	}

	con, err := StrictNewVersion("0.0.0")
	if err != nil {
		return nil, errors.New("constraint parser error")
	}

	cs := &constraint{
		con:        con,
		orig:       c,
		origfunc:   "",
		cf:         constraintOps[""],
		minorDirty: false,
		patchDirty: false,
		dirty:      true,
	}
	return cs, nil
}

func refRewriteRange(i string) string {
	m := refConstraintRangeRegex.FindAllStringSubmatch(i, -1)
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

// sameConstraints compares two parsed constraint sets field by field. The
// original string held on each constraint version is not compared: it is not
// reachable through the public API and the two parsers build it differently.
func sameConstraints(a, b *Constraints) error {
	if len(a.constraints) != len(b.constraints) {
		return fmt.Errorf("%d or groups, want %d", len(a.constraints), len(b.constraints))
	}
	for i := range a.constraints {
		if a.containsPre[i] != b.containsPre[i] {
			return fmt.Errorf("group %d containsPre %t, want %t", i, a.containsPre[i], b.containsPre[i])
		}
		if len(a.constraints[i]) != len(b.constraints[i]) {
			return fmt.Errorf("group %d has %d constraints, want %d", i,
				len(a.constraints[i]), len(b.constraints[i]))
		}
		for j := range a.constraints[i] {
			x, y := a.constraints[i][j], b.constraints[i][j]
			if x.orig != y.orig || x.origfunc != y.origfunc {
				return fmt.Errorf("constraint %d.%d is %q%q, want %q%q", i, j,
					x.origfunc, x.orig, y.origfunc, y.orig)
			}
			if x.dirty != y.dirty || x.minorDirty != y.minorDirty || x.patchDirty != y.patchDirty {
				return fmt.Errorf("constraint %d.%d dirty %t/%t/%t, want %t/%t/%t", i, j,
					x.dirty, x.minorDirty, x.patchDirty, y.dirty, y.minorDirty, y.patchDirty)
			}
			if !sameConstraintVersion(x.con, y.con) {
				return fmt.Errorf("constraint %d.%d version %#v, want %#v", i, j, x.con, y.con)
			}
		}
	}
	return nil
}

// sameConstraintVersion compares the values of the version a constraint holds.
// The original string is left out: the two parsers build it differently and it
// is not reachable through the public API.
func sameConstraintVersion(a, b *Version) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.major == b.major && a.minor == b.minor && a.patch == b.patch &&
		a.pre == b.pre && a.metadata == b.metadata
}

// constraintCorpus holds constraint strings covering the operators, the
// wildcard forms, the separators, and a range of malformed input.
var constraintCorpus = []string{
	"", " ", "*", "x", "X", "1.x", "1.2.x", "x.2.3", "1.x.3",
	"=1.2.3", "= 1.2.3", "==1.2.3", "!=1.2.3", ">1.2.3", "<1.2.3",
	">=1.2.3", "=>1.2.3", "<=1.2.3", "=<1.2.3", "~1.2.3", "~>1.2.3",
	"^1.2.3", "^0.0.3", "^0.2", "~0.0.0", "v1.2.3", "V1.2.3",
	">=2.1.x, <3.1.0", ">= 2.1.x , < 3.1.0", ">=2.1.x <3.1.0",
	"~2.0.0 || =3.1.0", "1.0.0 - 2.0.0", "1.0.0 -2.0.0", "1.0.0- 2.0.0",
	"1.2.3-alpha", ">=1.2.3-alpha.1", "1.2.3+meta", "=1.2.3-alpha+meta",
	"1.2.3-", "1.2.3+", "1.2.3-alpha.", "1.2.3.4", "1.2.3.4.5.6",
	"1|2", "1.2|3", "01.2.3", "1.02.3", "1.2.3-01",
	">= 1.2 || < 3, > 4", ",1.0", "1.0,", "1.0,,2.0", "1.0<2.0",
	"> = 1.2", ">=", "~", "^", "foo", "lorem ipsum", "1.2.3 ", " 1.2.3",
	"18446744073709551616.0.0", "1.18446744073709551616", "x.18446744073709551616",
	"|| 1.2.3", "1.2.3 ||", "1.2.3 || || 2.0.0", "*.*.*", "1.*.3",
}

func FuzzNewConstraintDifferential(f *testing.F) {
	for _, c := range constraintCorpus {
		f.Add(c)
	}
	for _, v := range versionCorpus {
		f.Add(v)
	}

	// Versions the two constraint sets are checked against.
	var versions []*Version
	for _, v := range versionCorpus {
		if sv, err := NewVersion(v); err == nil {
			versions = append(versions, sv)
		}
	}

	f.Fuzz(func(t *testing.T, c string) {
		// Both parsers build their versions under the rules NewVersion
		// applies, so the coercion setting has to be covered too.
		for _, coerce := range []bool{true, false} {
			CoerceNewVersion = coerce
			checkConstraintPair(t, c, coerce, versions)
		}
		CoerceNewVersion = true
	})
}

func checkConstraintPair(t *testing.T, c string, coerce bool, versions []*Version) {
	t.Helper()

	{
		got, gotErr := NewConstraint(c)
		want, wantErr := refNewConstraint(c)

		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("NewConstraint(%q) with coerce=%t: error %v, want %v", c, coerce, gotErr, wantErr)
		}
		if gotErr != nil {
			// Both failed. The messages are allowed to differ, but a
			// sentinel error must still be the same sentinel.
			if errors.Is(wantErr, ErrConstraintTooLong) && !errors.Is(gotErr, ErrConstraintTooLong) {
				t.Fatalf("NewConstraint(%q): error %v, want ErrConstraintTooLong", c, gotErr)
			}
			if errors.Is(wantErr, ErrTooManyConstraintGroups) && !errors.Is(gotErr, ErrTooManyConstraintGroups) {
				t.Fatalf("NewConstraint(%q): error %v, want ErrTooManyConstraintGroups", c, gotErr)
			}
			return
		}

		if err := sameConstraints(got, want); err != nil {
			t.Fatalf("NewConstraint(%q) with coerce=%t: %s", c, coerce, err)
		}

		if got.String() != want.String() {
			t.Fatalf("NewConstraint(%q).String() = %q, want %q", c, got.String(), want.String())
		}

		for _, v := range versions {
			for _, pre := range []bool{false, true} {
				got.IncludePrerelease = pre
				want.IncludePrerelease = pre

				if a, b := got.Check(v), want.Check(v); a != b {
					t.Fatalf("NewConstraint(%q).Check(%s) with prerelease %t = %t, want %t",
						c, v, pre, a, b)
				}

				a, aerrs := got.Validate(v)
				b, berrs := want.Validate(v)
				if a != b {
					t.Fatalf("NewConstraint(%q).Validate(%s) with prerelease %t = %t, want %t",
						c, v, pre, a, b)
				}
				if fmt.Sprint(aerrs) != fmt.Sprint(berrs) {
					t.Fatalf("NewConstraint(%q).Validate(%s) with prerelease %t errors %v, want %v",
						c, v, pre, aerrs, berrs)
				}
			}
		}
	}
}
