package semver

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// CoerceNewVersion sets if leading 0's are allowd in the version part. Leading 0's are
// not allowed in a valid semantic version. When set to true, NewVersion will coerce
// leading 0's into a valid version.
var CoerceNewVersion = true

// DetailedNewVersionErrors specifies if detailed errors are returned from the NewVersion
// function. This is used when CoerceNewVersion is set to false. If set to false
// ErrInvalidSemVer is returned for an invalid version. This does not apply to
// StrictNewVersion. Setting this function to false returns errors more quickly.
var DetailedNewVersionErrors = true

var (
	// ErrInvalidSemVer is returned a version is found to be invalid when
	// being parsed.
	ErrInvalidSemVer = errors.New("invalid semantic version")

	// ErrEmptyString is returned when an empty string is passed in for parsing.
	ErrEmptyString = errors.New("version string empty")

	// ErrInvalidCharacters is returned when invalid characters are found as
	// part of a version
	ErrInvalidCharacters = errors.New("invalid characters in version")

	// ErrSegmentStartsZero is returned when a version segment starts with 0.
	// This is invalid in SemVer.
	ErrSegmentStartsZero = errors.New("version segment starts with 0")

	// ErrInvalidMetadata is returned when the metadata is an invalid format
	ErrInvalidMetadata = errors.New("invalid metadata string")

	// ErrInvalidPrerelease is returned when the pre-release is an invalid format
	ErrInvalidPrerelease = errors.New("invalid prerelease string")

	// ErrVersionTooLong is returned when a version string exceeds the
	// maximum allowed length.
	ErrVersionTooLong = fmt.Errorf("version string is too long (max %d bytes)", MaxVersionLen)

	// ErrIncrementOverflow is returned when incrementing a version segment
	// would exceed the maximum value of a uint64. Errors returned by
	// IncMajorE, IncMinorE, and IncPatchE wrap this value and name the segment
	// that overflowed, so they can be detected with errors.Is.
	ErrIncrementOverflow = errors.New("version increment would overflow uint64")
)

// MaxVersionLen is the maximum allowed length of a version string. This guards
// against unbounded input causing excessive memory allocations during parsing.
const MaxVersionLen = 256

// Version represents a single semantic version.
type Version struct {
	major, minor, patch uint64
	pre                 string
	metadata            string
	original            string
}

const (
	num     string = "0123456789"
	allowed string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-" + num
)

