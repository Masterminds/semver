package semver

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// oldLooseCoerce is a verbatim copy of the pre-rewrite (regex) loose version
// parser. It is kept here as a reference oracle so the hand-written parser
// can be differentially tested against the old behaviour before the regex was
// removed from the parse path.
func oldLooseCoerce(v string) (*Version, error) {
	if len(v) > MaxVersionLen {
		return nil, ErrVersionTooLong
	}
	m := looseVersionRegex.FindStringSubmatch(v)
	if m == nil {
		return nil, ErrInvalidSemVer
	}
	sv := &Version{metadata: m[8], pre: m[5], original: v}
	var err error
	sv.major, err = strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing version segment: %w", err)
	}
	if m[2] != "" {
		sv.minor, err = strconv.ParseUint(strings.TrimPrefix(m[2], "."), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing version segment: %w", err)
		}
	}
	if m[3] != "" {
		sv.patch, err = strconv.ParseUint(strings.TrimPrefix(m[3], "."), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing version segment: %w", err)
		}
	}
	if sv.pre != "" {
		if err = validatePrerelease(sv.pre); err != nil {
			return nil, err
		}
	}
	if sv.metadata != "" {
		if err = validateMetadata(sv.metadata); err != nil {
			return nil, err
		}
	}
	return sv, nil
}

// checkCoerceEqual reports whether the old and new loose parsers agree on an
// input, comparing both the parsed value and the error identity. Used by the
// differential test and the fuzz target.
func checkCoerceEqual(t testing.TB, in string) {
	t.Helper()
	o, oe := oldLooseCoerce(in)
	n, ne := coerceNewVersion(in)
	if (oe == nil) != (ne == nil) {
		t.Fatalf("error disagreement for %q (oldErr=%v newErr=%v)", in, oe, ne)
	}
	if oe != nil {
		return // both rejected; value not compared
	}
	if o.Major() != n.Major() || o.Minor() != n.Minor() || o.Patch() != n.Patch() ||
		o.Prerelease() != n.Prerelease() || o.Metadata() != n.Metadata() {
		t.Fatalf("value disagreement for %q\n  old=(%d.%d.%d pre=%q meta=%q)\n  new=(%d.%d.%d pre=%q meta=%q)",
			in, o.Major(), o.Minor(), o.Patch(), o.Prerelease(), o.Metadata(),
			n.Major(), n.Minor(), n.Patch(), n.Prerelease(), n.Metadata())
	}
}

// TestDifferentialNewVersionCoerce sweeps a large deterministic corpus of
// semver-ish strings and asserts the hand-written parser matches the old
// regex parser exactly (value + error identity).
func TestDifferentialNewVersionCoerce(t *testing.T) {
	segs := []string{"0", "1", "10", "99", "2147483648", "4294967296", "01"}
	divs := []string{"", ".", ".."}
	preSuffix := []string{"", "-alpha", "-alpha.1", "-0abc", "-a.b.c", "-rc.01", "-x.Y.0", "-a.b.", "0", "-01.2"}
	metaSuffix := []string{"", "+meta", "+meta.data", "+0meta", "+a.b.", "+meta..meta"}

	count := 0
	for _, s0 := range segs {
		for _, d1 := range divs {
			for _, s1 := range segs {
				for _, d2 := range divs {
					for _, s2 := range segs {
						base := s0 + d1 + s1 + d2 + s2
						for _, p := range preSuffix {
							for _, mi := range metaSuffix {
								v := base + p + mi
								checkCoerceEqual(t, v)
								checkCoerceEqual(t, "v"+v)
								count += 2
							}
						}
					}
				}
			}
		}
	}
	for _, v := range []string{"", "v", "1.2.3", "v1.2.3", "1.0.0-", "1.0.0+", "1.2.", "1.", " 1.2.3", "1.2 .3", "-1.2.3", "1.2.3-alpha 1"} {
		checkCoerceEqual(t, v)
		count++
	}
	t.Logf("differential corpus size=%d, all matched", count)
}

// FuzzNewVersionCoerce is a differential fuzz target: it feeds generated
// semver-ish strings to both the old regex parser and the new hand-written
// parser and requires them to agree on value and error identity.
func FuzzNewVersionCoerce(f *testing.F) {
	for _, s := range []string{"1.0.0", "v1.2.3", "1.2", "1.0.0-alpha.1+meta.data", "01.01.01", "1.2.3-rc.01", "1.2.3+meta..meta", "12.3.4.1234"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		if len(in) > MaxVersionLen {
			in = in[:MaxVersionLen]
		}
		checkCoerceEqual(t, in)
	})
}
