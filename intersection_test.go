package semver

import (
	"fmt"
	"strconv"
	"testing"
)

func TestIntersection_NilSafety(t *testing.T) {
	c := MustParseConstraint(">=0.0.0")
	if Intersection(nil, c) != nil {
		t.Fatal("Intersection(nil, c) should return nil")
	}
	if Intersection(c, nil) != nil {
		t.Fatal("Intersection(c, nil) should return nil")
	}
	if Intersection(nil, nil) != nil {
		t.Fatal("Intersection(nil, nil) should return nil")
	}
}

func TestIntersection(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{"^1", ">=1.4.0", ">=1.4.0 <2.0.0"},
		{"~1.2", "<1.2.5", ">=1.2.0 <1.2.5"},
		{"^0.2.3", ">=0.2.4", ">=0.2.4 <0.3.0"},
		{"~1", "<1.5.0", ">=1.0.0 <1.5.0"},
		{">=1.0.0 <2.0.0", ">=1.5.0 <3.0.0", ">=1.5.0 <2.0.0"},
		{"~1.2.0", ">=1.2.3 <1.3.0", ">=1.2.3 <1.3.0"},
		{"^1.2.0", ">=1.5.0 <2.0.0", ">=1.5.0 <2.0.0"},
		{"1.0.0 || 2.0.0", ">=1.0.0 <=2.0.0", "1.0.0 || 2.0.0"},
		{"^1.0.0 || ~2.1.0", ">=1.5.0 <2.2.0", ">=1.5.0 <2.0.0 || >=2.1.0 <2.2.0"},
		{">=1.0.0 <2.0.0", ">=3.0.0 <4.0.0", ""},
		{"1.2.3 || 1.2.4", ">=1.2.3 <=1.2.5", "1.2.3 || 1.2.4"},
		{"^2.0.0 || ~1.5.0", ">=1.5.2 <2.1.0", ">=1.5.2 <1.6.0 || >=2.0.0 <2.1.0"},
		{">=1.0.0 <2.0.0 || >=3.0.0 <4.0.0", ">=1.5.0 <3.5.0", ">=1.5.0 <2.0.0 || >=3.0.0 <3.5.0"},
		{">=1.0.0-alpha <1.0.0", ">=1.0.0-beta <1.0.0-gamma", ">=1.0.0-beta <1.0.0-gamma"},
		{">=1.0.0", ">=1.0.0", ">=1.0.0"},
		{">=1.0.0-alpha.1 <1.0.0-beta", ">=1.0.0-alpha.2 <1.0.0-alpha.10", ">=1.0.0-alpha.2 <1.0.0-alpha.10"},
		{">=1.0.0-1 <1.0.0-10", ">=1.0.0-2 <1.0.0-5", ">=1.0.0-2 <1.0.0-5"},
		{">=1.0.0-alpha+build1", ">=1.0.0-alpha+build2", ">=1.0.0-alpha+build1"},
		{">=1.0.0-alpha <2.0.0", ">=1.0.0 <1.5.0", ">=1.0.0 <1.5.0"},
		{">=1.0.0 <=2.0.0", ">2.0.0 <3.0.0", ""},
		{">=1.0.0 <=2.0.0", ">=2.0.0 <3.0.0", ">=2.0.0 <=2.0.0"},
		{">=0.0.0 <0.1.0", ">=0.0.1 <1.0.0", ">=0.0.1 <0.1.0"},
		{">=999999.999999.999999", ">=1000000.0.0 <2000000.0.0", ">=1000000.0.0 <2000000.0.0"},
		{">1.0.0 <1.0.1", ">=1.0.0 <=1.0.0", ""},
		{"1.0.0 || 3.0.0 || 5.0.0", "2.0.0 || 4.0.0 || 6.0.0", ""},
		{">=1.0.0 <2.0.0 || >=4.0.0 <5.0.0", ">=1.5.0 <3.0.0 || >=4.5.0 <6.0.0", ">=1.5.0 <2.0.0 || >=4.5.0 <5.0.0"},
		{">=4.0.0 <5.0.0 || >=1.0.0 <2.0.0", ">=4.5.0 <6.0.0 || >=1.5.0 <3.0.0", ">=1.5.0 <2.0.0 || >=4.5.0 <5.0.0"},
		{"1.0.0 || 1.1.0 || 1.2.0 || 1.3.0", ">=1.1.0 <=1.2.0", "1.1.0 || 1.2.0"},
		{"1.0.0 || >=2.0.0 <3.0.0", ">=0.9.0 <=1.0.0 || 2.5.0", "1.0.0 || 2.5.0"},
		{">=1.0.0 >=1.2.0", ">=1.1.0", ">=1.2.0"},
		{"<2.0.0 <1.8.0", "<1.9.0", "<1.8.0"},
		{">1.0.0 >=1.0.0", "<=2.0.0 <2.0.0", ">1.0.0 <2.0.0"},
		{">=2.0.0", "<1.0.0", ""},
		{"1.2.3 || 1.4.0", ">=1.0.0 <1.3.0", "1.2.3"},
		{"1.2.3", "=1.2.3", "1.2.3"},
		{"1.2.3", "=1.24", ""},
		{"1", ">=1.4.0", ">=1.4.0 <2.0.0"},
		// *
		{">=1.0.0 >=1.2.0", "*", ">=1.2.0"},
		{"<2.0.0 <1.8.0", "*", ">=0.0.0 <1.8.0"},
		{"1.x", "*", ">=1.0.0 <2.0.0"},
		{"1.x", "<1.5.0", ">=1.0.0 <1.5.0"},
		{">=1.2.0", "*", ">=1.2.0"},
		{"<2.0.0 <=1.8.0", "*", ">=0.0.0 <=1.8.0"},
		{">1.0.0 >=1.0.0", "*", ">1.0.0"},
		{">=1.0.0 >=1.2.0 <=2.0.0 <2.5.0", "*", ">=1.2.0 <=2.0.0"},
		{"1.2.x", ">=1.2.3", ">=1.2.3 <1.3.0"},
		{"1.2.x", "<1.2.1", ">=1.2.0 <1.2.1"},
		{"0.x.x", "<0.3.0", ">=0.0.0 <0.3.0"},
		{"1.x", ">=1.2.0 <1.4.0", ">=1.2.0 <1.4.0"},
		{"1.2.x", ">=1.2.3 <1.2.8", ">=1.2.3 <1.2.8"},
		{">=1.0.0-alpha <1.0.0-beta", ">=1.0.0-beta <1.0.0-rc", ""},
		{"=1.2.3", ">1.2.3", ""},
		{">=1 <=2", "~2", ">=2.0.0 <3.0.0"},
		{">=1.1.1-1", ">=1.1.1", ">=1.1.1"},
		{">=1.1.1-1", ">=1.1.1 <1.2.1-1", ">=1.1.1 <1.2.1-1"},

		{"1.0.6-1", ">=1.0.3-0 <1.0.6", "1.0.6-1"},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprint("WithoutIncludePrerelease ", strconv.Itoa(i)), func(t *testing.T) {
			got := Intersection(MustParseConstraint(tc.a), MustParseConstraint(tc.b)).String()
			if got != tc.want {
				t.Errorf("Intersection(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
			}
		})
		t.Run(fmt.Sprint("IncludePrerelease ", strconv.Itoa(i)), func(t *testing.T) {
			a := MustParseConstraint(tc.a)
			b := MustParseConstraint(tc.b)
			a.IncludePrerelease = true
			b.IncludePrerelease = true
			got := Intersection(a, b).String()
			if got != tc.want {
				t.Errorf("Intersection(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestIntersectionWithoutIncludePrerelease(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{">=1.1", "4.1.0-beta", ""},
		{">1.1", "4.1.0-beta", ""},
		{"<=1.1", "0.1.0-alpha", ""},
		{"<1.1", "0.1.0-alpha", ""},
		{"^1.x", "1.1.1-beta1", ""},
		{"~1.1", "1.1.1-alpha", ""},
		{"*", "1.2.3-alpha", ""},
		{"= 2.0", "2.0.1-beta", ""},
	}

	for i, tc := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			got := Intersection(MustParseConstraint(tc.a), MustParseConstraint(tc.b)).String()
			if got != tc.want {
				t.Errorf("Intersection(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestIntersectionIncludePrerelease(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{">=1.1", "4.1.0-beta", "4.1.0-beta"},
		{">1.1", "4.1.0-beta", "4.1.0-beta"},
		{"<=1.1", "0.1.0-alpha", "0.1.0-alpha"},
		{"<1.1", "0.1.0-alpha", "0.1.0-alpha"},
		{"^1.x", "1.1.1-beta1", "1.1.1-beta1"},
		{"~1.1", "1.1.1-alpha", "1.1.1-alpha"},
		{"*", "1.2.3-alpha", "1.2.3-alpha"},
		{"= 2.0", "2.0.1-beta", "2.0.1-beta"},
	}

	for i, tc := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			a := MustParseConstraint(tc.a)
			b := MustParseConstraint(tc.b)
			a.IncludePrerelease = true
			b.IncludePrerelease = true
			got := Intersection(a, b).String()
			if got != tc.want {
				t.Errorf("Intersection(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestIsSubset_NilSafety(t *testing.T) {
	c := MustParseConstraint(">=1.2.3 <4")
	if IsSubset(nil, c) {
		t.Fatal("IsSubset(nil, c) should not be false")
	}
	if IsSubset(c, nil) {
		t.Fatal("IsSubset(nil, c) should not be false")
	}
	if IsSubset(nil, nil) {
		t.Fatal("IsSubset(nil, nil) should not be false")
	}
}

func TestIsSubset(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"~8", ">=8 <=17", true},
		{"~1.2.x", "^1.2.x", true},
		{"~1.2.3", "~>1.2.3", true},
		{"~>2.0", "^2", true},
		{"~>1.2.x", "~1.2.x", true},
		{"~1.x", "^1", true},
		{"~1.x", "^1.1", false},
		{">=1.4.0", "^1", false},
		{"^1", ">=1.4.0", false},
		{">1 <2", ">=1 <3", true},
		{">1 <=2", ">=0 <3", true},
		{">=1.5.0 <2.0.0", ">=1.0.0 <2.5.0", true},
		{">=1.0.0 <2.0.0 || >=3.0.0 <4.0.0", ">=0.5.0 <5.0.0", true},
		{">=1.0.0 <2.0.0", ">=0.5.0 <3.0.0", true},
		{">=1.0.0 <2.0.0 || >=4.0.0 <5.0.0", ">=1.0.0 <3.0.0", false},
		{">=1.0.0 <3.0.0", ">=1.0.0 <2.0.0 || >=4.0.0 <5.0.0", false},
		{"1.4.0", ">=1.5.0 <2.0.0 || >=3.0.0 <3.5.0", false},
		{"1.5.0", ">=1.5.0 <2.0.0 || >=3.0.0 <3.5.0", true},
		{"2.5.0", ">=1.5.0 <2.0.0 || >=3.0.0 <3.5.0", false},
		{"3.2.0", ">=1.5.0 <2.0.0 || >=3.0.0 <3.5.0", true},
		{"3.6.0", ">=1.5.0 <2.0.0 || >=3.0.0 <3.5.0", false},
		{">=3.1.0 <3.5.0 || >=1.7.0 <1.9.0", ">=3.0.0 <3.5.0 || >=1.5.0 <2.0.0", true},
		{">1 <2", ">2 <3", false},
		{">1 <2", ">1.5 <2.5", false},
		{">=1.0.0 <=2.0.0", ">=1.0.0 <=2.0.0", true},
		{">1", ">=1", true},
		{"<2", "<=2", true},
		{">1 <=2", ">1 <2.5", false},
		{">=1.0.0", ">=0.0.0", true},
		{">=1.0.0", ">=1.0.0 <2.0.0", false},
		{">=1.2.3 <4", ">=1.2.3 <4", true},
		{"^1", "^1", true},
		{"^1.2.3", "^1.2.3", true},
		{"~1", "~1", true},
		{"~1.2", "~1.2", true},
		{"^1.2.0", "^1", true},
		{"~1.2", "~1", true},
		{"^1", "^1.2.0", false},
		{"~1", "~2", false},
		{"^1", "^2", false},
		{"^0.2.3", "^0.2.4", false},
		{"~1.2", ">=1.2.5 <1.3.0", false},
		{"^1.2.3", ">=1.2.3 <2.0.0", true},
		{"^1.2.3", ">=1.3.0 <2.0.0", false},
		{"^0.2", ">=0.2.0 <0.3.0", true},
		{"^0.2", ">=0.2.5 <0.3.0", false},
		{"~1", ">=1.0.0 <2.0.0", true},
		{"~1", ">=1.5.0 <2.0.0", false},
		{"^2", ">=2.3.0 <3.0.0", false},
		{"~1.2", ">=1.2.0 <1.3.0", true},
		{"~1.2", ">=1.0.0 <2.0.0", true},
		{"~1", ">=1.4.0 <2.0.0", false},
		{"^1", "<2.0.0", true},
		{"^1", ">=1.4.0 <2.0.0", false},
		{">=1.2.0 <1.3.0", ">=1.0.0 <2.0.0", true},
		{"~1.2.0", ">=1.0.0 <2.0.0", true},
		{"^1.2.0", ">=1.0.0 <2.0.0", true},
		{">=1.0.0 <3.0.0", ">=1.0.0 <2.0.0", false},
		{">=0.5.0 <2.0.0", ">=1.0.0 <2.0.0", false},
		{"1.2.3", ">=1.0.0 <=2.0.0", true},
		{"1.2.3 || 1.2.4", ">=1.2.0 <1.3.0", true},
		{"~1.2.0 || ^1.5.0", ">=1.0.0 <2.0.0", true},
		{"~1.2.0 || ^2.0.0", ">=1.0.0 <2.0.0", false},
		{">=1.2.0 <1.3.0", "~1.2.0", true},
		{">=1.0.0-alpha <1.0.0-beta", ">=1.0.0-alpha <1.0.0", true},
		{">=1.5.0 <2.5.0", ">=1.0.0 <2.0.0", false},
		{">=3.0.0 <4.0.0", ">=1.0.0 <2.0.0", false},
		{">=1.0.0 <2.0.0", ">=1.0.0 <2.0.0", true},
		{"1.0.0 || 2.0.0 || 3.0.0", ">=1.0.0 <=2.0.0", false},
		{"1.0.0 || 1.5.0 || 2.0.0", ">=1.0.0 <=2.0.0", true},
		{"1.5.0", ">=2.0.0 <1.0.0", false},
		{">=1.0.0-alpha <1.0.0-beta", ">=1.0.0 <2.0.0", false},
		{"1.0.0+build1", "1.0.0+build2", true},
		{"1.0.0 || 3.0.0", ">=0.9.0 <=1.1.0", false},
		{"^1.2.3", ">=1.0.0 <2.0.0", true},
		{">=1.2.4 <1.3.0", "~1.2.0", true},
		{">=1.0.0-beta.1 <1.0.0", ">=1.0.0-alpha <1.0.0", true},
		{"1.2.3 || 1.2.4 || 1.2.5", "~1.2.0", true},
		{"1.2.3", "=1.24", false},
		{">=1.2.0 >=1.0.0", ">=1.1.0", true},
		{">=1.1.0", ">=1.2.0 >=1.0.0", false},
		{"<1.8.0 <2.0.0", "<2.0.0", true},
		{"<2.0.0", "<1.8.0 <2.0.0", false},
		{">=1.2.0 <1.5.0 >=1.0.0", ">=1.1.0 <2.0.0", true},
		{">=1.2.0 <1.5.0", ">=1.2.0 <=1.4.0", false},
		{">=1.0.0 <=2.0.0 >=1.0.0", ">=1.0.0 <=2.0.0", true},
		{"<=2.0.0 <2.0.0", "<=2.0.0", true},
		{"<=2.0.0", "<2.0.0 <=2.0.0", false},
		// x
		{"1.x", "^1", true},
		{"^1", "1.x", true},
		{"1.2.x", "1.x", true},
		{"1.x", "1.2.x", false},
		{"1.2.x", "x.x.x", true},
		{"0.2.x", "0.x.x", true},
		{"^0.2.4", "0.x.x", true},
		{"~0.2.4", "0.x.x", true},
		{"=0.2.4", "=0.x.x", true},
		// *
		{">=3.0.0 <2.0.0", "*", true},
		{"*", "*", true},
		{"*", "<2.0.0", false},
		{"0.x", "<1.0.0", true},
		{"0.x", ">=0.1.0 <0.5.0", false},
		{"~2", ">=1 <=2", true},

		{"1.0.6-1", ">=1.0.3-0 <1.0.6", true},
		{"1.0.6-1", ">=1.0.3-0 <1.0.7", true},
		{"1.0.6-1", ">=1.0.3-0 <=1.0.6", true},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprint("WithoutIncludePrerelease ", strconv.Itoa(i)),
			func(t *testing.T) {
				got := IsSubset(MustParseConstraint(tc.a), MustParseConstraint(tc.b))
				if got != tc.want {
					t.Errorf("IsSubset(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
				}
			})

		t.Run(fmt.Sprint("IncludePrerelease ", strconv.Itoa(i)), func(t *testing.T) {
			a := MustParseConstraint(tc.a)
			b := MustParseConstraint(tc.b)
			a.IncludePrerelease = true
			b.IncludePrerelease = true
			got := IsSubset(a, b)
			if got != tc.want {
				t.Errorf("IsSubset(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})

	}
}

func TestIsSubsetWithoutIncludePrerelease(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"4.1.0-beta", ">=1.1", false},
		{"4.1.0-beta", ">1.1", false},
		{"0.1.0-alpha", "<=1.1", false},
		{"0.1.0-alpha", "<1.1", false},
		{"1.1.1-beta1", "^1.x", false},
		{"1.1.1-alpha", "~1.1", false},
		{"1.2.3-alpha", "*", false},
		{"2.0.1-beta", "= 2.0", false},
	}

	for i, tc := range cases {
		t.Run(strconv.Itoa(i),
			func(t *testing.T) {
				got := IsSubset(MustParseConstraint(tc.a), MustParseConstraint(tc.b))
				if got != tc.want {
					t.Errorf("IsSubset(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
				}
			})

	}
}

func TestIsSubsetIncludePrerelease(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"4.1.0-beta", ">=1.1", true},
		{"4.1.0-beta", ">1.1", true},
		{"0.1.0-alpha", "<=1.1", true},
		{"0.1.0-alpha", "<1.1", true},
		{"1.1.1-beta1", "^1.x", true},
		{"1.1.1-alpha", "~1.1", true},
		{"1.2.3-alpha", "*", true},
		{"2.0.1-beta", "= 2.0", true},
	}

	for i, tc := range cases {
		t.Run(strconv.Itoa(i),
			func(t *testing.T) {
				a := MustParseConstraint(tc.a)
				b := MustParseConstraint(tc.b)
				a.IncludePrerelease = true
				b.IncludePrerelease = true
				got := IsSubset(a, b)
				if got != tc.want {
					t.Errorf("IsSubset(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
				}
			})

	}
}
