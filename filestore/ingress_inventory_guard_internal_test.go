package filestore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"
)

func TestIngressProjectionExemptionRequiresExactOwningSignature(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		want         bool
	}{
		{name: "owning validator consumes only its receiver", source: `func(T) Validate() error{return nil}`, want: true},
		{name: "validator with external bytes must stay inventoried", source: `func(T) Validate([]byte) error{return nil}`},
		{name: "validator cannot hide a material result", source: `func(T) Validate() string{return ""}`},
		{name: "diagnostic projection admits no input", source: `func(T) String() string{return ""}`, want: true},
		{name: "String with input is not a projection", source: `func(T) String(string) string{return ""}`},
		{name: "String cannot hide a byte decoder by name", source: `func(T) String() []byte{return nil}`},
		{name: "validity predicate has exact bool result", source: `func(T) IsValid() bool{return false}`, want: true},
		{name: "validity with a new input stays visible", source: `func(T) IsValid(bool) bool{return false}`},
		{name: "off wire marker admits no representation", source: `func(T) OffWireEnum(){}`, want: true},
		{name: "off wire spelling cannot hide an input", source: `func(T) OffWireEnum(string){}`},
		{name: "off wire spelling cannot hide a result", source: `func(T) OffWireEnum() string{return ""}`},
		{name: "new decoder always requires a semantic target", source: `func(T) UnmarshalText([]byte) error{return nil}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture;"+tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			fn := file.Decls[0].(*ast.FuncDecl)
			if got := filestoreOwningProjection(fn); got != tc.want {
				t.Fatalf("projection exemption = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestIngressBindingRejectsNamesAndForeignFunctions(t *testing.T) {
	t.Parallel()
	var absent func()
	for _, tc := range []struct {
		name  string
		input reflect.Value
		want  string
	}{
		{name: "real free function keeps its compiler identity", input: reflect.ValueOf(NewDirectoryEntryMaximum), want: "NewDirectoryEntryMaximum"},
		{name: "pointer method keeps its receiver identity", input: reflect.ValueOf((*HeldDirectory).Close), want: "HeldDirectory.Close"},
		{name: "value method keeps its receiver identity", input: reflect.ValueOf(StagedFile.Path), want: "StagedFile.Path"},
		{name: "foreign Go function cannot impersonate a package door", input: reflect.ValueOf(os.Open)},
		{name: "missing reflection value cannot bind a door"},
		{name: "typed nil function cannot bind a door", input: reflect.ValueOf(absent)},
		{name: "raw function spelling is not a compiler binding", input: reflect.ValueOf("NewDirectoryEntryMaximum")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := FilestoreIngressSymbolForTest(tc.input); got != tc.want {
				t.Fatalf("binding = %q, want %q", got, tc.want)
			}
		})
	}
}
