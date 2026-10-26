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

type conversionFunctionDamage uint8

const (
	conversionFunctionIntact conversionFunctionDamage = iota
	conversionFunctionClone
	conversionFunctionForeignProgram
	conversionFunctionForeignLexicalParent
	conversionFunctionWrongSignature
	conversionFunctionNilSignature
	conversionFunctionNil
	conversionFunctionTypedNil
	conversionFunctionCancelled
)

func TestSSAConversionOperandAuthenticatesNativeFunctionValues(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, source string
	}{
		{"package declaration", `package sample;type Callback func();func Source(){};func Entry()Callback{return Callback(Source)}`},
		{"noncapturing literal", `package sample;type Callback func();func Entry()Callback{return Callback(func(){})}`},
		{"captured literal retains instruction ownership", `package sample;type Callback func();func Entry()Callback{value:=0;return Callback(func(){value++})}`},
		{"generic native instance", `package sample;type Callback func();func Source[T any](){var value T;_=value};func Entry()Callback{return Callback(Source[int])}`},
		{"imported declaration", `package sample;import "strings";type Callback func(string)string;func Entry()Callback{return Callback(strings.TrimSpace)}`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), row.source)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				function := view.Package.Func("Entry")
				conversion := nativeConversionValue(t, function).(*ssa.ChangeType)
				want := conversion.X
				if _, functionValue := want.(*ssa.Function); !functionValue {
					if _, captured := want.(*ssa.MakeClosure); !captured {
						t.Fatalf("conversion source=%T, want actual native function or captured closure", want)
					}
				}
				observation, queryErr := view.ConversionOperand(ctx, gotoolchain.SSAConversionOperandRequest{Function: function, Conversion: conversion})
				if queryErr != nil || observation.Validate() != nil || observation.Operand != want {
					t.Fatalf("native function conversion=%+v error=%v, want original compiler operand=%v and nil", observation, queryErr, want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("native function source loan error=%v, want nil", err)
			}
		})
	}
}

func TestSSAConversionFunctionIdentityRefusesDamagedLoans(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		damage conversionFunctionDamage
	}{
		{"copied function identity", conversionFunctionClone},
		{"other compiler program", conversionFunctionForeignProgram},
		{"other lexical parent", conversionFunctionForeignLexicalParent},
		{"incompatible function signature", conversionFunctionWrongSignature},
		{"missing native function signature", conversionFunctionNilSignature},
		{"missing native operand", conversionFunctionNil},
		{"typed nil native function", conversionFunctionTypedNil},
		{"native cancellation cause", conversionFunctionCancelled},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Callback func();func Source(){};func Wrong(int){};func Entry()Callback{return Callback(Source)};func Other()Callback{return Callback(func(){})}`)
			var foreign gotoolchain.PackageAnalysis
			if row.damage == conversionFunctionForeignProgram {
				foreign = nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Source(){}`)
			}
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				function := view.Package.Func("Entry")
				conversion := nativeConversionValue(t, function).(*ssa.ChangeType)
				wantErr := error(core.ErrGoToolchainOutput)
				switch row.damage {
				case conversionFunctionClone:
					copied := *conversion.X.(*ssa.Function)
					conversion.X = &copied
				case conversionFunctionForeignProgram:
					if err := foreign.WithSSAUnit(ctx, gotoolchain.SSAUnitRequest{Unit: foreign.Units[0], Use: func(_ context.Context, other gotoolchain.SSAUnitView) error {
						conversion.X = other.Package.Func("Source")
						return nil
					}}); err != nil {
						return err
					}
				case conversionFunctionForeignLexicalParent:
					conversion.X = nativeConversionValue(t, view.Package.Func("Other")).(*ssa.ChangeType).X
				case conversionFunctionWrongSignature:
					conversion.X = view.Package.Func("Wrong")
				case conversionFunctionNilSignature:
					conversion.X.(*ssa.Function).Signature = nil
				case conversionFunctionNil:
					conversion.X = nil
				case conversionFunctionTypedNil:
					conversion.X = (*ssa.Function)(nil)
				case conversionFunctionCancelled:
					cancelled, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					ctx, wantErr = cancelled, io.ErrClosedPipe
				default:
					t.Fatalf("function conversion damage=%d, want declared hostile identity", row.damage)
				}
				observation, queryErr := view.ConversionOperand(ctx, gotoolchain.SSAConversionOperandRequest{Function: function, Conversion: conversion})
				if observation != (gotoolchain.SSAConversionOperandObservation{}) || !errors.Is(queryErr, wantErr) || row.damage == conversionFunctionCancelled && !errors.Is(queryErr, context.Canceled) {
					t.Fatalf("damaged function conversion=%+v error=%v, want zero observation and %v", observation, queryErr, wantErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("native damaged-function source loan error=%v, want nil", err)
			}
		})
	}
}

func BenchmarkSSAConversionFunctionOperand(b *testing.B) {
	b.ReportAllocs()
	analysis := nativeSymbolSourceAnalysis(b, b.TempDir(), `package sample;type Callback func();func Entry()Callback{return Callback(func(){})}`)
	err := analysis.WithSSAUnit(b.Context(), gotoolchain.SSAUnitRequest{Unit: analysis.Units[0], Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		function := view.Package.Func("Entry")
		conversion := nativeConversionValue(b, function).(*ssa.ChangeType)
		want := conversion.X
		for b.Loop() {
			observation, queryErr := view.ConversionOperand(ctx, gotoolchain.SSAConversionOperandRequest{Function: function, Conversion: conversion})
			if queryErr != nil || observation.Operand != want {
				b.Fatalf("function conversion=%+v error=%v, want native operand=%v and nil", observation, queryErr, want)
			}
		}
		return nil
	}})
	if err != nil {
		b.Fatalf("native function benchmark loan error=%v, want nil", err)
	}
}
