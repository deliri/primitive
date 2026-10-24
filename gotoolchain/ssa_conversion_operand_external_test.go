package gotoolchain_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func nativeConversionValue(t testing.TB, fn *ssa.Function) ssa.Value {
	t.Helper()
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			switch value := instruction.(type) {
			case *ssa.ChangeType:
				return value
			case *ssa.Convert:
				return value
			}
		}
	}
	t.Fatalf("conversion fixture got no conversion in %v, want native Convert or ChangeType", fn)
	return nil
}

func TestSSAConversionOperandUsesPrimitiveCatalogOwner(t *testing.T) {
	t.Parallel()
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("conversion owner got=%+v,%v, want compiler capability,nil", owner, err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSAConversionOperandRequest]().PkgPath() != path {
		t.Fatalf("conversion owner got=%s,%v, want %s,nil", reflect.TypeFor[gotoolchain.SSAConversionOperandRequest]().PkgPath(), err, path)
	}
}

func TestSSAConversionOperandLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name             string
		hostile, untaken bool
	}{{"native input", false, false}, {"foreign input", true, false}, {"untaken conversion", false, true}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			body := "return int64(a)"
			if row.untaken {
				body = "if false{return int64(a)};return 0"
			}
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(a int)int64{`+body+`};func Other(a int)int64{return int64(a)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				value := nativeConversionValue(t, fn)
				if row.hostile {
					value.(*ssa.Convert).X = v.Package.Func("Other").Params[0]
				}
				got, err := v.ConversionOperand(ctx, gotoolchain.SSAConversionOperandRequest{Function: fn, Conversion: value})
				var wantErr error
				var want ssa.Value = fn.Params[0]
				if row.hostile {
					wantErr, want = core.ErrGoToolchainOutput, nil
				}
				if row.untaken {
					want = nil
				}
				if got.Operand != want || !errors.Is(err, wantErr) || got.Validate() != nil {
					t.Fatalf("conversion got=%+v,%v, want operand=%v,%v", got, err, want, wantErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAConversionOperandRefusesBrokenNativeCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		kind uint8
		want error
	}{
		{"missing owner", 0, core.ErrGoToolchainContract},
		{"missing conversion", 1, core.ErrGoToolchainContract},
		{"typed nil conversion", 2, core.ErrGoToolchainContract},
		{"different operation kind", 3, core.ErrGoToolchainContract},
		{"foreign function", 4, core.ErrGoToolchainContract},
		{"cloned conversion", 5, core.ErrGoToolchainContract},
		{"foreign input", 6, core.ErrGoToolchainOutput},
		{"nil input", 7, core.ErrGoToolchainOutput},
		{"typed nil input", 8, core.ErrGoToolchainOutput},
		{"native cancellation cause", 9, io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(a int)int64{return int64(a)};func Other(a int)int64{return int64(a)}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				value := nativeConversionValue(t, fn).(*ssa.Convert)
				request := gotoolchain.SSAConversionOperandRequest{Function: fn, Conversion: value}
				switch row.kind {
				case 0:
					request.Function = nil
				case 1:
					request.Conversion = nil
				case 2:
					request.Conversion = (*ssa.Convert)(nil)
				case 3:
					request.Conversion = fn.Params[0]
				case 4:
					request.Function = v.Package.Func("Other")
				case 5:
					copied := *value
					request.Conversion = &copied
				case 6:
					value.X = v.Package.Func("Other").Params[0]
				case 7:
					value.X = nil
				case 8:
					value.X = (*ssa.Parameter)(nil)
				case 9:
					var cancel context.CancelCauseFunc
					ctx, cancel = context.WithCancelCause(ctx)
					cancel(row.want)
				default:
					t.Fatalf("conversion hostile selector got=%d, want declared case", row.kind)
				}
				got, err := v.ConversionOperand(ctx, request)
				if got != (gotoolchain.SSAConversionOperandObservation{}) || !errors.Is(err, row.want) {
					t.Fatalf("conversion refusal got=%+v,%v, want zero,%v", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
