package semver

import (
	"fmt"
	"sort"
	"testing"
)

/* Constraint creation benchmarks */

func benchNewConstraint(c string, b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewConstraint(c)
	}
}

func BenchmarkNewConstraintUnary(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchNewConstraint("=2.0", b)
}

func BenchmarkNewConstraintTilde(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchNewConstraint("~2.0.0", b)
}

func BenchmarkNewConstraintCaret(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchNewConstraint("^2.0.0", b)
}

func BenchmarkNewConstraintWildcard(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchNewConstraint("1.x", b)
}

func BenchmarkNewConstraintRange(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchNewConstraint(">=2.1.x, <3.1.0", b)
}

func BenchmarkNewConstraintUnion(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchNewConstraint("~2.0.0 || =3.1.0", b)
}

/* Check benchmarks */

func benchCheckVersion(c, v string, b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	version, _ := NewVersion(v)
	constraint, _ := NewConstraint(c)

	for i := 0; i < b.N; i++ {
		constraint.Check(version)
	}
}

func BenchmarkCheckVersionUnary(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchCheckVersion("=2.0", "2.0.0", b)
}

func BenchmarkCheckVersionTilde(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchCheckVersion("~2.0.0", "2.0.5", b)
}

func BenchmarkCheckVersionCaret(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchCheckVersion("^2.0.0", "2.1.0", b)
}

func BenchmarkCheckVersionWildcard(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchCheckVersion("1.x", "1.4.0", b)
}

func BenchmarkCheckVersionRange(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchCheckVersion(">=2.1.x, <3.1.0", "2.4.5", b)
}

func BenchmarkCheckVersionUnion(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchCheckVersion("~2.0.0 || =3.1.0", "3.1.0", b)
}

func benchValidateVersion(c, v string, b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	version, _ := NewVersion(v)
	constraint, _ := NewConstraint(c)

	for i := 0; i < b.N; i++ {
		constraint.Validate(version)
	}
}

/* Validate benchmarks, including fails */

func BenchmarkValidateVersionUnary(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchValidateVersion("=2.0", "2.0.0", b)
}

func BenchmarkValidateVersionUnaryFail(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchValidateVersion("=2.0", "2.0.1", b)
}

func BenchmarkValidateVersionTilde(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchValidateVersion("~2.0.0", "2.0.5", b)
}

func BenchmarkValidateVersionTildeFail(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchValidateVersion("~2.0.0", "1.0.5", b)
}

func BenchmarkValidateVersionCaret(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchValidateVersion("^2.0.0", "2.1.0", b)
}

func BenchmarkValidateVersionCaretFail(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchValidateVersion("^2.0.0", "4.1.0", b)
}

func BenchmarkValidateVersionWildcard(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchValidateVersion("1.x", "1.4.0", b)
}

func BenchmarkValidateVersionWildcardFail(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchValidateVersion("1.x", "2.4.0", b)
}

func BenchmarkValidateVersionRange(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchValidateVersion(">=2.1.x, <3.1.0", "2.4.5", b)
}

func BenchmarkValidateVersionRangeFail(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchValidateVersion(">=2.1.x, <3.1.0", "1.4.5", b)
}

func BenchmarkValidateVersionUnion(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchValidateVersion("~2.0.0 || =3.1.0", "3.1.0", b)
}

func BenchmarkValidateVersionUnionFail(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchValidateVersion("~2.0.0 || =3.1.0", "3.1.1", b)
}

/* Version creation benchmarks */

func benchNewVersion(v string, b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = NewVersion(v)
	}
}

func benchStrictNewVersion(v string, b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = StrictNewVersion(v)
	}
}

func BenchmarkNewVersionSimple(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchNewVersion("1.0.0", b)
}

func BenchmarkCoerceNewVersionSimple(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchStrictNewVersion("1.0.0", b)
}

func BenchmarkNewVersionPre(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchNewVersion("1.0.0-alpha", b)
}

func BenchmarkStrictNewVersionPre(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchStrictNewVersion("1.0.0-alpha", b)
}

func BenchmarkNewVersionMeta(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchNewVersion("1.0.0+metadata", b)
}

func BenchmarkStrictNewVersionMeta(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchStrictNewVersion("1.0.0+metadata", b)
}

func BenchmarkNewVersionMetaDash(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchNewVersion("1.0.0-alpha.1+meta.data", b)
}

func BenchmarkStrictNewVersionMetaDash(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	benchStrictNewVersion("1.0.0-alpha.1+meta.data", b)
}

/* Compare and collection benchmarks */

func BenchmarkCompareSimple(b *testing.B) {
	b.ReportAllocs()
	a, _ := NewVersion("1.2.3")
	d, _ := NewVersion("2.0.0")
	for i := 0; i < b.N; i++ {
		_ = a.Compare(d)
	}
}

func BenchmarkComparePrerelease(b *testing.B) {
	b.ReportAllocs()
	a, _ := NewVersion("1.2.3-alpha.10.2-beta.3.4")
	d, _ := NewVersion("1.2.3-alpha.10.2-beta.3.4-c")
	for i := 0; i < b.N; i++ {
		_ = a.Compare(d)
	}
}

func BenchmarkLessThanPrerelease(b *testing.B) {
	b.ReportAllocs()
	a, _ := NewVersion("1.2.3-rc.1.a.b.1")
	d, _ := NewVersion("1.2.3-rc.1.a.b.2")
	for i := 0; i < b.N; i++ {
		_ = a.LessThan(d)
	}
}

func BenchmarkCollectionSort(b *testing.B) {
	b.ReportAllocs()
	versions := make([]*Version, 256)
	for i := range versions {
		versions[i], _ = NewVersion(fmt.Sprintf("%d.%d.%d-alpha.%d", i%50, i%20, i%10, i))
	}
	data := make([]*Version, len(versions))
	for i := 0; i < b.N; i++ {
		copy(data, versions)
		sort.Sort(Collection(data))
	}
}
