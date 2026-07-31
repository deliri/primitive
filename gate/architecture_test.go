package gate

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type (
	requestIngress[T any] struct{}
	capability[T any]     struct{}
	failureDetail[T any]  struct{}
)

// gateContractInventory classifies every Gate production struct by its real
// role. It is a compiler-visible wiring ratchet, not behavioral proof.
//
// Gate owns exactly two capability carriers and two typed results. A new
// production struct that is neither is a scope decision, not an implementation
// detail, and must fail here before it can quietly enter the package.
type gateContractInventory struct {
	AuthorizeRequest requestIngress[AuthorizeRequest]
	NewWorkPermit    capability[NewWorkPermit]
	DenialError      capability[denialError]
	ContractError    failureDetail[ContractError]
}

// TestGateProductionImportsMatchTheCompilerOwnedCatalog projects Gate's
// admitted production frontier from Core rather than restating it, so the
// catalog stays the single owner of the graph.
func TestGateProductionImportsMatchTheCompilerOwnedCatalog(t *testing.T) {
	t.Parallel()

	catalog := core.PrimitiveArchitecture()
	var want []string
	for contract := range catalog.DirectImports() {
		if contract.Importer != core.PackageGate {
			continue
		}
		want = append(want, mustImportPath(t, contract.Imported))
	}
	sort.Strings(want)
	got := siblingImports(t, productionSources)
	if !slices.Equal(got, want) {
		t.Fatalf("Gate production imports = %#v, want %#v", got, want)
	}
}

// TestGateTestImportsSpendOnlyDeclaredTestEdges proves the test sources use
// every declared test-only edge and nothing beyond the admitted frontier. An
// unused declared edge is a ceremonial import; an undeclared one is hidden
// coupling.
func TestGateTestImportsSpendOnlyDeclaredTestEdges(t *testing.T) {
	t.Parallel()

	catalog := core.PrimitiveArchitecture()
	var admitted []string
	for contract := range catalog.DirectImports() {
		if contract.Importer == core.PackageGate {
			admitted = append(admitted, mustImportPath(t, contract.Imported))
		}
	}
	var declared []string
	for contract := range catalog.DirectTestImports() {
		if contract.Importer != core.PackageGate {
			continue
		}
		declared = append(declared, mustImportPath(t, contract.Imported))
	}
	if len(declared) == 0 {
		t.Fatal("Gate declared test edges = 0, want the real signed-lease proof frontier")
	}
	admitted = append(admitted, declared...)

	got := siblingImports(t, testSources)
	for _, gotImport := range got {
		if !slices.Contains(admitted, gotImport) {
			t.Errorf("Gate test import %q is undeclared coupling, admitted = %#v", gotImport, admitted)
		}
	}
	for _, wantImport := range declared {
		if !slices.Contains(got, wantImport) {
			t.Errorf("declared test edge %q is unused ceremony, test imports = %#v", wantImport, got)
		}
	}
}

// TestGatePublicSurfaceStaysExactlyOneOperation ratchets the exported surface.
// Gate is one decision; a second exported function is a new owned
// responsibility and must be a deliberate review decision.
func TestGatePublicSurfaceStaysExactlyOneOperation(t *testing.T) {
	t.Parallel()

	var gotFunctions []string
	for _, file := range parseSources(t, productionSources) {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || !function.Name.IsExported() {
				continue
			}
			gotFunctions = append(gotFunctions, function.Name.Name)
		}
	}
	sort.Strings(gotFunctions)
	wantFunctions := []string{"AuthorizeNewWork"}
	if !slices.Equal(gotFunctions, wantFunctions) {
		t.Fatalf(
			"Gate exported functions = %#v, want %#v",
			gotFunctions, wantFunctions,
		)
	}
}

