package semver

import (
	"reflect"
	"sort"
	"testing"
)

func TestCollection(t *testing.T) {
	raw := []string{
		"1.2.3",
		"1.0",
		"1.3",
		"2",
		"0.4.2",
	}

	vs := make([]*Version, len(raw))
	for i, r := range raw {
		v, err := NewVersion(r)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		vs[i] = v
	}

	sort.Sort(Collection(vs))

	e := []string{
		"0.4.2",
		"1.0.0",
		"1.2.3",
		"1.3.0",
		"2.0.0",
	}

	a := make([]string, len(vs))
	for i, v := range vs {
		a[i] = v.String()
	}

	if !reflect.DeepEqual(a, e) {
		t.Error("Sorting Collection failed")
	}
}

func TestSort(t *testing.T) {
	raw := []string{
		"1.2.3",
		"1.0",
		"1.3",
		"2",
		"0.4.2",
		"1.0.0-alpha",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
	}

	vs := make(Collection, len(raw))
	for i, r := range raw {
		v, err := NewVersion(r)
		if err != nil {
			t.Errorf("Error parsing version: %s", err)
		}

		vs[i] = v
	}

	// Sorting through the sort interface and through the helper must agree.
	expected := make(Collection, len(vs))
	copy(expected, vs)
	sort.Sort(expected)

	Sort(vs)

	e := []string{
		"0.4.2",
		"1.0.0-alpha",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0",
		"1.2.3",
		"1.3.0",
		"2.0.0",
	}

	a := make([]string, len(vs))
	for i, v := range vs {
		a[i] = v.String()
		if !v.Equal(expected[i]) {
			t.Errorf("Sort and sort.Sort disagree at %d: %s and %s", i, v, expected[i])
		}
	}

	if !reflect.DeepEqual(a, e) {
		t.Errorf("Sorting Collection failed, got %v", a)
	}
}
