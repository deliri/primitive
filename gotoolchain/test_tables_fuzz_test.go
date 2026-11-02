package gotoolchain

import (
	"errors"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// testTablesFuzzMaximum bounds a parsed table, as Limits.PackageMaximum does.
const testTablesFuzzMaximum = 4

// testTablesFuzzSource is cmd/go's generated test main with one test name.
func testTablesFuzzSource(name string) string {
	return "package main\n\nimport (\"os\"; \"testing\"; \"testing/internal/testdeps\")\n\nvar tests = []testing.InternalTest{\n\t{" + strconv.Quote(name) + ", nil},\n}\n\nvar benchmarks = []testing.InternalBenchmark{}\n\nvar fuzzTargets = []testing.InternalFuzzTarget{}\n\nvar examples = []testing.InternalExample{}\n\nfunc main() {\n\tm := testing.MainStart(testdeps.TestDeps{}, tests, benchmarks, fuzzTargets, examples)\n\tos.Exit(m.Run())\n}\n"
}

// A generated test main's one name either reads back exactly when it is a Go
// identifier or is refused; arbitrary parsed source never yields a name that
// is not an identifier, more than the maximum, or tables after an error.
func FuzzReadTestTablesBoundedNames(f *testing.F) {
	f.Add("TestPortable", []byte(testTablesFuzzSource("TestPortable")))
	f.Add("Test Spaced", []byte(testTablesFuzzSource("Test Spaced")))
	f.Add("", []byte("package main\nvar tests = 7\n"))
	f.Fuzz(func(t *testing.T, name string, source []byte) {
		file, err := parser.ParseFile(token.NewFileSet(), "_testmain.go", testTablesFuzzSource(name), parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("generated template failed to parse for %q: %v", name, err)
		}
		tables, err := readTestTables(file, testTablesFuzzMaximum)
		if token.IsIdentifier(name) {
			if err != nil || len(tables.Tests) != 1 || tables.Tests[0].String() != name || len(tables.Benchmarks)+len(tables.FuzzTargets)+len(tables.Examples) != 0 || tables.CustomTestMain {
				t.Fatalf("identifier %q read %+v/%v, want exactly that test", name, tables, err)
			}
		} else if err == nil {
			t.Fatalf("non-identifier %q read %+v, want refusal", name, tables)
		}
		arbitrary, err := parser.ParseFile(token.NewFileSet(), "_testmain.go", source, parser.SkipObjectResolution)
		if err != nil {
			return
		}
		tables, err = readTestTables(arbitrary, testTablesFuzzMaximum)
		if err != nil {
			if len(tables.Tests)+len(tables.Benchmarks)+len(tables.FuzzTargets)+len(tables.Examples) != 0 || tables.CustomTestMain {
				t.Fatalf("refused source returned tables %+v", tables)
			}
			return
		}
		for _, table := range [][]TestName{tables.Tests, tables.Benchmarks, tables.FuzzTargets, tables.Examples} {
			if len(table) > testTablesFuzzMaximum {
				t.Fatalf("table of %d names exceeds the maximum %d", len(table), testTablesFuzzMaximum)
			}
			for _, entry := range table {
				if !token.IsIdentifier(entry.String()) {
					t.Fatalf("accepted non-identifier name %q", entry.String())
				}
			}
		}
	})
}

// Only cmd/go's exact generated shape is readable: slice tables of the named
// testing element types and one testing.MainStart in main.
func TestReadTestTablesRefusesUnexpectedGeneratedShape(t *testing.T) {
	t.Parallel()
	valid := testTablesFuzzSource("TestPortable")
	for _, tc := range []struct {
		name, source string
		wantErr      bool
	}{
		{name: "generated shape is read", source: valid},
		{name: "main without testing.MainStart", source: strings.Replace(valid, "testing.MainStart", "testing.Other", 1), wantErr: true},
		{name: "main starting testing twice", source: strings.Replace(valid, "os.Exit(m.Run())", "_ = testing.MainStart(testdeps.TestDeps{}, tests, benchmarks, fuzzTargets, examples)", 1), wantErr: true},
		{name: "tests table of another element type", source: strings.Replace(valid, "[]testing.InternalTest{", "[]testing.InternalBenchmark{", 1), wantErr: true},
		{name: "fixed-size array table", source: strings.Replace(valid, "[]testing.InternalExample{", "[0]testing.InternalExample{", 1), wantErr: true},
		{name: "table from another package", source: strings.Replace(valid, "[]testing.InternalFuzzTarget{", "[]other.InternalFuzzTarget{", 1), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "_testmain.go", tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("fixture source does not parse: %v", err)
			}
			tables, err := readTestTables(file, testTablesFuzzMaximum)
			if tc.wantErr != (err != nil) || tc.wantErr && (len(tables.Tests) != 0 || tables.CustomTestMain) || !tc.wantErr && (len(tables.Tests) != 1 || tables.Tests[0].String() != "TestPortable") {
				t.Fatalf("readTestTables = %+v/%v, want error %t", tables, err, tc.wantErr)
			}
		})
	}
}

