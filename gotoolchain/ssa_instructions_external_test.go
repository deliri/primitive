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

func TestSSAInstructionsConserveNativeLexicalExtent(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		extent     gotoolchain.SSAInstructionExtent
		want       int
	}{
		{"direct spawn", `go invoke()`, gotoolchain.SSAInstructionExtentFunction, 1},
		{"constant untaken spawn", `if false{go invoke()}`, gotoolchain.SSAInstructionExtentFunction, 0},
		{"runtime branch remains possible", `if flag{go invoke()}`, gotoolchain.SSAInstructionExtentFunction, 1},
		{"deferred call is no spawn", `defer invoke()`, gotoolchain.SSAInstructionExtentFunction, 0},
		{"uninvoked lexical body excluded from function", `unused:=func(){go invoke()};_=unused`, gotoolchain.SSAInstructionExtentFunction, 0},
		{"uninvoked lexical body observed as source", `unused:=func(){go invoke()};_=unused`, gotoolchain.SSAInstructionExtentLexical, 1},
		{"nested lexical source observed once", `go func(){go func(){go invoke()}()}()`, gotoolchain.SSAInstructionExtentLexical, 3},
		{"named helper body not lexical", `helper()`, gotoolchain.SSAInstructionExtentLexical, 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func helper(){go invoke()};func Entry(flag bool){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				count := 0
				for instruction, err := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: v.Package.Func("Entry"), Extent: row.extent}) {
					if err != nil || instruction == nil {
						t.Fatalf("instruction/refusal=(%v,%v), want native observation and nil", instruction, err)
					}
					if _, ok := instruction.(*ssa.Go); ok {
						count++
					}
				}
				if count != row.want {
					t.Fatalf("native spawn observations=%d, want %d", count, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSAInstructionsPreserveRefusalAndConsumerAuthority(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func invoke(){};func Entry(){invoke();invoke();_=func(){invoke()}}`)
	foreign := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(){}`)
	var foreignFunction *ssa.Function
	err := foreign.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: foreign.Units[0], Use: func(_ context.Context, v gotoolchain.SSAUnitView) error {
		foreignFunction = v.Package.Func("Entry")
		return nil
	}})
	if err != nil {
		t.Fatalf("foreign WithSSAUnit error=%v, want nil", err)
	}
	err = a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		r := gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentLexical}
		for _, row := range []struct {
			name   string
			fn     *ssa.Function
			extent gotoolchain.SSAInstructionExtent
		}{
			{"absent function", nil, r.Extent}, {"unknown extent", fn, gotoolchain.SSAInstructionExtentUnknown}, {"invalid extent", fn, gotoolchain.SSAInstructionExtent(255)}, {"foreign function", foreignFunction, r.Extent},
		} {
			refusals := 0
			for instruction, err := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: row.fn, Extent: row.extent}) {
				if instruction != nil || !errors.Is(err, core.ErrGoToolchainContract) {
					t.Fatalf("%s instruction/refusal=(%v,%v), want nil and contract refusal", row.name, instruction, err)
				}
				refusals++
			}
			if refusals != 1 {
				t.Fatalf("%s refusals=%d, want 1", row.name, refusals)
			}
		}
		cancelled, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		for _, current := range []context.Context{nil, cancelled} {
			want := error(core.ErrNilContext)
			if current != nil {
				want = io.ErrClosedPipe
			}
			n := 0
			for instruction, err := range v.Instructions(current, r) {
				if instruction != nil || !errors.Is(err, want) {
					t.Fatalf("cancelled=(%v,%v), want nil and %v", instruction, err, want)
				}
				n++
			}
			if n != 1 {
				t.Fatalf("cancelled refusals=%d, want 1", n)
			}
		}
		late, stop := context.WithCancelCause(ctx)
		observed, refusals := 0, 0
		for instruction, err := range v.Instructions(late, r) {
			if err != nil {
				if instruction != nil || !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("late=(%v,%v), want nil and closed pipe", instruction, err)
				}
				refusals++
			} else {
				observed++
				stop(io.ErrClosedPipe)
			}
		}
		stop(nil)
		if observed != 1 || refusals != 1 {
			t.Fatalf("late observed/refusals=%d/%d, want 1/1", observed, refusals)
		}
		// A malformed future lexical body must remain untouched after refusal.
		if len(fn.AnonFuncs) != 1 {
			t.Fatalf("lexical children=%d, want 1", len(fn.AnonFuncs))
		}
		fn.AnonFuncs[0].Blocks[0].Instrs[0] = nil
		observed = 0
		for instruction, err := range v.Instructions(ctx, r) {
			if err != nil || instruction == nil {
				t.Fatalf("stopped prefix=(%v,%v), want first native instruction and nil", instruction, err)
			}
			observed++
			break
		}
		if observed != 1 {
			t.Fatalf("prefix instructions=%d, want 1", observed)
		}
		refusals = 0
		for instruction, err := range v.Instructions(ctx, r) {
			if err != nil {
				if instruction != nil || !errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("future=(%v,%v), want nil and output refusal", instruction, err)
				}
				refusals++
			}
		}
		if refusals != 1 {
			t.Fatalf("future refusals=%d, want 1", refusals)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}

func TestSSAInstructionExtentExhaustsClosedDomain(t *testing.T) {
	t.Parallel()
	for i := 0; i < 256; i++ {
		value := gotoolchain.SSAInstructionExtent(i)
		want := value == gotoolchain.SSAInstructionExtentFunction || value == gotoolchain.SSAInstructionExtentLexical || value == gotoolchain.SSAInstructionExtentBlock || value == gotoolchain.SSAInstructionExtentBlockTerminator
		if value.IsValid() != want || (value.Validate() == nil) != want || (!want && !errors.Is(value.Validate(), core.ErrGoToolchainContract)) {
			t.Fatalf("extent %d validation=(%t,%v), want valid=%t and contract refusal otherwise", i, value.IsValid(), value.Validate(), want)
		}
		label := core.UnknownEnumDiagnostic
		if value == gotoolchain.SSAInstructionExtentFunction {
			label = "function"
		} else if value == gotoolchain.SSAInstructionExtentLexical {
			label = "lexical"
		} else if value == gotoolchain.SSAInstructionExtentBlock {
			label = "block"
		} else if value == gotoolchain.SSAInstructionExtentBlockTerminator {
			label = "block_terminator"
		}
		if value.String() != label {
			t.Fatalf("extent %d label=%q, want %q", i, value.String(), label)
		}
		value.OffWireEnum()
	}
}
