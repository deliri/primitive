package controlwire

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

//go:embed *.go
var controlwireGoSources embed.FS

type (
	// controlwireProtocolFact marks a value that crosses the control wire and
	// must mean the same thing on both ends.
	controlwireProtocolFact[T any] struct{}
	// controlwireSecretCarrier marks a value that holds unspent secret material
	// and therefore owns redaction and destruction.
	controlwireSecretCarrier[T any] struct{}
	// controlwireDerivedFact marks a one-way value derived from a secret that a
	// control plane may persist.
	controlwireDerivedFact[T any] struct{}
	// controlwireExecutionContract marks a typed in-process HTTP execution
	// boundary. It carries protocol facts but is not itself serialized.
	controlwireExecutionContract[T any] struct{}
	// controlwireClientCapability marks the opaque installed-tool side of the
	// paired control socket. It may send shared documents but owns no authority
	// support or response writer.
	controlwireClientCapability[T any] struct{}
	// controlwireServerCapability marks the opaque authority side of the paired
	// control socket. It may receive and answer shared documents but owns no
	// client endpoint or transport.
	controlwireServerCapability[T any] struct{}
	// controlwireInternalFlow marks a private typed projection used only across
	// one owner-controlled implementation boundary.
	controlwireInternalFlow[T any] struct{}

	controlwireProductionStructName string
)

// controlwireContractInventory classifies every production struct by its role.
// A new carrier cannot enter this package without being named here.
type controlwireContractInventory struct {
	RequestNonce              controlwireProtocolFact[RequestNonce]
	AuthorityNonce            controlwireProtocolFact[AuthorityNonce]
	RegistrationToken         controlwireSecretCarrier[RegistrationToken]
	RegistrationTokenVerifier controlwireDerivedFact[RegistrationTokenVerifier]
	AccessToken               controlwireSecretCarrier[AccessToken]
	AccessTokenVerifier       controlwireDerivedFact[AccessTokenVerifier]
	PolicyCursor              controlwireProtocolFact[PolicyCursor]
	RouteContract             controlwireProtocolFact[RouteContract]
	ProtocolCapability        controlwireProtocolFact[ProtocolCapability]
	ProtocolSupportRequest    controlwireExecutionContract[ProtocolSupportRequest]
	ProtocolSupport           controlwireExecutionContract[ProtocolSupport]
	ProtocolAssessmentRequest controlwireExecutionContract[ProtocolAssessmentRequest]
	ProtocolAssessment        controlwireExecutionContract[ProtocolAssessment]
	RequestCommitment         controlwireProtocolFact[RequestCommitment]
	ReplayIdentity            controlwireProtocolFact[ReplayIdentity]
	ReplayCheck               controlwireExecutionContract[ReplayCheck]
	replayIdentityWire        controlwireInternalFlow[replayIdentityWire]
	policyCursorWire          controlwireInternalFlow[policyCursorWire]
	ClientConfiguration       controlwireExecutionContract[ClientConfiguration]
	Client                    controlwireClientCapability[Client]
	AuthorityConfiguration    controlwireExecutionContract[AuthorityConfiguration]
	Authority                 controlwireServerCapability[Authority]
	ClientJSONCall            controlwireExecutionContract[ClientJSONCall[RoutedJSONRequest]]
	AuthorityJSONReceiveCall  controlwireExecutionContract[AuthorityJSONReceiveCall]
	RoutedJSONReceive         controlwireExecutionContract[RoutedJSONReceive[RoutedJSONRequest]]
	ControlJSONWriteCall      controlwireExecutionContract[ControlJSONWriteCall[AuthenticatedResponseProjection]]
}

func TestControlWireProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	_ = controlwireContractInventory{}.RequestNonce
	_ = controlwireContractInventory{}.AuthorityNonce
	_ = controlwireContractInventory{}.RegistrationToken
	_ = controlwireContractInventory{}.RegistrationTokenVerifier
	_ = controlwireContractInventory{}.replayIdentityWire
	_ = controlwireContractInventory{}.policyCursorWire

	gotProduction, err := controlwireProductionStructNames()
	if err != nil {
		t.Fatalf("controlwireProductionStructNames() error = %v, want nil", err)
	}
	wantClassified := controlwireClassifiedStructNames(t)
	for _, got := range gotProduction {
		if !slices.Contains(wantClassified, got) {
			t.Errorf("production struct %q has no compiler-visible data-flow role", got)
		}
	}
	for _, want := range wantClassified {
		if !slices.Contains(gotProduction, want) {
			t.Errorf("classified struct %q does not exist in production", want)
		}
	}
}

