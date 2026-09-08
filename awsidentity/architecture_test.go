package awsidentity

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/exchange"
)

type awsProtocolRole[T protocolFact] struct{}
type awsInternalRole[T internalFlow] struct{}
type awsCapabilityRole[T capabilityWrapper] struct{}
type awsFailureRole[T typedFailure] struct{}

// Every instantiated constraint checks the actual production marker signature.
// Exact discovered membership also catches an added or removed production struct.
type awsStructInventory struct {
	Audience                awsProtocolRole[Audience]
	Policy                  awsProtocolRole[Policy]
	RequestInput            awsProtocolRole[RequestInput]
	acquisitionCall         awsInternalRole[acquisitionCall]
	amazonResponse          awsInternalRole[amazonResponse]
	amazonResult            awsInternalRole[amazonResult]
	amazonUnexpectedElement awsInternalRole[amazonUnexpectedElement]
	amazonTokenElement      awsInternalRole[amazonTokenElement]
	amazonExpirationElement awsInternalRole[amazonExpirationElement]
	amazonResponseMetadata  awsInternalRole[amazonResponseMetadata]
	amazonRequestIDElement  awsInternalRole[amazonRequestIDElement]
	Client                  awsCapabilityRole[Client]
	Request                 awsCapabilityRole[Request]
	Token                   awsCapabilityRole[Token]
	requestError            awsFailureRole[requestError]
}

type awsFuzzDoor[T any] struct {
	Door T
	Fuzz func(*testing.F)
}
type awsPublicFunctions struct {
	Acquire       awsFuzzDoor[func(context.Context, Client, Request) (Token, error)]
	NewRequest    awsFuzzDoor[func(RequestInput) (Request, error)]
	ParseAudience awsFuzzDoor[func(string) (Audience, error)]
	// These receive validated internal capabilities or no input, not wire data.
	NewClient     func(exchange.Client) (Client, error)
	DefaultPolicy func() (Policy, error)
}

func TestAWSIdentityProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()
	got, err := scanAWSProduction(".")
	if err != nil {
		t.Fatalf("production scan error = %v, want nil", err)
	}
	inventory := awsStructInventory{
		acquisitionCall:         awsInternalRole[acquisitionCall]{},
		amazonResponse:          awsInternalRole[amazonResponse]{},
		amazonResult:            awsInternalRole[amazonResult]{},
		amazonUnexpectedElement: awsInternalRole[amazonUnexpectedElement]{},
		amazonTokenElement:      awsInternalRole[amazonTokenElement]{},
		amazonExpirationElement: awsInternalRole[amazonExpirationElement]{},
		amazonResponseMetadata:  awsInternalRole[amazonResponseMetadata]{},
		amazonRequestIDElement:  awsInternalRole[amazonRequestIDElement]{},
		requestError:            awsFailureRole[requestError]{},
	}
	want := awsFieldNames(reflect.TypeOf(inventory))
	if !slices.Equal(got.structs, want) {
		t.Fatalf("production structs = %q, want exactly classified %q", got.structs, want)
	}
	if len(got.aliases) != 0 {
		t.Fatalf("production aliases = %q, want no compatibility aliases", got.aliases)
	}
}

