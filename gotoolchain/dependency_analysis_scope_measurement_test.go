//go:build darwin || linux

package gotoolchain

import (
	"context"
	"go/constant"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

func dependencyScopeMeasurementFixture(t testing.TB, directory string) (Capability, AnalysisRequest) {
	t.Helper()
	for _, file := range []struct{ path, source string }{
		{path: "go.mod", source: "module example.com/dependency-measure\n\ngo 1.27.1\n"},
		{path: "a/first.go", source: "package a\nimport _ \"example.com/dependency-measure/b\"\n"},
		{path: "a/second.go", source: "package a\nimport _ \"example.com/dependency-measure/b\"\n"},
		{path: "b/value.go", source: "package b\nimport \"strings\"\ntype Kind uint8\ntype Alias=Kind\nconst NativeLayer Kind=7\nconst privateLayer Alias=NativeLayer+0\nvar Value=strings.Contains(\"value\",\"a\")\n"},
	} {
		path := filepath.Join(directory, file.path)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(file.source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"first.go", "second.go"} {
		body, err := os.ReadFile(filepath.Join(directory, "a", name))
		if err != nil || string(body) != "package a\nimport _ \"example.com/dependency-measure/b\"\n" {
			t.Fatalf("owned import source %s=%q error=%v, want exact real a-to-b import", name, body, err)
		}
	}
	root, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	path, err := gomodule.ParseImportPath("example.com/dependency-measure/a")
	if err != nil {
		t.Fatal(err)
	}
	limits, err := DefaultLimits()
	if err != nil {
		t.Fatal(err)
	}
	capability, err := Open(context.Background(), Configuration{Workspace: WorkspaceModeDisabled, Limits: limits})
	if err != nil {
		t.Fatal(err)
	}
	return capability, AnalysisRequest{WorkingDirectory: root, Package: path}
}

func dependencyScopeMeasurementPrivate(analysis PackageAnalysis) bool {
	if analysis.Incomplete || len(analysis.Units) != 1 || analysis.Units[0] == nil || analysis.Units[0].Types == nil {
		return false
	}
	object, ok := analysis.Units[0].Types.Scope().Lookup("privateLayer").(*types.Const)
	if !ok {
		return false
	}
	named, ok := types.Unalias(object.Type()).(*types.Named)
	value, exact := constant.Int64Val(object.Val())
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "example.com/dependency-measure/b" && named.Obj().Name() == "Kind" && exact && value == 7
}

func dependencyScopeMeasurementSeparate(ctx context.Context, capability Capability, request AnalysisRequest) (int, bool, error) {
	root, err := capability.AnalyzePackage(ctx, request)
	if err != nil {
		return 0, false, err
	}
	count, facts := 0, true
	for _, file := range root.Units[0].Syntax {
		for _, spec := range file.Imports {
			text, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return count, false, err
			}
			path, err := gomodule.ParseImportPath(text)
			if err != nil {
				return count, false, err
			}
			analysis, err := capability.AnalyzePackage(ctx, AnalysisRequest{WorkingDirectory: request.WorkingDirectory, Package: path})
			if err != nil {
				return count, false, err
			}
			private := dependencyScopeMeasurementPrivate(analysis)
			position := root.Units[0].Fset.PositionFor(spec.Pos(), false)
			want := "first.go"
			if count == 1 {
				want = "second.go"
			}
			physical := filepath.Base(position.Filename) == want && position.Line == 2 && position.Column == 8
			facts = facts && private && physical
			count++
		}
	}
	return count, facts, nil
}

func dependencyScopeMeasurementBorrowed(ctx context.Context, capability Capability, request AnalysisRequest) (int, bool, error) {
	count, facts := 0, true
	err := capability.WithDependencyAnalysis(ctx, request, func(ctx context.Context, root PackageAnalysis, scope *DependencyAnalysisScope) error {
		for _, file := range root.Units[0].Syntax {
			for _, spec := range file.Imports {
				text, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					return err
				}
				path, err := gomodule.ParseImportPath(text)
				if err != nil {
					return err
				}
				analysis, err := scope.AnalyzeDependency(ctx, DependencyAnalysisRequest{Unit: root.Units[0], Import: path})
				if err != nil {
					return err
				}
				// All compiler facts are consumed while the loan is live.
				private := dependencyScopeMeasurementPrivate(analysis)
				position := root.Units[0].Fset.PositionFor(spec.Pos(), false)
				want := "first.go"
				if count == 1 {
					want = "second.go"
				}
				physical := filepath.Base(position.Filename) == want && position.Line == 2 && position.Column == 8
				facts = facts && private && physical
				count++
			}
		}
		return nil
	})
	return count, facts, err
}

