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
		"Containment":         "validated isolation and cancel policy",
		"Execution":           "supervised running-child capability",
		"ProcessSighting":     "host process-table observation",
		"signalDelivery":      "typed signal-delivery handoff",
		"Environment":         "validated environment projection",
		"EnvironmentName":     "validated environment name fact",
		"EnvironmentLookup":   "typed single-variable ambient observation",
		"EnvironmentValue":    "validated environment value fact",
		"EnvironmentVariable": "validated environment pair",
		"ExitCode":            "validated exit observation",
		"failure":             "typed execution failure",
		"outputLimitExceeded": "typed output-bound failure",
		"Plan":                "validated stream-free execution capability",
		"planWire":            "private canonical execution-plan projection",
		"ResultObservation":   "durable exact direct-child result projection",
		"Request":             "validated execution ingress",
		"Result":              "fixed-size execution result",
		"streamFailure":       "typed stream failure",
		"Streams":             "caller-owned stream capabilities",
		"TruncatingWriter":    "bounded streaming output capability",
		"boundedWriter":       "bounded streaming output projection",
		"commandStreams":      "fixed execution stream carrier",
		"observedReader":      "streaming input observation",
		"preparedCommand":     "os/exec command and cancellation observation",
		"projectionExtension": "bounded projection arithmetic request",
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
		"encoding/json/v2",
		"errors",
		"github.com/deliri/primitive/v2026/contextstate",
		"github.com/deliri/primitive/v2026/core",
		"github.com/deliri/primitive/v2026/temporal",
		"golang.org/x/sys/unix",
		"golang.org/x/sys/windows",
		"io",
		"math",
		"os",
		"os/exec",
		"reflect",
		"strings",
		"sync",
		"sync/atomic",
		"syscall",
	}
}

// signalLeafFiles are the platform leaves permitted to speak a signal: the
// containment leaves that deliver one, and the termination leaf that reads
// one back out of a reaped wait status. Signals are not banned from this
// package; containment gives every cancellation and force stop exactly one
// owned address. What must never exist is a second, hidden signal path
// inside the execution flow.
func signalLeafFiles() []string {
	return []string{
		"containment_unix.go",
		"containment_windows.go",
		"containment_other.go",
		"termination_unix.go",
	}
}

// resolutionLeafFile is the one production file permitted to consult PATH.
// Resolution is not banned from this package: Request.Command is an absolute
// path precisely so the PATH decision is made once, visibly, and Resolve is
// where that happens. What must never exist is a second, hidden resolution
// inside the execution path, which is what a caller would get if Run ever
// looked a name up on its own.
const resolutionLeafFile = "resolve.go"

// ambientLeafFile is the one production file permitted the whole-environment
// read: AmbientEnvironment owns the calling process's inherited set so no
// consumer reads os.Environ itself, and the selector stays banned everywhere
// else in this package exactly as before.
const ambientLeafFile = "ambient.go"

func isAmbientEnvironmentSelector(selector *ast.SelectorExpr) bool {
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok || qualifier.Name != "os" {
		return false
	}
	return selector.Sel.Name == "Environ" || selector.Sel.Name == "LookupEnv"
}

// forbiddenPackageSelectors are package-qualified substrate calls that would
// move ownership out of this package: an unsupervised command, a raw process
// path, or ambient environment reads that bypass the typed Environment
// contract. These must stay qualified, because bare names such as Command also
// name legitimate typed struct fields. Path resolution is scoped by file rather
// than listed here; see resolutionLeafFile.
func forbiddenPackageSelectors() []string {
	return []string{
		"exec.Command",
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
		"Alive",
		"AmbientArguments",
		"AmbientEnvironment",
		"Begin",
		"Executable",
		"LookupAmbientEnvironment",
		"NewArgument",
		"NewEnvironmentName",
		"NewEnvironmentValue",
		"NewTruncatingWriter",
		"ObserveProcesses",
		"ParseArguments",
		"ParseEffectiveEnvironment",
		"ParseExactEnvironment",
		"Resolve",
		"ResolveExecutable",
		"Run",
		"Self",
		"StandardStreams",
		"WorkingDirectory",
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
	for _, production := range productionFiles(t) {
		file := production.file
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

// isPathResolutionSelector matches the ambient-PATH lookup. It stays qualified
// for the same reason the forbidden list does: a bare Sel name would also match
// an unrelated typed method some future contract introduces.
func isPathResolutionSelector(selector *ast.SelectorExpr) bool {
	qualifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	return qualifier.Name == "exec" && selector.Sel.Name == "LookPath"
}

// TestPathResolutionMatcherHasARedState proves the file-scoped rule can fail.
// Moving PATH resolution out of the shared forbidden list would be a quiet
// weakening if the new matcher were never shown rejecting anything, so it is
// exercised against synthetic source the same way the shared list is.
func TestPathResolutionMatcherHasARedState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		source     string
		wantReject bool
	}{
		{
			name:       "an ambient PATH lookup is matched",
			source:     "package process\nfunc run() { exec.LookPath(name) }\n",
			wantReject: true,
		},
		{
			name:   "the context-bound execution leaf is not a resolution",
			source: "package process\nfunc run() { exec.CommandContext(ctx, path) }\n",
		},
		{
			name:   "a same-named method on a typed value is not a resolution",
			source: "package process\nfunc run() { _ = catalog.LookPath(name) }\n",
		},
		{
			name:   "the typed parse that gates the result is not a resolution",
			source: "package process\nfunc run() { _, _ = core.ParseAbsolutePath(found) }\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			file, err := parser.ParseFile(
				token.NewFileSet(),
				"synthetic_resolution.go",
				tc.source,
				parser.SkipObjectResolution,
			)
			if err != nil {
				t.Fatalf("parser.ParseFile(synthetic) error = %v, want nil", err)
			}
			matched := false
			ast.Inspect(file, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if ok && isPathResolutionSelector(selector) {
					matched = true
				}
				return true
			})
			if matched != tc.wantReject {
				t.Fatalf("path resolution match = %t, want %t for %q", matched, tc.wantReject, tc.source)
			}
		})
	}
}