// The tables returned by the parser must be the tables passed directly to
// testing.MainStart, never unused declarations beside a different selection.
func TestReadTestTablesBindsStartedTablesLayerTriad(t *testing.T) {
	t.Parallel()
	valid := testTablesFuzzSource("TestPortable")
	for _, tc := range []struct {
		name, source        string
		wantTests           int
		wantCustom, wantErr bool
	}{
		{name: "positive/direct generated main", source: valid, wantTests: 1},
		{name: "positive/direct custom TestMain", source: strings.Replace(valid, "os.Exit(m.Run())", "_test.TestMain(m)\n os.Exit(0)", 1), wantTests: 1, wantCustom: true},
		{name: "neutral/direct empty tables", source: strings.Replace(valid, "{\"TestPortable\", nil},", "", 1)},
		{name: "negative/nonempty replacement for empty tests", source: strings.Replace(strings.Replace(valid, "{\"TestPortable\", nil},", "", 1), "TestDeps{}, tests,", "TestDeps{}, []testing.InternalTest{{\"TestHidden\", nil}},", 1), wantErr: true},
		{name: "negative/reordered tables", source: strings.Replace(valid, "tests, benchmarks,", "benchmarks, tests,", 1), wantErr: true},
		{name: "negative/missing table argument", source: strings.Replace(valid, ", fuzzTargets, examples)", ", fuzzTargets)", 1), wantErr: true},
		{name: "negative/additional table argument", source: strings.Replace(valid, ", fuzzTargets, examples)", ", fuzzTargets, examples, tests)", 1), wantErr: true},
		{name: "negative/unused nested start", source: strings.Replace(valid, "m := testing.MainStart(testdeps.TestDeps{}, tests, benchmarks, fuzzTargets, examples)", "m := func() { testing.MainStart(testdeps.TestDeps{}, tests, benchmarks, fuzzTargets, examples) }", 1), wantErr: true},
		{name: "negative/main never runs testing", source: strings.Replace(valid, "os.Exit(m.Run())", "_ = m", 1), wantErr: true},
		{name: "negative/foreign main package", source: strings.Replace(valid, "package main", "package foreign", 1), wantErr: true},
		{name: "negative/foreign testing import", source: strings.Replace(valid, "\"testing\";", "testing \"example.invalid/testing\";", 1), wantErr: true},
		{name: "negative/noncanonical testing dependencies", source: strings.Replace(valid, "testdeps.TestDeps{}", "nil", 1), wantErr: true},
		{name: "negative/custom main receives another runner", source: strings.Replace(valid, "os.Exit(m.Run())", "_test.TestMain(other)\n os.Exit(0)", 1), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "_testmain.go", tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("fixture parse = %v, want nil", err)
			}
			got, err := readTestTables(file, testTablesFuzzMaximum)
			if tc.wantErr {
				if !errors.Is(err, core.ErrGoToolchainOutput) || len(got.Tests)+len(got.Benchmarks)+len(got.FuzzTargets)+len(got.Examples) != 0 || got.CustomTestMain {
					t.Fatalf("readTestTables = %+v/%v, want typed refusal and zero tables", got, err)
				}
				return
			}
			if err != nil || len(got.Tests) != tc.wantTests || len(got.Benchmarks)+len(got.FuzzTargets)+len(got.Examples) != 0 || got.CustomTestMain != tc.wantCustom {
				t.Fatalf("readTestTables = %+v/%v, want %d tests and custom %t", got, err, tc.wantTests, tc.wantCustom)
			}
		})
	}
}
