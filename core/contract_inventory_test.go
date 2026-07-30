package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

type (
	architectureFact[T any]  struct{}
	protocolFact[T any]      struct{}
	internalFlow[T any]      struct{}
	capabilityWrapper[T any] struct{}
	productionStructName     string
)

const coreProductionStructMaximum = 32

type productionStructInventory struct {
	names [coreProductionStructMaximum]productionStructName
	count uint8
}

// coreContractInventory classifies every production struct by its real role.
// It is a compiler-visible wiring ratchet, not behavioral proof.
type coreContractInventory struct {
	PackageContract            architectureFact[PackageContract]
	DirectImportContract       architectureFact[DirectImportContract]
	ArchitectureCatalog        architectureFact[ArchitectureCatalog]
	StrictJSONLimits           protocolFact[StrictJSONLimits]
	strictJSONContainer        internalFlow[strictJSONContainer]
	jsonContractDiagnostic     internalFlow[jsonContractDiagnostic]
	errorIdentityParentSet     internalFlow[errorIdentityParentSet]
	ByteCount                  protocolFact[ByteCount]
	ByteLength                 protocolFact[ByteLength]
	DeclaredBodyLength         protocolFact[DeclaredBodyLength]
	SHA256Digest               protocolFact[SHA256Digest]
	CRC32C                     protocolFact[CRC32C]
	Ed25519PublicKey           protocolFact[Ed25519PublicKey]
	SecretMaterial             capabilityWrapper[SecretMaterial]
	secretMaterialState        internalFlow[secretMaterialState]
	GovernanceDocumentContract protocolFact[GovernanceDocumentContract]
	TestIsolationDeclaration   protocolFact[TestIsolationDeclaration]
	PathComponent              protocolFact[PathComponent]
	AbsolutePath               protocolFact[AbsolutePath]
	RelativePath               protocolFact[RelativePath]
	HTTPStatusCode             protocolFact[HTTPStatusCode]
	HTTPHeaderName             protocolFact[HTTPHeaderName]
	HTTPMediaType              protocolFact[HTTPMediaType]
	HTTPContentCoding          protocolFact[HTTPContentCoding]
	HTTPEndpoint               protocolFact[HTTPEndpoint]
	Platform                   protocolFact[Platform]
}

var (
	_ = coreContractInventory{}.strictJSONContainer
	_ = coreContractInventory{}.jsonContractDiagnostic
	_ = coreContractInventory{}.errorIdentityParentSet
	_ = coreContractInventory{}.secretMaterialState
)

// TestCoreProductionStructsHaveCompilerVisibleDataFlowRoles prevents a new
// production carrier from entering Core without an explicit role.
func TestCoreProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	gotProduction, gotErr := productionStructNames()
	if gotErr != nil {
		t.Fatalf("productionStructNames() error = %v, want nil", gotErr)
	}
	wantClassified, gotClassifiedErr := classifiedStructNames()
	if gotClassifiedErr != nil {
		t.Fatalf("classifiedStructNames() error = %v, want nil", gotClassifiedErr)
	}
	for _, gotName := range gotProduction.Values() {
		if !wantClassified.Contains(gotName) {
			t.Errorf("production struct %q has no compiler-visible data-flow role", gotName)
		}
	}
	for _, wantName := range wantClassified.Values() {
		if !gotProduction.Contains(wantName) {
			t.Errorf("classified struct %q does not exist in production", wantName)
		}
	}
}

func productionStructNames() (productionStructInventory, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return productionStructInventory{}, err
	}
	var names productionStructInventory
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(files, entry.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return productionStructInventory{}, parseErr
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				spec := raw.(*ast.TypeSpec)
				if _, ok := spec.Type.(*ast.StructType); ok {
					if addErr := names.Add(productionStructName(spec.Name.Name)); addErr != nil {
						return productionStructInventory{}, addErr
					}
				}
			}
		}
	}
	return names, nil
}

func classifiedStructNames() (productionStructInventory, error) {
	files := token.NewFileSet()
	file, err := parser.ParseFile(
		files,
		"contract_inventory_test.go",
		nil,
		parser.SkipObjectResolution,
	)
	if err != nil {
		return productionStructInventory{}, err
	}
	var names productionStructInventory
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			spec := raw.(*ast.TypeSpec)
			if spec.Name.Name != "coreContractInventory" {
				continue
			}
			structure := spec.Type.(*ast.StructType)
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					if addErr := names.Add(productionStructName(name.Name)); addErr != nil {
						return productionStructInventory{}, addErr
					}
				}
			}
			return names, nil
		}
	}
	return productionStructInventory{}, ErrPrimitiveContract
}

func (i *productionStructInventory) Add(name productionStructName) error {
	if name == "" {
		return ErrPrimitiveContract
	}
	if i.Contains(name) {
		return ErrPrimitiveContract
	}
	if int(i.count) >= len(i.names) {
		return ErrPrimitiveContract
	}
	i.names[i.count] = name
	i.count++
	return nil
}

func (i productionStructInventory) Contains(name productionStructName) bool {
	for _, candidate := range i.Values() {
		if candidate == name {
			return true
		}
	}
	return false
}

func (i productionStructInventory) Values() []productionStructName {
	return i.names[:i.count]
}
