package semver

// This file contains verbatim copies of the pre-rewrite (regex) constraint
// parser plus a differential test comparing it to the hand-written
// implementation. The oracle is the ground-truth behaviour that Steps 4b/4c/4d
// must preserve.

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// oldRewriteRange / oldSplitGroup / oldParseConstraint are verbatim copies of
// the master (regex) implementation captured before Step 4 rewrote the
// tokenizer. They are the oracle.

func oldRewriteRange(i string) string {
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

// oldSplitGroup validates an OR group with validConstraintRegex and returns the
// split atoms (exactly as findConstraintRegex did in master).
func oldSplitGroup(group string) ([]string, error) {
	if !validConstraintRegex.MatchString(group) {
		return nil, fmt.Errorf("improper constraint: %q", group)
	}
	cs := findConstraintRegex.FindAllString(group, -1)
	if cs == nil {
		cs = append(cs, group)
	}
	return cs, nil
}

func oldParseConstraint(c string) (*constraint, error) {
	if len(c) > 0 {
		m := constraintRegex.FindStringSubmatch(c)
		if m == nil {
			return nil, fmt.Errorf("improper constraint: %q", c)
		}
		cs := &constraint{orig: m[2], origfunc: m[1]}
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
		cs.cf = constraintOps[cs.origfunc]
		cs.pf = constraintPreds[cs.origfunc]
		return cs, nil
	}
	con, err := StrictNewVersion("0.0.0")
	if err != nil {
		return nil, errors.New("constraint parser error")
	}
	cs := &constraint{con: con, orig: c, origfunc: "", dirty: true}
	cs.cf = constraintOps[cs.origfunc]
	cs.pf = constraintPreds[cs.origfunc]
	return cs, nil
}

// checkConstraintsEqual asserts two parsed constraint sets are behaviourally
// identical (operator, original string, dirty flags, the constraint version,
// and the Check outcome across a version corpus).
func checkConstraintsEqual(t *testing.T, input string) {
	t.Helper()
	oldCS, oldErr := oldNewConstraint(input)
	newCS, newErr := NewConstraint(input)
	if (oldErr == nil) != (newErr == nil) {
		t.Fatalf("constraint-error mismatch for %q (oldErr=%v newErr=%v)", input, oldErr, newErr)
	}
	if oldErr != nil {
		return
	}
	if len(oldCS.constraints) != len(newCS.constraints) {
		t.Fatalf("group count mismatch for %q", input)
	}
	if !reflect.DeepEqual(oldCS.containsPre, newCS.containsPre) {
		t.Fatalf("containsPre mismatch for %q: old=%v new=%v", input, oldCS.containsPre, newCS.containsPre)
	}
	for g := range oldCS.constraints {
		if len(oldCS.constraints[g]) != len(newCS.constraints[g]) {
			t.Fatalf("constraint count mismatch in group %d for %q", g, input)
		}
		for i := range oldCS.constraints[g] {
			o := oldCS.constraints[g][i]
			n := newCS.constraints[g][i]
			if o.origfunc != n.origfunc {
				t.Fatalf("origfunc mismatch for %q: old=%q new=%q", input, o.origfunc, n.origfunc)
			}
			if o.orig != n.orig {
				t.Fatalf("orig mismatch for %q: old=%q new=%q", input, o.orig, n.orig)
			}
			if o.dirty != n.dirty || o.minorDirty != n.minorDirty || o.patchDirty != n.patchDirty {
				t.Fatalf("dirty flags mismatch for %q: old=(d=%v md=%v pd=%v) new=(d=%v md=%v pd=%v)",
					input, o.dirty, o.minorDirty, o.patchDirty, n.dirty, n.minorDirty, n.patchDirty)
			}
			if o.con.Major() != n.con.Major() || o.con.Minor() != n.con.Minor() || o.con.Patch() != n.con.Patch() ||
				o.con.Prerelease() != n.con.Prerelease() || o.con.Metadata() != n.con.Metadata() ||
				o.con.String() != n.con.String() {
				t.Fatalf("constraint-version mismatch for %q: old=%q new=%q", input, o.con.String(), n.con.String())
			}
		}
	}
	// Behaviour: Check across a version corpus.
	vers := []string{"0.0.0", "1.0.0", "1.2.3", "2.0.0", "2.1.0", "2.1.4", "2.2.0", "3.1.0", "4.0.0", "10.0.0", "2.1.0-alpha", "2.2.0-beta.1"}
	for _, s := range vers {
		v, err := NewVersion(s)
		if err != nil {
			continue
		}
		if oldCS.Check(v) != newCS.Check(v) {
			t.Fatalf("Check(%q) mismatch for %q: old=%v new=%v", s, input, oldCS.Check(v), newCS.Check(v))
		}
	}
}

func oldNewConstraint(str string) (*Constraints, error) {
	if len(str) > MaxConstraintLen {
		return nil, ErrConstraintTooLong
	}
	c := oldRewriteRange(str)
	ors := strings.Split(c, "||")
	if len(ors) > MaxConstraintGroups {
		return nil, ErrTooManyConstraintGroups
	}
	or := make([][]*constraint, len(ors))
	hasPre := make([]bool, len(ors))
	for k, v := range ors {
		atoms, err := oldSplitGroup(v)
		if err != nil {
			return nil, err
		}
		res := make([]*constraint, len(atoms))
		for i, s := range atoms {
			pc, err := oldParseConstraint(s)
			if err != nil {
				return nil, err
			}
			if pc.con.pre != "" {
				hasPre[k] = true
			}
			res[i] = pc
		}
		or[k] = res
	}
	return &Constraints{constraints: or, containsPre: hasPre}, nil
}

// TestDifferentialNewConstraint is the correctness gate for Step 4: it forces
// the old (regex) and new (hand-written) constraint parsers to agree on parse
// result and Check outcome.
func TestDifferentialNewConstraint(t *testing.T) {
	cases := []string{
		"2.3.5", "=1.5", "<=1.2", "=>1.2", "=<1.2", "> 1.3", "< 1.4.1", "!=2.0.0",
		"v2.3.5", "=v1.2", "~1.0.0", "~>1.0.0", "~1.2.3", "~2", "~* ", "~2.*",
		"^1.2.3", "^0.0", "^0.2", "^0.2.3", "^1", "^2", "^0.x",
		"1.x", "2.x", "2.1.x", "2.x.x", "x", "*", "1.*", "2.*.0", "=2.x", "^2.x", "~1.x",
		"2.1", "v1.2", ">= 1.1", ">40.50.60, < 50.70", ">=2.1.x, <3.1.0",
		"~2.0.0 || =3.1.0", "2.* || 5.*", "1.0 || 1.1 || 1.2 || 1.3",
		"2.1.x || 4.x", "~1.0.0-rc.1 || ^2.0.0-beta.2", ">= 1.0.0-0.3.7 || <= 2.0.0",
		"  2.1  ", "2.1 , 3.2", ">= 1.0, <= 2.0, >= 0.5",
		"1 - 2", "2 - 3", "4.0.0 - 5.1", "2 - 3, 4 - 5", "1.0.0 - 2.0.0",
		"1.0.0 - 2.0.0, 3.0.0", "~1.x.x", "=0.1.x, =0.2.x", "* - 2",
		"2.0.0+build", "1.2.3-alpha", "2.1.4-rc.1", "=1.2.3-x.y.z",
		// malformed (both should error)
		"foo", "= ", ">= ", "~ ", "^ ", "+1", "~=1.0.0", "2..3", "1.2.3.4",
		"2.", ".", "||", "|| || ||", "2.1 = 3.2", "x.x.x.x", "1.x-2.x",
		"> 1.3 > 2.4", "2.1 2.1", ">= 1.0.0.0", "1.2.3 +build",
	}
	for _, c := range cases {
		checkConstraintsEqual(t, c)
	}
}

// FuzzNewConstraintDifferential is a differential fuzz of the constraint
// parser: the old regex implementation and the new hand-written one must agree
// on parse result and Check outcome.
func FuzzNewConstraintDifferential(f *testing.F) {
	for _, s := range []string{
		"2.3.5", ">= 1.1", ">40.50.60, < 50.70", ">=2.1.x, <3.1.0",
		"~2.0.0 || =3.1.0", "2 - 3", "2.* || 5.*", "~1.x.x", "^0.2.3",
		"2.1.4-rc.1", "  2.1  ", "2.1 , 3.2", "* - 2", "foo", ">= ", "2..3",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		if len(in) > MaxConstraintLen {
			in = in[:MaxConstraintLen]
		}
		checkConstraintsEqual(t, in)
	})
}
