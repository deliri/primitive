package gotoolchain_test

import (
	"context"
	"errors"
	"go/token"
	"go/types"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAArgumentInvocationRootLayerTriadAndRefusals(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*gotoolchain.SSAUnitView, *gotoolchain.SSAArgumentInvocationsRequest)
		want   error
	}{
		{"missing function", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) { r.Function = nil }, core.ErrGoToolchainContract},
		{"missing argument", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) { r.Argument = nil }, core.ErrGoToolchainContract},
		{"typed nil argument", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) {
			r.Argument = (*ssa.Parameter)(nil)
		}, core.ErrGoToolchainContract},
		{"copied argument", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) {
			copied := *r.Function.Params[0]
			r.Argument = &copied
		}, core.ErrGoToolchainContract},
		{"foreign argument", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) {
			r.Argument = v.Package.Func("Foreign").Params[0]
		}, core.ErrGoToolchainContract},
		{"missing program", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAArgumentInvocationsRequest) { v.Program = nil }, core.ErrGoToolchainContract},
		{"missing package", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAArgumentInvocationsRequest) { v.Package = nil }, core.ErrGoToolchainContract},
		{"missing source", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAArgumentInvocationsRequest) { v.Source = nil }, core.ErrGoToolchainContract},
		{"detached function program", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) { r.Function.Prog = nil }, core.ErrGoToolchainContract},
		{"detached function package", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) { r.Function.Pkg = nil }, core.ErrGoToolchainContract},
		{"missing signature", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) {
			r.Function.Signature = nil
		}, core.ErrGoToolchainOutput},
		{"missing entry block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) {
			r.Function.Blocks[0] = nil
		}, core.ErrGoToolchainOutput},
		{"wrong entry ordinal", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) {
			r.Function.Blocks[0].Index = 9
		}, core.ErrGoToolchainOutput},
		{"foreign entry block", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) {
			r.Function.Blocks[0] = v.Package.Func("Foreign").Blocks[0]
		}, core.ErrGoToolchainOutput},
		{"slot without callback", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) {
			r.CallbackParameter = 1
		}, core.ErrGoToolchainContract},
		{"callback slot outside signature", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) {
			selection, _ := types.LookupSelection(r.Argument.Type(), false, nil, "Run")
			r.CallbackMethod = selection.Obj().(*types.Func)
			r.CallbackParameter = 1
		}, core.ErrGoToolchainContract},
		{"callback without function argument", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) {
			r.CallbackMethod = v.Package.Func("helper").Object().(*types.Func)
		}, core.ErrGoToolchainContract},
		{"callback without arguments", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) {
			r.CallbackMethod = v.Package.Func("NoArgs").Object().(*types.Func)
		}, core.ErrGoToolchainContract},
		{"foreign callback object", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAArgumentInvocationsRequest) {
			selection, _ := types.LookupSelection(r.Argument.Type(), false, nil, "Run")
			native := selection.Obj().(*types.Func)
			r.CallbackMethod = types.NewFunc(token.NoPos, types.NewPackage("foreign", "foreign"), "Run", native.Signature())
		}, core.ErrGoToolchainContract},
		{"canceled root", func(_ *gotoolchain.SSAUnitView, _ *gotoolchain.SSAArgumentInvocationsRequest) {}, io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), argumentInvocationDeclarations+`func NoArgs(){};func Entry(a,b *Runner){helper(a)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				request := gotoolchain.SSAArgumentInvocationsRequest{Function: fn, Argument: fn.Params[0]}
				root, queryErr := v.ArgumentInvocationRoot(ctx, request)
				if queryErr != nil || root == nil || root.Binding().Function != fn || root.Binding().Local != fn.Params[0] || root.Kind() != gotoolchain.SSAInvocationKindRoot || root.Caller() != nil || root.Call() != nil {
					t.Fatalf("root loan=%v,%v lost exact selected native identity or invented an invocation", root, queryErr)
				}
				row.mutate(&v, &request)
				if row.name == "canceled root" {
					canceled, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					ctx = canceled
				}
				refused, queryErr := v.ArgumentInvocationRoot(ctx, request)
				if refused != nil || !errors.Is(queryErr, row.want) {
					t.Fatalf("root refusal=%v,%v want nil,%v", refused, queryErr, row.want)
				}
				if row.name == "canceled root" && !errors.Is(queryErr, context.Canceled) {
					t.Fatalf("root lost standard cancellation identity: %v", queryErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
