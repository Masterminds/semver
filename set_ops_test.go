package semver

import "testing"

func TestRangeIntersection(t *testing.T) {
	var actual Constraint
	// Test basic overlap case
	rc1 := rangeConstraint{
		min: newV(1, 0, 0),
		max: newV(2, 0, 0),
	}
	rc2 := rangeConstraint{
		min: newV(1, 2, 0),
		max: newV(2, 2, 0),
	}
	result := rangeConstraint{
		min: newV(1, 2, 0),
		max: newV(2, 0, 0),
	}

	if actual = rc1.Intersect(rc2); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}
	if actual = rc2.Intersect(rc1); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}

	// And with includes
	rc1.includeMin = true
	rc1.includeMax = true
	rc2.includeMin = true
	rc2.includeMax = true
	result.includeMin = true
	result.includeMax = true

	if actual = rc1.Intersect(rc2); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}
	if actual = rc2.Intersect(rc1); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}

	// Overlaps with nils
	rc1 = rangeConstraint{
		min: newV(1, 0, 0),
	}
	rc2 = rangeConstraint{
		max: newV(2, 2, 0),
	}
	result = rangeConstraint{
		min: newV(1, 0, 0),
		max: newV(2, 2, 0),
	}

	if actual = rc1.Intersect(rc2); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}
	if actual = rc2.Intersect(rc1); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}

	// And with includes
	rc1.includeMin = true
	rc2.includeMax = true
	result.includeMin = true
	result.includeMax = true

	if actual = rc1.Intersect(rc2); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}
	if actual = rc2.Intersect(rc1); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}

	// Test superset overlap case
	rc1 = rangeConstraint{
		min: newV(1, 5, 0),
		max: newV(2, 0, 0),
	}
	rc2 = rangeConstraint{
		min: newV(1, 0, 0),
		max: newV(3, 0, 0),
	}
	result = rangeConstraint{
		min: newV(1, 5, 0),
		max: newV(2, 0, 0),
	}

	if actual = rc1.Intersect(rc2); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}
	if actual = rc2.Intersect(rc1); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}

	// Make sure irrelevant includes don't leak in
	rc2.includeMin = true
	rc2.includeMax = true

	if actual = rc1.Intersect(rc2); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}
	if actual = rc2.Intersect(rc1); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}

	// But relevant includes get used
	rc1.includeMin = true
	rc1.includeMax = true
	result.includeMin = true
	result.includeMax = true

	if actual = rc1.Intersect(rc2); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}
	if actual = rc2.Intersect(rc1); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}

	// Test disjoint case
	rc1 = rangeConstraint{
		min: newV(1, 5, 0),
		max: newV(1, 6, 0),
	}
	rc2 = rangeConstraint{
		min: newV(2, 0, 0),
		max: newV(3, 0, 0),
	}

	if actual = rc1.Intersect(rc2); !constraintEq(actual, None()) {
		t.Errorf("Got constraint %q, but expected %q", actual, None())
	}
	if actual = rc2.Intersect(rc1); !constraintEq(actual, None()) {
		t.Errorf("Got constraint %q, but expected %q", actual, None())
	}

	// Test disjoint at gt/lt boundary (non-adjacent)
	rc1 = rangeConstraint{
		min: newV(1, 5, 0),
		max: newV(2, 0, 0),
	}
	rc2 = rangeConstraint{
		min: newV(2, 0, 0),
		max: newV(3, 0, 0),
	}

	if actual = rc1.Intersect(rc2); !constraintEq(actual, None()) {
		t.Errorf("Got constraint %q, but expected %q", actual, None())
	}
	if actual = rc2.Intersect(rc1); !constraintEq(actual, None()) {
		t.Errorf("Got constraint %q, but expected %q", actual, None())
	}

	// Now, just have them touch at a single version
	rc1.includeMax = true
	rc2.includeMin = true

	vresult := newV(2, 0, 0)
	if actual = rc1.Intersect(rc2); !constraintEq(actual, vresult) {
		t.Errorf("Got constraint %q, but expected %q", actual, vresult)
	}
	if actual = rc2.Intersect(rc1); !constraintEq(actual, vresult) {
		t.Errorf("Got constraint %q, but expected %q", actual, vresult)
	}

	// Test excludes in intersection range
	rc1 = rangeConstraint{
		min: newV(1, 5, 0),
		max: newV(2, 0, 0),
		excl: []*Version{
			newV(1, 6, 0),
		},
	}
	rc2 = rangeConstraint{
		min: newV(1, 0, 0),
		max: newV(3, 0, 0),
	}

	if actual = rc1.Intersect(rc2); !constraintEq(actual, rc1) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc1)
	}
	if actual = rc2.Intersect(rc1); !constraintEq(actual, rc1) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc1)
	}

	// Test excludes not in intersection range
	rc1 = rangeConstraint{
		min: newV(1, 5, 0),
		max: newV(2, 0, 0),
	}
	rc2 = rangeConstraint{
		min: newV(1, 0, 0),
		max: newV(3, 0, 0),
		excl: []*Version{
			newV(1, 1, 0),
		},
	}

	if actual = rc1.Intersect(rc2); !constraintEq(actual, rc1) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc1)
	}
	if actual = rc2.Intersect(rc1); !constraintEq(actual, rc1) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc1)
	}

	// Ensure pure excludes come through as they should
	rc1 = rangeConstraint{
		excl: []*Version{
			newV(1, 6, 0),
		},
	}

	rc2 = rangeConstraint{
		excl: []*Version{
			newV(1, 6, 0),
			newV(1, 7, 0),
		},
	}

	if actual = Any().Intersect(rc1); !constraintEq(actual, rc1) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc1)
	}
	if actual = rc1.Intersect(Any()); !constraintEq(actual, rc1) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc1)
	}
	if actual = rc1.Intersect(rc2); !constraintEq(actual, rc2) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc2)
	}

	// TODO test the pre-release special range stuff
}

