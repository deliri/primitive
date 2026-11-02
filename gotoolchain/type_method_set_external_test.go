package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestTypeMethodSetUsesNativePromotionAndReceiverRules(t *testing.T) {
	t.Parallel()
	positions := token.NewFileSet()
	syntax, err := parser.ParseFile(positions, "methods.go", `package fixture
 type Scalar int
 func (Scalar) Value() {}
 func (*Scalar) Pointer() {}
 type Promoted struct { Scalar }
 type Left struct { Scalar }
 type Right struct { Scalar }
 type Ambiguous struct { Left; Right }
 type Shadow struct { Scalar; Value int }
 type Contract interface { Value() }
 type Alias = Scalar
 type NamedPointer *Scalar
 type Generic[T any] struct { Item T }
 func (Generic[T]) Value() {}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	checker := types.Config{}
	pkg, err := checker.Check("example.invalid/methods", positions, []*ast.File{syntax}, nil)
	if err != nil {
		t.Fatal(err)
	}
	scalar := pkg.Scope().Lookup("Scalar").Type()
	generic, err := types.Instantiate(nil, pkg.Scope().Lookup("Generic").Type(), []types.Type{types.Typ[types.String]}, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		kind types.Type
		want []string
	}{
		{name: "basic type has no methods", kind: types.Typ[types.Int]},
		{name: "value excludes pointer receiver", kind: scalar, want: []string{"Value"}},
		{name: "pointer includes both receivers", kind: types.NewPointer(scalar), want: []string{"Pointer", "Value"}},
		{name: "embedded value promotes value method", kind: pkg.Scope().Lookup("Promoted").Type(), want: []string{"Value"}},
		{name: "embedded pointer promotion preserves pointer method", kind: types.NewPointer(pkg.Scope().Lookup("Promoted").Type()), want: []string{"Pointer", "Value"}},
		{name: "equal depth ambiguity supplies no method", kind: pkg.Scope().Lookup("Ambiguous").Type()},
		{name: "field shadows promoted method", kind: pkg.Scope().Lookup("Shadow").Type()},
		{name: "interface owns its declared method", kind: pkg.Scope().Lookup("Contract").Type(), want: []string{"Value"}},
		{name: "pointer to interface has no method", kind: types.NewPointer(pkg.Scope().Lookup("Contract").Type())},
		{name: "alias preserves receiver facts", kind: pkg.Scope().Lookup("Alias").Type(), want: []string{"Value"}},
		{name: "named pointer has no method set", kind: pkg.Scope().Lookup("NamedPointer").Type()},
		{name: "instantiated generic retains method", kind: generic, want: []string{"Value"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			methods, err := gotoolchain.ObserveTypeMethodSet(t.Context(), gotoolchain.TypeMethodSetRequest{Type: tc.kind})
			if err != nil || methods == nil {
				t.Fatalf("method set=%v error=%v, want native set", methods, err)
			}
			var got []string
			for index := range methods.Len() {
				selection := methods.At(index)
				if selection.Obj() == nil || selection.Recv() != tc.kind {
					t.Fatalf("selection=%v, want original borrowed receiver %v", selection, tc.kind)
				}
				got = append(got, selection.Obj().Name())
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("methods=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestTypeMethodSetRefusesInvalidTypesAndContextsWithoutFacts(t *testing.T) {
	t.Parallel()
	cancelled, cancel := context.WithCancelCause(t.Context())
	cancel(core.ErrGoToolchainOutput)
	var absentNamed *types.Named
	for _, tc := range []struct {
		name string
		ctx  context.Context
		kind types.Type
		want error
	}{
		{name: "absent type", ctx: t.Context(), want: core.ErrGoToolchainContract},
		{name: "typed nil native type", ctx: t.Context(), kind: absentNamed, want: core.ErrGoToolchainContract},
		{name: "absent context", kind: types.Typ[types.Int], want: core.ErrNilContext},
		{name: "cancelled context preserves owner cause", ctx: cancelled, kind: types.Typ[types.Int], want: core.ErrGoToolchainOutput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := gotoolchain.ObserveTypeMethodSet(tc.ctx, gotoolchain.TypeMethodSetRequest{Type: tc.kind})
			if got != nil || !errors.Is(err, tc.want) {
				t.Fatalf("method set=%v error=%v, want no facts and %v", got, err, tc.want)
			}
		})
	}
}