// StrictNewVersion parses a given version and returns an instance of Version or
// an error if unable to parse the version. Only parses valid semantic versions.
// Performs checking that can find errors within the version.
// If you want to coerce a version such as 1 or 1.2 and parse it as the 1.x
// releases of semver did, use the NewVersion() function.
func StrictNewVersion(v string) (*Version, error) {
	// Parsing here does not use RegEx in order to increase performance and reduce
	// allocations.

	if len(v) == 0 {
		return nil, ErrEmptyString
	}

	if len(v) > MaxVersionLen {
		return nil, ErrVersionTooLong
	}

	// Split the parts into [0]major, [1]minor, and [2]patch,prerelease,build.
	// The parts are walked in place so that no slice is allocated to hold them.
	var parts [3]string
	rest := v
	for i := 0; i < 2; i++ {
		j := strings.IndexByte(rest, '.')
		if j < 0 {
			return nil, ErrInvalidSemVer
		}
		parts[i], rest = rest[:j], rest[j+1:]
	}
	parts[2] = rest

	sv := &Version{
		original: v,
	}

	// Extract build metadata
	if i := strings.IndexByte(parts[2], '+'); i >= 0 {
		sv.metadata = parts[2][i+1:]
		parts[2] = parts[2][:i]
		if err := validateMetadata(sv.metadata); err != nil {
			return nil, err
		}
	}

	// Extract build prerelease
	if i := strings.IndexByte(parts[2], '-'); i >= 0 {
		sv.pre = parts[2][i+1:]
		parts[2] = parts[2][:i]
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

// NewVersion parses a given version and returns an instance of Version or
// an error if unable to parse the version. If the version is SemVer-ish it
// attempts to convert it to SemVer. If you want  to validate it was a strict
// semantic version at parse time see StrictNewVersion().
func NewVersion(v string) (*Version, error) {
	if len(v) > MaxVersionLen {
		return nil, ErrVersionTooLong
	}
	if CoerceNewVersion {
		return coerceNewVersion(v)
	}
	return exactNewVersion(v)
}

// parseVersionParts splits a SemVer-ish version into its numeric segments,
// prerelease, and metadata. Parsing here does not use RegEx in order to
// increase performance and reduce allocations.
//
// The shape accepted is the loose one: a leading v, one to three numeric
// segments, then optional dot separated prerelease and metadata identifiers
// made up of the characters [0-9A-Za-z-]. Rules that a valid semantic version
// adds on top of that, such as numeric values not having a leading 0, are left
// to the caller since NewVersion coerces some of them. n is the number of
// numeric segments found. ok is false when v does not fit the shape at all.
func parseVersionParts(v string) (segs [3]string, n int, pre, metadata string, ok bool) {
	s := v
	if len(s) > 0 && s[0] == 'v' {
		s = s[1:]
	}

	// Metadata is everything following the first +. It is separated first
	// because a - is a valid character within metadata.
	if i := strings.IndexByte(s, '+'); i >= 0 {
		metadata = s[i+1:]
		s = s[:i]
		if !validIdentifiers(metadata) {
			return segs, 0, "", "", false
		}
	}

	// The prerelease is everything following the first - that remains once
	// the metadata has been removed.
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = s[i+1:]
		s = s[:i]
		if !validIdentifiers(pre) {
			return segs, 0, "", "", false
		}
	}

	// What remains are the major, minor, and patch segments. The segments are
	// all checked before any is parsed so that an invalid segment is reported
	// ahead of a numeric one that is out of range.
	for {
		var seg string
		more := false
		if i := strings.IndexByte(s, '.'); i >= 0 {
			seg, s, more = s[:i], s[i+1:], true
		} else {
			seg, s = s, ""
		}

		if n > 2 || seg == "" || !containsOnlyNum(seg) {
			return segs, 0, "", "", false
		}
		segs[n] = seg
		n++

		if !more {
			return segs, n, pre, metadata, true
		}
	}
}

// validIdentifiers reports if s is a series of dot separated identifiers made
// up of the characters allowed in a prerelease or metadata string. Identifiers
// must not be empty.
func validIdentifiers(s string) bool {
	for {
		var part string
		more := false
		if i := strings.IndexByte(s, '.'); i >= 0 {
			part, s, more = s[:i], s[i+1:], true
		} else {
			part, s = s, ""
		}

		if part == "" || !containsOnlyAllowed(part) {
			return false
		}

		if !more {
			return true
		}
	}
}

// coerceNewVersion parses a SemVer-ish version, coercing versions such as 1 or
// 1.2 into a full version. Leading 0's on the numeric segments are allowed.
func coerceNewVersion(v string) (*Version, error) {
	segs, n, pre, metadata, ok := parseVersionParts(v)
	if !ok {
		return nil, ErrInvalidSemVer
	}

	sv := &Version{
		metadata: metadata,
		pre:      pre,
		original: v,
	}

	var err error
	if sv.major, err = strconv.ParseUint(segs[0], 10, 64); err != nil {
		return nil, fmt.Errorf("error parsing version segment: %w", err)
	}

	if n > 1 {
		if sv.minor, err = strconv.ParseUint(segs[1], 10, 64); err != nil {
			return nil, fmt.Errorf("error parsing version segment: %w", err)
		}
	}

	if n > 2 {
		if sv.patch, err = strconv.ParseUint(segs[2], 10, 64); err != nil {
			return nil, fmt.Errorf("error parsing version segment: %w", err)
		}
	}

	// The characters in the prerelease are already known to be valid. This
	// catches the numeric identifiers that have a leading 0.
	if sv.pre != "" {
		if err = validatePrerelease(sv.pre); err != nil {
			return nil, err
		}
	}

	return sv, nil
}

// exactNewVersion parses a version without coercing leading 0's on the numeric
// segments or on the numeric identifiers of a prerelease. Missing minor and
// patch segments are still filled in with 0.
func exactNewVersion(v string) (*Version, error) {
	segs, n, pre, metadata, ok := parseVersionParts(v)
	if !ok {
		return nil, ErrInvalidSemVer
	}

	// The loose shape allows leading 0's where a valid semantic version does
	// not. Detect that before anything is parsed so that the fast path when
	// detailed errors are disabled does no extra work.
	valid := true
	for i := 0; i < n; i++ {
		if len(segs[i]) > 1 && segs[i][0] == '0' {
			valid = false
			break
		}
	}
	if valid && pre != "" && validatePrerelease(pre) != nil {
		valid = false
	}

	if !valid {

		// Disabling detailed errors is first so that it is in the fast path.
		if !DetailedNewVersionErrors {
			return nil, ErrInvalidSemVer
		}

		// Check for specific errors with the semver string and return a more
		// detailed error.
		return nil, detailedVersionError(segs, n, pre, metadata)
	}

	sv := &Version{
		metadata: metadata,
		pre:      pre,
		original: v,
	}

	var err error
	if sv.major, err = strconv.ParseUint(segs[0], 10, 64); err != nil {
		return nil, fmt.Errorf("error parsing version segment: %w", err)
	}

	if n > 1 {
		if sv.minor, err = strconv.ParseUint(segs[1], 10, 64); err != nil {
			return nil, fmt.Errorf("error parsing version segment: %w", err)
		}
	}

	if n > 2 {
		if sv.patch, err = strconv.ParseUint(segs[2], 10, 64); err != nil {
			return nil, fmt.Errorf("error parsing version segment: %w", err)
		}
	}

	return sv, nil
}

// detailedVersionError reports the first problem found with a version that is
// shaped like a semantic version but is not a valid one. Problems are looked
// for in the order the parts appear in the version.
func detailedVersionError(segs [3]string, n int, pre, metadata string) error {
	for i := 0; i < n; i++ {
		if len(segs[i]) > 1 && segs[i][0] == '0' {
			return ErrSegmentStartsZero
		}
		if _, err := strconv.ParseUint(segs[i], 10, 64); err != nil {
			return fmt.Errorf("error parsing version segment: %w", err)
		}
	}

	if pre != "" {
		if err := validatePrerelease(pre); err != nil {
			return err
		}
	}

	if metadata != "" {
		if err := validateMetadata(metadata); err != nil {
			return err
		}
	}

	return ErrInvalidSemVer
}

// New creates a new instance of Version with each of the parts passed in as
// arguments instead of parsing a version string.
// Note, New does not validate prerelease or metadata. Incorrect information can
// be passed in.
func New(major, minor, patch uint64, pre, metadata string) *Version {
	v := Version{
		major:    major,
		minor:    minor,
		patch:    patch,
		pre:      pre,
		metadata: metadata,
		original: "",
	}

	v.original = v.String()

	// TODO: In the next semver major version validate the pre and metadata. Return error if there is one.
	return &v
}

// MustParse parses a given version and panics on error.
func MustParse(v string) *Version {
	sv, err := NewVersion(v)
	if err != nil {
		panic(err)
	}
	return sv
}

// String converts a Version object to a string.
// Note, if the original version contained a leading v this version will not.
// See the Original() method to retrieve the original value. Semantic Versions
// don't contain a leading v per the spec. Instead it's optional on
// implementation.
func (v Version) String() string {
	// The buffer starts on the stack. It is large enough for three uint64
	// segments and their separators, so only a version with a prerelease or
	// metadata can grow it.
	var b [64]byte
	buf := b[:0]

	buf = strconv.AppendUint(buf, v.major, 10)
	buf = append(buf, '.')
	buf = strconv.AppendUint(buf, v.minor, 10)
	buf = append(buf, '.')
	buf = strconv.AppendUint(buf, v.patch, 10)
	if v.pre != "" {
		buf = append(buf, '-')
		buf = append(buf, v.pre...)
	}
	if v.metadata != "" {
		buf = append(buf, '+')
		buf = append(buf, v.metadata...)
	}

	return string(buf)
}

// Original returns the original value passed in to be parsed.
func (v *Version) Original() string {
	return v.original
}

// Major returns the major version.
func (v Version) Major() uint64 {
	return v.major
}

// Minor returns the minor version.
func (v Version) Minor() uint64 {
	return v.minor
}

// Patch returns the patch version.
func (v Version) Patch() uint64 {
	return v.patch
}

// Prerelease returns the pre-release version.
func (v Version) Prerelease() string {
	return v.pre
}

// Metadata returns the metadata on the version.
func (v Version) Metadata() string {
	return v.metadata
}

// originalVPrefix returns the original 'v' prefix if any.
func (v Version) originalVPrefix() string {
	// Note, only lowercase v is supported as a prefix by the parser.
	if v.original != "" && v.original[0] == 'v' {
		return v.original[:1]
	}
	return ""
}

// IncPatch produces the next patch version.
// If the current version does not have prerelease/metadata information,
// it unsets metadata and prerelease values, increments patch number.
// If the current version has any of prerelease or metadata information,
// it unsets both values and keeps current patch value
//
// Note, this panics if the patch segment is math.MaxUint64 and would overflow.
// A version parsed from untrusted input can reach that value, so callers
// handling input they do not control should use IncPatchE instead.
func (v Version) IncPatch() Version {
	vNext, err := v.IncPatchE()
	if err != nil {
		panic(err)
	}
	return vNext
}

// IncPatchE produces the next patch version. It behaves the same as IncPatch
// but returns an error wrapping ErrIncrementOverflow instead of panicking when
// the patch segment is math.MaxUint64. On error the version is returned
// unchanged.
func (v Version) IncPatchE() (Version, error) {
	vNext := v
	// according to http://semver.org/#spec-item-9
	// Pre-release versions have a lower precedence than the associated normal version.
	// according to http://semver.org/#spec-item-10
	// Build metadata SHOULD be ignored when determining version precedence.
	if v.pre != "" {
		vNext.metadata = ""
		vNext.pre = ""
	} else {
		vNext.metadata = ""
		vNext.pre = ""
		if v.patch == math.MaxUint64 {
			return v, fmt.Errorf("patch %w", ErrIncrementOverflow)
		}
		vNext.patch = v.patch + 1
	}
	vNext.original = v.originalVPrefix() + "" + vNext.String()
	return vNext, nil
}

// IncMinor produces the next minor version.
// Sets patch to 0.
// Increments minor number.
// Unsets metadata.
// Unsets prerelease status.
//
// Note, this panics if the minor segment is math.MaxUint64 and would overflow.
// A version parsed from untrusted input can reach that value, so callers
// handling input they do not control should use IncMinorE instead.
func (v Version) IncMinor() Version {
	vNext, err := v.IncMinorE()
	if err != nil {
		panic(err)
	}
	return vNext
}

// IncMinorE produces the next minor version. It behaves the same as IncMinor
// but returns an error wrapping ErrIncrementOverflow instead of panicking when
// the minor segment is math.MaxUint64. On error the version is returned
// unchanged.
func (v Version) IncMinorE() (Version, error) {
	vNext := v
	vNext.metadata = ""
	vNext.pre = ""
	vNext.patch = 0
	if v.minor == math.MaxUint64 {
		return v, fmt.Errorf("minor %w", ErrIncrementOverflow)
	}
	vNext.minor = v.minor + 1
	vNext.original = v.originalVPrefix() + "" + vNext.String()
	return vNext, nil
}

// IncMajor produces the next major version.
// Sets patch to 0.
// Sets minor to 0.
// Increments major number.
// Unsets metadata.
// Unsets prerelease status.
//
// Note, this panics if the major segment is math.MaxUint64 and would overflow.
// A version parsed from untrusted input can reach that value, so callers
// handling input they do not control should use IncMajorE instead.
func (v Version) IncMajor() Version {
	vNext, err := v.IncMajorE()
	if err != nil {
		panic(err)
	}
	return vNext
}

// IncMajorE produces the next major version. It behaves the same as IncMajor
// but returns an error wrapping ErrIncrementOverflow instead of panicking when
// the major segment is math.MaxUint64. On error the version is returned
// unchanged.
func (v Version) IncMajorE() (Version, error) {
	vNext := v
	vNext.metadata = ""
	vNext.pre = ""
	vNext.patch = 0
	vNext.minor = 0
	if v.major == math.MaxUint64 {
		return v, fmt.Errorf("major %w", ErrIncrementOverflow)
	}
	vNext.major = v.major + 1
	vNext.original = v.originalVPrefix() + "" + vNext.String()
	return vNext, nil
}

// SetPrerelease defines the prerelease value.
// Value must not include the required 'hyphen' prefix.
func (v Version) SetPrerelease(prerelease string) (Version, error) {
	vNext := v
	if len(prerelease) > 0 {
		if err := validatePrerelease(prerelease); err != nil {
			return vNext, err
		}
	}
	vNext.pre = prerelease
	vNext.original = v.originalVPrefix() + "" + vNext.String()
	return vNext, nil
}

// SetMetadata defines metadata value.
// Value must not include the required 'plus' prefix.
func (v Version) SetMetadata(metadata string) (Version, error) {
	vNext := v
	if len(metadata) > 0 {
		if err := validateMetadata(metadata); err != nil {
			return vNext, err
		}
	}
	vNext.metadata = metadata
	vNext.original = v.originalVPrefix() + "" + vNext.String()
	return vNext, nil
}

// LessThan tests if one version is less than another one.
func (v *Version) LessThan(o *Version) bool {
	return v.Compare(o) < 0
}

// LessThanEqual tests if one version is less or equal than another one.
func (v *Version) LessThanEqual(o *Version) bool {
	return v.Compare(o) <= 0
}

// GreaterThan tests if one version is greater than another one.
func (v *Version) GreaterThan(o *Version) bool {
	return v.Compare(o) > 0
}

// GreaterThanEqual tests if one version is greater or equal than another one.
func (v *Version) GreaterThanEqual(o *Version) bool {
	return v.Compare(o) >= 0
}

// Equal tests if two versions are equal to each other.
// Note, versions can be equal with different metadata since metadata
// is not considered part of the comparable version.
func (v *Version) Equal(o *Version) bool {
	if v == o {
		return true
	}
	if v == nil || o == nil {
		return false
	}
	return v.Compare(o) == 0
}

// Compare compares this version to another one. It returns -1, 0, or 1 if
// the version smaller, equal, or larger than the other version.
//
// Versions are compared by X.Y.Z. Build metadata is ignored. Prerelease is
// lower than the version without a prerelease. Compare always takes into account
// prereleases. If you want to work with ranges using typical range syntaxes that
// skip prereleases if the range is not looking for them use constraints.
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

// UnmarshalJSON implements JSON.Unmarshaler interface.
func (v *Version) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	temp, err := NewVersion(s)
	if err != nil {
		return err
	}
	v.major = temp.major
	v.minor = temp.minor
	v.patch = temp.patch
	v.pre = temp.pre
	v.metadata = temp.metadata
	v.original = temp.original
	return nil
}

