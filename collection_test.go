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

func TestCollectionWithNil(t *testing.T) {
	vs := Collection{
		MustParse("1.2.3"),
		nil,
		MustParse("0.4.2"),
		nil,
		MustParse("2.0.0"),
	}

	sort.Sort(vs)

	if vs[0] != nil || vs[1] != nil {
		t.Errorf("Expected nil versions at the beginning, got %v, %v", vs[0], vs[1])
	}
	if vs[2].String() != "0.4.2" || vs[3].String() != "1.2.3" || vs[4].String() != "2.0.0" {
		t.Errorf("Unexpected sorted order: %v, %v, %v", vs[2], vs[3], vs[4])
	}
}
