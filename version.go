package semver

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// The compiled version of the regex created at init() is cached here so it
// only needs to be created once.
var versionRegex *regexp.Regexp

var (
	// ErrInvalidSemVer is returned a version is found to be invalid when
	// being parsed.
	ErrInvalidSemVer = errors.New("Invalid Semantic Version")
)

// SemVerRegex id the regular expression used to parse a semantic version.
const SemVerRegex string = `v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?`

// Version represents a single semantic version.
type Version struct {
	major, minor, patch int64
	pre                 string
	metadata            string
	original            string
}

func init() {
	versionRegex = regexp.MustCompile("^" + SemVerRegex + "$")
}

// NewVersion parses a given version and returns an instance of Version or
// an error if unable to parse the version.
func NewVersion(v string) (*Version, error) {
	m := versionRegex.FindStringSubmatch(v)
	if m == nil {
		return nil, ErrInvalidSemVer
	}

	sv := &Version{
		metadata: m[8],
		pre:      m[5],
		original: v,
	}

	var temp int64
	temp, err := strconv.ParseInt(m[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("Error parsing version segment: %s", err)
	}
	sv.major = temp

	if m[2] != "" {
		temp, err = strconv.ParseInt(strings.TrimPrefix(m[2], "."), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Error parsing version segment: %s", err)
		}
		sv.minor = temp
	} else {
		sv.minor = 0
	}

	if m[3] != "" {
		temp, err = strconv.ParseInt(strings.TrimPrefix(m[3], "."), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Error parsing version segment: %s", err)
		}
		sv.patch = temp
	} else {
		sv.patch = 0
	}

	return sv, nil
}

// String converts a Version object to a string.
// Note, if the original version contained a leading v this version will not.
// See the Original() method to retrieve the original value. Semantic Versions
// don't contain a leading v per the spec. Instead it's optional on
// impelementation.
func (v *Version) String() string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "%d.%d.%d", v.major, v.minor, v.patch)
	if v.pre != "" {
		fmt.Fprintf(&buf, "-%s", v.pre)
	}
	if v.metadata != "" {
		fmt.Fprintf(&buf, "+%s", v.metadata)
	}

	return buf.String()
}

// Original returns the original value passed in to be parsed.
func (v *Version) Original() string {
	return v.original
}

// Major returns the major version.
func (v *Version) Major() int64 {
	return v.major
}

// Minor returns the minor version.
func (v *Version) Minor() int64 {
	return v.minor
}

// Patch returns the patch version.
func (v *Version) Patch() int64 {
	return v.patch
}

// Prerelease returns the pre-release version.
func (v *Version) Prerelease() string {
	return v.pre
}

// Metadata returns the metadata on the version.
func (v *Version) Metadata() string {
	return v.metadata
}

// Increment version number,
// How can be one of: patch, minor, major, prerelease
func (v *Version) Inc(how string) bool {
	if how == "prerelease" {
		return v.IncPrelease()
	} else if how == "patch" {
		return v.IncPatch()
	} else if how == "minor" {
		return v.IncMinor()
	} else if how == "major" {
		return v.IncMajor()
	}
	return false
}

// Increment version number by the prerelease number.
// when version is 1.0.0-beta => 1.0.0-beta1
// when version is 1.0.0-beta2 => 1.0.0-beta3
func (v *Version) IncPrerelease() bool {
	if len(v.pre) == 0 {
		return false
	}
	r := regexp.MustCompile("^([a-z]+)([.-]?)([0-9]+)?")
	if r.MatchString(v.pre) == false {
		return false
	}
	oldParts := r.FindAllStringSubmatch(v.pre, -1)
	newParts := oldParts[0][1] + oldParts[0][2]
	if len(oldParts[0][3]) > 0 {
		i, err := strconv.Atoi(oldParts[0][3])
		if err != nil {
			return false
		}
		newParts += strconv.Itoa(i + 1)
	} else {
		newParts += string("1")
	}
	v.pre = newParts
	return true
}

// Increment version number by the minor number.
// Unsets prerelease status.
// Add +1 to patch number.
func (v *Version) IncPatch() bool {
	v.pre = ""
	v.patch += 1
	return true
}

