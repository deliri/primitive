package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const nativeReferrerSource = `package sample;func sink(int){};func Entry(x int){f:=func(){sink(x)};f();f()}`

func nativeReferrerValue(ctx context.Context, view gotoolchain.SSAUnitView) (ssa.Instruction, error) {
	for instruction, err := range view.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: view.Package.Func("Entry"), Extent: gotoolchain.SSAInstructionExtentFunction}) {
		if err != nil {
			return nil, err
		}
		if _, closure := instruction.(*ssa.MakeClosure); closure {
			return instruction, nil
		}
	}
	return nil, nil
}

func TestSSAReferrersConserveNativeUsesAndConsumerAuthority(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeReferrerSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		value, err := nativeReferrerValue(ctx, view)
		if err != nil || value == nil {
			t.Fatalf("native closure=(%v,%v), want nonnil,nil", value, err)
		}
		references := value.(ssa.Value).Referrers()
		if references == nil || len(*references) != 2 {
			t.Fatalf("native uses=%v, want two calls", references)
		}
		request := gotoolchain.SSAReferrerRequest{Instruction: value}
		index := 0
		for instruction, queryErr := range view.Referrers(ctx, request) {
			if queryErr != nil || index >= len(*references) || instruction != (*references)[index] {
				t.Fatalf("native reference=%v,%v at%d, want exact Go use order", instruction, queryErr, index)
			}
			index++
		}
		if index != 2 {
			t.Fatalf("native use count=%d, want 2", index)
		}
		canceled, cancel := context.WithCancelCause(ctx)
		defer cancel(nil)
		uses, terminals := 0, 0
		for instruction, queryErr := range view.Referrers(canceled, request) {
			if queryErr != nil {
				if instruction != nil || !errors.Is(queryErr, io.ErrClosedPipe) || !errors.Is(queryErr, context.Canceled) {
					t.Fatalf("late reference refusal=%v,%v, want nil and native cancellation/cause", instruction, queryErr)
				}
				terminals++
				continue
			}
			if uses != 0 || instruction != (*references)[0] {
				t.Fatalf("reference prefix=%v at%d, want first native use", instruction, uses)
			}
			uses++
			cancel(io.ErrClosedPipe)
		}
		if uses != 1 || terminals != 1 {
			t.Fatalf("reference uses/terminals=%d/%d, want 1/1", uses, terminals)
		}
		stopped, stop := context.WithCancelCause(ctx)
		defer stop(nil)
		uses = 0
		for instruction, queryErr := range view.Referrers(stopped, request) {
			if queryErr != nil || instruction != (*references)[0] {
				t.Fatalf("reference stop prefix=%v,%v, want first use,nil", instruction, queryErr)
			}
			uses++
			stop(io.ErrClosedPipe)
			break
		}
		if uses != 1 {
			t.Fatalf("stopped native uses=%d, want 1", uses)
		}
		recovered := false
		func() {
			defer func() { recovered = recover() == io.ErrClosedPipe }()
			for range view.Referrers(ctx, request) {
				panic(io.ErrClosedPipe)
			}
		}()
		if !recovered {
			t.Fatal("native reference consumer panic lost its identity")
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}

func TestSSAReferrersPreserveLoanAndOutputRefusals(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"absent loan", "nil value", "typed nil value", "nonvalue instruction", "cloned value", "bad ordinal", "nil native use", "cloned native use", "foreign native use"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeReferrerSource+`;func Other(){sink(1)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				value, err := nativeReferrerValue(ctx, view)
				if err != nil || value == nil {
					t.Fatalf("native closure=(%v,%v), want nonnil,nil", value, err)
				}
				request := gotoolchain.SSAReferrerRequest{Instruction: value}
				loan, want := view, core.ErrGoToolchainContract
				switch name {
				case "absent loan":
					loan = gotoolchain.SSAUnitView{}
				case "nil value":
					request.Instruction = nil
				case "typed nil value":
					request.Instruction = (*ssa.MakeClosure)(nil)
				case "nonvalue instruction":
					fn := value.Parent()
					block := fn.Blocks[len(fn.Blocks)-1]
					request.Instruction = block.Instrs[len(block.Instrs)-1]
				case "cloned value":
					clone := *value.(*ssa.MakeClosure)
					request.Instruction = &clone
				case "bad ordinal":
					value.Block().Index += 100
					want = core.ErrGoToolchainOutput
				case "nil native use":
					(*value.(ssa.Value).Referrers())[0] = nil
					want = core.ErrGoToolchainOutput
				case "cloned native use":
					uses := value.(ssa.Value).Referrers()
					clone := *(*uses)[0].(*ssa.Call)
					(*uses)[0] = &clone
					want = core.ErrGoToolchainOutput
				case "foreign native use":
					other := view.Package.Func("Other")
					(*value.(ssa.Value).Referrers())[0] = other.Blocks[0].Instrs[0]
					want = core.ErrGoToolchainOutput
				default:
					t.Fatalf("referrer case=%q, want declared boundary", name)
				}
				refusals := 0
				for instruction, queryErr := range loan.Referrers(ctx, request) {
					if instruction != nil || !errors.Is(queryErr, want) {
						t.Fatalf("native reference refusal=%v,%v, want nil,%v", instruction, queryErr, want)
					}
					refusals++
				}
				if refusals != 1 {
					t.Fatalf("native reference refusals=%d, want 1", refusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
