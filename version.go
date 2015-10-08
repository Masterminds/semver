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

// The regular expression used to parse a semantic version.
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
