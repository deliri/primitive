package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const nativeGenericMethodSource = `package sample
type Box[T any] struct{}
func (Box[T]) Check() error { return nil }
func Identity[T any](value T) T { return value }
func Concrete(b Box[int]) { _ = b.Check() }
func Parametric[T any](b Box[T]) { _ = b.Check() }
func Ordinary() { _ = Identity[int](1) }
`

func TestSSACallableCustodyAcceptsNativeGenericInstances(t *testing.T) {
	t.Parallel()
	for _, name := range [...]string{"Concrete", "Parametric", "Ordinary"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeGenericMethodSource)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				call := nativeArgumentBindingLastCall(t, view.Package.Func(name))
				target := call.Common().StaticCallee()
				if target == nil || target.Origin() == nil || target.Origin() == target || len(target.TypeArgs()) == 0 || len(target.Blocks) == 0 || target.Blocks[0].Parent() != target {
					t.Fatalf("native %s target=%v, want a native instantiated body with its own block backlink", name, target)
				}
				object, ok := target.Object().(*types.Func)
				if !ok {
					t.Fatalf("native %s object=%T, want *types.Func", name, target.Object())
				}
				got, queryErr := view.CallFunction(ctx, gotoolchain.SSACallFunctionRequest{Call: call})
				if queryErr != nil || got.Function != object {
					t.Fatalf("native %s callable=(%+v,%v), want exact object %v and nil", name, got, queryErr, object)
				}
				receiver, queryErr := view.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
				if queryErr != nil {
					t.Fatalf("native %s receiver error=%v, want nil", name, queryErr)
				}
				if name == "Ordinary" {
					if receiver != (gotoolchain.SSAMethodReceiverObservation{}) {
						t.Fatalf("ordinary generic receiver=%+v, want absent", receiver)
					}
				} else if receiver.Method != object || receiver.Receiver != call.Common().Args[0] {
					t.Fatalf("generic receiver=%+v, want exact method %v and native operand %v", receiver, object, call.Common().Args[0])
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("native generic unit=%v, want nil", err)
			}
		})
	}
}

func TestSSACallableCustodyRejectsCopiedGenericInstances(t *testing.T) {
	t.Parallel()
	for _, name := range [...]string{"Concrete", "Parametric", "Ordinary"} {
		for _, damage := range []struct {
			name   string
			mutate func(*ssa.Function)
		}{
			{name: "copied function"},
			{name: "copied first block", mutate: func(fn *ssa.Function) { block := *fn.Blocks[0]; fn.Blocks = []*ssa.BasicBlock{&block} }},
			{name: "absent instance body", mutate: func(fn *ssa.Function) { fn.Blocks = nil }},
		} {
			t.Run(name+"/"+damage.name, func(t *testing.T) {
				t.Parallel()
				analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeGenericMethodSource)
				err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
					call := nativeArgumentBindingLastCall(t, view.Package.Func(name))
					original := call.Common().StaticCallee()
					if original == nil || original.Origin() == nil || original.Origin() == original || len(original.Blocks) == 0 {
						t.Fatalf("native %s original=%v, want instantiated source body", name, original)
					}
					baseline, queryErr := view.CallFunction(ctx, gotoolchain.SSACallFunctionRequest{Call: call})
					if queryErr != nil || baseline.Function != original.Object() {
						t.Fatalf("native %s baseline=(%+v,%v), want original object before damage", name, baseline, queryErr)
					}
					copied := *original
					if damage.mutate != nil {
						damage.mutate(&copied)
					}
					call.Common().Value = &copied
					got, queryErr := view.CallFunction(ctx, gotoolchain.SSACallFunctionRequest{Call: call})
					if got != (gotoolchain.SSACallFunctionObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
						t.Fatalf("copied %s/%s callable=(%+v,%v), want zero and %v", name, damage.name, got, queryErr, core.ErrGoToolchainOutput)
					}
					receiver, queryErr := view.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
					if receiver != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
						t.Fatalf("copied %s/%s receiver=(%+v,%v), want zero and %v", name, damage.name, receiver, queryErr, core.ErrGoToolchainOutput)
					}
					return nil
				}})
				if err != nil {
					t.Fatalf("native generic unit=%v, want nil", err)
				}
			})
		}
	}
}

func TestSSACallableCustodyRejectsForeignGenericInstances(t *testing.T) {
	t.Parallel()
	for _, name := range [...]string{"Concrete", "Ordinary"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeGenericMethodSource)
			unit := analysis.Units[0]
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				call := nativeArgumentBindingLastCall(t, view.Package.Func(name))
				original := call.Common().StaticCallee()
				baseline, queryErr := view.CallFunction(ctx, gotoolchain.SSACallFunctionRequest{Call: call})
				if original == nil || original.Origin() == nil || queryErr != nil || baseline.Function != original.Object() {
					t.Fatalf("native %s baseline=(%+v,%v), want original object before foreign replacement", name, baseline, queryErr)
				}
				return analysis.WithSSAUnit(ctx, gotoolchain.SSAUnitRequest{Unit: unit, Use: func(ctx context.Context, foreign gotoolchain.SSAUnitView) error {
					foreignCall := nativeArgumentBindingLastCall(t, foreign.Package.Func(name))
					target := foreignCall.Common().StaticCallee()
					if target == nil || target == original || target.Prog == view.Program || target.Origin() == nil || target.Origin().Object() != original.Origin().Object() {
						t.Fatalf("foreign %s target=%v, want a distinct native program with the same declaration object", name, target)
					}
					call.Common().Value = target
					got, queryErr := view.CallFunction(ctx, gotoolchain.SSACallFunctionRequest{Call: call})
					if got != (gotoolchain.SSACallFunctionObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
						t.Fatalf("foreign %s callable=(%+v,%v), want zero and %v", name, got, queryErr, core.ErrGoToolchainOutput)
					}
					receiver, queryErr := view.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
					if receiver != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
						t.Fatalf("foreign %s receiver=(%+v,%v), want zero and %v", name, receiver, queryErr, core.ErrGoToolchainOutput)
					}
					return nil
				}})
			}})
			if err != nil {
				t.Fatalf("foreign generic unit=%v, want nil", err)
			}
		})
	}
}