// TestOnlyTheResolutionLeafConsultsPath is the ratchet the file-scoped rule
// exists for, stated as its own proof rather than hidden inside a broader
// structural scan. Exactly one production file may consult PATH, and it is not
// the one that executes.
func TestOnlyTheResolutionLeafConsultsPath(t *testing.T) {
	t.Parallel()

	var consulting []string
	for _, production := range productionFiles(t) {
		ast.Inspect(production.file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if ok && isPathResolutionSelector(selector) &&
				!slices.Contains(consulting, production.name) {
				consulting = append(consulting, production.name)
			}
			return true
		})
	}
	slices.Sort(consulting)
	want := []string{resolutionLeafFile}
	if !slices.Equal(consulting, want) {
		t.Fatalf("production files consulting PATH = %q, want exactly %q", consulting, want)
	}
}

func TestAmbientEnvironmentEffectLeafIsExact(t *testing.T) {
	t.Parallel()

	var got []string
	for _, production := range productionFiles(t) {
		ast.Inspect(production.file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if ok && isAmbientEnvironmentSelector(selector) {
				got = append(got, production.name+"."+selector.Sel.Name)
			}
			return true
		})
	}
	slices.Sort(got)
	want := []string{ambientLeafFile + ".Environ", ambientLeafFile + ".LookupEnv"}
	if !slices.Equal(got, want) {
		t.Fatalf("production ambient-environment effects = %q, want exactly %q", got, want)
	}
}

func TestProductionStructureForbidsWorldModelsAndWholeOutputPaths(t *testing.T) {
	t.Parallel()

	for _, production := range productionFiles(t) {
		file := production.file
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
				if forbiddenSelector(typed) &&
					!(typed.Sel.Name == "Signal" && slices.Contains(signalLeafFiles(), production.name)) &&
					!(isAmbientEnvironmentSelector(typed) && production.name == ambientLeafFile) {
					t.Errorf(
						"production selector %s in %s at token position %d, want streamed caller-owned output",
						typed.Sel.Name,
						production.name,
						typed.Sel.NamePos,
					)
				}
				if production.name != resolutionLeafFile && isPathResolutionSelector(typed) {
					t.Errorf(
						"production selector %s in %s at token position %d, want PATH resolution only in %s",
						typed.Sel.Name,
						production.name,
						typed.Sel.NamePos,
						resolutionLeafFile,
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
	for _, production := range productionFiles(t) {
		file := production.file
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
	for _, production := range productionFiles(t) {
		file := production.file
		for _, structure := range productionStructTypes(t, file) {
			names = append(names, structure.name)
		}
	}
	slices.Sort(names)
	return names
}

// productionFile pairs one parsed production file with its name so a rule can
// name the single inventoried leaf that owns an effect instead of banning the
// effect from the package that exists to own it.
type productionFile struct {
	file *ast.File
	name string
}

func productionFiles(t *testing.T) []productionFile {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(package directory) error = %v, want nil", err)
	}
	fileSet := token.NewFileSet()
	var files []productionFile
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
		files = append(files, productionFile{file: file, name: entry.Name()})
	}
	if len(files) == 0 {
		t.Fatal("production file count = 0, want > 0")
	}
	return files
}
