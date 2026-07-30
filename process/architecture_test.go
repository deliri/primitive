package process

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// productionStructRoles is the data-flow inventory. Process owns one command
// and fixed stream observations; every internal carrier must remain visible.
func productionStructRoles() map[string]string {
	return map[string]string{
		"Argument":            "validated argv fact",
		"Environment":         "validated environment projection",
		"EnvironmentName":     "validated environment name fact",
		"EnvironmentValue":    "validated environment value fact",
		"EnvironmentVariable": "validated environment pair",
		"ExitCode":            "validated exit observation",
		"Failure":             "typed execution failure",
		"OutputLimitExceeded": "typed output-bound failure",
		"Request":             "validated execution ingress",
		"Result":              "fixed-size execution result",
		"StreamFailure":       "typed stream failure",
		"Streams":             "caller-owned stream capabilities",
		"boundedWriter":       "bounded streaming output projection",
		"commandStreams":      "fixed execution stream carrier",
		"observedReader":      "streaming input observation",
		"preparedCommand":     "os/exec command and cancellation observation",
		"streamFailures":      "fixed-size stream failure carrier",
		"waitRequest":         "typed wait-phase handoff",
	}
}

// productionImportAllowlist is the exact standard-library and Primitive frontier
// Process is permitted. PLAN section 3 gives Process the sibling frontier core,
// contextstate, and temporal; PLAN section 5 makes an extra edge a coupling
// violation. An allowlist fails on any new dependency, where a blocklist only
// fails on the ones already imagined.
func productionImportAllowlist() []string {
	return []string{
		"context",
		"errors",
		"github.com/deliri/primitive/v2026/contextstate",
		"github.com/deliri/primitive/v2026/core",
		"github.com/deliri/primitive/v2026/temporal",
		"io",
		"math",
		"os",
		"os/exec",
		"strings",
		"sync",
		"sync/atomic",
	}
}

// forbiddenPackageSelectors are package-qualified substrate calls that would
// move ownership out of this package: an unsupervised command, path resolution
// outside the typed command, a raw process path, or ambient environment reads
// that bypass the typed Environment contract. These must stay qualified, because
// bare names such as Command also name legitimate typed struct fields.
func forbiddenPackageSelectors() []string {
	return []string{
		"exec.Command",
		"exec.LookPath",
		"os.Clearenv",
		"os.Environ",
		"os.FindProcess",
		"os.Getenv",
		"os.LookupEnv",
		"os.Setenv",
		"os.StartProcess",
	}
}

// forbiddenMethodSelectors are method names that retain whole output, bypass
// caller-owned streams, or reach a raw signal path. No typed field in this
// package shares these names, so matching the bare selector is unambiguous.
func forbiddenMethodSelectors() []string {
	return []string{
		"CombinedOutput",
		"Output",
		"Signal",
		"StderrPipe",
		"StdinPipe",
		"StdoutPipe",
	}
}

// forbiddenSelector reports whether one selector expression names a substrate
// call this package must not make. It is the single matcher used against both
// the live package and the synthetic red-state cases.
func forbiddenSelector(selector *ast.SelectorExpr) bool {
	if slices.Contains(forbiddenMethodSelectors(), selector.Sel.Name) {
		return true
	}
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	return slices.Contains(
		forbiddenPackageSelectors(),
		qualifier.Name+"."+selector.Sel.Name,
	)
}

func TestProductionStructDataFlowInventory(t *testing.T) {
	t.Parallel()

	roles := productionStructRoles()
	for _, name := range productionStructNames(t) {
		role, classified := roles[name]
		if !classified || role == "" {
			t.Errorf(
				"production struct %s has role %q classified %t, want an intentional data-flow role",
				name,
				role,
				classified,
			)
			continue
		}
		delete(roles, name)
	}
	for name, role := range roles {
		t.Errorf(
			"inventory classifies %s as %q, but Process declares no such production struct",
			name,
			role,
		)
	}
}

func TestPublicOperationsAreOnlyTypedConstructionAndExecution(t *testing.T) {
	t.Parallel()

	got := productionFunctionNames(t)
	want := []string{
		"NewArgument",
		"NewEnvironmentName",
		"NewEnvironmentValue",
		"Run",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("exported Process operations = %q, want exactly %q", got, want)
	}
}

// TestProductionImportsMatchTheExactDeclaredFrontier ratchets the dependency
// surface itself. It rejects a new sibling edge, a consumer edge, a raw syscall
// or signal package, a whole-output buffer package, and any unsafe import.
func TestProductionImportsMatchTheExactDeclaredFrontier(t *testing.T) {
	t.Parallel()

	allowed := productionImportAllowlist()
	var got []string
	for _, file := range productionFiles(t) {
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf(
					"import path %s cannot be decoded: %v",
					imported.Path.Value,
					err,
				)
			}
			if !slices.Contains(got, path) {
				got = append(got, path)
			}
			if !slices.Contains(allowed, path) {
				t.Errorf(
					"production import %q is outside the declared frontier %q",
					path,
					allowed,
				)
			}
		}
	}
	slices.Sort(got)
	for _, path := range allowed {
		if !slices.Contains(got, path) {
			t.Errorf(
				"allowlisted import %q is unused, want an allowlist that only names real edges",
				path,
			)
		}
	}
}

