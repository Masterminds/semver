package semver

import (
	"reflect"
	"testing"
)

func TestParseConstraint(t *testing.T) {
	tests := []struct {
		in  string
		f   cfunc
		v   string
		err bool
	}{
		{">= 1.2", constraintGreaterThanEqual, "1.2.0", false},
		{"1.0", constraintEqual, "1.0.0", false},
		{"foo", nil, "", true},
		{"<= 1.2", constraintLessThanEqual, "1.2.0", false},
	}

	for _, tc := range tests {
		c, err := parseConstraint(tc.in)
		if tc.err && err == nil {
			t.Errorf("Expected error for %s didn't occur", tc.in)
		} else if !tc.err && err != nil {
			t.Errorf("Unexpected error for %s", tc.in)
		}

		// If an error was expected continue the loop and don't try the other
		// tests as they will cause errors.
		if tc.err {
			continue
		}

		if tc.v != c.con.String() {
			t.Errorf("Incorrect version found on %s", tc.in)
		}

		f1 := reflect.ValueOf(tc.f)
		f2 := reflect.ValueOf(c.function)
		if f1 != f2 {
			t.Errorf("Wrong constraint found for %s", tc.in)
		}
	}
}