// MarshalJSON implements JSON.Marshaler interface.
func (v Version) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (v *Version) UnmarshalText(text []byte) error {
	temp, err := NewVersion(string(text))
	if err != nil {
		return err
	}

	*v = *temp

	return nil
}

// MarshalText implements the encoding.TextMarshaler interface.
func (v Version) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

// Scan implements the SQL.Scanner interface.
func (v *Version) Scan(value interface{}) error {
	var s string
	switch t := value.(type) {
	case string:
		s = t
	case []byte:
		s = string(t)
	case nil:
		return fmt.Errorf("cannot scan nil into Version")
	default:
		return fmt.Errorf("unsupported Scan type %T", value)
	}
	temp, err := NewVersion(s)
	if err != nil {
		return err
	}
	v.major = temp.major
	v.minor = temp.minor
	v.patch = temp.patch
	v.pre = temp.pre
	v.metadata = temp.metadata
	v.original = temp.original
	return nil
}

// Value implements the Driver.Valuer interface.
func (v Version) Value() (driver.Value, error) {
	return v.String(), nil
}

func compareSegment(v, o uint64) int {
	if v < o {
		return -1
	}
	if v > o {
		return 1
	}

	return 0
}

func comparePrerelease(v, o string) int {
	// Walk the dot separated parts of both prereleases. The separator, per the
	// spec, is a . The parts are walked in place so that no slices are
	// allocated to hold them.
	for v != "" || o != "" {
		var sp, op string
		sp, v = nextPart(v)
		op, o = nextPart(o)

		if d := comparePrePart(sp, op); d != 0 {
			return d
		}
	}

	// Reaching here means two versions are of equal value but have different
	// metadata (the part following a +). They are not identical in string form
	// but the version comparison finds them to be equal.
	return 0
}

