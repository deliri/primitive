package gotoolchain_test

import (
	"context"
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAMethodReceiverRefusesDamagedUntakenAncestry(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(gotoolchain.SSAUnitView, *ssa.Function)
	}{
		{"missing ancestor instruction", func(_ gotoolchain.SSAUnitView, fn *ssa.Function) { b := fn.Blocks[0]; b.Instrs[len(b.Instrs)-1] = nil }},
		{"typed nil ancestor", func(_ gotoolchain.SSAUnitView, fn *ssa.Function) {
			b := fn.Blocks[0]
			b.Instrs[len(b.Instrs)-1] = (*ssa.If)(nil)
		}},
		{"foreign ancestor instruction", func(v gotoolchain.SSAUnitView, fn *ssa.Function) {
			b := fn.Blocks[0]
			other := v.Package.Func("Other").Blocks[0]
			b.Instrs[len(b.Instrs)-1] = other.Instrs[len(other.Instrs)-1]
		}},
		{"missing branch condition", func(_ gotoolchain.SSAUnitView, fn *ssa.Function) {
			b := fn.Blocks[0]
			b.Instrs[len(b.Instrs)-1].(*ssa.If).Cond = nil
		}},
		{"typed nil branch condition", func(_ gotoolchain.SSAUnitView, fn *ssa.Function) {
			b := fn.Blocks[0]
			b.Instrs[len(b.Instrs)-1].(*ssa.If).Cond = (*ssa.Parameter)(nil)
		}},
		{"missing branch successor", func(_ gotoolchain.SSAUnitView, fn *ssa.Function) { fn.Blocks[0].Succs[0] = nil }},
		{"missing successor slot", func(_ gotoolchain.SSAUnitView, fn *ssa.Function) { b := fn.Blocks[0]; b.Succs = b.Succs[:1] }},
		{"foreign successor", func(v gotoolchain.SSAUnitView, fn *ssa.Function) {
			fn.Blocks[0].Succs[0] = v.Package.Func("Other").Blocks[0]
		}},
		{"detached successor", func(_ gotoolchain.SSAUnitView, fn *ssa.Function) {
			b := fn.Blocks[0]
			copied := *b.Succs[0]
			b.Succs[0] = &copied
		}},
		{"damaged ancestor ordinal", func(_ gotoolchain.SSAUnitView, fn *ssa.Function) { fn.Blocks[0].Index = -1 }},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type sample struct{};func(*sample)M(){};func invoke(){};func Entry(flag bool,r *sample){if flag{invoke()};r.M()};func Other(flag bool){if flag{invoke()}}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				request := gotoolchain.SSAMethodReceiverRequest{Call: nativeReceiverParameterLastCall(ctx, t, v, fn)}
				got, queryErr := v.MethodReceiver(ctx, request)
				if queryErr != nil || got.Method == nil || got.Receiver != fn.Params[1] {
					t.Fatalf("ancestry fixture=%+v,%v, want M on Entry.r,nil", got, queryErr)
				}
				row.mutate(v, fn)
				got, queryErr = v.MethodReceiver(ctx, request)
				if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
					t.Fatalf("damaged ancestry=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainOutput)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

type nativeUntakenQuery uint8

const (
	nativeUntakenMethod nativeUntakenQuery = iota
	nativeUntakenParameter
	nativeUntakenStorage
	nativeUntakenInstructions
	nativeUntakenBlocks
	nativeUntakenLinks
	nativeUntakenSuccessorLinks
	nativeUntakenSource
	nativeUntakenBinary
	nativeUntakenReferrers
	nativeUntakenSynchronous
	nativeUntakenArgument
)

func TestSSAUntakenAncestryRefusalReachesNativeQueries(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		kind nativeUntakenQuery
	}{{"method", nativeUntakenMethod}, {"parameter", nativeUntakenParameter}, {"storage", nativeUntakenStorage}, {"instructions", nativeUntakenInstructions}, {"blocks", nativeUntakenBlocks}, {"links", nativeUntakenLinks}, {"successor links", nativeUntakenSuccessorLinks}, {"source", nativeUntakenSource}, {"binary", nativeUntakenBinary}, {"referrers", nativeUntakenReferrers}, {"synchronous", nativeUntakenSynchronous}, {"argument", nativeUntakenArgument}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type sample struct{};func(*sample)M()int{return 1};func invoke(){};func use(int){};func Entry(flag,other bool,r *sample){if flag{invoke()};first:=r.M();use(first);_=flag==other;_=r.M()}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var first, last *ssa.Call
				for instruction, err := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
					if err != nil {
						return err
					}
					call, ok := instruction.(*ssa.Call)
					if !ok || call.Common().StaticCallee().Name() != "M" {
						continue
					}
					if first == nil {
						first = call
					}
					last = call
				}
				if first == nil || last == nil || first == last || first.Block() != last.Block() {
					t.Fatalf("native method calls=%v/%v, want two calls in the join block", first, last)
				}
				var comparison *ast.BinaryExpr
				var expression *ast.CallExpr
				for syntax, err := range gotoolchain.WalkSyntax(ctx, gotoolchain.SyntaxWalkRequest{Root: fn.Syntax()}) {
					if err != nil {
						return err
					}
					if node, ok := syntax.Node.(*ast.BinaryExpr); ok && node.Op == token.EQL {
						comparison = node
					}
					if node, ok := syntax.Node.(*ast.CallExpr); ok {
						if selector, ok := node.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "M" {
							expression = node
						}
					}
				}
				if comparison == nil || expression == nil {
					t.Fatalf("native source expressions=%v/%v, want comparison and method call", comparison, expression)
				}
				root := fn.Blocks[0]
				root.Instrs[len(root.Instrs)-1] = (*ssa.If)(nil)
				var queryErr error
				refusals := 0
				switch row.kind {
				case nativeUntakenMethod:
					got, err := v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: last})
					if got != (gotoolchain.SSAMethodReceiverObservation{}) {
						t.Fatalf("refused method=%+v, want zero", got)
					}
					queryErr = err
				case nativeUntakenParameter:
					got, err := v.ReceiverParameter(ctx, gotoolchain.SSAReceiverParameterRequest{Call: last, Parameter: fn.Params[2]})
					if got != (gotoolchain.SSARelationObservation{}) {
						t.Fatalf("refused parameter=%+v, want zero", got)
					}
					queryErr = err
				case nativeUntakenStorage:
					got, err := v.SameReceiverStorage(ctx, gotoolchain.SSACallReceiverStorageRequest{Left: first, Right: last})
					if got != (gotoolchain.SSARelationObservation{}) {
						t.Fatalf("refused storage=%+v, want zero", got)
					}
					queryErr = err
				case nativeUntakenSource:
					got, err := v.CallForExpression(ctx, gotoolchain.SSASourceCallRequest{Function: fn, Expression: expression})
					if got != (gotoolchain.SSASourceCallObservation{}) {
						t.Fatalf("refused source=%+v, want zero", got)
					}
					queryErr = err
				case nativeUntakenBinary:
					got, err := v.BinaryOperationForExpression(ctx, gotoolchain.SSABinaryOperationRequest{Function: fn, Expression: comparison})
					if got != (gotoolchain.SSABinaryOperationObservation{}) {
						t.Fatalf("refused binary=%+v, want zero", got)
					}
					queryErr = err
				case nativeUntakenArgument:
					target := first.Common().StaticCallee().Object().(*types.Func)
					got, err := v.ArgumentCall(ctx, gotoolchain.SSAArgumentCallRequest{Function: fn, Argument: fn.Params[2], Target: target, Index: 0, Timing: gotoolchain.SSACallTimingImmediate})
					if got != (gotoolchain.SSARelationObservation{}) {
						t.Fatalf("refused argument=%+v, want zero", got)
					}
					queryErr = err
				case nativeUntakenInstructions:
					for got, err := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Block: first.Block(), Extent: gotoolchain.SSAInstructionExtentBlock}) {
						if got != nil || !errors.Is(err, core.ErrGoToolchainOutput) {
							t.Fatalf("refused instruction=%v,%v, want nil,%v", got, err, core.ErrGoToolchainOutput)
						}
						queryErr, refusals = err, refusals+1
					}
				case nativeUntakenBlocks:
					for got, err := range v.Blocks(ctx, gotoolchain.SSABlockRequest{Function: fn, DominatedBy: first.Block()}) {
						if got != nil || !errors.Is(err, core.ErrGoToolchainOutput) {
							t.Fatalf("refused block=%v,%v, want nil,%v", got, err, core.ErrGoToolchainOutput)
						}
						queryErr, refusals = err, refusals+1
					}
				case nativeUntakenLinks, nativeUntakenSuccessorLinks:
					selected := first.Block()
					if row.kind == nativeUntakenSuccessorLinks {
						selected = root
					}
					for got, err := range v.BlockLinks(ctx, gotoolchain.SSABlockLinkRequest{Block: selected, Kind: gotoolchain.SSABlockLinkSuccessors}) {
						if got != nil || !errors.Is(err, core.ErrGoToolchainOutput) {
							t.Fatalf("refused link=%v,%v, want nil,%v", got, err, core.ErrGoToolchainOutput)
						}
						queryErr, refusals = err, refusals+1
					}
				case nativeUntakenReferrers:
					for got, err := range v.Referrers(ctx, gotoolchain.SSAReferrerRequest{Instruction: first}) {
						if got != nil || !errors.Is(err, core.ErrGoToolchainOutput) {
							t.Fatalf("refused referrer=%v,%v, want nil,%v", got, err, core.ErrGoToolchainOutput)
						}
						queryErr, refusals = err, refusals+1
					}
				case nativeUntakenSynchronous:
					for got, err := range v.SynchronousCalls(ctx, gotoolchain.SSASynchronousCallRequest{Function: fn, Timing: gotoolchain.SSACallTimingImmediate}) {
						if got != nil || !errors.Is(err, core.ErrGoToolchainOutput) {
							t.Fatalf("refused synchronous call=%v,%v, want nil,%v", got, err, core.ErrGoToolchainOutput)
						}
						queryErr, refusals = err, refusals+1
					}
				default:
					t.Fatalf("query=%d, want named owning query", row.kind)
				}
				if !errors.Is(queryErr, core.ErrGoToolchainOutput) || refusals > 1 {
					t.Fatalf("query refusal=%v count=%d, want %v exactly once", queryErr, refusals, core.ErrGoToolchainOutput)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSAUntakenAncestryProducerLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type sample struct{};func(*sample)M(){};func invoke(){};func plain(*sample){};func Entry(flag bool,r *sample){if flag{invoke()};r.M()};func Other(r *sample){plain(r)}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		request := gotoolchain.SSAMethodReceiverRequest{Call: nativeReceiverParameterLastCall(ctx, t, v, fn)}
		got, queryErr := v.MethodReceiver(ctx, request)
		if queryErr != nil || got.Method == nil || got.Method.Name() != "M" || got.Receiver != fn.Params[1] {
			t.Fatalf("positive method=%+v,%v, want M on Entry.r,nil", got, queryErr)
		}
		root := fn.Blocks[0]
		root.Instrs[len(root.Instrs)-1] = (*ssa.If)(nil)
		got, queryErr = v.MethodReceiver(ctx, request)
		if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("negative method=%+v,%v, want zero,%v", got, queryErr, core.ErrGoToolchainOutput)
		}
		other := v.Package.Func("Other")
		got, queryErr = v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: nativeReceiverParameterLastCall(ctx, t, v, other)})
		if got != (gotoolchain.SSAMethodReceiverObservation{}) || queryErr != nil {
			t.Fatalf("neutral other function=%+v,%v, want zero,nil", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
