package gotoolchain

import (
	"errors"
	"go/version"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

// This ratchets previously unavailable native ordering. Fault triads challenge
// each result and admission branch; the test oracle is Go's public API.
func TestToolchainVersionComparisonMatchesNativeGo(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, left, right string }{
		{name: "language below iteration floor", left: "go1.21", right: "go1.22"},
		{name: "equal language floor", left: "go1.22", right: "go1.22"},
		{name: "language above floor", left: "go1.23", right: "go1.22"},
		{name: "language precedes patch release", left: "go1.22", right: "go1.22.0"},
		{name: "patch order is decimal", left: "go1.27.10", right: "go1.27.9"},
		{name: "beta order is decimal", left: "go1.27beta10", right: "go1.27beta9"},
		{name: "release candidate follows beta", left: "go1.27rc1", right: "go1.27beta9"},
		{name: "release follows candidate", left: "go1.27.0", right: "go1.27rc9"},
		{name: "custom suffix does not alter order", left: "go1.27.2-custom", right: "go1.27.2"},
		{name: "empty custom suffix remains native", left: "go1.27.2-", right: "go1.27.2"},
		{name: "lowercase native kind order", left: "go1.27foo1", right: "go1.27rc1"},
		{name: "native future major", left: "go2.0", right: "go1.999"},
		{name: "native zero major", left: "go0.0", right: "go1.0"},
		{name: "native minor decimal extent", left: "go1." + strings.Repeat("9", 4097), right: "go1.999"},
		{name: "unrelated custom extent", left: "go1.27.2-" + strings.Repeat("x", 131073), right: "go1.27.2-other"},
		{name: "invalid left cannot become ordered", left: "go1.027", right: "go1.22"},
		{name: "invalid right cannot become ordered", left: "go1.22", right: "go1.027"},
		{name: "both unknown cannot become equal", left: "", right: ""},
		{name: "invalid native UTF8 suffix", left: "go1.27.2-\xff", right: "go1.27.2"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := (ToolchainVersion{value: row.left}).Compare(ToolchainVersion{value: row.right})
			valid := utf8.ValidString(row.left) && utf8.ValidString(row.right) && version.IsValid(row.left) && version.IsValid(row.right)
			if !valid {
				if !errors.Is(err, core.ErrGoToolchainContract) || got != core.ComparisonUnknown {
					t.Fatalf("refused order=%v/%v", got, err)
				}
				return
			}
			native := version.Compare(row.left, row.right)
			want := core.ComparisonEqual
			if native < 0 {
				want = core.ComparisonLess
			} else if native > 0 {
				want = core.ComparisonGreater
			}
			if err != nil || got != want {
				t.Fatalf("native order=%v/%v want%v", got, err, want)
			}
			reverse, err := (ToolchainVersion{value: row.right}).Compare(ToolchainVersion{value: row.left})
			if err != nil || got == core.ComparisonEqual && reverse != got || got == core.ComparisonLess && reverse != core.ComparisonGreater || got == core.ComparisonGreater && reverse != core.ComparisonLess {
				t.Fatalf("native order inversion=%v/%v/%v", got, reverse, err)
			}
		})
	}
}

func TestToolchainVersionComparisonBorrowsNativeExtent(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, extent := range []int{64, 4097, 131073} {
		left, err := ParseToolchainVersion("go1.27.2-" + strings.Repeat("x", extent))
		if err != nil {
			t.Fatal(err)
		}
		right, err := ParseToolchainVersion("go1.27.2")
		if err != nil {
			t.Fatal(err)
		}
		var got core.Comparison
		allocations := testing.AllocsPerRun(16, func() { got, err = left.Compare(right) })
		if err != nil || got != core.ComparisonEqual || allocations != 0 {
			t.Fatalf("native extent%d order%v/%v allocations%g", extent, got, err, allocations)
		}
	}
}

func FuzzToolchainVersionComparisonNativeOrdering(f *testing.F) {
	for _, row := range []struct{ left, right string }{{"go1.21", "go1.22"}, {"go1.22", "go1.22"}, {"go1.27rc1", "go1.27.0"}, {"go1.27.2-custom", "go1.27.2"}} {
		left, err := ParseToolchainVersion(row.left)
		if err != nil {
			f.Fatal(err)
		}
		right, err := ParseToolchainVersion(row.right)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(left.String(), right.String())
	}
	f.Add("", "go1.22")
	f.Add("go1.22", "go1.027")
	f.Fuzz(func(t *testing.T, left, right string) {
		a, ea := ParseToolchainVersion(left)
		b, eb := ParseToolchainVersion(right)
		got, err := a.Compare(b)
		if ea != nil || eb != nil {
			if !errors.Is(err, core.ErrGoToolchainContract) || got != core.ComparisonUnknown {
				t.Fatalf("invalid parsed ordering=%v/%v", got, err)
			}
			return
		}
		native := version.Compare(left, right)
		want := core.ComparisonEqual
		if native < 0 {
			want = core.ComparisonLess
		} else if native > 0 {
			want = core.ComparisonGreater
		}
		if err != nil || got != want {
			t.Fatalf("native order%v/%v want%v", got, err, want)
		}
	})
}
