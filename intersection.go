package semver

import (
	"cmp"
	"slices"
	"strings"
)

// Intersection returns a Constraints struct satisfied by all versions that satisfy a and b (a ∩ b).
// Returns nil if either input is nil.
func Intersection(a, b *Constraints) *Constraints {
	if a == nil || b == nil {
		return nil
	}

	ca, cb := canonicalise(a), canonicalise(b)
	var out [][]*constraint
	for _, ga := range ca.constraints {
		for _, gb := range cb.constraints {
			g := intersect(ga, gb)
			out = append(out, g)

		}
	}
	if len(out) == 0 {
		return &Constraints{}
	}
	return canonicalise(&Constraints{constraints: out})
}

// IsSubset returns true if every version satisfying sub also satisfies sup (sub ⊆ sup).
// Returns false if either input is nil.
func IsSubset(sub, sup *Constraints) bool {
	return sub != nil && sup != nil &&
		Intersection(sub, sup).String() == canonicalise(sub).String()
}

func intersect(a, b []*constraint) []*constraint {
	ea, ra := splitExact(a)
	eb, rb := splitExact(b)

	switch {
	case len(ra) == 0 && len(rb) == 0:
		return exactIntersection(ea, eb)
	case len(ra) == 0:
		return filterExact(ea, b)
	case len(rb) == 0:
		return filterExact(eb, a)
	default:
		return simplify(append(append([]*constraint{}, a...), b...))
	}
}

func splitExact(cs []*constraint) (exact, ranges []*constraint) {
	for _, c := range cs {
		if c.origfunc == "" || c.origfunc == "=" {
			exact = append(exact, c)
		} else {
			ranges = append(ranges, c)
		}
	}
	return exact, ranges
}

func exactIntersection(a, b []*constraint) (res []*constraint) {
	for _, ea := range a {
		for _, eb := range b {
			if ea.con.Equal(eb.con) {
				res = append(res, ea)
			}
		}
	}
	return res
}

func filterExact(exact, cs []*constraint) (res []*constraint) {
	for _, e := range exact {
		if satisfiesAll(e.con, cs) {
			res = append(res, e)
		}
	}
	return res
}

func satisfiesAll(v *Version, cs []*constraint) bool {
	for _, c := range cs {
		compare := v.Compare(c.con)
		switch c.origfunc {
		case ">":
			if compare <= 0 {
				return false
			}
		case ">=":
			if compare < 0 {
				return false
			}
		case "<":
			if compare >= 0 {
				return false
			}
		case "<=":
			if compare > 0 {
				return false
			}
		}
	}
	return true
}

func canonicalise(c *Constraints) *Constraints {
	if c == nil {
		return nil
	}

	seen := make(map[string]struct{})
	var groups [][]*constraint
	for _, g := range c.constraints {
		clean := simplify(expand(g))
		if isValid(clean) {
			k := groupKey(clean)
			_, ok := seen[k]
			if !ok {
				seen[k] = struct{}{}
				groups = append(groups, clean)
			}
		}
	}
	slices.SortFunc(groups, func(a, b []*constraint) int {
		return cmp.Compare(groupKey(a), groupKey(b))
	})

	return &Constraints{constraints: groups}
}

func expand(cs []*constraint) (res []*constraint) {
	for _, c := range cs {
		res = append(res, expandConstraint(c)...)
	}
	return res
}

func expandConstraint(c *constraint) []*constraint {
	switch c.origfunc {
	case "^":
		return createRange(c, func() Version {
			if c.con.Major() > 0 {
				return c.con.IncMajor()
			}
			return c.con.IncMinor()
		})
	case "~", "~>":
		return createRange(c, func() Version {
			if c.minorDirty {
				return c.con.IncMajor()
			}
			return c.con.IncMinor()
		})
	case "", "=":
		if c.dirty {
			return expandWildcard(c)
		}
	case "<=":
		if c.dirty {
			var hi Version
			if c.minorDirty {
				hi = c.con.IncMajor()
			} else {
				hi = c.con.IncMinor()
			}
			return []*constraint{upperConstraint(hi)}
		}
	}

	return []*constraint{c}
}

func createRange(c *constraint, upper func() Version) []*constraint {
	return []*constraint{clone(c, ">="), upperConstraint(upper())}
}

func expandWildcard(c *constraint) []*constraint {
	lo := clone(c, ">=")
	var hi Version
	switch {
	case c.minorDirty:
		hi = c.con.IncMajor()
	case c.patchDirty:
		hi = c.con.IncMinor()
	default:
		return []*constraint{lo}
	}

	return []*constraint{lo, upperConstraint(hi)}
}

func simplify(cs []*constraint) (res []*constraint) {
	if len(cs) <= 1 {
		return cs
	}
	lo, hi := bounds(cs)
	if lo != nil {
		res = append(res, lo)
	}
	if hi != nil {
		res = append(res, hi)
	}

	return res
}

func better(cur, cand *constraint, dir int) bool {
	if cand == nil {
		return false
	}
	if cur == nil {
		return true
	}
	diff := cand.con.Compare(cur.con)
	if diff != 0 {
		return diff*dir > 0
	}
	if dir > 0 {
		return cur.origfunc == ">=" && cand.origfunc == ">"
	}

	return cur.origfunc == "<=" && cand.origfunc == "<"
}

func clone(c *constraint, op string) *constraint {
	return &constraint{con: c.con, orig: c.con.String(), origfunc: op}
}

func upperConstraint(v Version) *constraint {
	return &constraint{con: &v, orig: v.String(), origfunc: "<"}
}

func groupKey(cs []*constraint) string {
	var sb strings.Builder
	for _, c := range cs {
		sb.WriteString(c.string())
		sb.WriteByte(' ')
	}
	return sb.String()
}

func isValid(cs []*constraint) bool {
	if len(cs) == 0 {
		return false
	}

	lo, hi := bounds(cs)
	if lo == nil || hi == nil {
		return true
	}

	compare := lo.con.Compare(hi.con)
	if compare > 0 || (compare == 0 && (lo.origfunc != ">=" || hi.origfunc != "<=")) {
		return false
	}
	return true
}

func bounds(cs []*constraint) (lo, hi *constraint) {
	for _, c := range cs {
		switch c.origfunc {
		case ">", ">=":
			if better(lo, c, 1) {
				lo = c
			}
		case "<", "<=":
			if better(hi, c, -1) {
				hi = c
			}
		}
	}
	return lo, hi
}
