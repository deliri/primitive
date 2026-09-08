package exchange

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"testing"
)

func TestIngressProjectionExemptionRequiresExactSignature(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, source string
		want         bool
	}{
		{name: "owning validation preserves ordinary error identity", source: "func (T) Validate() error {return nil}", want: true},
		{name: "validation spelling cannot hide a new material result", source: "func (T) Validate() string {return \"\"}"},
		{name: "validation input remains an ingress", source: "func (T) Validate(string) error {return nil}"},
		{name: "string projection preserves exact signature", source: "func (T) String() string {return \"\"}", want: true},
		{name: "String returning bytes stays visible", source: "func (T) String() []byte {return nil}"},
		{name: "nominal Value retains typed refusal", source: "func (T) Value() (string,error) {return \"\",nil}", want: true},
		{name: "Value exposing a reader stays visible", source: "import \"io\"; func (T) Value() io.Reader {return nil}"},
		{name: "Value without refusal is not exempted by its name", source: "func (T) Value() string {return \"\"}"},
		{name: "JSON encoder with native bytes and error is projection", source: "func (T) MarshalJSON() ([]byte,error) {return nil,nil}", want: true},
		{name: "JSON encoder cannot hide input admission", source: "func (T) MarshalJSON([]byte) ([]byte,error) {return nil,nil}"},
		{name: "JSON encoder cannot reorder results", source: "func (T) MarshalJSON() (error,[]byte) {return nil,nil}"},
		{name: "validity observation is boolean", source: "func (T) IsValid() bool {return false}", want: true},
		{name: "zero observation with named result remains projection", source: "func (T) IsZero() (empty bool) {return}", want: true},
		{name: "validity cannot acquire a new argument unseen", source: "func (T) IsValid(bool) bool {return false}"},
		{name: "Go formatting retains standard state and rune", source: "import \"fmt\"; func (T) Format(fmt.State,rune) {}", want: true},
		{name: "renamed Go import does not change formatting ownership", source: "import formatting \"fmt\"; func (T) Format(formatting.State,int32) {}", want: true},
		{name: "foreign State qualifier cannot impersonate fmt", source: "import fmt \"foreign\"; func (T) Format(fmt.State,rune) {}"},
		{name: "same arity with wrong formatting state stays visible", source: "func (T) Format(string,rune) {}"},
		{name: "new formatting result stays visible", source: "import \"fmt\"; func (T) Format(fmt.State,rune) error {return nil}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture;"+tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			fn := file.Decls[len(file.Decls)-1].(*ast.FuncDecl)
			if got := owningProjectionMethodForTest(file, fn); got != tc.want {
				t.Fatalf("projection exemption=%t,want %t", got, tc.want)
			}
		})
	}
}

func TestIngressRuntimeBindingRejectsUnownedFunctions(t *testing.T) {
	t.Parallel()
	var absent func()
	cases := []struct {
		name string
		door any
		want string
	}{
		{name: "actual admission retains compiler symbol", door: NewHeaderValue, want: "NewHeaderValue"},
		{name: "method expression retains owning receiver", door: SocketServerCall.RawQuery, want: "SocketServerCall.RawQuery"},
		{name: "generic admission strips instantiation only", door: ReceiveJSON[admissionJSONDocument, *admissionJSONDocument], want: "ReceiveJSON"},
		{name: "foreign package cannot impersonate an inventory binding", door: http.Error},
		{name: "untyped nil cannot bind a door"},
		{name: "typed nil function cannot bind a door", door: absent},
		{name: "string symbol cannot replace compiler ownership", door: "NewHeaderValue"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IngressSymbolForTest(tc.door); got != tc.want {
				t.Fatalf("compiler binding=%q,want %q", got, tc.want)
			}
		})
	}
}
