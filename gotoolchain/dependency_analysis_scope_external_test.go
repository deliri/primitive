package gotoolchain_test

import (
	"context"
	"errors"
	"go/constant"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/packages"
)

func dependencyScopeFixture(t testing.TB, directory string) (gotoolchain.Capability, gotoolchain.AnalysisRequest, gomodule.ImportPath) {
	t.Helper()
	capability, batch := analysisBatchFixture(t, directory)
	for _, name := range []string{"first.go", "second.go"} {
		if err := os.WriteFile(filepath.Join(directory, "a", name), []byte("package a\nimport _ \"example.com/batch/b\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dependencyScopePrivateSource(t, directory, "NativeLayer+0")
	return capability, gotoolchain.AnalysisRequest{WorkingDirectory: batch.WorkingDirectory, Package: batch.Packages[0]}, batch.Packages[1]
}

func dependencyScopePrivateSource(t testing.TB, directory, expression string) {
	t.Helper()
	body := "package b\ntype Kind uint8\ntype Alias=Kind\nconst NativeLayer Kind=7\nconst privateLayer Alias=" + expression + "\n"
	if err := os.WriteFile(filepath.Join(directory, "b", "private.go"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func dependencyScopePrivateValue(t testing.TB, analysis gotoolchain.PackageAnalysis) int64 {
	t.Helper()
	if err := analysis.Validate(); err != nil || analysis.Incomplete || len(analysis.Units) != 1 {
		t.Fatalf("dependency analysis units=%d incomplete=%t error=%v, want one complete unit", len(analysis.Units), analysis.Incomplete, err)
	}
	object, ok := analysis.Units[0].Types.Scope().Lookup("privateLayer").(*types.Const)
	if !ok {
		t.Fatalf("private declaration=%T, want actual types.Const", analysis.Units[0].Types.Scope().Lookup("privateLayer"))
	}
	named, ok := types.Unalias(object.Type()).(*types.Named)
	if !ok || named.Obj().Name() != "Kind" || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != "example.com/batch/b" {
		t.Fatalf("private declaration type=%v, want native b.Kind through alias", object.Type())
	}
	value, exact := constant.Int64Val(object.Val())
	if !exact {
		t.Fatalf("private constant=%v, want exact integer", object.Val())
	}
	return value
}

func TestDependencyAnalysisScopePrivateRepeatedSites(t *testing.T) {
	t.Parallel()
	capability, request, imported := dependencyScopeFixture(t, t.TempDir())
	reference, err := capability.AnalyzePackage(t.Context(), gotoolchain.AnalysisRequest{WorkingDirectory: request.WorkingDirectory, Package: imported})
	if err != nil {
		t.Fatal(err)
	}
	wantValue := dependencyScopePrivateValue(t, reference)
	if wantValue != 7 {
		t.Fatalf("original private value=%d, want 7", wantValue)
	}
	err = capability.WithDependencyAnalysis(t.Context(), request, func(ctx context.Context, root gotoolchain.PackageAnalysis, scope *gotoolchain.DependencyAnalysisScope) error {
		var first *packages.Package
		count := 0
		for _, file := range root.Units[0].Syntax {
			for _, spec := range file.Imports {
				path, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					return err
				}
				if path != imported.String() {
					continue
				}
				analysis, err := scope.AnalyzeDependency(ctx, gotoolchain.DependencyAnalysisRequest{Unit: root.Units[0], Import: imported})
				if err != nil {
					return err
				}
				if value := dependencyScopePrivateValue(t, analysis); value != wantValue {
					t.Fatalf("private value=%d, want original %d", value, wantValue)
				}
				if first == nil {
					first = analysis.Units[0]
				} else if first != analysis.Units[0] {
					t.Fatal("repeated dependency returned a different compiler unit, want same borrowed unit")
				}
				physical := root.Units[0].Fset.PositionFor(spec.Pos(), false)
				want := "first.go"
				if count == 1 {
					want = "second.go"
				}
				if filepath.Base(physical.Filename) != want || physical.Line != 2 || physical.Column != 8 {
					t.Fatalf("physical occurrence=%s:%d:%d, want %s:2:8", physical.Filename, physical.Line, physical.Column, want)
				}
				count++
			}
		}
		if count != 2 {
			t.Fatalf("physical occurrences=%d, want 2", count)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("dependency scope error=%v, want nil", err)
	}
}

func TestDependencyAnalysisScopeOwnershipAndExpiry(t *testing.T) {
	t.Parallel()
	capability, request, imported := dependencyScopeFixture(t, t.TempDir())
	foreign, err := capability.AnalyzePackage(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	var expired *gotoolchain.DependencyAnalysisScope
	var copied gotoolchain.DependencyAnalysisScope
	var owned *packages.Package
	err = capability.WithDependencyAnalysis(t.Context(), request, func(ctx context.Context, root gotoolchain.PackageAnalysis, scope *gotoolchain.DependencyAnalysisScope) error {
		expired, copied, owned = scope, *scope, root.Units[0]
		copyOfOwned := *owned
		for _, unit := range []*packages.Package{foreign.Units[0], &copyOfOwned} {
			result, err := scope.AnalyzeDependency(ctx, gotoolchain.DependencyAnalysisRequest{Unit: unit, Import: imported})
			if !errors.Is(err, core.ErrGoToolchainContract) || len(result.Units) != 0 {
				t.Fatalf("foreign dependency units=%d error=%v, want zero and contract refusal", len(result.Units), err)
			}
		}
		missing, err := gomodule.ParseImportPath("example.com/batch/not-imported")
		if err != nil {
			return err
		}
		result, err := scope.AnalyzeDependency(ctx, gotoolchain.DependencyAnalysisRequest{Unit: owned, Import: missing})
		if !errors.Is(err, core.ErrGoToolchainContract) || len(result.Units) != 0 {
			t.Fatalf("unowned import units=%d error=%v, want zero and contract refusal", len(result.Units), err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []*gotoolchain.DependencyAnalysisScope{expired, &copied} {
		result, err := scope.AnalyzeDependency(t.Context(), gotoolchain.DependencyAnalysisRequest{Unit: owned, Import: imported})
		if !errors.Is(err, core.ErrGoToolchainContract) || len(result.Units) != 0 {
			t.Fatalf("expired original/copied dependency units=%d error=%v, want zero and contract refusal", len(result.Units), err)
		}
	}
}

func TestDependencyAnalysisScopeCancellationCauses(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"query context", "owning context", "final completion"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			capability, request, imported := dependencyScopeFixture(t, t.TempDir())
			owner, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			err := capability.WithDependencyAnalysis(owner, request, func(ctx context.Context, root gotoolchain.PackageAnalysis, scope *gotoolchain.DependencyAnalysisScope) error {
				query := gotoolchain.DependencyAnalysisRequest{Unit: root.Units[0], Import: imported}
				analysis, err := scope.AnalyzeDependency(ctx, query)
				if err != nil || dependencyScopePrivateValue(t, analysis) != 7 {
					t.Fatalf("first dependency error=%v, want complete private value 7", err)
				}
				if name == "final completion" {
					cancel(io.ErrClosedPipe)
					return nil
				}
				queryContext := context.Background()
				if name == "query context" {
					child, stop := context.WithCancelCause(ctx)
					stop(io.ErrClosedPipe)
					queryContext = child
				} else {
					cancel(io.ErrClosedPipe)
				}
				result, err := scope.AnalyzeDependency(queryContext, query)
				if len(result.Units) != 0 || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrGoToolchainExecution) {
					t.Fatalf("canceled reused dependency units=%d error=%v, want zero and both native cancellation causes", len(result.Units), err)
				}
				return nil
			})
			if name == "query context" {
				if err != nil {
					t.Fatalf("healthy owner completion error=%v, want nil", err)
				}
			} else if !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) {
				t.Fatalf("scope completion error=%v, want canceled and closed-pipe cause", err)
			}
		})
	}
}

func TestDependencyAnalysisScopeRootBodyRefusal(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	capability, request, _ := dependencyScopeFixture(t, directory)
	if err := os.WriteFile(filepath.Join(directory, "b", "broken.go"), []byte("package b\nfunc broken(){ absent() }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// go list -export can find this body error while opening the root. This
	// is deliberately not claimed to be a later first-demand-only failure.
	_, referenceErr := capability.AnalyzePackage(t.Context(), request)
	if !errors.Is(referenceErr, core.ErrGoToolchainOutput) {
		t.Fatalf("reference full analysis error=%v, want compiler output refusal", referenceErr)
	}
	calls := 0
	err := capability.WithDependencyAnalysis(t.Context(), request, func(context.Context, gotoolchain.PackageAnalysis, *gotoolchain.DependencyAnalysisScope) error {
		calls++
		return nil
	})
	if calls != 0 || !errors.Is(err, core.ErrGoToolchainOutput) {
		t.Fatalf("root refusal callbacks=%d error=%v, want zero and compiler output refusal", calls, err)
	}
}

func TestDependencyAnalysisScopeReopensChangedSource(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	capability, request, imported := dependencyScopeFixture(t, directory)
	for _, want := range []int64{7, 9} {
		if want == 9 {
			// Mutation occurs only AFTER the previous scope closed. Inputs and
			// borrowed compiler/metadata objects remain immutable within each
			// callback; no automatic within-loan mutation detection is claimed.
			dependencyScopePrivateSource(t, directory, "NativeLayer+2")
		}
		err := capability.WithDependencyAnalysis(t.Context(), request, func(ctx context.Context, root gotoolchain.PackageAnalysis, scope *gotoolchain.DependencyAnalysisScope) error {
			analysis, err := scope.AnalyzeDependency(ctx, gotoolchain.DependencyAnalysisRequest{Unit: root.Units[0], Import: imported})
			if err != nil {
				return err
			}
			if got := dependencyScopePrivateValue(t, analysis); got != want {
				t.Fatalf("reopened private value=%d, want %d", got, want)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("reopened scope error=%v, want nil", err)
		}
	}
}

func TestDependencyAnalysisScopeIngress(t *testing.T) {
	t.Parallel()
	capability, request, _ := dependencyScopeFixture(t, t.TempDir())
	for _, name := range []string{"nil context", "canceled context", "test variants", "nil consumer", "consumer refusal"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			candidate := request
			var want error = core.ErrGoToolchainContract
			calls := 0
			use := func(context.Context, gotoolchain.PackageAnalysis, *gotoolchain.DependencyAnalysisScope) error {
				calls++
				return os.ErrPermission
			}
			switch name {
			case "nil context":
				ctx = nil
			case "canceled context":
				cancelled, cancel := context.WithCancelCause(ctx)
				cancel(io.ErrClosedPipe)
				ctx, want = cancelled, io.ErrClosedPipe
			case "test variants":
				candidate.IncludeTests = true
			case "nil consumer":
				use = nil
			case "consumer refusal":
				want = os.ErrPermission
			}
			err := capability.WithDependencyAnalysis(ctx, candidate, use)
			wantCalls := 0
			if name == "consumer refusal" {
				wantCalls = 1
			}
			if calls != wantCalls || !errors.Is(err, want) {
				t.Fatalf("scope ingress callbacks=%d error=%v, want %d and %v", calls, err, wantCalls, want)
			}
		})
	}
}