// nextPart returns the leading dot separated segment of s along with the
// remainder of s following the dot.
func nextPart(s string) (part, rest string) {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func comparePrePart(s, o string) int {
	// Fastpath if they are equal
	if s == o {
		return 0
	}

	// When s or o are empty we can use the other in an attempt to determine
	// the response.
	if s == "" {
		if o != "" {
			return -1
		}
		return 1
	}

	if o == "" {
		if s != "" {
			return 1
		}
		return -1
	}

	// When comparing strings "99" is greater than "103". To handle
	// cases like this we need to detect numbers and compare them. According
	// to the semver spec, numbers are always positive. If there is a - at the
	// start like -99 this is to be evaluated as an alphanum. numbers always
	// have precedence over alphanum. Parsing as Uints because negative numbers
	// are ignored.

	oi, onum := parseIdentifierNum(o)
	si, snum := parseIdentifierNum(s)

	// The case where both are strings compare the strings
	if !onum && !snum {
		if s > o {
			return 1
		}
		return -1
	} else if !onum {
		// o is a string and s is a number
		return -1
	} else if !snum {
		// s is a string and o is a number
		return 1
	}
	// Both are numbers
	if si > oi {
		return 1
	}
	return -1
}

// parseIdentifierNum reports if a prerelease identifier is a number and, when
// it is, its value. An identifier that would overflow a uint64 is not treated
// as a number. Parsing here does not use strconv so that an identifier that is
// not a number costs no allocation.
func parseIdentifierNum(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}

	var n uint64
	for i := 0; i < len(s); i++ {
		if !numChars[s[i]] {
			return 0, false
		}
		d := uint64(s[i] - '0')
		if n > (math.MaxUint64-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}

	return n, true
}

// allowedChars and numChars are lookup tables for the characters allowed in
// the identifier and numeric portions of a version.
var allowedChars, numChars [256]bool

func init() {
	for i := 0; i < len(allowed); i++ {
		allowedChars[allowed[i]] = true
	}
	for i := 0; i < len(num); i++ {
		numChars[num[i]] = true
	}
}

// containsOnlyNum reports if s is made up only of the digits 0-9.
func containsOnlyNum(s string) bool {
	for i := 0; i < len(s); i++ {
		if !numChars[s[i]] {
			return false
		}
	}
	return true
}

// containsOnlyAllowed reports if s is made up only of the characters valid in
// a prerelease or metadata identifier.
func containsOnlyAllowed(s string) bool {
	for i := 0; i < len(s); i++ {
		if !allowedChars[s[i]] {
			return false
		}
	}
	return true
}

// From the spec, "Identifiers MUST comprise only
// ASCII alphanumerics and hyphen [0-9A-Za-z-]. Identifiers MUST NOT be empty.
// Numeric identifiers MUST NOT include leading zeroes.". These segments can
// be dot separated.
func validatePrerelease(p string) error {
	if !validIdentifiers(p) {
		return ErrInvalidPrerelease
	}

	// The identifiers are known to be valid and non-empty. Numeric identifiers
	// must not have a leading 0.
	for p != "" {
		var part string
		part, p = nextPart(p)
		if len(part) > 1 && part[0] == '0' && containsOnlyNum(part) {
			return ErrSegmentStartsZero
		}
	}

	return nil
}

// From the spec, "Build metadata MAY be denoted by
// appending a plus sign and a series of dot separated identifiers immediately
// following the patch or pre-release version. Identifiers MUST comprise only
// ASCII alphanumerics and hyphen [0-9A-Za-z-]. Identifiers MUST NOT be empty."
func validateMetadata(m string) error {
	if !validIdentifiers(m) {
		return ErrInvalidMetadata
	}
	return nil
}
