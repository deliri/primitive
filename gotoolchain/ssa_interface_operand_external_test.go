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

func TestSSAConversionOperandLendsNativeInterfaceInput(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, source string }{
		{"concrete error boxing", `package sample;type Problem struct{};func(*Problem)Error()string{return "detail"};func Entry(value *Problem)error{return value}`},
		{"wider interface conversion", `package sample;type Rich interface{Error()string;Extra()};func Entry(value Rich)error{return value}`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), row.source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				fn := view.Package.Func("Entry")
				var conversion ssa.Value
				for instruction, err := range view.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
					if err != nil {
						return err
					}
					switch actual := instruction.(type) {
					case *ssa.MakeInterface:
						conversion = actual
					case *ssa.ChangeInterface:
						conversion = actual
					}
				}
				if conversion == nil {
					t.Fatal("interface fixture has no actual native representation edge")
				}
				observation, err := view.ConversionOperand(ctx, gotoolchain.SSAConversionOperandRequest{Function: fn, Conversion: conversion})
				if err != nil || observation.Operand != fn.Params[0] || observation.Validate() != nil {
					t.Fatalf("interface operand got=%+v,%v, want exact native parameter,nil", observation, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func nativeInterfaceConversion(t testing.TB, fn *ssa.Function) ssa.Value {
	t.Helper()
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			switch value := instruction.(type) {
			case *ssa.MakeInterface:
				return value
			case *ssa.ChangeInterface:
				return value
			}
		}
	}
	t.Fatal("fixture has no native interface conversion")
	return nil
}

func TestSSAInterfaceOperandCustodyAndUntakenTriad(t *testing.T) {
	t.Parallel()
	for _, form := range []struct{ name, declarations, from string }{
		{"boxing", `type Problem struct{};func(*Problem)Error()string{return "detail"};`, "*Problem"},
		{"interface change", `type Rich interface{Error()string;Extra()};`, "Rich"},
	} {
		t.Run(form.name, func(t *testing.T) {
			t.Parallel()
			for _, row := range []struct {
				name string
				kind uint8
				want error
			}{
				{"native", 0, nil}, {"foreign input", 1, core.ErrGoToolchainOutput},
				{"untaken", 2, nil}, {"cloned instruction", 3, core.ErrGoToolchainContract},
				{"nil input", 4, core.ErrGoToolchainOutput}, {"typed nil input", 5, core.ErrGoToolchainOutput},
				{"self input", 6, core.ErrGoToolchainOutput}, {"wrong function", 7, core.ErrGoToolchainContract},
				{"typed nil request", 8, core.ErrGoToolchainContract}, {"missing view", 9, core.ErrGoToolchainContract},
				{"cancelled", 10, io.ErrClosedPipe},
			} {
				t.Run(row.name, func(t *testing.T) {
					t.Parallel()
					body := "return value"
					if row.kind == 2 {
						body = "if false{return value};return nil"
					}
					a := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;"+form.declarations+"func Entry(value "+form.from+")error{"+body+"};func Other(value "+form.from+")error{return value}")
					err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
						fn := view.Package.Func("Entry")
						value := nativeInterfaceConversion(t, fn)
						request := gotoolchain.SSAConversionOperandRequest{Function: fn, Conversion: value}
						var replacement ssa.Value
						switch row.kind {
						case 1:
							replacement = view.Package.Func("Other").Params[0]
						case 5:
							replacement = (*ssa.Parameter)(nil)
						case 6:
							replacement = value
						case 7:
							request.Function = view.Package.Func("Other")
						case 9:
							view = gotoolchain.SSAUnitView{}
						case 10:
							var cancel context.CancelCauseFunc
							ctx, cancel = context.WithCancelCause(ctx)
							cancel(row.want)
						}
						switch operation := value.(type) {
						case *ssa.MakeInterface:
							if row.kind == 3 {
								copy := *operation
								request.Conversion = &copy
							}
							if row.kind == 8 {
								request.Conversion = (*ssa.MakeInterface)(nil)
							}
							if row.kind == 1 || row.kind == 4 || row.kind == 5 || row.kind == 6 {
								operation.X = replacement
							}
						case *ssa.ChangeInterface:
							if row.kind == 3 {
								copy := *operation
								request.Conversion = &copy
							}
							if row.kind == 8 {
								request.Conversion = (*ssa.ChangeInterface)(nil)
							}
							if row.kind == 1 || row.kind == 4 || row.kind == 5 || row.kind == 6 {
								operation.X = replacement
							}
						}
						got, err := view.ConversionOperand(ctx, request)
						var want ssa.Value
						if row.kind == 0 {
							want = fn.Params[0]
						}
						if got.Operand != want || !errors.Is(err, row.want) || got.Validate() != nil {
							t.Fatalf("interface input got=%+v,%v, want exact operand=%v,%v", got, err, want, row.want)
						}
						return nil
					}})
					if err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
