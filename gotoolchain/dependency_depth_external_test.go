package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/packages"
)

func nativeDependencyFixture(t *testing.T, directory string, bits uint8) gotoolchain.PackageAnalysis {
	t.Helper()
	a, b := "package a\n", "package b\n"
	if bits&1 != 0 {
		a += `import _ "example.com/native-symbols/internal/b"` + "\n"
	}
	if bits&2 != 0 {
		a += `import _ "example.com/native-symbols/internal/c"` + "\n"
	}
	if bits&4 != 0 {
		b += `import _ "example.com/native-symbols/internal/c"` + "\n"
	}
	for _, file := range []struct{ name, body string }{
		{name: "internal/a/a.go", body: a}, {name: "internal/b/b.go", body: b}, {name: "internal/c/c.go", body: "package c"},
		{name: "internal/c/excluded.go", body: "//go:build ignore\n\npackage c\nimport _ \"example.com/native-symbols/internal/a\""},
	} {
		name := filepath.Join(directory, file.name)
		if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(file.body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return nativeSymbolSourceAnalysis(t, directory, `package sample;import(_ "fmt";_ "example.com/native-symbols/internal/a");func F(){}`)
}

func dependencyScope(t testing.TB, text string) gomodule.ImportPath {
	t.Helper()
	scope, err := gomodule.ParseImportPath(text)
	if err != nil {
		t.Fatal(err)
	}
	return scope
}

func TestDependencyDepthReadsActualNativeClosure(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, within string
		bits         uint8
		want         uint64
	}{
		{name: "native blank imports retain full chain", within: "example.com/native-symbols/internal", bits: 7, want: 3},
		{name: "selected root traverses unselected dependencies", within: "example.com/native-symbols/internal", bits: 5, want: 3},
		{name: "root edge is a real hop", within: "example.com/native-symbols/internal", want: 1},
		{name: "sibling branches retain longest depth", within: "example.com/native-symbols/internal", bits: 3, want: 2},
		{name: "unreachable source package is not an edge", within: "example.com/native-symbols/internal", bits: 4, want: 1},
		{name: "exact subtree root is admitted", within: "example.com/native-symbols/internal/a", bits: 7, want: 1},
		{name: "scope does not tunnel through excluded parents", within: "example.com/native-symbols/internal/c", bits: 7},
		{name: "foreign prefix contributes no depth", within: "example.com/foreign", bits: 7},
		{name: "similar path prefix is not subtree", within: "example.com/native-symbols/interna", bits: 7},
		{name: "native standard package can define scope", within: "fmt", bits: 7, want: 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeDependencyFixture(t, t.TempDir(), row.bits)
			scope := dependencyScope(t, row.within)
			got, err := analysis.DependencyDepth(t.Context(), gotoolchain.PackageDependencyDepthRequest{Unit: analysis.Metadata[0], Within: scope})
			if err != nil || got.Depth != row.want || got.Source != analysis.Package || got.Within != scope || got.Validate() != nil {
				t.Fatalf("native dependency depth%+v/%v want%d", got, err, row.want)
			}
		})
	}
}

func TestDependencyDepthRefusesUnownedAndIncompleteFacts(t *testing.T) {
	t.Parallel()
	analysis := nativeDependencyFixture(t, t.TempDir(), 7)
	foreign := nativeDependencyFixture(t, t.TempDir(), 7)
	clone := *analysis.Metadata[0]
	scope := dependencyScope(t, "example.com/native-symbols/internal")
	for _, row := range []struct {
		name   string
		source gotoolchain.PackageAnalysis
		unit   *packages.Package
		scope  gomodule.ImportPath
		want   error
	}{
		{name: "missing root", source: analysis, scope: scope, want: core.ErrGoToolchainContract},
		{name: "copied equal root has no ownership", source: analysis, unit: &clone, scope: scope, want: core.ErrGoToolchainContract},
		{name: "foreign equal source has no ownership", source: analysis, unit: foreign.Metadata[0], scope: scope, want: core.ErrGoToolchainContract},
		{name: "dependency is not selected root", source: analysis, unit: analysis.Metadata[0].Imports["example.com/native-symbols/internal/a"], scope: scope, want: core.ErrGoToolchainContract},
		{name: "absent scope", source: analysis, unit: analysis.Metadata[0], want: core.ErrGoToolchainContract},
		{name: "absent parent", unit: analysis.Metadata[0], scope: scope, want: core.ErrGoToolchainContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			got, err := row.source.DependencyDepth(t.Context(), gotoolchain.PackageDependencyDepthRequest{Unit: row.unit, Within: row.scope})
			if got != (gotoolchain.PackageDependencyDepthObservation{}) || !errors.Is(err, row.want) {
				t.Fatalf("refused depth%+v/%v want%v", got, err, row.want)
			}
		})
	}
	incomplete := analysis
	incomplete.Incomplete = true
	refused, refusal := incomplete.DependencyDepth(t.Context(), gotoolchain.PackageDependencyDepthRequest{Unit: analysis.Metadata[0], Within: scope})
	if refused != (gotoolchain.PackageDependencyDepthObservation{}) || !errors.Is(refusal, core.ErrGoToolchainOutput) {
		t.Fatalf("incomplete parent depth%+v/%v", refused, refusal)
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrClosedPipe)
	got, err := analysis.DependencyDepth(ctx, gotoolchain.PackageDependencyDepthRequest{Unit: analysis.Metadata[0], Within: scope})
	if got != (gotoolchain.PackageDependencyDepthObservation{}) || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("cancelled depth%+v/%v", got, err)
	}
	expired, release := context.WithDeadlineCause(t.Context(), time.Time{}, io.ErrClosedPipe)
	defer release()
	for _, row := range []struct {
		name        string
		ctx         context.Context
		want, cause error
	}{
		{"nil context refuses missing ownership", nil, core.ErrNilContext, nil},
		{"expired deadline retains native cause", expired, context.DeadlineExceeded, io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := analysis.DependencyDepth(row.ctx, gotoolchain.PackageDependencyDepthRequest{Unit: analysis.Metadata[0], Within: scope})
			if got != (gotoolchain.PackageDependencyDepthObservation{}) || !errors.Is(err, row.want) || (row.cause != nil && !errors.Is(err, row.cause)) {
				t.Fatalf("context depth got%+v/%v wantzero/%v and%v", got, err, row.want, row.cause)
			}
		})
	}
}

