package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func nativeArgumentOperandMode(t testing.TB, directory string, mode uint8) {
	t.Helper()
	mode %= 36
	form := mode % 6
	foreign := (mode/6)%2 != 0
	damage := mode / 12
	selected := "r"
	if foreign {
		selected = "other"
	}
	body := "sink(" + selected + ")"
	parameters := "r,other *Runner"
	var path []int
	switch form {
	case 1:
		body = "sink(Named(" + selected + "))"
	case 2:
		body = "var i Invoker=" + selected + ";sink(i)"
	case 3:
		body = "func(){sink(" + selected + ");r=other}()"
	case 4:
		body = "func(){r=other;sink(" + selected + ")}()"
	case 5:
		parameters = "r,other *Owner"
		body = "sink(" + selected + ".Runner)"
		path = []int{0}
	}
	a := nativeSymbolSourceAnalysis(t, directory, nativeArgumentOperandPrelude+`func Entry(`+parameters+`){`+body+`}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		var argument ssa.Value = fn.Params[0]
		if form == 3 || form == 4 {
			fn = fn.AnonFuncs[0]
			argument = nil
			for _, capture := range fn.FreeVars {
				if capture.Name() == "r" {
					argument = capture
				}
			}
			if argument == nil {
				t.Fatalf("capture fixture got no selected r, want native reference")
			}
		}
		box := nativeArgumentOperandSink(t, fn).Common().Args[0]
		value := box
		if form == 5 {
			boxed, ok := box.(*ssa.MakeInterface)
			if !ok {
				t.Fatalf("promoted fixture got=%T, want native boxing", box)
			}
			value = boxed.X
		}
		request := gotoolchain.SSAArgumentOperandRequest{Function: fn, Value: value, Argument: argument, Path: path}
		want := !foreign && form != 4
		// The source's selected receiver and write ordering pin the oracle before
		// mutation; no production matcher is used to calculate the answer.
		before, err := v.ArgumentOperand(ctx, request)
		if err != nil || before.Proven != want {
			t.Fatalf("mode=%d baseline got=%+v,%v, want proven=%t,nil", mode, before, err, want)
		}
		switch damage {
		case 0:
			return nil
		case 1:
			switch arg := argument.(type) {
			case *ssa.Parameter:
				copy := *arg
				request.Argument = &copy
			case *ssa.FreeVar:
				copy := *arg
				request.Argument = &copy
			default:
				t.Fatalf("argument got=%T, want native parameter or capture", argument)
			}
			if request.Argument == argument {
				t.Fatalf("argument mutation got unchanged identity, want detached clone")
			}
		case 2:
			request.Value = box
			var old ssa.Value
			switch representation := box.(type) {
			case *ssa.MakeInterface:
				old = representation.X
				representation.X = nil
			case *ssa.ChangeInterface:
				old = representation.X
				representation.X = nil
			default:
				t.Fatalf("semantic representation got=%T, want native boxing or interface change", box)
			}
			if old == nil {
				t.Fatalf("boxing mutation got nil prior operand, want nonvacuous mutation")
			}
		}
		got, err := v.ArgumentOperand(ctx, request)
		wantErr := error(core.ErrGoToolchainContract)
		if damage == 2 {
			wantErr = core.ErrGoToolchainOutput
		}
		if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(err, wantErr) || damage == 1 && errors.Is(err, core.ErrGoToolchainOutput) {
			t.Fatalf("mode=%d mutation got=%+v,%v, want zero,%v", mode, got, err, wantErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSAArgumentOperandFiniteSemanticDomain(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 36; mode++ {
		t.Run(fmt.Sprint(mode), func(t *testing.T) { t.Parallel(); nativeArgumentOperandMode(t, t.TempDir(), mode) })
	}
}

func FuzzSSAArgumentOperandConservesNativeIdentity(f *testing.F) {
	f.Add(uint8(0))
	f.Fuzz(func(t *testing.T, mode uint8) { nativeArgumentOperandMode(t, t.TempDir(), mode) })
}
