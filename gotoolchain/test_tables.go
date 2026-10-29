package gotoolchain

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// TestTableRequest selects the test main cmd/go generates for one exact
// package under this capability's environment, which carries the target
// GOOS, GOARCH and CGO_ENABLED. Race adds the race build tag exactly as go test
// -race does. Nothing is compiled and no test, TestMain or init runs.
type TestTableRequest struct {
	WorkingDirectory core.AbsolutePath
	Package          gomodule.ImportPath
	Race             bool
}

func (r TestTableRequest) Validate() error {
	if err := errors.Join(r.WorkingDirectory.Validate(), r.Package.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	return nil
}

// TestName is one name from a generated test main table, exactly as cmd/go
// selected and wrote it.
type TestName struct{ value string }

func (n TestName) String() string { return n.value }

// TestTables is cmd/go's complete generated selection for one package and
// build context: the top-level tests, benchmarks, fuzz targets and runnable
// examples go test would hand testing.MainStart, and whether a package
// TestMain runs instead of m.Run.
type TestTables struct {
	Tests          []TestName
	Benchmarks     []TestName
	FuzzTargets    []TestName
	Examples       []TestName
	CustomTestMain bool
}

// testTableTemplate prints each listed package's import path and its first Go
// file; for the synthetic <package>.test entry that is the generated test main.
const testTableTemplate = `-f={{.ImportPath}}{{"\t"}}{{if .GoFiles}}{{index .GoFiles 0}}{{end}}`

// ObserveTestTables reads the generated test main of one package. A cmd/go
// failure, a missing or unexpected generated file, or a table beyond the
// capability's package maximum is an error, never an empty selection.
func (c Capability) ObserveTestTables(ctx context.Context, request TestTableRequest) (TestTables, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return TestTables{}, err
	}
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return TestTables{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	values := []string{goListSubcommand, "-test"}
	if request.Race {
		values = append(values, "-race")
	}
	values = append(values, testTableTemplate, "--", request.Package.String())
	output, _, err := c.execute(ctx, request.WorkingDirectory, values...)
	if err != nil {
		return TestTables{}, err
	}
	listed, main := request.Package.String()+"\t", request.Package.String()+".test\t"
	var generated string
	var packages int
	for line := range strings.SplitSeq(string(bytes.TrimRight(output, "\n")), "\n") {
		if strings.HasPrefix(line, listed) {
			packages++
		}
		path, found := strings.CutPrefix(line, main)
		if !found {
			continue
		}
		if generated != "" || path == "" {
			return TestTables{}, outputError("generated test main is ambiguous", nil)
		}
		generated = path
	}
	if packages != 1 {
		return TestTables{}, outputError("selected package is not listed exactly once", nil)
	}
	// cmd/go generates a test main only for a package with test files in this
	// build context; without one the complete selection is empty.
	if generated == "" {
		return TestTables{}, nil
	}
	path, err := core.ParseAbsolutePath(generated)
	if err != nil {
		return TestTables{}, outputError("generated test main path is invalid", err)
	}
	limit, err := c.configuration.Limits.OutputBytes.Int64()
	if err != nil {
		return TestTables{}, errors.Join(core.ErrGoToolchainContract, err)
	}
	// One owned regular handle, read no further than the output bound before
	// any parse allocation.
	source, err := openAnalysisInput(ctx, path.String())
	if err != nil {
		return TestTables{}, errors.Join(core.ErrGoToolchainExecution, err)
	}
	data, readErr := io.ReadAll(io.LimitReader(source, limit+1))
	if err := errors.Join(readErr, source.Close(), contextstate.Validate(ctx)); err != nil {
		return TestTables{}, errors.Join(core.ErrGoToolchainOutput, err)
	}
	if int64(len(data)) > limit {
		return TestTables{}, outputError("generated test main exceeds the output bound", nil)
	}
	file, err := parser.ParseFile(token.NewFileSet(), path.String(), data, parser.SkipObjectResolution)
	if err != nil {
		return TestTables{}, outputError("generated test main does not parse", err)
	}
	return readTestTables(file, c.configuration.Limits.PackageMaximum)
}

// testTableTypes are the element types cmd/go declares for each table, in
// readTestTables' index order.
var testTableTypes = [4]string{"InternalTest", "InternalBenchmark", "InternalFuzzTarget", "InternalExample"}

// testingSelector reports a testing.<name> reference.
func testingSelector(expression ast.Expr, name string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "testing"
}

// readTestTables reads only the four generated tables and main's TestMain call.
func readTestTables(file *ast.File, maximum uint64) (TestTables, error) {
	if file.Name == nil || file.Name.Name != "main" || !testMainImport(file, "testing", "testing") || !testMainImport(file, "testdeps", "testing/internal/testdeps") || !testMainImport(file, "os", "os") {
		return TestTables{}, outputError("generated test main has unexpected package imports", nil)
	}
	var tables TestTables
	var seen [4]bool
	var mains int
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.GenDecl:
			if declaration.Tok != token.VAR {
				continue
			}
			for _, spec := range declaration.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
					continue
				}
				var table *[]TestName
				var index int
				switch value.Names[0].Name {
				case "tests":
					table, index = &tables.Tests, 0
				case "benchmarks":
					table, index = &tables.Benchmarks, 1
				case "fuzzTargets":
					table, index = &tables.FuzzTargets, 2
				case "examples":
					table, index = &tables.Examples, 3
				default:
					continue
				}
				if seen[index] {
					return TestTables{}, outputError("generated test main repeats a table", nil)
				}
				seen[index] = true
				if !testTableLiteral(value.Values[0], testTableTypes[index]) {
					return TestTables{}, outputError("generated test table has an unexpected type", nil)
				}
				names, err := readTestTable(value.Values[0], maximum)
				if err != nil {
					return TestTables{}, err
				}
				*table = names
			}
		case *ast.FuncDecl:
			if declaration.Name.Name != "main" || declaration.Recv != nil || declaration.Body == nil {
				continue
			}
			mains++
			custom, err := readTestMain(declaration)
			if err != nil {
				return TestTables{}, err
			}
			tables.CustomTestMain = custom
		}
	}
	if seen != [4]bool{true, true, true, true} {
		return TestTables{}, outputError("generated test main lacks a table", nil)
	}
	if mains != 1 {
		return TestTables{}, outputError("generated test main does not start testing exactly once", nil)
	}
	return tables, nil
}

