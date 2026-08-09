package cloudidentity

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type (
	protocolFact[T any]      struct{ Value T }
	sealedCapability[T any]  struct{ Value T }
	internalFlow[T any]      struct{ Value T }
	operationRequest[T any]  struct{ Value T }
	providerWire[T any]      struct{ Value T }
	capabilityWrapper[T any] struct{ Value T }
)

type cloudidentityStructInventory struct {
	AmazonRequestError            internalFlow[amazonRequestError]
	Client                        capabilityWrapper[Client]
	Audience                      protocolFact[Audience]
	AmazonResponse                providerWire[amazonResponse]
	AmazonResult                  providerWire[amazonResult]
	AmazonTokenElement            providerWire[amazonTokenElement]
	AmazonUnexpectedElement       providerWire[amazonUnexpectedElement]
	Token                         sealedCapability[Token]
	AmazonWebServicesRequestInput operationRequest[AmazonWebServicesRequestInput]
	Request                       operationRequest[Request]
	AmazonWebServicesRequest      sealedCapability[AmazonWebServicesRequest]
	GoogleProtocolContracts       internalFlow[googleProtocolContracts]
	AcquisitionCall               internalFlow[acquisitionCall]
	Policy                        protocolFact[Policy]
}

var _ = cloudidentityStructInventory{}

func TestCloudidentityProductionStructsHaveCompilerVisibleDataFlowRoles(
	t *testing.T,
) {
	t.Parallel()

	gotStructs, gotImports, gotErr := scanCloudidentityProduction(".")
	if gotErr != nil {
		t.Fatalf(
			"scanCloudidentityProduction() error = %v, want nil",
			gotErr,
		)
	}
	wantStructs := classifiedCloudidentityStructs()
	if !slices.Equal(gotStructs, wantStructs) {
		t.Fatalf(
			"Cloudidentity production structs = %q, want classified %q",
			gotStructs,
			wantStructs,
		)
	}
	wantImports := []string{
		"bytes",
		"context",
		"encoding/xml",
		"errors",
		"fmt",
		"github.com/deliri/primitive/v2026/core",
		"github.com/deliri/primitive/v2026/exchange",
		"github.com/deliri/primitive/v2026/temporal",
		"io",
		"net/url",
		"strconv",
		"strings",
		"time",
		"unicode/utf8",
	}
	if !slices.Equal(gotImports, wantImports) {
		t.Fatalf(
			"Cloudidentity production imports = %q, want exact %q",
			gotImports,
			wantImports,
		)
	}
}

// TestProviderSelectionHasOnlyExplicitAcquisitionEntryPoints enumerates every
// exported function in the package, not only those already named Acquire. A
// prefix filter cannot detect the thing this ratchet exists to forbid: a runtime
// dispatcher would simply be named something else.
func TestProviderSelectionHasOnlyExplicitAcquisitionEntryPoints(t *testing.T) {
	t.Parallel()

	got, gotErr := exportedFunctions(".")
	if gotErr != nil {
		t.Fatalf("exportedFunctions() error = %v, want nil", gotErr)
	}
	want := []string{
		"AcquireAmazonWebServices",
		"AcquireGoogleCloud",
		"DefaultPolicy",
		"NewAmazonWebServicesRequest",
		"NewClient",
		"ParseAudience",
		"ParseGoogleCloudCommandOutput",
	}
	if !slices.Equal(got, want) {
		t.Fatalf(
			"exported functions = %q, want exactly %q with no runtime provider dispatcher",
			got,
			want,
		)
	}
}

// TestProviderIsNeverAnExportedArgument is the compiler-visible form of "provider
// choice is made at compile time". Provider may be returned, so a caller can see
// which authority issued a token, but the moment it becomes a parameter the
// package has grown the runtime selector the architecture forbids.
func TestProviderIsNeverAnExportedArgument(t *testing.T) {
	t.Parallel()

	files, gotErr := parseProductionFiles(".")
	if gotErr != nil {
		t.Fatalf("parseProductionFiles() error = %v, want nil", gotErr)
	}
	for name, file := range files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || !ast.IsExported(function.Name.Name) {
				continue
			}
			for _, parameter := range function.Type.Params.List {
				if !mentionsProvider(parameter.Type) {
					continue
				}
				t.Fatalf(
					"%s: exported %s takes a Provider argument, want compile-time provider selection",
					name,
					function.Name.Name,
				)
			}
		}
	}
}

// mentionsProvider reports whether an exported parameter's type names Provider
// anywhere, including behind a pointer, slice, or map.
func mentionsProvider(node ast.Expr) bool {
	found := false
	ast.Inspect(node, func(inner ast.Node) bool {
		identifier, ok := inner.(*ast.Ident)
		if ok && identifier.Name == "Provider" {
			found = true
		}
		return !found
	})
	return found
}

func classifiedCloudidentityStructs() []string {
	inventory := reflect.TypeFor[cloudidentityStructInventory]()
	classified := make([]string, 0, inventory.NumField())
	for field := range inventory.Fields() {
		role := field.Type
		classified = append(
			classified,
			role.Field(0).Type.Name(),
		)
	}
	slices.Sort(classified)
	return classified
}

func scanCloudidentityProduction(
	root string,
) ([]string, []string, error) {
	files, err := parseProductionFiles(root)
	if err != nil {
		return nil, nil, err
	}
	structs := make([]string, 0, len(files))
	imports := make([]string, 0, len(files))
	for _, file := range files {
		structs = append(structs, structNames(file)...)
		for _, spec := range file.Imports {
			path, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				return nil, nil, unquoteErr
			}
			imports = append(imports, path)
		}
	}
	slices.Sort(structs)
	slices.Sort(imports)
	return structs, slices.Compact(imports), nil
}

func structNames(file *ast.File) []string {
	names := make([]string, 0)
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, spec := range generic.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, ok := typeSpec.Type.(*ast.StructType); ok {
				names = append(names, typeSpec.Name.Name)
			}
		}
	}
	return names
}

// parseProductionFiles is the one production-source walk both structural
// ratchets share, so neither can inspect a different set of files than the other.
func parseProductionFiles(root string) (map[string]*ast.File, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	parsed := make(map[string]*ast.File, len(entries))
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			files,
			filepath.Join(root, entry.Name()),
			nil,
			0,
		)
		if parseErr != nil {
			return nil, parseErr
		}
		parsed[entry.Name()] = file
	}
	if len(parsed) == 0 {
		return nil, errors.New("no production sources found")
	}
	return parsed, nil
}

// exportedFunctions lists every exported top-level function in the package.
func exportedFunctions(root string) ([]string, error) {
	files, err := parseProductionFiles(root)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for _, file := range files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil ||
				!ast.IsExported(function.Name.Name) {
				continue
			}
			names = append(names, function.Name.Name)
		}
	}
	slices.Sort(names)
	return names, nil
}
