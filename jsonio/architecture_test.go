package jsonio

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type jsonioIngress[T any] struct{ Value T }
type jsonioObservation[T any] struct{ Value T }
type jsonioCapability[T any] struct{ Value T }
type jsonioKernelFlow[T any] struct{ Value T }
type jsonioError[T any] struct{ Value T }

type jsonioStructInventory struct {
	Request                  jsonioIngress[Request]
	TokenRequest             jsonioIngress[TokenRequest]
	ObjectSourceRequest      jsonioIngress[ObjectSourceRequest]
	ObjectDestinationRequest jsonioIngress[ObjectDestinationRequest]
	Token                    jsonioObservation[Token]
	ObjectEncoder            jsonioCapability[ObjectEncoder[Token]]
	TokenEncoder             jsonioCapability[TokenEncoder]
	CheckedSource            jsonioKernelFlow[checkedJSONSource]
	CheckedDestination       jsonioKernelFlow[checkedJSONDestination]
	ContextSource            jsonioKernelFlow[contextJSONSource]
	ExtentError              jsonioError[ExtentError]
}

func TestProductionStructDataFlowInventory(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := spec.Type.(*ast.StructType); ok {
				got = append(got, spec.Name.Name)
			}
			return false
		})
	}
	var want []string
	for field := range reflect.TypeFor[jsonioStructInventory]().Fields() {
		owner := field.Type.Field(0).Type
		if owner.Kind() != reflect.Struct {
			t.Fatalf("inventory owner %s is not a struct", field.Name)
		}
		name, _, _ := strings.Cut(owner.Name(), "[")
		want = append(want, name)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("production structs = %q, want typed inventory %q", got, want)
	}
}
