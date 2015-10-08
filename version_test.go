package semver

import (
	"testing"
)

func TestNewVersion(t *testing.T) {
	tests := []struct {
		version string
		err     bool
	}{
		{"1.2.3", false},
		{"v1.2.3", false},
		{"1.0", false},
		{"v1.0", false},
		{"1", false},
		{"v1", false},
		{"1.2.beta", true},
		{"v1.2.beta", true},
		{"foo", true},
		{"1.2-5", false},
		{"v1.2-5", false},
		{"1.2-beta.5", false},
		{"v1.2-beta.5", false},
		{"\n1.2", true},
		{"\nv1.2", true},
		{"1.2.0-x.Y.0+metadata", false},
		{"v1.2.0-x.Y.0+metadata", false},
		{"1.2.0-x.Y.0+metadata-width-hypen", false},
		{"v1.2.0-x.Y.0+metadata-width-hypen", false},
		{"1.2.3-rc1-with-hypen", false},
		{"v1.2.3-rc1-with-hypen", false},
		{"1.2.3.4", true},
		{"v1.2.3.4", true},
	}

	for _, tc := range tests {
		_, err := NewVersion(tc.version)
		if tc.err && err == nil {
			t.Fatalf("expected error for version: %s", tc.version)
		} else if !tc.err && err != nil {
			t.Fatalf("error for version %s: %s", tc.version, err)
		}
	}
}

func TestOriginal(t *testing.T) {
	tests := []string{
		"1.2.3",
		"v1.2.3",
		"1.0",
		"v1.0",
		"1",
		"v1",
		"1.2-5",
		"v1.2-5",
		"1.2-beta.5",
		"v1.2-beta.5",
		"1.2.0-x.Y.0+metadata",
		"v1.2.0-x.Y.0+metadata",
		"1.2.0-x.Y.0+metadata-width-hypen",
		"v1.2.0-x.Y.0+metadata-width-hypen",
		"1.2.3-rc1-with-hypen",
		"v1.2.3-rc1-with-hypen",
	}

	for _, tc := range tests {
		v, err := NewVersion(tc)
		if err != nil {
			t.Errorf("Error parsing version %s", tc)
		}

		o := v.Original()
		if o != tc {
			t.Errorf("Error retrieving originl. Expected '%s' but got '%s'", tc, v)
		}
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		version  string
		expected string
	}{
		{"1.2.3", "1.2.3"},
		{"v1.2.3", "1.2.3"},
		{"1.0", "1.0.0"},
		{"v1.0", "1.0.0"},
		{"1", "1.0.0"},
		{"v1", "1.0.0"},
		{"1.2-5", "1.2.0-5"},
		{"v1.2-5", "1.2.0-5"},
		{"1.2-beta.5", "1.2.0-beta.5"},
		{"v1.2-beta.5", "1.2.0-beta.5"},
		{"1.2.0-x.Y.0+metadata", "1.2.0-x.Y.0+metadata"},
		{"v1.2.0-x.Y.0+metadata", "1.2.0-x.Y.0+metadata"},
		{"1.2.0-x.Y.0+metadata-width-hypen", "1.2.0-x.Y.0+metadata-width-hypen"},
		{"v1.2.0-x.Y.0+metadata-width-hypen", "1.2.0-x.Y.0+metadata-width-hypen"},
		{"1.2.3-rc1-with-hypen", "1.2.3-rc1-with-hypen"},
		{"v1.2.3-rc1-with-hypen", "1.2.3-rc1-with-hypen"},
	}

	for _, tc := range tests {
		v, err := NewVersion(tc.version)
		if err != nil {
			t.Errorf("Error parsing version %s", tc)
		}

		s := v.String()
		if s != tc.expected {
			t.Errorf("Error generating string. Expected '%s' but got '%s'", tc.expected, s)
		}
	}
}
