package semver

import "fmt"

var rangeErrs = [...]string{
	"%s is less than %s",
	"%s is less than or equal to %s",
	"%s is greater than %s",
	"%s is greater than or equal to %s",
	"%s is specifically disallowed by %s",
}

const (
	rerrLT = iota
	rerrLTE
	rerrGT
	rerrGTE
	rerrNE
)

type rangeConstraintError struct {
	v   *Version
	rc  rangeConstraint
	typ int8
}

func (rce rangeConstraintError) Error() string {
	return fmt.Sprintf(rangeErrs[rce.typ], rce.v, rce.rc)
}

type versionConstraintError struct {
	v, other *Version
}

func (vce versionConstraintError) Error() string {
	return fmt.Sprintf("%s is not equal to %s", vce.v, vce.other)
}