func TestAWSExternalDoorInventoryMatchesProduction(t *testing.T) {
	t.Parallel()
	inventory := awsPublicFunctions{
		Acquire:       awsFuzzDoor[func(context.Context, Client, Request) (Token, error)]{Acquire, FuzzAWSProviderResponseSemanticClosure},
		NewRequest:    awsFuzzDoor[func(RequestInput) (Request, error)]{NewRequest, FuzzAWSRequestQueryClosure},
		ParseAudience: awsFuzzDoor[func(string) (Audience, error)]{ParseAudience, FuzzAWSAudienceExactUTF8},
		NewClient:     NewClient, DefaultPolicy: DefaultPolicy,
	}
	got, err := scanAWSProduction(".")
	if err != nil {
		t.Fatalf("production scan error = %v, want nil", err)
	}
	want := awsFieldNames(reflect.TypeOf(inventory))
	if !slices.Equal(got.functions, want) {
		t.Fatalf("public functions = %q, want inventory %q", got.functions, want)
	}
	// Method signatures are compiler checked; the scan catches new disclosure or
	// decoder methods, including ones absent from the old ingress name allowlist.
	var (
		_ func(Audience) string          = Audience.String
		_ func(Audience) error           = Audience.Validate
		_ func(Client) error             = Client.Validate
		_ func(Policy) error             = Policy.Validate
		_ func(RequestInput) error       = RequestInput.Validate
		_ func(Request) error            = Request.Validate
		_ func(Request, fmt.State, rune) = Request.Format
		_ func(Token) error              = Token.Validate
		_ func(Token) (string, error)    = Token.BearerValue
		_ func(Token, fmt.State, rune)   = Token.Format
	)
	wantMethods := []string{"Audience.String", "Audience.Validate", "Client.Validate", "Policy.Validate", "Request.Format", "Request.Validate", "RequestInput.Validate", "Token.BearerValue", "Token.Format", "Token.Validate"}
	if !slices.Equal(got.methods, wantMethods) {
		t.Fatalf("public methods = %q, want %q", got.methods, wantMethods)
	}
}

type awsSourceFacts struct{ structs, functions, methods, aliases []string }

func awsFieldNames(typ reflect.Type) []string {
	names := make([]string, 0, typ.NumField())
	for field := range typ.Fields() {
		names = append(names, field.Name)
	}
	slices.Sort(names)
	return names
}
func scanAWSProduction(root string) (awsSourceFacts, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return awsSourceFacts{}, err
	}
	var facts awsSourceFacts
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, entry.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return awsSourceFacts{}, err
		}
		collectAWSFacts(file, &facts)
	}
	slices.Sort(facts.structs)
	slices.Sort(facts.functions)
	slices.Sort(facts.methods)
	slices.Sort(facts.aliases)
	return facts, nil
}
func collectAWSFacts(file *ast.File, facts *awsSourceFacts) {
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				declaration, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if declaration.Assign.IsValid() {
					facts.aliases = append(facts.aliases, declaration.Name.Name)
					continue
				}
				if _, ok := declaration.Type.(*ast.StructType); ok {
					facts.structs = append(facts.structs, declaration.Name.Name)
				}
			}
		case *ast.FuncDecl:
			if !typed.Name.IsExported() {
				continue
			}
			if typed.Recv == nil {
				facts.functions = append(facts.functions, typed.Name.Name)
				continue
			}
			receiver := typed.Recv.List[0].Type
			if pointer, ok := receiver.(*ast.StarExpr); ok {
				receiver = pointer.X
			}
			if name, ok := receiver.(*ast.Ident); ok && name.IsExported() {
				facts.methods = append(facts.methods, name.Name+"."+typed.Name.Name)
			}
		}
	}
}

func TestAWSArchitectureDiscoveryCannotHideNewDoors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		want         awsSourceFacts
	}{
		{"empty package has no invented facts", "package fixture", awsSourceFacts{}},
		{"new decoder outside old name allowlist is discovered", "package fixture; func DecodeFuture([]byte){}", awsSourceFacts{functions: []string{"DecodeFuture"}}},
		{"new pointer decoder method is discovered", "package fixture; type Token struct{}; func (*Token) UnmarshalText([]byte){}", awsSourceFacts{structs: []string{"Token"}, methods: []string{"Token.UnmarshalText"}}},
		{"unclassified private struct is discovered", "package fixture; type hidden struct{}", awsSourceFacts{structs: []string{"hidden"}}},
		{"alias cannot substitute a real struct", "package fixture; type Alias = Original", awsSourceFacts{aliases: []string{"Alias"}}},
		{"private function is not external ingress", "package fixture; func decode(){}", awsSourceFacts{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "fixture.go"), []byte(tc.source), 0o600); err != nil {
				t.Fatalf("write source fixture = %v, want nil", err)
			}
			got, err := scanAWSProduction(root)
			if err != nil || !slices.Equal(got.structs, tc.want.structs) || !slices.Equal(got.functions, tc.want.functions) || !slices.Equal(got.methods, tc.want.methods) || !slices.Equal(got.aliases, tc.want.aliases) {
				t.Fatalf("source facts = (%+v,%v), want %+v", got, err, tc.want)
			}
		})
	}
}