// testMainImport binds the names in cmd/go's template to the real imports.
func testMainImport(file *ast.File, name, path string) bool {
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err == nil && value == path && (spec.Name == nil || spec.Name.Name == name) {
			return true
		}
	}
	return false
}

func testMainIdent(expression ast.Expr, name string) bool {
	ident, ok := expression.(*ast.Ident)
	return ok && ident.Name == name
}

// readTestMain binds the four observed declarations to the direct MainStart
// assignment in cmd/go's template. Merely finding a call below main would
// accept an unused closure or a different, nonempty selection as empty.
func readTestMain(declaration *ast.FuncDecl) (bool, error) {
	refused := outputError("generated test main does not use the observed tables", nil)
	if declaration.Type.Params.NumFields() != 0 || declaration.Type.Results.NumFields() != 0 || (len(declaration.Body.List) != 2 && len(declaration.Body.List) != 3) {
		return false, refused
	}
	assignment, ok := declaration.Body.List[0].(*ast.AssignStmt)
	if !ok || assignment.Tok != token.DEFINE || len(assignment.Lhs) != 1 || len(assignment.Rhs) != 1 || !testMainIdent(assignment.Lhs[0], "m") {
		return false, refused
	}
	start, ok := assignment.Rhs[0].(*ast.CallExpr)
	if !ok || !testingSelector(start.Fun, "MainStart") || len(start.Args) != 5 || start.Ellipsis.IsValid() {
		return false, refused
	}
	deps, ok := start.Args[0].(*ast.CompositeLit)
	if !ok || len(deps.Elts) != 0 {
		return false, refused
	}
	selector, ok := deps.Type.(*ast.SelectorExpr)
	if !ok || !testMainIdent(selector.X, "testdeps") || selector.Sel.Name != "TestDeps" {
		return false, refused
	}
	for i, name := range [4]string{"tests", "benchmarks", "fuzzTargets", "examples"} {
		if !testMainIdent(start.Args[i+1], name) {
			return false, refused
		}
	}
	tail, ok := declaration.Body.List[len(declaration.Body.List)-1].(*ast.ExprStmt)
	if !ok {
		return false, refused
	}
	exit, ok := tail.X.(*ast.CallExpr)
	if !ok || len(exit.Args) != 1 || exit.Ellipsis.IsValid() {
		return false, refused
	}
	selector, ok = exit.Fun.(*ast.SelectorExpr)
	if !ok || !testMainIdent(selector.X, "os") || selector.Sel.Name != "Exit" {
		return false, refused
	}
	if len(declaration.Body.List) == 2 {
		run, ok := exit.Args[0].(*ast.CallExpr)
		if !ok || len(run.Args) != 0 {
			return false, refused
		}
		selector, ok := run.Fun.(*ast.SelectorExpr)
		if !ok || !testMainIdent(selector.X, "m") || selector.Sel.Name != "Run" {
			return false, refused
		}
		return false, nil
	}
	custom, ok := declaration.Body.List[1].(*ast.ExprStmt)
	if !ok {
		return false, refused
	}
	call, ok := custom.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || !testMainIdent(call.Args[0], "m") || call.Ellipsis.IsValid() {
		return false, refused
	}
	selector, ok = call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "TestMain" || (!testMainIdent(selector.X, "_test") && !testMainIdent(selector.X, "_xtest")) {
		return false, refused
	}
	// A direct package TestMain is always reported, irrespective of its exit
	// expression. Its arbitrary user behavior prevents certifying absence.
	return true, nil
}

// testTableLiteral reports a []testing.<element>{...} composite literal.
func testTableLiteral(expression ast.Expr, element string) bool {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return false
	}
	slice, ok := literal.Type.(*ast.ArrayType)
	return ok && slice.Len == nil && testingSelector(slice.Elt, element)
}

// readTestTable reads each element's first field, the quoted name.
func readTestTable(expression ast.Expr, maximum uint64) ([]TestName, error) {
	literal, ok := expression.(*ast.CompositeLit)
	if !ok {
		return nil, outputError("generated test table is not a literal", nil)
	}
	if uint64(len(literal.Elts)) > maximum {
		return nil, outputError("generated test table exceeds the package maximum", nil)
	}
	names := make([]TestName, 0, len(literal.Elts))
	for _, element := range literal.Elts {
		entry, ok := element.(*ast.CompositeLit)
		if !ok || len(entry.Elts) == 0 {
			return nil, outputError("generated test table entry is malformed", nil)
		}
		quoted, ok := entry.Elts[0].(*ast.BasicLit)
		if !ok || quoted.Kind != token.STRING {
			return nil, outputError("generated test table name is not a string", nil)
		}
		name, err := strconv.Unquote(quoted.Value)
		if err != nil || !token.IsIdentifier(name) {
			return nil, outputError("generated test table name is invalid", err)
		}
		names = append(names, TestName{value: name})
	}
	return names, nil
}
