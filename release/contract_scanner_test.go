package release

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

func TestStructInventoryMatcherCannotHideCarriersBehindSyntax(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		source string
		want   []string
	}{
		{name: "direct declaration is inventoried", source: "type Direct struct{}", want: []string{"Direct"}},
		{name: "defined projection cannot disappear", source: "type Origin struct{}; type Projection Origin", want: []string{"Origin", "Projection"}},
		{name: "forward projection resolves its owner", source: "type Projection Origin; type Origin struct{}", want: []string{"Origin", "Projection"}},
		{name: "alias cannot hide a carrier", source: "type Origin struct{}; type Alias = Origin", want: []string{"Alias", "Origin"}},
		{name: "generic instantiation retains its shape", source: "type Generic[T any] struct{ Value T }; type Bound Generic[int]", want: []string{"Bound", "Generic"}},
		{name: "multiple type arguments retain shape", source: "type Generic[A, B any] struct{ Left A; Right B }; type Bound Generic[int, string]", want: []string{"Bound", "Generic"}},
		{name: "parenthesized shape cannot disappear", source: "type Grouped (struct{})", want: []string{"Grouped"}},
		{name: "local literal cannot impersonate a global inventory slot", source: "func f() { type local struct{} }", want: []string{"fixture.go:2:17 local local"}},
		{name: "local projection remains visible", source: "type Origin struct{}; func f() { type local Origin }", want: []string{"Origin", "fixture.go:2:39 local local"}},
		{name: "anonymous value cannot bypass naming", source: "var value = struct{}{}", want: []string{"fixture.go:2:13 anonymous struct"}},
		{name: "nested anonymous field is independently visible", source: "type Outer struct { Inner struct{} }", want: []string{"Outer", "fixture.go:2:27 anonymous struct"}},
		{name: "anonymous argument is a carrier", source: "func f(value struct{}) {}", want: []string{"fixture.go:2:14 anonymous struct"}},
		{name: "scalar and interface declarations do not invent structs", source: "type Scalar string; type Capability interface{ Read() }; func f() { type local int }"},
		{name: "cyclic invalid source cannot spin the matcher", source: "type A B; type B A"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			set := token.NewFileSet()
			file, err := parser.ParseFile(set, "fixture.go", "package fixture\n"+tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("ParseFile() error = %v, want nil", err)
			}
			got := releaseStructNames(set, []*ast.File{file})
			if !slices.Equal(got, tc.want) {
				t.Fatalf("struct inventory = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestJSONIngressMatcherCannotLoseReceiverForms(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		source string
		want   []string
	}{
		{name: "pointer decoder is external", source: "func (*Value) UnmarshalJSON([]byte) error { return nil }", want: []string{"Value"}},
		{name: "value decoder cannot evade inventory", source: "func (Value) UnmarshalJSON([]byte) error { return nil }", want: []string{"Value"}},
		{name: "generic pointer decoder remains external", source: "func (*Value[T]) UnmarshalJSON([]byte) error { return nil }", want: []string{"Value"}},
		{name: "generic value decoder remains external", source: "func (Value[T, U]) UnmarshalJSON([]byte) error { return nil }", want: []string{"Value"}},
		{name: "parentheses cannot hide a decoder", source: "func ((Value)) UnmarshalJSON([]byte) error { return nil }", want: []string{"Value"}},
		{name: "private receiver does not invent a public door", source: "func (*private) UnmarshalJSON([]byte) error { return nil }"},
		{name: "encoder does not impersonate ingress", source: "func (Value) MarshalJSON() ([]byte, error) { return nil, nil }"},
		{name: "top level namesake is not a decoder method", source: "func UnmarshalJSON([]byte) error { return nil }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\n"+tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("ParseFile() error = %v, want nil", err)
			}
			if got := releaseJSONReceiverNames(file); !slices.Equal(got, tc.want) {
				t.Fatalf("JSON ingress receivers = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSelectionSourceMatcherRejectsLookalikeDeclarations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source    string
		fields          []string
		embedded, found bool
	}{
		{name: "direct call in the governed function", source: "type Intent struct { Value int }; func Execute() { EmbeddedBuildIdentity() }", fields: []string{"Value"}, embedded: true, found: true},
		{name: "parenthesized direct call remains visible", source: "type Intent struct{}; func Execute() { (EmbeddedBuildIdentity)() }", embedded: true, found: true},
		{name: "other function cannot supply a missing governed call", source: "type Intent struct{}; func Execute() {}; func Other() { EmbeddedBuildIdentity() }", found: true},
		{name: "method cannot masquerade as a package function", source: "type Intent struct{}; func (Intent) Execute() { EmbeddedBuildIdentity() }"},
		{name: "missing request cannot borrow an unrelated struct", source: "type Other struct{}; func Execute() {}"},
		{name: "nonstruct request cannot masquerade as an empty struct", source: "type Intent int; func Execute() {}"},
		{name: "missing function cannot become neutral success", source: "type Intent struct{}"},
		{name: "grouped fields cannot hide a second field", source: "type Intent struct { First, Second int }; func Execute() {}", fields: []string{"First", "Second"}, found: true},
		{name: "embedded field cannot disappear from the shape", source: "type Intent struct { Hidden }; func Execute() {}", fields: []string{""}, found: true},
		{name: "declaration without a body cannot supply execution", source: "type Intent struct{}; func Execute()"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture; "+tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("parse fixture error = %v, want nil", err)
			}
			fields, embedded, found := releaseSelectionShape(file, "Intent", "Execute")
			if !slices.Equal(fields, tc.fields) || embedded != tc.embedded || found != tc.found {
				t.Fatalf("selection shape = (%v, %t, %t), want (%v, %t, %t)", fields, embedded, found, tc.fields, tc.embedded, tc.found)
			}
		})
	}
}