// controlwireProductionStructNames scans the package's own non-test sources, so
// adding a struct without classifying it fails this test rather than passing on
// a stale hand-maintained list.
func controlwireProductionStructNames() ([]controlwireProductionStructName, error) {
	entries, err := controlwireGoSources.ReadDir(".")
	if err != nil {
		return nil, err
	}
	files := token.NewFileSet()
	var declarations []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := controlwireParseSource(files, entry.Name())
		if parseErr != nil {
			return nil, parseErr
		}
		declarations = append(declarations, file)
	}
	return controlwireStructNames(declarations), nil
}

func controlwireParseSource(files *token.FileSet, name string) (*ast.File, error) {
	data, err := controlwireGoSources.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return parser.ParseFile(files, name, data, parser.SkipObjectResolution)
}

// Follow local named types as well as direct struct syntax. A private wire
// projection must not escape classification by declaring `type wire Public`.
func controlwireStructNames(files []*ast.File) []controlwireProductionStructName {
	types := map[string]ast.Expr{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, raw := range gen.Specs {
				if spec, ok := raw.(*ast.TypeSpec); ok {
					types[spec.Name.Name] = spec.Type
				}
			}
		}
	}
	var names []controlwireProductionStructName
	for name, expression := range types {
		if controlwireStructExpression(expression, types) {
			names = append(names, controlwireProductionStructName(name))
		}
	}
	slices.Sort(names)
	return names
}

func controlwireStructExpression(expression ast.Expr, types map[string]ast.Expr) bool {
	seen := map[string]bool{}
	for expression != nil {
		switch node := expression.(type) {
		case *ast.StructType:
			return node != nil
		case *ast.Ident:
			if node == nil || seen[node.Name] {
				return false
			}
			seen[node.Name] = true
			next, ok := types[node.Name]
			if !ok {
				return false
			}
			expression = next
		case *ast.IndexExpr:
			if node == nil {
				return false
			}
			expression = node.X
		case *ast.IndexListExpr:
			if node == nil {
				return false
			}
			expression = node.X
		default:
			return false
		}
	}
	return false
}

// controlwireClassifiedStructNames reads the inventory's own field names
// through the AST rather than reflection, so the classification stays a
// compiler-visible declaration instead of a runtime lookup.
func controlwireClassifiedStructNames(t *testing.T) []controlwireProductionStructName {
	t.Helper()

	file, err := controlwireParseSource(token.NewFileSet(), "contract_inventory_test.go")
	if err != nil {
		t.Fatalf("parser.ParseFile() error = %v, want nil", err)
	}
	var names []controlwireProductionStructName
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			spec, ok := raw.(*ast.TypeSpec)
			if !ok || spec.Name.Name != "controlwireContractInventory" {
				continue
			}
			structure, ok := spec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					names = append(names, controlwireProductionStructName(name.Name))
				}
			}
		}
	}
	if len(names) == 0 {
		t.Fatalf("controlwireContractInventory classified structs = %d, want at least one", len(names))
	}
	return names
}

func TestStructInventoryRecognizesNamedProjections(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		want         []controlwireProductionStructName
	}{
		{name: "direct struct", source: "type A struct{}", want: []controlwireProductionStructName{"A"}},
		{name: "named wire projection", source: "type A struct{};type B A", want: []controlwireProductionStructName{"A", "B"}},
		{name: "alias chain", source: "type A struct{};type B=A;type C B", want: []controlwireProductionStructName{"A", "B", "C"}},
		{name: "generic projection", source: "type A[T any] struct{X T};type B A[int]", want: []controlwireProductionStructName{"A", "B"}},
		{name: "scalar and pointer are not structs", source: "type A uint8;type B A;type C *A"},
		{name: "malformed type cycle terminates", source: "type A B;type B A"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "synthetic.go", "package fixture;"+tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("fixture parse error=%v, want nil", err)
			}
			got := controlwireStructNames([]*ast.File{file})
			if !slices.Equal(got, tc.want) {
				t.Fatalf("struct inventory=%v, want %v", got, tc.want)
			}
		})
	}
}
