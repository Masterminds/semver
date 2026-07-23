package semver

import "testing"

func TestConstraintsCheckNilVersion(t *testing.T) {
	c, err := NewConstraint(">=1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panicked: %v", r)
		}
	}()
	if c.Check(nil) {
		t.Fatal("nil version should not match")
	}
}
