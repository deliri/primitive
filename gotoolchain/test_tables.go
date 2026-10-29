package gotoolchain

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/token"
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
	file, err := ParseFile(ctx, FileParseRequest{FileSet: token.NewFileSet(), Path: path})
	if err != nil {
		return TestTables{}, err
	}
	return readTestTables(file, c.configuration.Limits.PackageMaximum)
}

// readTestTables reads only the four generated tables and main's TestMain call.
func readTestTables(file *ast.File, maximum uint64) (TestTables, error) {
	var tables TestTables
	var seen [4]bool
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
			ast.Inspect(declaration.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "TestMain" {
					tables.CustomTestMain = true
				}
				return true
			})
		}
	}
	if seen != [4]bool{true, true, true, true} {
		return TestTables{}, outputError("generated test main lacks a table", nil)
	}
	return tables, nil
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
