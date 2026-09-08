package gotoolchain_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

// Independent exact-package loads own the oracle. Each mutation pressures one
// compiler unit or dependency, while unchanged sibling facts must be identical.
func TestAnalysisBatchPreservesExactPackageResults(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, file, source string
		tests              bool
		testOnly           bool
		transitiveTest     bool
		testExport         bool
		wantFailure        bool
	}{
		{name: "ordinary packages with shared exports"},
		{name: "internal test variant", tests: true, file: "a/value_test.go", source: "package a\nconst TestValue = 2\n"},
		{name: "external test imports tested package", tests: true, file: "a/value_test.go", source: "package a_test\nimport \"example.com/batch/a\"\nvar TestValue = a.Value\n"},
		{name: "external test selects declarations present only in test export", tests: true, testExport: true, file: "a/value_test.go", source: "package a_test\nimport \"example.com/batch/a\"\nvar TestValue a.TestOnly\n"},
		{name: "foreign transitive test variant cannot join sibling", tests: true, transitiveTest: true, file: "a/value_test.go", source: "package a_test\nimport \"example.com/batch/b\"\nvar TestValue = b.Value\n"},
		{name: "failed internal test retains production and sibling", tests: true, wantFailure: true, file: "a/value_test.go", source: "package a\nconst TestValue = missing\n"},
		{name: "failed external test retains production and sibling", tests: true, wantFailure: true, file: "a/value_test.go", source: "package a_test\nconst TestValue = missing\n"},
		{name: "syntax failure retains independent sibling", tests: true, wantFailure: true, file: "a/value.go", source: "package a\nfunc Broken(\n"},
		{name: "missing dependency retains independent sibling", tests: true, wantFailure: true, file: "a/value.go", source: "package a\nimport _ \"example.invalid/absent\"\n"},
		{name: "ordinary test file is ignored when tests excluded", file: "a/value_test.go", source: "package a\nconst TestValue = missing\n"},
		{name: "excluded file cannot enter batch", tests: true, file: "a/excluded.go", source: "//go:build never_selected\n\npackage a\nconst Excluded = missing\n"},
		{name: "test-only package retains empty production identity", tests: true, testOnly: true, file: "a/value.go", source: "//go:build never_selected\n\npackage a\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			capability, request := analysisBatchFixture(t, t.TempDir())
			request.IncludeTests = tc.tests
			if tc.file != "" {
				if err := os.WriteFile(filepath.Join(request.WorkingDirectory.String(), tc.file), []byte(tc.source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			// A real internal test also exercises cmd/go's synthetic test-main output.
			if tc.testOnly {
				if err := os.WriteFile(filepath.Join(request.WorkingDirectory.String(), "a/value_test.go"), []byte("package a\nconst TestValue = 1\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.transitiveTest {
				for _, file := range []struct{ path, source string }{
					{"a/internal_test.go", "package a\nconst Extra = 3\n"},
					{"b/value.go", "package b\nimport \"example.com/batch/a\"\nvar Value = a.Value\n"},
				} {
					if err := os.WriteFile(filepath.Join(request.WorkingDirectory.String(), file.path), []byte(file.source), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if tc.testExport {
				if err := os.WriteFile(filepath.Join(request.WorkingDirectory.String(), "a/internal_test.go"), []byte("package a\ntype TestOnly struct{ Value string }\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			expected := make([]gotoolchain.PackageAnalysis, len(request.Packages))
			failures := make([]error, len(request.Packages))
			for i, pkg := range request.Packages {
				expected[i], failures[i] = capability.AnalyzePackage(t.Context(), gotoolchain.AnalysisRequest{WorkingDirectory: request.WorkingDirectory, Package: pkg, IncludeTests: tc.tests})
				var want error
				if i == 0 && tc.wantFailure {
					want = core.ErrGoToolchainOutput
				}
				if !errors.Is(failures[i], want) {
					t.Fatalf("fixture compiler outcome for %s = %v, want %v", pkg, failures[i], want)
				}
			}
			var mu sync.Mutex
			seen := make([]bool, len(expected))
			err := capability.AnalyzePackages(t.Context(), request, func(_ context.Context, got gotoolchain.AnalysisResult) error {
				if err := got.Validate(); err != nil {
					return err
				}
				i := slices.Index(request.Packages, got.Request.Package)
				if i < 0 {
					t.Errorf("foreign result package: %s", got.Request.Package)
					return core.ErrGoToolchainContract
				}
				mu.Lock()
				if seen[i] {
					t.Errorf("duplicate result for %s", got.Request.Package)
				}
				seen[i] = true
				mu.Unlock()
				want := expected[i]
				if (got.Err == nil) != (failures[i] == nil) || failures[i] != nil && !errors.Is(got.Err, core.ErrGoToolchainOutput) {
					t.Errorf("batch error = %v, exact load = %v", got.Err, failures[i])
				}
				if got.Analysis.Incomplete != want.Incomplete || len(got.Analysis.Units) != len(want.Units) || len(got.Analysis.Metadata) != len(want.Metadata) {
					t.Errorf("batch unit census = %d/%d/%t, want %d/%d/%t", len(got.Analysis.Units), len(got.Analysis.Metadata), got.Analysis.Incomplete, len(want.Units), len(want.Metadata), want.Incomplete)
					return nil
				}
				for j, unit := range got.Analysis.Units {
					previous := want.Units[j]
					if unit.ID != previous.ID || unit.PkgPath != previous.PkgPath || unit.ForTest != previous.ForTest || !slices.Equal(unit.CompiledGoFiles, previous.CompiledGoFiles) || !slices.Equal(unit.Types.Scope().Names(), previous.Types.Scope().Names()) {
						t.Errorf("batch unit %s differs from independent %s", unit.ID, previous.ID)
					}
					for _, name := range previous.Types.Scope().Names() {
						actual, expected := unit.Types.Scope().Lookup(name), previous.Types.Scope().Lookup(name)
						if actual.String() != expected.String() {
							t.Errorf("compiler object %s = %s, want %s", name, actual, expected)
						}
					}
				}
				for j, unit := range got.Analysis.Metadata {
					previous := want.Metadata[j]
					if unit.ID != previous.ID || len(unit.Imports) != len(previous.Imports) {
						t.Errorf("metadata ownership changed: %s vs %s", unit.ID, previous.ID)
						continue
					}
					for name, imported := range previous.Imports {
						actual := unit.Imports[name]
						if actual == nil || actual.ID != imported.ID || actual.Dir != imported.Dir || actual.ExportFile != imported.ExportFile {
							t.Errorf("compiler import selection changed for %s -> %s", unit.ID, name)
						}
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if slices.Contains(seen, false) {
				t.Fatalf("batch omitted requested result: %v", seen)
			}
		})
	}
}

func analysisBatchFixture(t testing.TB, directory string) (gotoolchain.Capability, gotoolchain.AnalysisBatchRequest) {
	t.Helper()
	for _, file := range []struct{ path, body string }{
		{"go.mod", "module example.com/batch\n\ngo 1.27.1\n"},
		{"a/value.go", "package a\nimport \"strings\"\nvar Value = strings.Contains(\"value\",\"a\")\n"},
		{"b/value.go", "package b\nimport \"strings\"\nvar Value = strings.Contains(\"other\",\"b\")\n"},
	} {
		name := filepath.Join(directory, file.path)
		if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(file.body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	request := gotoolchain.AnalysisBatchRequest{WorkingDirectory: root}
	for _, path := range []string{"example.com/batch/a", "example.com/batch/b"} {
		pkg, err := gomodule.ParseImportPath(path)
		if err != nil {
			t.Fatal(err)
		}
		request.Packages = append(request.Packages, pkg)
	}
	limits, err := gotoolchain.DefaultLimits()
	if err != nil {
		t.Fatal(err)
	}
	capability, err := gotoolchain.Open(context.Background(), gotoolchain.Configuration{Workspace: gotoolchain.WorkspaceModeDisabled, Limits: limits})
	if err != nil {
		t.Fatal(err)
	}
	return capability, request
}

func TestAnalysisBatchBoundaryRefusesBeforeDeliveringFacts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                              string
		mutate                            func(*gotoolchain.AnalysisBatchRequest)
		nilContext, canceled, nilConsumer bool
		want                              error
	}{
		{name: "empty batch cannot invent completion", mutate: func(r *gotoolchain.AnalysisBatchRequest) { r.Packages = nil }, want: core.ErrGoToolchainContract},
		{name: "duplicate subject cannot double deliver", mutate: func(r *gotoolchain.AnalysisBatchRequest) { r.Packages[1] = r.Packages[0] }, want: core.ErrGoToolchainContract},
		{name: "reversed subjects cannot change canonical contract", mutate: func(r *gotoolchain.AnalysisBatchRequest) { slices.Reverse(r.Packages) }, want: core.ErrGoToolchainContract},
		{name: "invalid subject cannot execute", mutate: func(r *gotoolchain.AnalysisBatchRequest) { r.Packages[0] = gomodule.ImportPath{} }, want: core.ErrGoToolchainContract},
		{name: "invalid directory cannot execute", mutate: func(r *gotoolchain.AnalysisBatchRequest) { r.WorkingDirectory = core.AbsolutePath{} }, want: core.ErrGoToolchainContract},
		{name: "nil context cannot acquire process", nilContext: true, want: core.ErrGoToolchainContract},
		{name: "canceled context cannot deliver facts", canceled: true, want: context.Canceled},
		{name: "nil consumer cannot launch unused work", nilConsumer: true, want: core.ErrGoToolchainContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			capability, request := analysisBatchFixture(t, t.TempDir())
			if tc.mutate != nil {
				tc.mutate(&request)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.nilContext {
				ctx = nil
			}
			if tc.canceled {
				cancel()
			}
			consumer := func(_ context.Context, result gotoolchain.AnalysisResult) error {
				t.Errorf("invalid request delivered result %+v, want no callback", result)
				return nil
			}
			if tc.nilConsumer {
				consumer = nil
			}
			if err := capability.AnalyzePackages(ctx, request, consumer); !errors.Is(err, tc.want) {
				t.Fatalf("refused batch = %v, want %v", err, tc.want)
			}
		})
	}
}

// Two callbacks rendezvous before one fails. The other must observe the owned
// context's cancellation, and the call must join it before returning. No sleeps.
func TestAnalysisBatchConsumerFailureCancelsAndJoinsSibling(t *testing.T) {
	t.Parallel()
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("concurrency proof requires GOMAXPROCS >= 2; serial execution is not this proof")
	}
	capability, request := analysisBatchFixture(t, t.TempDir())
	entered := make(chan struct{})
	joined := make(chan struct{})
	var cancelled atomic.Bool
	backstop, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	err := capability.AnalyzePackages(backstop, request, func(ctx context.Context, result gotoolchain.AnalysisResult) error {
		if result.Err != nil {
			return result.Err
		}
		if result.Request.Package == request.Packages[0] {
			select {
			case <-entered:
			case <-backstop.Done():
				return backstop.Err()
			}
			return os.ErrPermission
		}
		close(entered)
		select {
		case <-ctx.Done():
		case <-backstop.Done():
			return backstop.Err()
		}
		cancelled.Store(errors.Is(ctx.Err(), context.Canceled))
		close(joined)
		return nil
	})
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("consumer failure = %v, want original permission identity", err)
	}
	select {
	case <-joined:
	default:
		t.Fatalf("batch callback joined = false, want true; returned error = %v", err)
	}
	if !cancelled.Load() {
		t.Fatalf("sibling cancellation observed = %t, want true", cancelled.Load())
	}
}

func TestAnalysisResultCannotCertifyForeignOrMissingFacts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*gotoolchain.AnalysisResult)
		want   error
	}{
		{name: "unchanged compiler result remains valid"},
		{name: "zero request cannot claim valid units", mutate: func(r *gotoolchain.AnalysisResult) { r.Request = gotoolchain.AnalysisRequest{} }, want: core.ErrGoToolchainContract},
		{name: "foreign package cannot borrow valid units", mutate: func(r *gotoolchain.AnalysisResult) { r.Request.Package = gomodule.ImportPath{} }, want: core.ErrGoToolchainContract},
		{name: "different test policy cannot borrow units", mutate: func(r *gotoolchain.AnalysisResult) { r.Request.IncludeTests = !r.Request.IncludeTests }, want: core.ErrGoToolchainContract},
		{name: "missing units cannot become complete", mutate: func(r *gotoolchain.AnalysisResult) { r.Analysis.Units = nil }, want: core.ErrGoToolchainContract},
		{name: "missing metadata cannot certify membership", mutate: func(r *gotoolchain.AnalysisResult) { r.Analysis.Metadata = nil }, want: core.ErrGoToolchainContract},
		{name: "partial marker requires compiler failure", mutate: func(r *gotoolchain.AnalysisResult) { r.Analysis.Incomplete = true }, want: core.ErrGoToolchainContract},
		{name: "failed empty analysis remains explicit", mutate: func(r *gotoolchain.AnalysisResult) {
			r.Analysis = gotoolchain.PackageAnalysis{}
			r.Err = core.ErrGoToolchainExecution
		}},
		{name: "failed empty analysis cannot contain fabricated test metadata", mutate: func(r *gotoolchain.AnalysisResult) {
			r.Analysis = gotoolchain.PackageAnalysis{IncludeTests: true}
			r.Err = core.ErrGoToolchainExecution
		}, want: core.ErrGoToolchainContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			capability, batch := analysisBatchFixture(t, t.TempDir())
			request := gotoolchain.AnalysisRequest{WorkingDirectory: batch.WorkingDirectory, Package: batch.Packages[0]}
			analysis, err := capability.AnalyzePackage(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			result := gotoolchain.AnalysisResult{Request: request, Analysis: analysis}
			if err := result.Validate(); err != nil {
				t.Fatal(err)
			}
			if tc.mutate != nil {
				tc.mutate(&result)
			}
			if err := result.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("result ownership = %v, want %v", err, tc.want)
			}
		})
	}
}
