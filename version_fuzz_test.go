package semver

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// This file keeps the regular expression based implementation of NewVersion
// that the hand written parser replaced. It is only ever compiled into the
// tests. FuzzNewVersion runs the two against each other so that the parser
// stays behaviour preserving, including which error is returned.

// refSemVerRegex is the regular expression that was used to parse a semantic
// version. This is not the official regex from the semver spec. It has been
// modified to allow for loose handling where versions like 2.1 are detected.
const refSemVerRegex string = `v?(0|[1-9]\d*)(?:\.(0|[1-9]\d*))?(?:\.(0|[1-9]\d*))?` +
	`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
	`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?`

// refLooseSemVerRegex is a regular expression that lets invalid semver
// expressions through with enough detail that certain errors can be checked
// for.
const refLooseSemVerRegex string = `v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?`

var (
	refVersionRegex      = regexp.MustCompile("^" + refSemVerRegex + "$")
	refLooseVersionRegex = regexp.MustCompile("^" + refLooseSemVerRegex + "$")
)

func refNewVersion(v string) (*Version, error) {
	if len(v) > MaxVersionLen {
		return nil, ErrVersionTooLong
	}
	if CoerceNewVersion {
		return refCoerceNewVersion(v)
	}
	m := refVersionRegex.FindStringSubmatch(v)
	if m == nil {

		// Disabling detailed errors is first so that it is in the fast path.
		if !DetailedNewVersionErrors {
			return nil, ErrInvalidSemVer
		}

		// Check for specific errors with the semver string and return a more detailed
		// error.
		m = refLooseVersionRegex.FindStringSubmatch(v)
		if m == nil {
			return nil, ErrInvalidSemVer
		}
		err := refValidateVersion(m)
		if err != nil {
			return nil, err
		}
		return nil, ErrInvalidSemVer
	}

	sv := &Version{
		metadata: m[5],
		pre:      m[4],
		original: v,
	}

	var err error
	sv.major, err = strconv.ParseUint(m[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing version segment: %w", err)
	}

	if m[2] != "" {
		sv.minor, err = strconv.ParseUint(m[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing version segment: %w", err)
		}
	} else {
		sv.minor = 0
	}

	if m[3] != "" {
		sv.patch, err = strconv.ParseUint(m[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing version segment: %w", err)
		}
	} else {
		sv.patch = 0
	}

	// Perform some basic due diligence on the extra parts to ensure they are
	// valid.

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

func refCoerceNewVersion(v string) (*Version, error) {
	m := refLooseVersionRegex.FindStringSubmatch(v)
	if m == nil {
		return nil, ErrInvalidSemVer
	}

	sv := &Version{
		metadata: m[8],
		pre:      m[5],
		original: v,
	}

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
	} else {
		sv.minor = 0
	}

	if m[3] != "" {
		sv.patch, err = strconv.ParseUint(strings.TrimPrefix(m[3], "."), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing version segment: %w", err)
		}
	} else {
		sv.patch = 0
	}

	// Perform some basic due diligence on the extra parts to ensure they are
	// valid.

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

// refValidateVersion checks for common validation issues but may not catch all errors
func refValidateVersion(m []string) error {
	var err error
	var v string
	if m[1] != "" {
		if len(m[1]) > 1 && m[1][0] == '0' {
			return ErrSegmentStartsZero
		}
		_, err = strconv.ParseUint(m[1], 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing version segment: %w", err)
		}
	}

	if m[2] != "" {
		v = strings.TrimPrefix(m[2], ".")
		if len(v) > 1 && v[0] == '0' {
			return ErrSegmentStartsZero
		}
		_, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing version segment: %w", err)
		}
	}

	if m[3] != "" {
		v = strings.TrimPrefix(m[3], ".")
		if len(v) > 1 && v[0] == '0' {
			return ErrSegmentStartsZero
		}
		_, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			return fmt.Errorf("error parsing version segment: %w", err)
		}
	}

	if m[5] != "" {
		if err = validatePrerelease(m[5]); err != nil {
			return err
		}
	}

	if m[8] != "" {
		if err = validateMetadata(m[8]); err != nil {
			return err
		}
	}

	return nil
}

// sameErr reports if two errors are the same for the purposes of the
// differential test. Sentinel errors must be identical. Wrapped errors, which
// only come from strconv, must carry the same message.
func sameErr(a, b error) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if errors.Is(a, b) || errors.Is(b, a) {
		return true
	}
	return a.Error() == b.Error()
}

func sameVersion(a, b *Version) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.major == b.major && a.minor == b.minor && a.patch == b.patch &&
		a.pre == b.pre && a.metadata == b.metadata && a.original == b.original
}

// versionCorpus holds inputs that exercise the interesting edges of the
// parser. It seeds the differential fuzz targets.
var versionCorpus = []string{
	"1.2.3", "v1.2.3", "1.2", "1", "v1", "",
	"1.2.3-alpha", "1.2.3-alpha.1", "1.2.3-alpha.beta", "1.2.3-0abc123",
	"1.2.3+meta", "1.2.3+meta.data", "1.2.3-alpha.1+meta.data",
	"01.2.3", "1.02.3", "1.2.03", "1.2.3-01", "1.2.3-alpha.01",
	"1.2.3-", "1.2.3+", "1.2.3-alpha.", "1.2.3+meta.", "1.2.3-alpha..1",
	"1.2.", "1.", ".1.2", "1..2", "1.2.3.4", "-1.2.3", "+1.2.3",
	"1.2.3-alpha_beta", "1.2.3 ", " 1.2.3", "1.2 .3", "\n1.2",
	"18446744073709551615.0.0", "18446744073709551616.0.0",
	"0.0.0", "0.0.0-0", "20221209-update-renovatejson-v4",
	"9.8.7+meta+meta", "1.2.31----RC-SNAPSHOT.12.09.1--.12+788",
	"v1.2.0-x.Y.0+metadata-width-hypen", "1.2-5", "1-2.3", "1.2+3-4",
	"x", "1.x", "*", "v", "vv1.2.3",
}

// refStrictNewVersion is the splitting StrictNewVersion used before it walked
// its parts in place.
func refStrictNewVersion(v string) (*Version, error) {
	if len(v) == 0 {
		return nil, ErrEmptyString
	}

	if len(v) > MaxVersionLen {
		return nil, ErrVersionTooLong
	}

	// Split the parts into [0]major, [1]minor, and [2]patch,prerelease,build
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil, ErrInvalidSemVer
	}

	sv := &Version{
		original: v,
	}

	// Extract build metadata
	if strings.Contains(parts[2], "+") {
		extra := strings.SplitN(parts[2], "+", 2)
		sv.metadata = extra[1]
		parts[2] = extra[0]
		if err := validateMetadata(sv.metadata); err != nil {
			return nil, err
		}
	}

	// Extract build prerelease
	if strings.Contains(parts[2], "-") {
		extra := strings.SplitN(parts[2], "-", 2)
		sv.pre = extra[1]
		parts[2] = extra[0]
		if err := validatePrerelease(sv.pre); err != nil {
			return nil, err
		}
	}

	// Validate the number segments are valid. This includes only having positive
	// numbers and no leading 0's.
	for _, p := range parts {
		if !containsOnlyNum(p) {
			return nil, ErrInvalidCharacters
		}

		if len(p) > 1 && p[0] == '0' {
			return nil, ErrSegmentStartsZero
		}
	}

	// Extract major, minor, and patch
	var err error
	sv.major, err = strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return nil, err
	}

	sv.minor, err = strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return nil, err
	}

	sv.patch, err = strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return nil, err
	}

	return sv, nil
}

func FuzzStrictNewVersionDifferential(f *testing.F) {
	for _, v := range versionCorpus {
		f.Add(v)
	}

	f.Fuzz(func(t *testing.T, v string) {
		got, gotErr := StrictNewVersion(v)
		want, wantErr := refStrictNewVersion(v)

		if !sameErr(gotErr, wantErr) {
			t.Fatalf("StrictNewVersion(%q): error %v, want %v", v, gotErr, wantErr)
		}
		if !sameVersion(got, want) {
			t.Fatalf("StrictNewVersion(%q): %#v, want %#v", v, got, want)
		}
	})
}

func FuzzNewVersionDifferential(f *testing.F) {
	for _, v := range versionCorpus {
		f.Add(v)
	}

	f.Fuzz(func(t *testing.T, v string) {
		for _, coerce := range []bool{true, false} {
			for _, detailed := range []bool{true, false} {
				// The detailed setting only applies when versions are not
				// coerced.
				if coerce && !detailed {
					continue
				}

				CoerceNewVersion = coerce
				DetailedNewVersionErrors = detailed

				got, gotErr := NewVersion(v)
				want, wantErr := refNewVersion(v)

				if !sameErr(gotErr, wantErr) {
					t.Fatalf("NewVersion(%q) with coerce=%t detailed=%t: error %v, want %v",
						v, coerce, detailed, gotErr, wantErr)
				}
				if !sameVersion(got, want) {
					t.Fatalf("NewVersion(%q) with coerce=%t detailed=%t: %#v, want %#v",
						v, coerce, detailed, got, want)
				}
			}
		}

		CoerceNewVersion = true
		DetailedNewVersionErrors = true
	})
}
