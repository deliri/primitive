package gotoolchain_test

import (
	"context"
	"errors"
	"go/token"
	"io"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const nativeBinaryOperandsSource = `package sample;func Entry(a,b int)int{return a+b};func Other(a,b int)int{return a+b};func Mixed(a,b int,flag bool)int{if flag{return a+b};return a-b}`

func nativeBinaryOperandsAdd(t testing.TB, fn *ssa.Function) *ssa.BinOp {
	t.Helper()
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			if op, ok := instruction.(*ssa.BinOp); ok && op.Op == token.ADD {
				return op
			}
		}
	}
	t.Fatalf("binary fixture got no addition in %v, want native operation", fn)
	return nil
}

func TestSSABinaryOperandsUsesPrimitiveCatalogOwner(t *testing.T) {
	t.Parallel()
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("binary owner got=%+v,%v, want compiled capability,nil", owner, err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSABinaryOperandsRequest]().PkgPath() != path {
		t.Fatalf("binary owner got=%s,%v, want%s,nil", reflect.TypeFor[gotoolchain.SSABinaryOperandsRequest]().PkgPath(), err, path)
	}
}

func TestSSABinaryOperandsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name             string
		hostile, untaken bool
	}{
		{"native inputs", false, false}, {"hostile second input", true, false}, {"neutral untaken", false, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := nativeBinaryOperandsSource
			if row.untaken {
				source = `package sample;func Entry(a,b int)int{if false{return a+b};return a};func Other(a,b int)int{return a+b}`
			}
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				op := nativeBinaryOperandsAdd(t, fn)
				if row.hostile {
					op.Y = v.Package.Func("Other").Params[1]
				}
				got, queryErr := v.BinaryOperands(ctx, gotoolchain.SSABinaryOperandsRequest{Function: fn, Operation: op})
				if row.hostile {
					if got != (gotoolchain.SSABinaryOperandsObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
						t.Fatalf("binary refusal got=%+v,%v, want zero,typed output refusal", got, queryErr)
					}
					return nil
				}
				if row.untaken {
					if got != (gotoolchain.SSABinaryOperandsObservation{}) || queryErr != nil {
						t.Fatalf("untaken binary got=%+v,%v, want zero,nil", got, queryErr)
					}
					return nil
				}
				if queryErr != nil || got.Left != fn.Params[0] || got.Right != fn.Params[1] || got.Validate() != nil {
					t.Fatalf("binary inputs got=%+v,%v, want exact native parameters,nil", got, queryErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSABinaryOperandsRefusesBrokenNativeCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		damage uint8
		want   error
	}{
		{"missing function", 0, core.ErrGoToolchainContract}, {"missing operation", 1, core.ErrGoToolchainContract}, {"foreign function", 2, core.ErrGoToolchainContract}, {"detached operation", 3, core.ErrGoToolchainContract},
		{"foreign left input", 4, core.ErrGoToolchainOutput}, {"foreign right input", 5, core.ErrGoToolchainOutput}, {"nil left input", 6, core.ErrGoToolchainOutput}, {"typed nil right input", 7, core.ErrGoToolchainOutput},
		{"detached left input", 8, core.ErrGoToolchainOutput}, {"missing loan", 9, core.ErrGoToolchainContract}, {"native cancellation", 10, io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeBinaryOperandsSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				op := nativeBinaryOperandsAdd(t, fn)
				r := gotoolchain.SSABinaryOperandsRequest{Function: fn, Operation: op}
				switch row.damage {
				case 0:
					r.Function = nil
				case 1:
					r.Operation = nil
				case 2:
					r.Function = v.Package.Func("Other")
				case 3:
					copy := *op
					r.Operation = &copy
				case 4:
					op.X = v.Package.Func("Other").Params[0]
				case 5:
					op.Y = v.Package.Func("Other").Params[1]
				case 6:
					op.X = nil
				case 7:
					op.Y = (*ssa.Parameter)(nil)
				case 8:
					copy := *fn.Params[0]
					op.X = &copy
				case 9:
					v = gotoolchain.SSAUnitView{}
				case 10:
					query, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					ctx = query
				default:
					t.Fatalf("binary damage got=%d, want named boundary", row.damage)
				}
				got, queryErr := v.BinaryOperands(ctx, r)
				if got != (gotoolchain.SSABinaryOperandsObservation{}) || !errors.Is(queryErr, row.want) {
					t.Fatalf("binary custody got=%+v,%v, want zero,%v", got, queryErr, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