// TestGateProductionStructsHaveCompilerVisibleDataFlowRoles prevents a new
// production carrier from entering Gate without an explicit role.
func TestGateProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	gotProduction := structNames(t, parseSources(t, productionSources))
	wantClassified := inventoryTypeNames(t)
	for _, gotName := range gotProduction {
		if !slices.Contains(wantClassified, gotName) {
			t.Errorf("production struct %q has no compiler-visible data-flow role", gotName)
		}
	}
	for _, wantName := range wantClassified {
		if !slices.Contains(gotProduction, wantName) {
			t.Errorf("classified struct %q does not exist in Gate production", wantName)
		}
	}
}

type sourceSet uint8

const (
	sourceSetUnknown sourceSet = iota
	productionSources
	testSources
)

func (s sourceSet) admits(filename string) bool {
	isTest := strings.HasSuffix(filename, "_test.go")
	switch s {
	case productionSources:
		return !isTest
	case testSources:
		return isTest
	default:
		return false
	}
}

func siblingImports(t *testing.T, set sourceSet) []string {
	t.Helper()

	var got []string
	for _, file := range parseSources(t, set) {
		for _, specification := range file.Imports {
			path, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				t.Fatalf(
					"strconv.Unquote(%s) error = %v",
					specification.Path.Value, err,
				)
			}
			if !strings.HasPrefix(path, core.PrimitivePackagePathPrefix) {
				continue
			}
			if path == core.PrimitivePackagePathPrefix+"gate" {
				continue
			}
			if !slices.Contains(got, path) {
				got = append(got, path)
			}
		}
	}
	sort.Strings(got)
	return got
}

func structNames(t *testing.T, files []*ast.File) []string {
	t.Helper()

	var got []string
	for _, file := range files {
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				specification, ok := raw.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := specification.Type.(*ast.StructType); ok {
					got = append(got, specification.Name.Name)
				}
			}
		}
	}
	sort.Strings(got)
	return got
}

func inventoryTypeNames(t *testing.T) []string {
	t.Helper()

	files := token.NewFileSet()
	file, err := parser.ParseFile(
		files, "architecture_test.go", nil, parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatalf("parser.ParseFile(architecture_test.go) error = %v", err)
	}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			specification, ok := raw.(*ast.TypeSpec)
			if !ok || specification.Name.Name != "gateContractInventory" {
				continue
			}
			structure, ok := specification.Type.(*ast.StructType)
			if !ok {
				t.Fatalf(
					"gateContractInventory AST type = %T, want *ast.StructType",
					specification.Type,
				)
			}
			var got []string
			for _, field := range structure.Fields.List {
				classification, ok := field.Type.(*ast.IndexExpr)
				if !ok {
					t.Fatalf(
						"gateContractInventory field AST type = %T, want *ast.IndexExpr",
						field.Type,
					)
				}
				classified, ok := classification.Index.(*ast.Ident)
				if !ok {
					t.Fatalf(
						"gateContractInventory classified type AST = %T, want *ast.Ident",
						classification.Index,
					)
				}
				got = append(got, classified.Name)
			}
			sort.Strings(got)
			return got
		}
	}
	t.Fatalf(
		"gateContractInventory declarations found = 0 among %d declarations, want 1",
		len(file.Decls),
	)
	return nil
}

func parseSources(t *testing.T, set sourceSet) []*ast.File {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(.) error = %v", err)
	}
	files := token.NewFileSet()
	var got []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			!set.admits(entry.Name()) {
			continue
		}
		parsed, err := parser.ParseFile(files, entry.Name(), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v", entry.Name(), err)
		}
		got = append(got, parsed)
	}
	if len(got) == 0 {
		t.Fatalf("parsed Gate sources for set %d = 0, want at least one file", set)
	}
	return got
}

func mustImportPath(t *testing.T, identity core.PackageIdentity) string {
	t.Helper()

	path, err := identity.ImportPath()
	if err != nil {
		t.Fatalf("PackageIdentity(%v).ImportPath() error = %v, want nil", identity, err)
	}
	return path
}

var _ = gateContractInventory{}
