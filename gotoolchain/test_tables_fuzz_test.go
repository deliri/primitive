package gotoolchain

import (
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// testTablesFuzzMaximum bounds a parsed table, as Limits.PackageMaximum does.
const testTablesFuzzMaximum = 4

// testTablesFuzzSource is cmd/go's generated test main with one test name.
func testTablesFuzzSource(name string) string {
	return "package main\n\nimport \"testing\"\n\nvar tests = []testing.InternalTest{\n\t{" + strconv.Quote(name) + ", nil},\n}\n\nvar benchmarks = []testing.InternalBenchmark{}\n\nvar fuzzTargets = []testing.InternalFuzzTarget{}\n\nvar examples = []testing.InternalExample{}\n\nfunc main() {}\n"
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