func TestDependencyDepthMissingNativeEdgesCannotBecomeClean(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"missing edge", "native cycle", "native diagnostic", "missing package identity"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			analysis := nativeDependencyFixture(t, t.TempDir(), 7)
			root := analysis.Metadata[0]
			a := root.Imports["example.com/native-symbols/internal/a"]
			b := a.Imports["example.com/native-symbols/internal/b"]
			c := a.Imports["example.com/native-symbols/internal/c"]
			if a == nil || b == nil || c == nil {
				t.Fatal("native import closure absent before mutation")
			}
			switch mode {
			case "missing edge":
				a.Imports["example.com/native-symbols/internal/b"] = nil
			case "native cycle":
				c.Imports["example.com/native-symbols/internal/a"] = a
			case "native diagnostic":
				b.Errors = append(b.Errors, packages.Error{Msg: "native withheld diagnostic"})
			case "missing package identity":
				b.PkgPath = ""
			}
			got, err := analysis.DependencyDepth(t.Context(), gotoolchain.PackageDependencyDepthRequest{Unit: root, Within: dependencyScope(t, "example.com/native-symbols/internal")})
			if got != (gotoolchain.PackageDependencyDepthObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
				t.Fatalf("withheld native fact%+v/%v", got, err)
			}
		})
	}
}

func TestDependencyDepthObservationRequiresOwnedShape(t *testing.T) {
	t.Parallel()
	scope := dependencyScope(t, "example.com/native-symbols/internal")
	source := dependencyScope(t, "example.com/native-symbols")
	for _, observation := range []gotoolchain.PackageDependencyDepthObservation{{}, {Source: source}, {Within: scope}, {Source: source, Within: scope, Depth: math.MaxUint64}} {
		if err := observation.Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
			t.Fatalf("invalid native depth%+v/%v", observation, err)
		}
	}
	for _, depth := range []uint64{0, 1, uint64(math.MaxInt)} {
		if err := (gotoolchain.PackageDependencyDepthObservation{Source: source, Within: scope, Depth: depth}).Validate(); err != nil {
			t.Fatalf("representable native depth%d/%v", depth, err)
		}
	}
}

func FuzzDependencyDepthNativeGraphConservation(f *testing.F) {
	for bits := uint8(0); bits < 8; bits++ {
		f.Add(bits)
	}
	f.Fuzz(func(t *testing.T, bits uint8) {
		analysis := nativeDependencyFixture(t, t.TempDir(), bits)
		scope := dependencyScope(t, "example.com/native-symbols/internal")
		got, err := analysis.DependencyDepth(t.Context(), gotoolchain.PackageDependencyDepthRequest{Unit: analysis.Metadata[0], Within: scope})
		if err != nil {
			t.Fatal(err)
		}
		// Enumerate the three possible actual paths rather than replay traversal
		// or memoization. Unreachable source files cannot contribute an edge.
		want := uint64(1)
		if bits&1 != 0 || bits&2 != 0 {
			want = 2
		}
		if bits&1 != 0 && bits&4 != 0 {
			want = 3
		}
		if got.Depth != want || got.Source != analysis.Package || got.Within != scope || got.Validate() != nil {
			t.Fatalf("native DAG bits%d depth%+v want%d", bits, got, want)
		}
	})
}
