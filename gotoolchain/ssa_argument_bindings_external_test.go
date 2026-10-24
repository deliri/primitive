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

func TestSSAArgumentBindingsUsesPrimitiveCatalogOwner(t *testing.T) {
	t.Parallel()
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("binding owner got=%+v,%v, want compiled capability,nil", owner, err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSAArgumentBindingsRequest]().PkgPath() != path {
		t.Fatalf("binding request owner got=%s,%v, want %s,nil", reflect.TypeFor[gotoolchain.SSAArgumentBindingsRequest]().PkgPath(), err, path)
	}
}

const argumentBindingSource = `package sample;func helper(a,b *int){};func Other(a,b *int){};func Entry(a,b *int){helper(a,b)}`

func TestSSAArgumentBindingsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		hostile    bool
		want       int
	}{
		{"native argument", `helper(a,b)`, false, 1},
		{"copied argument", `x:=a;helper(x,b)`, false, 1},
		{"multiple bindings", `helper(a,a)`, false, 2},
		{"foreign argument", `helper(a,b)`, true, 0},
		{"neutral distinct operand", `helper(b,b)`, false, 0},
		{"captured operand", `func(){_=a}()`, false, 1},
		{"capture and parameter", `func(c *int){_,_=a,c}(a)`, false, 2},
		{"neutral nil constant", `helper(nil,b)`, false, 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func helper(a,b *int){};func Other(a,b *int){};func Entry(a,b *int){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeReceiverParameterLastCall(ctx, t, v, fn)
				if row.hostile {
					call.Common().Args[1] = v.Package.Func("Other").Params[1]
				}
				count, refusals, parameters := 0, 0, 0
				for got, queryErr := range v.ArgumentBindings(ctx, gotoolchain.SSAArgumentBindingsRequest{Call: call, Argument: fn.Params[0]}) {
					if row.hostile {
						if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
							t.Fatalf("hostile binding got=%+v,%v, want zero,typed output refusal", got, queryErr)
						}
						refusals++
						continue
					}
					if queryErr != nil || got.Validate() != nil || got.Function != call.Common().StaticCallee() {
						t.Fatalf("native binding got=%+v,%v, want exact callee and local,nil", got, queryErr)
					}
					if parameter, ok := got.Local.(*ssa.Parameter); ok {
						if parameter != got.Function.Params[parameters] {
							t.Fatalf("argument local got=%v, want native ordinal=%d", got.Local, parameters)
						}
						parameters++
					}
					count++
				}
				wantRefusals := 0
				if row.hostile {
					wantRefusals = 1
				}
				if count != row.want || refusals != wantRefusals {
					t.Fatalf("binding observations got=%d refusals=%d, want %d,%d", count, refusals, row.want, wantRefusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAArgumentBindingsRefusesCorruptCustodyBeforeYield(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		damage uint8
		want   error
	}{
		{"missing call", 0, core.ErrGoToolchainContract}, {"missing reference", 1, core.ErrGoToolchainContract},
		{"typed nil reference", 2, core.ErrGoToolchainContract}, {"foreign reference", 3, core.ErrGoToolchainContract},
		{"detached call", 4, core.ErrGoToolchainContract}, {"foreign trailing argument", 5, core.ErrGoToolchainOutput},
		{"nil trailing argument", 6, core.ErrGoToolchainOutput}, {"typed nil trailing argument", 7, core.ErrGoToolchainOutput},
		{"detached trailing argument", 8, core.ErrGoToolchainOutput}, {"missing argument", 9, core.ErrGoToolchainOutput},
		{"extra argument", 10, core.ErrGoToolchainOutput}, {"detached target", 11, core.ErrGoToolchainOutput},
		{"missing target", 12, core.ErrGoToolchainOutput}, {"typed nil target", 13, core.ErrGoToolchainOutput},
		{"missing loan", 14, core.ErrGoToolchainContract}, {"native cause", 15, io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), argumentBindingSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeReceiverParameterLastCall(ctx, t, v, fn)
				r := gotoolchain.SSAArgumentBindingsRequest{Call: call, Argument: fn.Params[0]}
				switch row.damage {
				case 0:
					r.Call = nil
				case 1:
					r.Argument = nil
				case 2:
					r.Argument = (*ssa.Parameter)(nil)
				case 3:
					r.Argument = v.Package.Func("Other").Params[0]
				case 4:
					copied := *call
					r.Call = &copied
				case 5:
					call.Common().Args[1] = v.Package.Func("Other").Params[1]
				case 6:
					call.Common().Args[1] = nil
				case 7:
					call.Common().Args[1] = (*ssa.Parameter)(nil)
				case 8:
					copied := *fn.Params[1]
					call.Common().Args[1] = &copied
				case 9:
					call.Common().Args = call.Common().Args[:1]
				case 10:
					call.Common().Args = append(call.Common().Args, fn.Params[0])
				case 11:
					copied := *call.Common().StaticCallee()
					call.Common().Value = &copied
				case 12:
					call.Common().Value = nil
				case 13:
					call.Common().Value = (*ssa.Function)(nil)
				case 14:
					v = gotoolchain.SSAUnitView{}
				case 15:
					cancelled, cancel := context.WithCancelCause(ctx)
					cancel(io.ErrClosedPipe)
					ctx = cancelled
				default:
					t.Fatalf("binding damage got=%d, want named native boundary", row.damage)
				}
				refusals := 0
				for got, queryErr := range v.ArgumentBindings(ctx, r) {
					if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(queryErr, row.want) {
						t.Fatalf("binding custody got=%+v,%v, want zero,%v", got, queryErr, row.want)
					}
					refusals++
				}
				if refusals != 1 {
					t.Fatalf("binding refusals got=%d, want one and no prefix", refusals)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
