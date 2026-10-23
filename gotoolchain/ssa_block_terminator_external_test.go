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

func TestSSAInstructionsBorrowEachNativeTerminator(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"Conditional", "Jump", "Return", "Panic", "Anonymous"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Conditional(flag bool){if flag{invoke()}};func Jump(flag bool){if flag{invoke()};invoke()};func Return(){invoke()};func Panic(){panic("detail")};func Closure(){_=func(){invoke()}}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func(name)
				if name == "Anonymous" {
					fn = v.Package.Func("Closure").AnonFuncs[0]
				}
				var want ssa.Instruction
				// Find the unique authored control operation through the ordinary
				// native stream, independently of the new last-element selection.
				for instruction, err := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
					if err != nil {
						return err
					}
					selected := false
					switch instruction.(type) {
					case *ssa.If:
						selected = name == "Conditional"
					case *ssa.Jump:
						selected = name == "Jump"
					case *ssa.Return:
						selected = name == "Return" || name == "Anonymous"
					case *ssa.Panic:
						selected = name == "Panic"
					}
					if selected {
						if want != nil {
							t.Fatalf("native %s controls=%v and %v, want one", name, want, instruction)
						}
						want = instruction
					}
				}
				if want == nil {
					t.Fatalf("native %s control=%v, want authored control operation", name, want)
				}
				observed := 0
				for got, err := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Block: want.Block(), Extent: gotoolchain.SSAInstructionExtentBlockTerminator}) {
					if err != nil || got != want {
						t.Fatalf("native terminator=%v,%v, want exact %v,nil", got, err, want)
					}
					observed++
				}
				if observed != 1 {
					t.Fatalf("native terminators=%d, want 1", observed)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSAInstructionsTerminatorRefusesDamagedSelection(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*gotoolchain.SSAUnitView, *gotoolchain.SSAInstructionRequest)
		want   error
	}{
		{"absent view", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAInstructionRequest) { *v = gotoolchain.SSAUnitView{} }, core.ErrGoToolchainContract},
		{"absent function", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) { r.Function = nil }, core.ErrGoToolchainContract},
		{"absent block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) { r.Block = nil }, core.ErrGoToolchainContract},
		{"foreign function block", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Block = v.Package.Func("Other").Blocks[0]
		}, core.ErrGoToolchainContract},
		{"foreign program", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) { r.Function.Prog = nil }, core.ErrGoToolchainContract},
		{"detached block", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			copied := *r.Block
			r.Block = &copied
		}, core.ErrGoToolchainOutput},
		{"negative block ordinal", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) { r.Block.Index = -1 }, core.ErrGoToolchainOutput},
		{"ordinal beyond function", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Block.Index = len(r.Function.Blocks)
		}, core.ErrGoToolchainOutput},
		{"missing terminator", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Block.Instrs[len(r.Block.Instrs)-1] = nil
		}, core.ErrGoToolchainOutput},
		{"typed nil terminator", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Block.Instrs[len(r.Block.Instrs)-1] = (*ssa.Return)(nil)
		}, core.ErrGoToolchainOutput},
		{"foreign terminator", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Block.Instrs[len(r.Block.Instrs)-1] = v.Package.Func("Other").Blocks[0].Instrs[0]
		}, core.ErrGoToolchainOutput},
		{"ordinary instruction cannot end control flow", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAInstructionRequest) {
			r.Block.Instrs = r.Block.Instrs[:len(r.Block.Instrs)-1]
		}, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){invoke()};func Other(){}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				r := gotoolchain.SSAInstructionRequest{Function: fn, Block: fn.Blocks[0], Extent: gotoolchain.SSAInstructionExtentBlockTerminator}
				row.mutate(&v, &r)
				refusals := 0
				for got, err := range v.Instructions(ctx, r) {
					if got != nil || !errors.Is(err, row.want) || errors.Is(err, core.ErrGoToolchainOutput) != errors.Is(row.want, core.ErrGoToolchainOutput) {
						t.Fatalf("terminator refusal=%v,%v, want nil,%v", got, err, row.want)
					}
					refusals++
				}
				if refusals != 1 {
					t.Fatalf("terminator refusals=%d, want 1", refusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSAInstructionsTerminatorProducerLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){invoke()};func Other(){}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		block := fn.Blocks[0]
		request := gotoolchain.SSAInstructionRequest{Function: fn, Block: block, Extent: gotoolchain.SSAInstructionExtentBlockTerminator}
		// A broken earlier instruction is outside a terminator query. The
		// full-block stream must still refuse that same broken prefix.
		block.Instrs[0] = nil
		observed := 0
		for got, err := range v.Instructions(ctx, request) {
			if _, ok := got.(*ssa.Return); err != nil || !ok || got.Block() != block {
				t.Fatalf("selected tail=%v,%v, want native Return,nil beside unqueried prefix", got, err)
			}
			observed++
		}
		if observed != 1 {
			t.Fatalf("selected tails=%d, want 1", observed)
		}
		request.Extent = gotoolchain.SSAInstructionExtentBlock
		refusals := 0
		for got, err := range v.Instructions(ctx, request) {
			if got != nil || !errors.Is(err, core.ErrGoToolchainOutput) {
				t.Fatalf("whole-block refusal=%v,%v, want nil,%v", got, err, core.ErrGoToolchainOutput)
			}
			refusals++
		}
		if refusals != 1 {
			t.Fatalf("whole-block refusals=%d, want 1", refusals)
		}
		request.Extent = gotoolchain.SSAInstructionExtentBlockTerminator
		block.Instrs[len(block.Instrs)-1] = v.Package.Func("Other").Blocks[0].Instrs[0]
		refusals = 0
		for got, err := range v.Instructions(ctx, request) {
			if got != nil || !errors.Is(err, core.ErrGoToolchainOutput) {
				t.Fatalf("foreign tail=%v,%v, want nil,%v", got, err, core.ErrGoToolchainOutput)
			}
			refusals++
		}
		if refusals != 1 {
			t.Fatalf("foreign tail refusals=%d, want 1", refusals)
		}
		// No control fact exists for an empty source block.
		block.Instrs = nil
		request.Extent = gotoolchain.SSAInstructionExtentBlockTerminator
		for got, err := range v.Instructions(ctx, request) {
			t.Fatalf("empty block observation=%v,%v, want no observation or refusal", got, err)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}

func TestSSAInstructionsTerminatorPreservesCancellationAndStop(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(){}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		r := gotoolchain.SSAInstructionRequest{Function: fn, Block: fn.Blocks[0], Extent: gotoolchain.SSAInstructionExtentBlockTerminator}
		cancelled, cancelBefore := context.WithCancelCause(ctx)
		cancelBefore(io.ErrClosedPipe)
		for _, current := range []context.Context{nil, cancelled} {
			want := error(core.ErrNilContext)
			if current != nil {
				want = io.ErrClosedPipe
			}
			refused := 0
			for got, err := range v.Instructions(current, r) {
				if got != nil || !errors.Is(err, want) {
					t.Fatalf("entry cancellation=%v,%v, want nil,%v", got, err, want)
				}
				refused++
			}
			if refused != 1 {
				t.Fatalf("entry cancellation refusals=%d, want 1", refused)
			}
		}
		for _, stopEarly := range []bool{false, true} {
			queryCtx, cancel := context.WithCancelCause(ctx)
			observed, refused := 0, 0
			v.Instructions(queryCtx, r)(func(got ssa.Instruction, err error) bool {
				if err != nil {
					if got != nil || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, context.Canceled) {
						t.Fatalf("terminal cancellation=%v,%v, want nil,cancelled with native cause", got, err)
					}
					refused++
					return true
				}
				if _, ok := got.(*ssa.Return); !ok {
					t.Fatalf("terminal instruction=%v, want native Return", got)
				}
				observed++
				cancel(io.ErrClosedPipe)
				return !stopEarly
			})
			cancel(nil)
			wantRefused := 1
			if stopEarly {
				wantRefused = 0
			}
			if observed != 1 || refused != wantRefused {
				t.Fatalf("terminal observed/refused=%d/%d, want 1/%d", observed, refused, wantRefused)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}

func TestSSAInstructionsTerminatorKeepsUntakenBlockNeutral(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){if false{go invoke()}}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		selected := 0
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if _, ok := instruction.(*ssa.Go); !ok {
					continue
				}
				selected++
				for got, err := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Block: block, Extent: gotoolchain.SSAInstructionExtentBlockTerminator}) {
					t.Fatalf("untaken terminator=%v,%v, want no observation or refusal", got, err)
				}
			}
		}
		if selected != 1 {
			t.Fatalf("untaken source blocks=%d, want one containing the source spawn", selected)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
