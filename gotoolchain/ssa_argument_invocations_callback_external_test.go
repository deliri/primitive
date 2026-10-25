package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSAArgumentInvocationsRejectsFunctionAsCallbackMethod(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), argumentInvocationDeclarations+`func Plain(callback func(*Runner)){};func Entry(a,b *Runner){a.Run("child",func(x *Runner){use(x)})}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		selection, ok := types.LookupSelection(fn.Params[0].Type(), false, nil, "Run")
		if !ok || len(fn.AnonFuncs) != 1 || len(fn.AnonFuncs[0].Params) != 1 {
			t.Fatalf("callback fixture got method=%t children=%v want Run and one callback parameter", ok, fn.AnonFuncs)
		}
		method := selection.Obj().(*types.Func)
		plain := v.Package.Func("Plain").Object().(*types.Func)
		if method.Signature().Recv() == nil || plain.Signature().Recv() != nil || !types.Identical(method.Signature().Params().At(1).Type(), plain.Signature().Params().At(0).Type()) {
			t.Fatalf("method/ordinary signatures got=%v / %v want receiver presence to differ with identical callback type", method.Signature(), plain.Signature())
		}
		request := gotoolchain.SSAArgumentInvocationsRequest{Function: fn, Argument: fn.Params[0], CallbackMethod: method}
		loans := 0
		for frame, queryErr := range v.ArgumentInvocations(ctx, request) {
			if queryErr != nil {
				return queryErr
			}
			if frame == nil {
				t.Fatalf("native method frame got=%v want non-nil", frame)
			}
			switch loans {
			case 0:
				if frame.Kind() != gotoolchain.SSAInvocationKindRoot || frame.Binding().Function != fn || frame.Binding().Local != fn.Params[0] {
					t.Fatalf("native method root got=%+v want function=%v local=%v kind=root", frame, fn, fn.Params[0])
				}
			case 1:
				if frame.Kind() != gotoolchain.SSAInvocationKindCallback || frame.Binding().Function != fn.AnonFuncs[0] || frame.Binding().Local != fn.AnonFuncs[0].Params[0] || frame.Caller() == nil || frame.Caller().Binding().Function != fn || frame.Call() == nil || frame.Call().Parent() != fn {
					t.Fatalf("callback got=%+v want function=%v parameter=%v caller=%v and non-nil call", frame, fn.AnonFuncs[0], fn.AnonFuncs[0].Params[0], fn)
				}
			default:
				t.Fatalf("native method frame index got%d want below2", loans)
			}
			loans++
		}
		if loans != 2 {
			t.Fatalf("native method loans=%d want2", loans)
		}
		request.CallbackMethod = plain
		if err := request.Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
			t.Fatalf("plain function callback request=%v want typed contract refusal", err)
		}
		root, queryErr := v.ArgumentInvocationRoot(ctx, request)
		if root != nil || !errors.Is(queryErr, core.ErrGoToolchainContract) {
			t.Fatalf("plain function root=%v,%v want nil and typed contract refusal", root, queryErr)
		}
		refusals := 0
		for frame, queryErr := range v.ArgumentInvocations(ctx, request) {
			if frame != nil || !errors.Is(queryErr, core.ErrGoToolchainContract) {
				t.Fatalf("plain function invocation=%v,%v want nil and typed contract refusal", frame, queryErr)
			}
			refusals++
		}
		if refusals != 1 {
			t.Fatalf("plain function refusals=%d want1", refusals)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