// Increment version number by the minor number.
// Unsets prerelease status.
// Sets patch number to 0.
// Add +1 to minor number.
func (v *Version) IncMinor() bool {
	v.pre = ""
	v.patch = 0
	v.minor += 1
	return true
}

// Increment version number by the major number.
// Unsets prerelease status.
// Sets patch number to 0.
// Sets minor number to 0.
// Add +1 to major number.
func (v *Version) IncMajor() bool {
	v.pre = ""
	v.patch = 0
	v.minor = 0
	v.major += 1
	return true
}

// Set prerelease value.
func (v *Version) SetPrelease(prerelease string) bool {
	r := regexp.MustCompile("^([a-z]+)(.-)?([0-9]+)?")
	if len(prerelease) > 0 && r.MatchString(prerelease) == false {
		return false
	}
	v.pre = prerelease
	return true
}

// LessThan tests if one version is less than another one.
func (v *Version) LessThan(o *Version) bool {
	return v.Compare(o) < 0
}

// GreaterThan tests if one version is greater than another one.
func (v *Version) GreaterThan(o *Version) bool {
	return v.Compare(o) > 0
}

// Equal tests if two versions are equal to each other.
// Note, versions can be equal with different metadata since metadata
// is not considered part of the comparable version.
func (v *Version) Equal(o *Version) bool {
	return v.Compare(o) == 0
}

// Compare compares this version to another one. It returns -1, 0, or 1 if
// the version smaller, equal, or larger than the other version.
//
// Versions are compared by X.Y.Z. Build metadata is ignored. Prerelease is
// lower than the version without a prerelease.
func (v *Version) Compare(o *Version) int {
	// Compare the major, minor, and patch version for differences. If a
	// difference is found return the comparison.
	if d := compareSegment(v.Major(), o.Major()); d != 0 {
		return d
	}
	if d := compareSegment(v.Minor(), o.Minor()); d != 0 {
		return d
	}
	if d := compareSegment(v.Patch(), o.Patch()); d != 0 {
		return d
	}

	// At this point the major, minor, and patch versions are the same.
	ps := v.pre
	po := o.Prerelease()

	if ps == "" && po == "" {
		return 0
	}
	if ps == "" {
		return 1
	}
	if po == "" {
		return -1
	}

	return comparePrerelease(ps, po)
}

func compareSegment(v, o int64) int {
	if v < o {
		return -1
	}
	if v > o {
		return 1
	}

	return 0
}

func comparePrerelease(v, o string) int {

	// split the prelease versions by their part. The separator, per the spec,
	// is a .
	sparts := strings.Split(v, ".")
	oparts := strings.Split(o, ".")

	// Find the longer length of the parts to know how many loop iterations to
	// go through.
	slen := len(sparts)
	olen := len(oparts)

	l := slen
	if olen > slen {
		l = olen
	}

	// Iterate over each part of the prereleases to compare the differences.
	for i := 0; i < l; i++ {
		// Since the lentgh of the parts can be different we need to create
		// a placeholder. This is to avoid out of bounds issues.
		stemp := ""
		if i < slen {
			stemp = sparts[i]
		}

		otemp := ""
		if i < olen {
			otemp = oparts[i]
		}

		d := comparePrePart(stemp, otemp)
		if d != 0 {
			return d
		}
	}

	// Reaching here means two versions are of equal value but have different
	// metadata (the part following a +). They are not identical in string form
	// but the version comparison finds them to be equal.
	return 0
}

func comparePrePart(s, o string) int {
	// Fastpath if they are equal
	if s == o {
		return 0
	}

	// When s or o are empty we can use the other in an attempt to determine
	// the response.
	if o == "" {
		_, n := strconv.ParseInt(s, 10, 64)
		if n != nil {
			return -1
		}
		return 1
	}
	if s == "" {
		_, n := strconv.ParseInt(o, 10, 64)
		if n != nil {
			return 1
		}
		return -1
	}

	if s > o {
		return 1
	}
	return -1
}