func TestRangeUnion(t *testing.T) {
	var actual Constraint
	// Test basic overlap case
	rc1 := rangeConstraint{
		min: newV(1, 0, 0),
		max: newV(2, 0, 0),
	}
	rc2 := rangeConstraint{
		min: newV(1, 2, 0),
		max: newV(2, 2, 0),
	}
	result := rangeConstraint{
		min: newV(1, 0, 0),
		max: newV(2, 2, 0),
	}

	if actual = rc1.Union(rc2); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}
	if actual = rc2.Union(rc1); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}

	// And with includes
	rc1.includeMin = true
	rc1.includeMax = true
	rc2.includeMin = true
	rc2.includeMax = true
	result.includeMin = true
	result.includeMax = true

	if actual = rc1.Union(rc2); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}
	if actual = rc2.Union(rc1); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}

	// Overlaps with nils
	rc1 = rangeConstraint{
		min: newV(1, 0, 0),
	}
	rc2 = rangeConstraint{
		max: newV(2, 2, 0),
	}

	if actual = rc1.Union(rc2); !constraintEq(actual, Any()) {
		t.Errorf("Got constraint %q, but expected %q", actual, Any())
	}
	if actual = rc2.Union(rc1); !constraintEq(actual, Any()) {
		t.Errorf("Got constraint %q, but expected %q", actual, Any())
	}

	// Test superset overlap case
	rc1 = rangeConstraint{
		min: newV(1, 5, 0),
		max: newV(2, 0, 0),
	}
	rc2 = rangeConstraint{
		min: newV(1, 0, 0),
		max: newV(3, 0, 0),
	}

	if actual = rc1.Union(rc2); !constraintEq(actual, rc2) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc2)
	}
	if actual = rc2.Union(rc1); !constraintEq(actual, rc2) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc2)
	}

	// Test disjoint case
	rc1 = rangeConstraint{
		min: newV(1, 5, 0),
		max: newV(1, 6, 0),
	}
	rc2 = rangeConstraint{
		min: newV(2, 0, 0),
		max: newV(3, 0, 0),
	}
	uresult := unionConstraint{rc1, rc2}

	if actual = rc1.Union(rc2); !constraintEq(actual, uresult) {
		t.Errorf("Got constraint %q, but expected %q", actual, uresult)
	}
	if actual = rc2.Union(rc1); !constraintEq(actual, uresult) {
		t.Errorf("Got constraint %q, but expected %q", actual, uresult)
	}

	// Test disjoint at gt/lt boundary (non-adjacent)
	rc1 = rangeConstraint{
		min: newV(1, 5, 0),
		max: newV(2, 0, 0),
	}
	rc2 = rangeConstraint{
		min: newV(2, 0, 0),
		max: newV(3, 0, 0),
	}
	uresult = unionConstraint{rc1, rc2}

	if actual = rc1.Union(rc2); !constraintEq(actual, uresult) {
		t.Errorf("Got constraint %q, but expected %q", actual, uresult)
	}
	if actual = rc2.Union(rc1); !constraintEq(actual, uresult) {
		t.Errorf("Got constraint %q, but expected %q", actual, uresult)
	}

	// Now, just have them touch at a single version
	rc1.includeMax = true
	rc2.includeMin = true
	result = rangeConstraint{
		min: newV(1, 5, 0),
		max: newV(3, 0, 0),
	}

	if actual = rc1.Union(rc2); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}
	if actual = rc2.Union(rc1); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}

	// Test excludes in overlapping range
	rc1 = rangeConstraint{
		min: newV(1, 5, 0),
		max: newV(2, 0, 0),
		excl: []*Version{
			newV(1, 6, 0),
		},
	}
	rc2 = rangeConstraint{
		min: newV(1, 0, 0),
		max: newV(3, 0, 0),
	}

	if actual = rc1.Union(rc2); !constraintEq(actual, rc2) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc2)
	}
	if actual = rc2.Union(rc1); !constraintEq(actual, rc2) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc2)
	}

	// Test excludes not in non-overlapping range
	rc1 = rangeConstraint{
		min: newV(1, 5, 0),
		max: newV(2, 0, 0),
	}
	rc2 = rangeConstraint{
		min: newV(1, 0, 0),
		max: newV(3, 0, 0),
		excl: []*Version{
			newV(1, 1, 0),
		},
	}

	if actual = rc1.Union(rc2); !constraintEq(actual, rc2) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc2)
	}
	if actual = rc2.Union(rc1); !constraintEq(actual, rc2) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc2)
	}

	// Ensure pure excludes come through as they should
	rc1 = rangeConstraint{
		excl: []*Version{
			newV(1, 6, 0),
		},
	}

	rc2 = rangeConstraint{
		excl: []*Version{
			newV(1, 6, 0),
			newV(1, 7, 0),
		},
	}

	if actual = rc1.Union(rc2); !constraintEq(actual, rc2) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc2)
	}
	if actual = rc2.Union(rc1); !constraintEq(actual, rc2) {
		t.Errorf("Got constraint %q, but expected %q", actual, rc2)
	}

	rc1 = rangeConstraint{
		excl: []*Version{
			newV(1, 5, 0),
		},
	}
	result = rangeConstraint{
		excl: []*Version{
			newV(1, 5, 0),
			newV(1, 6, 0),
			newV(1, 7, 0),
		},
	}

	if actual = rc1.Union(rc2); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}
	if actual = rc2.Union(rc1); !constraintEq(actual, result) {
		t.Errorf("Got constraint %q, but expected %q", actual, result)
	}

	// TODO test the pre-release special range stuff
}

func TestAreAdjacent(t *testing.T) {
	rc1 := rangeConstraint{
		min: newV(1, 0, 0),
		max: newV(2, 0, 0),
	}
	rc2 := rangeConstraint{
		min: newV(1, 2, 0),
		max: newV(2, 2, 0),
	}

	if areAdjacent(rc1, rc2) {
		t.Errorf("Ranges overlap, should not indicate as adjacent")
	}

	rc2 = rangeConstraint{
		min: newV(2, 0, 0),
	}

	if areAdjacent(rc1, rc2) {
		t.Errorf("Ranges are non-overlapping and non-adjacent, but reported as adjacent")
	}

	rc2.includeMin = true

	if !areAdjacent(rc1, rc2) {
		t.Errorf("Ranges are non-overlapping and adjacent, but reported as non-adjacent")
	}

	rc1.includeMax = true

	if areAdjacent(rc1, rc2) {
		t.Errorf("Ranges are overlapping at a single version, but reported as adjacent")
	}

	rc2.includeMin = false
	if !areAdjacent(rc1, rc2) {
		t.Errorf("Ranges are non-overlapping and adjacent, but reported as non-adjacent")
	}
}