func TestProductionStructureForbidsWorldModelsAndWholeOutputPaths(t *testing.T) {
	t.Parallel()

	for _, file := range productionFiles(t) {
		ast.Inspect(file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.GoStmt:
				t.Errorf(
					"production go statement at token position %d, want os/exec-owned execution",
					typed.Go,
				)
			case *ast.MapType:
				t.Errorf(
					"production map at token position %d, want fixed carriers or streaming traversal",
					typed.Map,
				)
			case *ast.SelectorExpr:
				if forbiddenSelector(typed) {
					t.Errorf(
						"production selector %s at token position %d, want streamed caller-owned output",
						typed.Sel.Name,
						typed.Sel.NamePos,
					)
				}
			case *ast.FuncDecl:
				if countParameters(typed.Type.Params) >= 4 {
					t.Errorf(
						"production function %s has %d parameters, want a typed request below four",
						typed.Name.Name,
						countParameters(typed.Type.Params),
					)
				}
			}
			return true
		})
		for _, structure := range productionStructTypes(t, file) {
			rejectRetainedByteSlices(t, structure.name, structure.definition)
		}
	}
}

// TestForbiddenSelectorMatcherHasARedState proves the selector scan can fail.
// A structural ratchet that cannot go red is decoration, so the matcher used
// against the live package is exercised here against synthetic source.
func TestForbiddenSelectorMatcherHasARedState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		source     string
		wantReject bool
	}{
		{
			name:   "the shipped context-bound execution leaf is admitted",
			source: "package process\nfunc run() { exec.CommandContext(ctx, path) }\n",
		},
		{
			name:   "starting and killing the direct child is admitted",
			source: "package process\nfunc run() { command.Start(); command.Process.Kill() }\n",
		},
		{
			name:   "a typed Command field access is admitted",
			source: "package process\nfunc run() { _ = request.Command.String() }\n",
		},
		{
			name:       "an unsupervised command is rejected",
			source:     "package process\nfunc run() { exec.Command(path) }\n",
			wantReject: true,
		},
		{
			name:       "whole-output retention is rejected",
			source:     "package process\nfunc run() { command.CombinedOutput() }\n",
			wantReject: true,
		},
		{
			name:       "a pipe that bypasses caller streams is rejected",
			source:     "package process\nfunc run() { command.StdoutPipe() }\n",
			wantReject: true,
		},
		{
			name:       "an ambient environment read is rejected",
			source:     "package process\nfunc run() { os.Getenv(name) }\n",
			wantReject: true,
		},
		{
			name:       "a raw signal path is rejected",
			source:     "package process\nfunc run() { command.Process.Signal(sig) }\n",
			wantReject: true,
		},
		{
			name:       "path resolution outside the typed command is rejected",
			source:     "package process\nfunc run() { exec.LookPath(name) }\n",
			wantReject: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fileSet := token.NewFileSet()
			file, err := parser.ParseFile(
				fileSet,
				"synthetic.go",
				tc.source,
				parser.SkipObjectResolution,
			)
			if err != nil {
				t.Fatalf("parser.ParseFile(synthetic) error = %v, want nil", err)
			}
			gotReject := false
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if ok && forbiddenSelector(selector) {
					gotReject = true
				}
				return true
			})
			if gotReject != tc.wantReject {
				t.Fatalf(
					"forbidden selector match = %t, want %t for %q",
					gotReject,
					tc.wantReject,
					tc.source,
				)
			}
		})
	}
}

func countParameters(fields *ast.FieldList) int {
	if fields == nil {
		return 0
	}
	count := 0
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			count++
			continue
		}
		count += len(field.Names)
	}
	return count
}

func rejectRetainedByteSlices(
	t *testing.T,
	structureName string,
	structure *ast.StructType,
) {
	t.Helper()

	for _, field := range structure.Fields.List {
		array, ok := field.Type.(*ast.ArrayType)
		if !ok || array.Len != nil {
			continue
		}
		element, ok := array.Elt.(*ast.Ident)
		if ok && element.Name == "byte" {
			t.Errorf(
				"production struct %s retains []byte, want streaming fixed-size state",
				structureName,
			)
		}
	}
}

func productionFunctionNames(t *testing.T) []string {
	t.Helper()

	var names []string
	for _, file := range productionFiles(t) {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || !ast.IsExported(function.Name.Name) {
				continue
			}
			names = append(names, function.Name.Name)
		}
	}
	slices.Sort(names)
	return names
}

type namedStructType struct {
	definition *ast.StructType
	name       string
}

func productionStructTypes(t *testing.T, file *ast.File) []namedStructType {
	t.Helper()

	var found []namedStructType
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, raw := range general.Specs {
			specification, ok := raw.(*ast.TypeSpec)
			if !ok {
				t.Fatalf("type declaration spec = %T, want *ast.TypeSpec", raw)
			}
			structure, ok := specification.Type.(*ast.StructType)
			if !ok {
				continue
			}
			found = append(found, namedStructType{
				name:       specification.Name.Name,
				definition: structure,
			})
		}
	}
	return found
}

func productionStructNames(t *testing.T) []string {
	t.Helper()

	var names []string
	for _, file := range productionFiles(t) {
		for _, structure := range productionStructTypes(t, file) {
			names = append(names, structure.name)
		}
	}
	slices.Sort(names)
	return names
}

func productionFiles(t *testing.T) []*ast.File {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(package directory) error = %v, want nil", err)
	}
	fileSet := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			fileSet,
			entry.Name(),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			t.Fatalf(
				"parser.ParseFile(%q) error = %v, want nil",
				entry.Name(),
				parseErr,
			)
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		t.Fatal("production file count = 0, want > 0")
	}
	return files
}
