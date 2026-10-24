package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAArgumentOperandRefusesMalformedNativeField(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		ordinal int
	}{{"negative native field", -1}, {"field at record extent", 2}, {"field beyond record extent", 3}, {"maximum native field", int(^uint(0) >> 1)}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeArgumentOperandPrelude+`func Entry(r *Owner){sink(r.Runner)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				value := nativeArgumentOperandSink(t, fn).Common().Args[0].(*ssa.MakeInterface).X
				read, ok := value.(*ssa.UnOp)
				if !ok {
					t.Fatalf("field fixture got=%T, want native field load", value)
				}
				address, ok := read.X.(*ssa.FieldAddr)
				if !ok || address.Field != 0 {
					t.Fatalf("field fixture got=%v, want native first field", read.X)
				}
				request := gotoolchain.SSAArgumentOperandRequest{Function: fn, Value: value, Argument: fn.Params[0], Path: []int{0}}
				before, err := v.ArgumentOperand(ctx, request)
				if err != nil || !before.Proven {
					t.Fatalf("field baseline got=%+v,%v, want proven,nil", before, err)
				}
				address.Field = row.ordinal
				got, err := v.ArgumentOperand(ctx, request)
				if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("native field got=%+v,%v, want zero,output refusal", got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAArgumentOperandAuthenticatesNativeFieldForms(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, parameter string
		address, damage bool
	}{{"borrowed direct field address", "*Owner", true, false}, {"bad direct field address", "*Owner", true, true}, {"borrowed value record field", "Owner", false, false}, {"bad value record field", "Owner", false, true}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := nativeArgumentOperandPrelude + `func Entry(r ` + row.parameter + `){sink(r.Runner)}`
			if !row.address {
				source = nativeArgumentOperandPrelude + `func owner() Owner{return Owner{}};func Entry(r *Runner){sink(owner().Runner)}`
			}
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				value := nativeArgumentOperandSink(t, fn).Common().Args[0].(*ssa.MakeInterface).X
				if row.address {
					read, ok := value.(*ssa.UnOp)
					if !ok {
						t.Fatalf("field address got=%T, want native read", value)
					}
					value = read.X
				}
				argument := ssa.Value(fn.Params[0])
				if !row.address {
					field, ok := value.(*ssa.Field)
					if !ok {
						t.Fatalf("value field fixture got=%T, want native Field of returned record", value)
					}
					argument = field.X
				}
				request := gotoolchain.SSAArgumentOperandRequest{Function: fn, Value: value, Argument: argument, Path: []int{0}}
				before, err := v.ArgumentOperand(ctx, request)
				if err != nil || !before.Proven {
					t.Fatalf("field form baseline got=%+v,%v, want proven,nil", before, err)
				}
				if row.damage {
					switch field := value.(type) {
					case *ssa.FieldAddr:
						field.Field = 2
					case *ssa.Field:
						field.Field = 2
					default:
						t.Fatalf("field form got=%T, want native address or value field", value)
					}
				}
				got, err := v.ArgumentOperand(ctx, request)
				if row.damage {
					if got != (gotoolchain.SSARelationObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
						t.Fatalf("field form refusal got=%+v,%v, want zero,output", got, err)
					}
				} else if err != nil || !got.Proven {
					t.Fatalf("field form got=%+v,%v, want proven,nil", got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
