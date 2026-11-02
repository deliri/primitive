package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const functionOperandPrelude = `package sample;type Item uint8;func sink(func(Item,error)bool){};func named(Item,error)bool{return false};`

func TestSSAArgumentOperandTreatsNativeFunctionsAsNonmatching(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		want       bool
	}{
		{name: "native parameter still matches", body: `sink(cb)`, want: true},
		{name: "readonly capture still matches", body: `func(){sink(cb)}()`, want: true},
		{name: "inline anonymous function is a legal nonmatch", body: `sink(func(Item,error)bool{return false})`},
		{name: "named native function is a legal nonmatch", body: `sink(named)`},
		{name: "captured callback replaced by anonymous function", body: `func(){cb=func(Item,error)bool{return false};sink(cb)}()`},
		{name: "conditional anonymous replacement is unproved", body: `func(){if flag{cb=func(Item,error)bool{return false}};sink(cb)}()`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := functionOperandPrelude + `func Entry(cb func(Item,error)bool,flag bool){` + row.body + `}`
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			loanErr := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				call := functionOperandSink(ctx, t, view)
				fn := call.Parent()
				var parameter ssa.Value
				if fn == view.Package.Func("Entry") {
					parameter = fn.Params[0]
				} else {
					for _, capture := range fn.FreeVars {
						if capture.Name() == "cb" {
							parameter = capture
						}
					}
				}
				if parameter == nil {
					t.Fatalf("callback binding=%v in %v, want native parameter/capture", parameter, fn)
				}
				got, err := view.ArgumentOperand(ctx, gotoolchain.SSAArgumentOperandRequest{Function: fn, Value: call.Common().Args[0], Argument: parameter})
				if err != nil || got.Proven != row.want {
					t.Fatalf("function operand=%+v/%v, want proven=%t and no refusal", got, err, row.want)
				}
				return nil
			}})
			if loanErr != nil {
				t.Fatal(loanErr)
			}
		})
	}
}

func functionOperandSink(ctx context.Context, t testing.TB, view gotoolchain.SSAUnitView) *ssa.Call {
	t.Helper()
	var selected *ssa.Call
	for instruction, err := range view.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: view.Package.Func("Entry"), Extent: gotoolchain.SSAInstructionExtentLexical}) {
		if err != nil {
			t.Fatal(err)
		}
		call, ok := instruction.(*ssa.Call)
		if !ok || call.Common().StaticCallee() != view.Package.Func("sink") {
			continue
		}
		if selected != nil || len(call.Common().Args) != 1 {
			t.Fatalf("sink=%v/%v, want one native operand", selected, call)
		}
		selected = call
	}
	if selected == nil {
		t.Fatalf("selected sink=%v, want one native call", selected)
	}
	return selected
}

func TestSSAArgumentOperandRefusesCopiedNativeFunction(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), functionOperandPrelude+`func Entry(cb func(Item,error)bool,flag bool){sink(func(Item,error)bool{return false})}`)
	loanErr := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		call := functionOperandSink(ctx, t, view)
		fn := call.Parent()
		native, ok := call.Common().Args[0].(*ssa.Function)
		if !ok {
			t.Fatalf("native function=%T, want anonymous declaration", call.Common().Args[0])
		}
		copy := *native
		got, err := view.ArgumentOperand(ctx, gotoolchain.SSAArgumentOperandRequest{Function: fn, Value: &copy, Argument: fn.Params[0]})
		if got.Proven || !errors.Is(err, core.ErrGoToolchainContract) {
			t.Fatalf("copied function=%+v/%v, want zero contract refusal", got, err)
		}
		return nil
	}})
	if loanErr != nil {
		t.Fatal(loanErr)
	}
}

// Compiler declarations and conditional callback replacement are the external
// input. Aliases cannot turn a distinct declaration into parameter identity.
func FuzzSSAArgumentOperandNativeFunctionNonmatches(f *testing.F) {
	for mode := uint8(0); mode < 4; mode++ {
		f.Add(mode, uint16(0))
	}
	f.Fuzz(func(t *testing.T, mode uint8, suffix uint16) {
		bodies := [...]string{
			`sink(func(Item,error)bool{return false})`,
			`sink(named)`,
			`func(){cb=func(Item,error)bool{return false};sink(cb)}()`,
			`func(){if flag{cb=func(Item,error)bool{return false}};sink(cb)}()`,
		}
		source := functionOperandPrelude + `func Entry(cb func(Item,error)bool,flag bool){` + bodies[mode%4] + `}`
		source = strings.ReplaceAll(source, "Item", fmt.Sprintf("Item%d", suffix))
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		loanErr := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			call := functionOperandSink(ctx, t, view)
			fn := call.Parent()
			var argument ssa.Value
			if fn == view.Package.Func("Entry") {
				argument = fn.Params[0]
			} else {
				for _, capture := range fn.FreeVars {
					if capture.Name() == "cb" {
						argument = capture
					}
				}
			}
			if argument == nil {
				t.Fatalf("callback binding=%v, want native parameter/capture", argument)
			}
			got, err := view.ArgumentOperand(ctx, gotoolchain.SSAArgumentOperandRequest{Function: fn, Value: call.Common().Args[0], Argument: argument})
			if err != nil || got.Proven {
				t.Fatalf("mode=%d suffix=%d operand=%+v/%v, want false/nil", mode, suffix, got, err)
			}
			return nil
		}})
		if loanErr != nil {
			t.Fatal(loanErr)
		}
	})
}
