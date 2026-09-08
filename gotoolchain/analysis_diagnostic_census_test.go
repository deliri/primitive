package gotoolchain

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"golang.org/x/tools/go/packages"
)

// The independent producer is Go's checker. Check every diagnostic, including
// its identity, order, position and multiplicity, rather than accepting the
// first errors.As match as evidence that the entire failure survived.
func TestAnalysisDiagnosticCensusLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		count int
	}{
		{name: "healthy checker creates no refusal", count: 0},
		{name: "first checker error cannot become healthy", count: 1},
		{name: "later checker error cannot hide behind first", count: 2},
		{name: "large failure census retains its final diagnostic", count: 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			originals := goCheckerDiagnosticFixture(t, tc.count)
			var got analysisDiagnostics
			got.add(nil)
			for _, original := range originals {
				got.add(original)
				got.add(nil)
			}
			for phase := range 2 {
				failure := got.failure()
				if (failure == nil) != (len(originals) == 0) {
					t.Fatalf("failure=%v, want presence %t", failure, len(originals) != 0)
				}
				if len(got.records) != len(originals) || !slices.Equal(got.typeErrors, originals) {
					t.Fatalf("records=%+v typed=%+v, want exact Go errors %+v", got.records, got.typeErrors, originals)
				}
				messages := make([]string, 0, len(originals))
				for i, original := range originals {
					if !errors.Is(failure, original) {
						t.Fatalf("failure=%v, want original typed diagnostic %d: %+v", failure, i, original)
					}
					want := packages.Error{Pos: original.Fset.Position(original.Pos).String(), Msg: original.Msg, Kind: packages.TypeError}
					if got.records[i] != want {
						t.Fatalf("record[%d]=%+v, want %+v", i, got.records[i], want)
					}
					messages = append(messages, original.Error())
				}
				gotDisplay := ""
				if failure != nil {
					gotDisplay = failure.Error()
				}
				if gotDisplay != strings.Join(messages, "\n") {
					t.Fatalf("display=%q, want exact ordered Go diagnostics %q", failure.Error(), messages)
				}
				if phase == 0 && tc.count != 0 {
					// Diagnostic multiplicity is preserved, while a previously returned
					// error remains immutable when the collector receives another fact.
					before := failure.Error()
					got.add(originals[0])
					originals = append(slices.Clone(originals), originals[0])
					after := failure.Error()
					if after != before {
						t.Fatalf("prior failure=%q, want preserved %q", failure.Error(), before)
					}
				}
			}
		})
	}
}

func goCheckerDiagnosticFixture(t testing.TB, count int) []types.Error {
	t.Helper()
	var source strings.Builder
	source.WriteString("package subject\n")
	for i := range count {
		fmt.Fprintf(&source, "var _ = missing%d\n", i)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "subject.go", source.String(), parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var originals []types.Error
	configuration := types.Config{Error: func(err error) {
		var original types.Error
		if !errors.As(err, &original) {
			t.Fatalf("checker returned unexpected typed error: %T", err)
		}
		originals = append(originals, original)
	}}
	_, checkErr := configuration.Check("example.test/diagnostics", fset, []*ast.File{file}, nil)
	if len(originals) != count || (checkErr == nil) != (count == 0) {
		t.Fatalf("independent checker census=%d/%v, want %d", len(originals), checkErr, count)
	}
	return originals
}

// A successful file close must not turn a scanner.ErrorList into an unknown
// diagnostic. Compare the filesystem-backed parser with Go parsing the exact
// original bytes, and retain every scanner position after classification.
func TestAnalysisParserDiagnosticOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, source string }{
		{name: "healthy parse produces no diagnostic", source: "package subject\nconst Value = 1\n"},
		{name: "incomplete declaration retains parser refusal", source: "package subject\nvar =\n"},
		{name: "parser recovery retains every separate error", source: "package subject\nfunc Broken( {\nvar =\n"},
		{name: "lexical string failure retains scanner position", source: "package subject\nvar Text = \"unterminated\nvar =\n"},
		{name: "unterminated comment cannot become healthy EOF", source: "package subject\n/* unterminated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, err := core.ParseAbsolutePath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			held, err := filestore.OpenRoot(t.Context(), root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := held.Close(); err != nil {
					t.Error(err)
				}
			})
			writeVendorCompilerFixture(t, held, "subject/value.go", tc.source)
			path := filepath.Join(root.String(), "subject/value.go")
			_, oracleErr := parser.ParseFile(token.NewFileSet(), path, tc.source, parser.ParseComments|parser.SkipObjectResolution|parser.AllErrors)
			compiler := analysisUnitCompiler{ctx: t.Context(), fset: token.NewFileSet()}
			_, gotErr := compiler.parseFile(path)
			if (gotErr == nil) != (oracleErr == nil) {
				t.Fatalf("parser outcome=%v, oracle=%v", gotErr, oracleErr)
			}
			if oracleErr == nil {
				return
			}
			var originals, retained scanner.ErrorList
			if !errors.As(oracleErr, &originals) || !errors.As(gotErr, &retained) || !slices.EqualFunc(originals, retained, func(want, got *scanner.Error) bool { return *got == *want }) {
				t.Fatalf("original scanner errors changed: %v/%v", gotErr, oracleErr)
			}
			var diagnostics analysisDiagnostics
			diagnostics.add(gotErr)
			if len(diagnostics.records) != len(originals) {
				t.Fatalf("scanner census=%d, want %d", len(diagnostics.records), len(originals))
			}
			for i, original := range originals {
				want := packages.Error{Pos: original.Pos.String(), Msg: original.Msg, Kind: packages.ParseError}
				if diagnostics.records[i] != want {
					t.Fatalf("scanner diagnostic %d=%+v, want %+v", i, diagnostics.records[i], want)
				}
			}
		})
	}
}

func BenchmarkAnalysisDiagnosticRendering(b *testing.B) {
	b.ReportAllocs()
	for _, count := range []int{1, 64, 1024} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			originals := goCheckerDiagnosticFixture(b, count)
			b.ReportAllocs()
			for b.Loop() {
				var diagnostics analysisDiagnostics
				for _, original := range originals {
					diagnostics.add(original)
				}
				if text := diagnostics.failure().Error(); len(text) == 0 {
					b.Fatalf("diagnostic output bytes=%d, want nonempty Go display payload", len(text))
				}
			}
		})
	}
}
