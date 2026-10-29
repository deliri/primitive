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

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func nativeTypeQueryFixture(t *testing.T) *types.Package {
	t.Helper()
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, "queries.go", `package fixture
 type Scalar int
 func (Scalar) Value() {}
 func (*Scalar) Pointer() {}
 type Other int
 type Alias = Scalar
 type Contract interface { Value() }
 type PointerContract interface { Pointer() }
 type Promoted struct { Scalar }
 type Left struct { Scalar }
 type Right struct { Scalar }
 type Ambiguous struct { Left; Right }
 type Shadow struct { Scalar; Value int }
 type Private int
 func (Private) hidden() {}
 var _ Contract = Scalar(0)
 var _ PointerContract = (*Scalar)(nil)
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	checker := types.Config{}
	pkg, err := checker.Check("example.invalid/nativequeries", positions, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return pkg
}

func TestTypeRelationsPreserveGoIdentityAssignmentAndInterfaces(t *testing.T) {
	t.Parallel()
	pkg := nativeTypeQueryFixture(t)
	scalar := pkg.Scope().Lookup("Scalar").Type()
	other := pkg.Scope().Lookup("Other").Type()
	alias := pkg.Scope().Lookup("Alias").Type()
	contract := pkg.Scope().Lookup("Contract").Type()
	pointerContract := pkg.Scope().Lookup("PointerContract").Type()
	for _, tc := range []struct {
		name        string
		left, right types.Type
		relation    gotoolchain.TypeRelation
		want        bool
	}{
		{name: "same basic identity", left: types.Typ[types.Int], right: types.Typ[types.Int], relation: gotoolchain.TypeRelationIdentical, want: true},
		{name: "different basic identity", left: types.Typ[types.Int], right: types.Typ[types.String], relation: gotoolchain.TypeRelationIdentical},
		{name: "alias retains identity", left: scalar, right: alias, relation: gotoolchain.TypeRelationIdentical, want: true},
		{name: "named backing does not erase identity", left: scalar, right: other, relation: gotoolchain.TypeRelationIdentical},
		{name: "separate native pointer objects share type identity", left: types.NewPointer(scalar), right: types.NewPointer(scalar), relation: gotoolchain.TypeRelationIdentical, want: true},
		{name: "native accepted interface assignment", left: scalar, right: contract, relation: gotoolchain.TypeRelationAssignable, want: true},
		{name: "interface assignment is directional", left: contract, right: scalar, relation: gotoolchain.TypeRelationAssignable},
		{name: "different named backing cannot assign", left: scalar, right: other, relation: gotoolchain.TypeRelationAssignable},
		{name: "alias assignment", left: alias, right: scalar, relation: gotoolchain.TypeRelationAssignable, want: true},
		{name: "pointer implements its method contract", left: types.NewPointer(scalar), right: pointerContract, relation: gotoolchain.TypeRelationImplements, want: true},
		{name: "value lacks pointer method", left: scalar, right: pointerContract, relation: gotoolchain.TypeRelationImplements},
		{name: "value implements declared contract", left: scalar, right: contract, relation: gotoolchain.TypeRelationImplements, want: true},
		{name: "unrelated native named type lacks method", left: other, right: contract, relation: gotoolchain.TypeRelationImplements},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := gotoolchain.ObserveTypeRelation(t.Context(), gotoolchain.TypeRelationRequest{Left: tc.left, Right: tc.right, Relation: tc.relation})
			if err != nil || got != tc.want {
				t.Fatalf("relation=%t error=%v, want %t and nil", got, err, tc.want)
			}
		})
	}
}

func TestTypeSelectionsPreserveAddressabilityPromotionFieldsAndVisibility(t *testing.T) {
	t.Parallel()
	pkg := nativeTypeQueryFixture(t)
	foreign := types.NewPackage("example.invalid/foreign", "foreign")
	for _, tc := range []struct {
		name, receiver, member     string
		scope                      *types.Package
		addressable, pointer, want bool
		kind                       types.SelectionKind
		index                      []int
	}{
		{name: "value method", receiver: "Scalar", member: "Value", want: true, kind: types.MethodVal, index: []int{0}},
		{name: "nonaddressable value cannot borrow pointer method", receiver: "Scalar", member: "Pointer"},
		{name: "addressable value borrows pointer method", receiver: "Scalar", member: "Pointer", addressable: true, want: true, kind: types.MethodVal, index: []int{1}},
		{name: "pointer owns pointer method", receiver: "Scalar", member: "Pointer", pointer: true, want: true, kind: types.MethodVal, index: []int{1}},
		{name: "embedded method retains native path", receiver: "Promoted", member: "Value", want: true, kind: types.MethodVal, index: []int{0, 0}},
		{name: "ambiguous embedding supplies no selection", receiver: "Ambiguous", member: "Value"},
		{name: "field shadows embedded method", receiver: "Shadow", member: "Value", want: true, kind: types.FieldVal, index: []int{1}},
		{name: "interface member", receiver: "Contract", member: "Value", want: true, kind: types.MethodVal, index: []int{0}},
		{name: "pointer to interface supplies no member", receiver: "Contract", member: "Value", pointer: true},
		{name: "same package private method", receiver: "Private", member: "hidden", scope: pkg, want: true, kind: types.MethodVal, index: []int{0}},
		{name: "foreign package cannot select private method", receiver: "Private", member: "hidden", scope: foreign},
		{name: "absent member remains absent", receiver: "Scalar", member: "Missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			name, err := capabilities.ParseSymbolName(tc.member)
			if err != nil {
				t.Fatal(err)
			}
			receiver := pkg.Scope().Lookup(tc.receiver).Type()
			if tc.pointer {
				receiver = types.NewPointer(receiver)
			}
			got, err := gotoolchain.ObserveTypeSelection(t.Context(), gotoolchain.TypeSelectionRequest{Type: receiver, Package: tc.scope, Name: name, Addressable: tc.addressable})
			if err != nil || got.Validate() != nil || got.Present != tc.want {
				t.Fatalf("selection=%+v error=%v, want presence=%t and valid native result", got, err, tc.want)
			}
			if !tc.want {
				if got.Selection.Obj() != nil {
					t.Fatalf("absent selection object=%v, want nil", got.Selection.Obj())
				}
				return
			}
			if got.Selection.Obj().Name() != tc.member || got.Selection.Recv() != receiver || got.Selection.Kind() != tc.kind || !slices.Equal(got.Selection.Index(), tc.index) {
				t.Fatalf("selection=%v kind=%v index=%v, want member=%s receiver=%v kind=%v index=%v", got.Selection, got.Selection.Kind(), got.Selection.Index(), tc.member, receiver, tc.kind, tc.index)
			}
		})
	}
}

func TestTypeQueriesRefuseInvalidAgreementAndContextWithoutFacts(t *testing.T) {
	t.Parallel()
	name, err := capabilities.ParseSymbolName("Missing")
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancelCause(t.Context())
	cancel(core.ErrGoToolchainOutput)
	var absent *types.Named
	for _, tc := range []struct {
		name      string
		ctx       context.Context
		relation  gotoolchain.TypeRelationRequest
		selection gotoolchain.TypeSelectionRequest
		want      error
	}{
		{name: "missing native operands", ctx: t.Context(), want: core.ErrGoToolchainContract},
		{name: "typed nil native type", ctx: t.Context(), relation: gotoolchain.TypeRelationRequest{Left: absent, Right: types.Typ[types.Int], Relation: gotoolchain.TypeRelationIdentical}, selection: gotoolchain.TypeSelectionRequest{Type: absent, Name: name}, want: core.ErrGoToolchainContract},
		{name: "absent operation and selector", ctx: t.Context(), relation: gotoolchain.TypeRelationRequest{Left: types.Typ[types.Int], Right: types.Typ[types.Int]}, selection: gotoolchain.TypeSelectionRequest{Type: types.Typ[types.Int]}, want: core.ErrGoToolchainContract},
		{name: "absent query context", relation: gotoolchain.TypeRelationRequest{Left: types.Typ[types.Int], Right: types.Typ[types.Int], Relation: gotoolchain.TypeRelationIdentical}, selection: gotoolchain.TypeSelectionRequest{Type: types.Typ[types.Int], Name: name}, want: core.ErrNilContext},
		{name: "cancelled query context retains cause", ctx: cancelled, relation: gotoolchain.TypeRelationRequest{Left: types.Typ[types.Int], Right: types.Typ[types.Int], Relation: gotoolchain.TypeRelationIdentical}, selection: gotoolchain.TypeSelectionRequest{Type: types.Typ[types.Int], Name: name}, want: core.ErrGoToolchainOutput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			matched, relationErr := gotoolchain.ObserveTypeRelation(tc.ctx, tc.relation)
			selected, selectionErr := gotoolchain.ObserveTypeSelection(tc.ctx, tc.selection)
			if matched || !errors.Is(relationErr, tc.want) || selected.Present || selected.Selection.Obj() != nil || !errors.Is(selectionErr, tc.want) {
				t.Fatalf("relations=%t/%v selection=%+v/%v, want absent facts and %v", matched, relationErr, selected, selectionErr, tc.want)
			}
		})
	}
	matched, err := gotoolchain.ObserveTypeRelation(t.Context(), gotoolchain.TypeRelationRequest{Left: types.Typ[types.Int], Right: types.Typ[types.Int], Relation: gotoolchain.TypeRelationImplements})
	if matched || !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("noninterface implementation=%t/%v, want no fact and native contract refusal", matched, err)
	}
}

func TestTypeRelationDomainIsClosedAcrossEveryByte(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		relation := gotoolchain.TypeRelation(value)
		want := value >= int(gotoolchain.TypeRelationIdentical) && value <= int(gotoolchain.TypeRelationImplements)
		if relation.IsValid() != want || (relation.Validate() == nil) != want || relation.String() == "" {
			t.Fatalf("relation=%d valid=%t error=%v text=%q, want admitted=%t and diagnostic text", value, relation.IsValid(), relation.Validate(), relation.String(), want)
		}
		relation.OffWireEnum()
	}
}