// This reuses the existing analysis_batch_commands_unix_test.go technique:
// record each actual bound Go process, then exec the real resolved Go binary.
// Counts cover direct env/list invocations, not every compiler child process.
func TestDependencyAnalysisScopeRealCompilerCommands(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"separate", "borrowed"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			capability, request := dependencyScopeMeasurementFixture(t, directory)
			marker := filepath.Join(directory, "commands")
			wrapper := filepath.Join(directory, "go-probe")
			script := "#!/bin/sh\nset -eu\nprintf x >> " + shellFixtureQuote(marker) + "\nexec " + shellFixtureQuote(capability.command.String()) + " \"$@\"\n"
			if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			command, err := core.ParseAbsolutePath(wrapper)
			if err != nil {
				t.Fatal(err)
			}
			capability.command = command
			want := 6
			var count int
			var facts bool
			if name == "separate" {
				count, facts, err = dependencyScopeMeasurementSeparate(t.Context(), capability, request)
			} else {
				count, facts, err = dependencyScopeMeasurementBorrowed(t.Context(), capability, request)
				want = 2
			}
			if err != nil || count != 2 || !facts {
				t.Fatalf("native operation count/facts/error=%d/%t/%v, want2/true/nil", count, facts, err)
			}
			recorded, err := os.ReadFile(marker)
			if err != nil || len(recorded) != want {
				t.Fatalf("actual bound Go commands=%d error=%v, want%d/nil", len(recorded), err, want)
			}
		})
	}
}

// Both arms time the full root plus two physical dependency demands and assert
// the same genuine private facts. Only module construction and Open are setup.
// Parent Go allocations omit child process heaps; warm caches/order are not a
// matched cold-start comparison. This is not a Witness or Work speed proof.
func BenchmarkDependencyAnalysisScopeRepeatedImports(b *testing.B) {
	capability, request := dependencyScopeMeasurementFixture(b, b.TempDir())
	b.Run("separate", func(b *testing.B) {
		ctx := b.Context()
		count, facts := 0, false
		var firstErr error
		b.ReportAllocs()
		for b.Loop() {
			count, facts, firstErr = dependencyScopeMeasurementSeparate(ctx, capability, request)
			if firstErr != nil || count != 2 || !facts {
				break
			}
		}
		b.StopTimer()
		if firstErr != nil || count != 2 || !facts {
			b.Fatalf("separate count/facts/error=%d/%t/%v, want2/true/nil", count, facts, firstErr)
		}
	})
	b.Run("borrowed", func(b *testing.B) {
		ctx := b.Context()
		count, facts := 0, false
		var firstErr error
		b.ReportAllocs()
		for b.Loop() {
			count, facts, firstErr = dependencyScopeMeasurementBorrowed(ctx, capability, request)
			if firstErr != nil || count != 2 || !facts {
				break
			}
		}
		b.StopTimer()
		if firstErr != nil || count != 2 || !facts {
			b.Fatalf("borrowed count/facts/error=%d/%t/%v, want2/true/nil", count, facts, firstErr)
		}
	})
}
