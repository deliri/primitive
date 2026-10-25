package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/types"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAInvocationKind distinguishes the selected root, an actual argument binding,
// and a callback whose selected parameter starts a new argument scope.
type SSAInvocationKind uint8

const (
	SSAInvocationKindUnknown SSAInvocationKind = iota
	SSAInvocationKindRoot
	SSAInvocationKindArgument
	SSAInvocationKindCallback
)

func (k SSAInvocationKind) Validate() error {
	if k < SSAInvocationKindRoot || k > SSAInvocationKindCallback {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (k SSAInvocationKind) IsValid() bool { return k.Validate() == nil }
func (SSAInvocationKind) OffWireEnum()    {}
func (k SSAInvocationKind) String() string {
	switch k {
	case SSAInvocationKindRoot:
		return "root"
	case SSAInvocationKindArgument:
		return "argument"
	case SSAInvocationKindCallback:
		return "callback"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// SSAArgumentInvocationFrame borrows its owning SSA unit, not a retained call
// graph. It must not outlive that unit. Callback ancestry is preserved; Kind
// marks its fresh scope. Retaining child loans also retains their caller depth.
type SSAArgumentInvocationFrame struct {
	binding SSAArgumentBindingObservation
	caller  *SSAArgumentInvocationFrame
	call    *ssa.Call
	kind    SSAInvocationKind
	owner   *ssa.Program
}

func (f *SSAArgumentInvocationFrame) Binding() SSAArgumentBindingObservation { return f.binding }
func (f *SSAArgumentInvocationFrame) Caller() *SSAArgumentInvocationFrame    { return f.caller }
func (f *SSAArgumentInvocationFrame) Call() *ssa.Call                        { return f.call }
func (f *SSAArgumentInvocationFrame) Kind() SSAInvocationKind                { return f.kind }
func (SSAArgumentInvocationFrame) goToolchainInternalFlow()                  {}

// SSAArgumentInvocationsRequest follows immediate lexical calls carrying the
// selected argument. An optional exact native method selects its last-function
// callback; CallbackParameter names the new selected native parameter.
type SSAArgumentInvocationsRequest struct {
	Function          *ssa.Function
	Argument          ssa.Value
	CallbackMethod    *types.Func
	CallbackParameter uint64
}

func (r SSAArgumentInvocationsRequest) Validate() error {
	if err := (SSACaptureFramesRequest{Function: r.Function, Argument: r.Argument}).Validate(); err != nil {
		return err
	}
	if r.CallbackMethod == nil {
		if r.CallbackParameter != 0 {
			return core.ErrGoToolchainContract
		}
		return nil
	}
	if r.CallbackMethod.Pkg() == nil {
		return core.ErrGoToolchainContract
	}
	signature := r.CallbackMethod.Signature()
	if signature == nil || signature.Recv() == nil || signature.Params().Len() == 0 {
		return core.ErrGoToolchainContract
	}
	callback, ok := signature.Params().At(signature.Params().Len() - 1).Type().Underlying().(*types.Signature)
	if !ok || r.CallbackParameter >= uint64(callback.Params().Len()) {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAArgumentInvocationsRequest) goToolchainInternalFlow() {}

// SSAArgumentInvocationRequest borrows one delivered caller and one actual
// immediate call. Named-source helper policy belongs to the consumer.
type SSAArgumentInvocationRequest struct {
	Caller *SSAArgumentInvocationFrame
	Call   *ssa.Call
}

func (r SSAArgumentInvocationRequest) Validate() error {
	if r.Caller == nil || r.Call == nil || r.Caller.owner == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAArgumentInvocationRequest) goToolchainInternalFlow() {}

// ArgumentInvocationRoot authenticates one selected native source argument and
// lends its root context without traversing calls or constructing a population.
func (v SSAUnitView) ArgumentInvocationRoot(ctx context.Context, r SSAArgumentInvocationsRequest) (*SSAArgumentInvocationFrame, error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
		return nil, err
	}
	if r.Function.Prog != v.Program || r.Function.Pkg != v.Package || r.Argument.Parent() != r.Function || !v.ownsStorageValue(r.Argument) {
		return nil, core.ErrGoToolchainContract
	}
	for _, err := range v.FunctionAncestors(ctx, SSAFunctionAncestorsRequest{Function: r.Function}) {
		if err != nil {
			return nil, err
		}
		break
	}
	if r.CallbackMethod != nil && v.Program.FuncValue(r.CallbackMethod.Origin()) == nil {
		return nil, core.ErrGoToolchainContract
	}
	return &SSAArgumentInvocationFrame{binding: SSAArgumentBindingObservation{Function: r.Function, Local: r.Argument}, kind: SSAInvocationKindRoot, owner: v.Program}, nil
}

// ArgumentInvocationBindings authenticates each actual binding before lending
// its frame. It constructs no environment; separate slots and calls remain
// separate observations. The loan may be used synchronously by a policy query.
func (v SSAUnitView) ArgumentInvocationBindings(ctx context.Context, r SSAArgumentInvocationRequest) iter.Seq2[*SSAArgumentInvocationFrame, error] {
	return func(yield func(*SSAArgumentInvocationFrame, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
			yield(nil, err)
			return
		}
		if r.Caller.owner != v.Program || r.Call.Parent() != r.Caller.binding.Function {
			yield(nil, core.ErrGoToolchainContract)
			return
		}
		for binding, err := range v.ArgumentBindings(ctx, SSAArgumentBindingsRequest{Call: r.Call, Argument: r.Caller.binding.Local}) {
			if err != nil {
				yield(nil, err)
				return
			}
			frame := SSAArgumentInvocationFrame{binding: binding, caller: r.Caller, call: r.Call, kind: SSAInvocationKindArgument, owner: v.Program}
			if !yield(&frame, nil) {
				return
			}
		}
	}
}

// ArgumentInvocations lends the root and actual invoked literal frames in native
// instruction order. Named helpers, defers, goroutines and uncalled literals do
// not enter this stream. Repeated calls are distinct, including their ancestry.
// These are possible source invocations, not proof that every runtime path
// executes a call or eventually returns.
// Primitive retains active native invocation depth only, with no copied frame,
// instruction, binding or function population. This is not an O(1) depth claim.
func (v SSAUnitView) ArgumentInvocations(ctx context.Context, r SSAArgumentInvocationsRequest) iter.Seq2[*SSAArgumentInvocationFrame, error] {
	return func(yield func(*SSAArgumentInvocationFrame, error) bool) {
		frame, err := v.ArgumentInvocationRoot(ctx, r)
		if err != nil {
			yield(nil, err)
			return
		}
		if !v.argumentInvocationsInFrame(ctx, frame, r, yield) {
			return
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			yield(nil, err)
		}
	}
}

func (v SSAUnitView) argumentInvocationsInFrame(ctx context.Context, frame *SSAArgumentInvocationFrame, request SSAArgumentInvocationsRequest, yield func(*SSAArgumentInvocationFrame, error) bool) bool {
	if !yield(frame, nil) {
		return false
	}
	for instruction, instructionErr := range v.Instructions(ctx, SSAInstructionRequest{Function: frame.binding.Function, Extent: SSAInstructionExtentFunction}) {
		if instructionErr != nil {
			yield(nil, instructionErr)
			return false
		}
		call, immediate := instruction.(*ssa.Call)
		if immediate && !v.argumentInvocationsInCall(ctx, SSAArgumentInvocationRequest{Caller: frame, Call: call}, request, yield) {
			return false
		}
	}
	return true
}

func (v SSAUnitView) argumentInvocationsInCall(ctx context.Context, invocation SSAArgumentInvocationRequest, request SSAArgumentInvocationsRequest, yield func(*SSAArgumentInvocationFrame, error) bool) bool {
	frame, call := invocation.Caller, invocation.Call
	source, sourceErr := v.CalleeSource(ctx, SSACalleeSourceRequest{Call: call})
	if sourceErr != nil {
		yield(nil, sourceErr)
		return false
	}
	target := source.Function
	literal := false
	if target != nil {
		_, literal = target.Syntax().(*ast.FuncLit)
	}
	if target == nil || !literal || target.Parent() != frame.binding.Function {
		return v.argumentInvocationsInCallback(ctx, invocation, request, yield)
	}
	for child, bindingErr := range v.ArgumentInvocationBindings(ctx, SSAArgumentInvocationRequest{Caller: frame, Call: call}) {
		if bindingErr != nil {
			yield(nil, bindingErr)
			return false
		}
		if !v.argumentInvocationsInFrame(ctx, child, request, yield) {
			return false
		}
	}
	return true
}

func (v SSAUnitView) argumentInvocationsInCallback(ctx context.Context, invocation SSAArgumentInvocationRequest, request SSAArgumentInvocationsRequest, yield func(*SSAArgumentInvocationFrame, error) bool) bool {
	frame, call := invocation.Caller, invocation.Call
	if request.CallbackMethod == nil {
		return true
	}
	method, methodErr := v.MethodReceiver(ctx, SSAMethodReceiverRequest{Call: call})
	if methodErr != nil {
		yield(nil, methodErr)
		return false
	}
	if method.Method == nil || method.Method.Origin() != request.CallbackMethod.Origin() {
		return true
	}
	callback, callbackErr := v.LastFunctionArgument(ctx, SSAFunctionArgumentRequest{Call: call})
	if callbackErr != nil {
		yield(nil, callbackErr)
		return false
	}
	target := callback.Function
	if target == nil || target.Parent() != frame.binding.Function {
		return true
	}
	if request.CallbackParameter >= uint64(len(target.Params)) {
		yield(nil, core.ErrGoToolchainOutput)
		return false
	}
	binding := SSAArgumentBindingObservation{Function: target, Local: target.Params[request.CallbackParameter]}
	if bindingErr := binding.Validate(); bindingErr != nil {
		yield(nil, bindingErr)
		return false
	}
	child := SSAArgumentInvocationFrame{binding: binding, caller: frame, call: call, kind: SSAInvocationKindCallback, owner: v.Program}
	return v.argumentInvocationsInFrame(ctx, &child, request, yield)
}

var _ internalFlow = SSAArgumentInvocationFrame{}
var _ internalFlow = SSAArgumentInvocationsRequest{}
var _ internalFlow = SSAArgumentInvocationRequest{}
