package gotoolchain

import (
	"context"
	"errors"
	"go/types"
	"reflect"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// TypeRelation selects one native Go type relation; products own its meaning.
type TypeRelation uint8

const (
	TypeRelationUnknown TypeRelation = iota
	TypeRelationIdentical
	TypeRelationAssignable
	TypeRelationImplements
)

func (r TypeRelation) Validate() error {
	if r < TypeRelationIdentical || r > TypeRelationImplements {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (r TypeRelation) IsValid() bool { return r.Validate() == nil }
func (TypeRelation) OffWireEnum()    {}
func (r TypeRelation) String() string {
	switch r {
	case TypeRelationIdentical:
		return "identical"
	case TypeRelationAssignable:
		return "assignable"
	case TypeRelationImplements:
		return "implements"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// TypeRelationRequest borrows completed native types. Right must have an
// interface underlying type for Implements; other relations admit every type.
type TypeRelationRequest struct {
	Left     types.Type
	Right    types.Type
	Relation TypeRelation
}

func (r TypeRelationRequest) Validate() error {
	if err := errors.Join(validateBorrowedType(r.Left), validateBorrowedType(r.Right), r.Relation.Validate()); err != nil {
		return err
	}
	if r.Relation == TypeRelationImplements {
		if target, ok := r.Right.Underlying().(*types.Interface); !ok || target == nil {
			return core.ErrGoToolchainContract
		}
	}
	return nil
}
func (TypeRelationRequest) goToolchainInternalFlow() {}

// ObserveTypeRelation delegates identity, assignment or implementation to Go.
// Refusal returns no positive fact and preserves the caller's context cause.
func ObserveTypeRelation(ctx context.Context, request TypeRelationRequest) (bool, error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), request.Validate()); err != nil {
		return false, err
	}
	var matches bool
	switch request.Relation {
	case TypeRelationIdentical:
		matches = types.Identical(request.Left, request.Right)
	case TypeRelationAssignable:
		matches = types.AssignableTo(request.Left, request.Right)
	case TypeRelationImplements:
		target, ok := request.Right.Underlying().(*types.Interface)
		if !ok || target == nil {
			return false, core.ErrGoToolchainContract
		}
		matches = types.Implements(request.Left, target)
	default:
		return false, core.ErrGoToolchainContract
	}
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return false, err
	}
	return matches, nil
}

// TypeSelectionRequest borrows Go's receiver and package-visibility context.
// Addressable has Go's exact meaning for implicit pointer method selection.
type TypeSelectionRequest struct {
	Type        types.Type
	Package     *types.Package
	Name        capabilities.SymbolName
	Addressable bool
}

func (r TypeSelectionRequest) Validate() error {
	if err := errors.Join(validateBorrowedType(r.Type), r.Name.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	return nil
}
func (TypeSelectionRequest) goToolchainInternalFlow() {}

// TypeSelectionObservation contains Go's selected field/method, or absence.
// Index and compiler objects remain borrowed; callers must not mutate them.
type TypeSelectionObservation struct {
	Selection types.Selection
	Present   bool
}

func (o TypeSelectionObservation) Validate() error {
	if o.Present != (o.Selection.Obj() != nil) {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (TypeSelectionObservation) goToolchainProtocolFact() {}

// ObserveTypeSelection delegates field/method promotion and visibility to Go.
// Ambiguity and unavailable pointer methods are absence, not invented targets.
func ObserveTypeSelection(ctx context.Context, request TypeSelectionRequest) (TypeSelectionObservation, error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), request.Validate()); err != nil {
		return TypeSelectionObservation{}, err
	}
	selection, present := types.LookupSelection(request.Type, request.Addressable, request.Package, request.Name.String())
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return TypeSelectionObservation{}, err
	}
	observation := TypeSelectionObservation{Selection: selection, Present: present}
	if err := observation.Validate(); err != nil {
		return TypeSelectionObservation{}, err
	}
	return observation, nil
}

func validateBorrowedType(value types.Type) error {
	switch value.(type) {
	case *types.Basic, *types.Array, *types.Slice, *types.Struct, *types.Pointer,
		*types.Tuple, *types.Signature, *types.Interface, *types.Map, *types.Chan,
		*types.Named, *types.TypeParam, *types.Union, *types.Alias:
		if !reflect.ValueOf(value).IsNil() {
			return nil
		}
	}
	return core.ErrGoToolchainContract
}

var (
	_ core.Validatable = TypeRelation(0)
	_ core.Validatable = TypeRelationRequest{}
	_ core.Validatable = TypeSelectionRequest{}
	_ core.Validatable = TypeSelectionObservation{}
)
