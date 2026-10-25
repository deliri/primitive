package gotoolchain

import (
	"context"
	"errors"
	"go/types"
	"iter"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/ssa"
)

// SSAFunctionAncestorsRequest selects a source frame in the current unit.
// Synthetic package initialization is a source root; bound method wrappers
// without a source frame are outside this query.
type SSAFunctionAncestorsRequest struct{ Function *ssa.Function }

func (r SSAFunctionAncestorsRequest) Validate() error {
	if r.Function == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAFunctionAncestorsRequest) goToolchainInternalFlow() {}

// FunctionAncestors lends the selected native frame, then its lexical parents
// through the source root. Parent identity does not establish invocation.
// Each loan authenticates Go's existing child membership or declaration map.
// Only the current native pointer is retained; no ancestor slice is built.
func (v SSAUnitView) FunctionAncestors(ctx context.Context, r SSAFunctionAncestorsRequest) iter.Seq2[*ssa.Function, error] {
	return func(yield func(*ssa.Function, error) bool) {
		if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), v.Validate(), r.Validate()); err != nil {
			yield(nil, err)
			return
		}
		owner := r.Function.Pkg
		if owner == nil && r.Function.Origin() != nil {
			owner = r.Function.Origin().Pkg
		}
		if r.Function.Prog != v.Program || owner != v.Package {
			yield(nil, core.ErrGoToolchainContract)
			return
		}
		for fn := r.Function; fn != nil; fn = fn.Parent() {
			if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
				yield(nil, err)
				return
			}
			owner := fn.Pkg
			if owner == nil && fn.Origin() != nil {
				owner = fn.Origin().Pkg
			}
			if fn.Prog != v.Program || owner != v.Package || fn.Signature == nil {
				yield(nil, core.ErrGoToolchainOutput)
				return
			}
			if len(fn.Blocks) != 0 && (fn.Blocks[0] == nil || fn.Blocks[0].Parent() != fn || fn.Blocks[0].Index != 0) {
				yield(nil, core.ErrGoToolchainOutput)
				return
			}
			parent := fn.Parent()
			owned := false
			if parent != nil {
				for _, child := range parent.AnonFuncs {
					if child == fn {
						owned = true
						break
					}
				}
			} else if object, ok := fn.Object().(*types.Func); ok {
				declaration := v.Program.FuncValue(object.Origin())
				owned = declaration == fn || declaration != nil && fn.Origin() == declaration && len(fn.Blocks) != 0 && fn.Syntax() == declaration.Syntax()
			} else {
				initializer, err := v.SourceFunction(ctx, SSASourceFunctionRequest{Kind: SSASourceFunctionInitializer})
				if err != nil {
					yield(nil, err)
					return
				}
				owned = initializer.Function == fn
			}
			if !owned {
				yield(nil, core.ErrGoToolchainOutput)
				return
			}
			if !yield(fn, nil) {
				return
			}
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
			yield(nil, err)
		}
	}
}

var _ internalFlow = SSAFunctionAncestorsRequest{}
