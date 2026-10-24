package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"io"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const nativeMethodReceiverDeclarations = `package sample;type sample struct{};func(*sample)M(*sample)int{return 1};func(sample)V()int{return 1};type outer struct{*sample};var global sample;func plain(*sample)int{return 1};`

// Bind the compiler-visible API location to Primitive's existing capability
// authority. No product registry or second package/effect catalog is needed.
func TestSSAMethodReceiverUsesCatalogOwner(t *testing.T) {
	t.Parallel()
	match, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("receiver capability resolution error=%v, want nil", err)
	}
	path, err := match.Capability.ImportPath()
	if err != nil {
		t.Fatalf("receiver capability import path error=%v, want nil", err)
	}
	for _, contract := range []reflect.Type{reflect.TypeFor[gotoolchain.SSAMethodReceiverRequest](), reflect.TypeFor[gotoolchain.SSAMethodReceiverObservation]()} {
		if contract.PkgPath() != path {
			t.Fatalf("receiver contract %s owner=%s, want catalog path %s", contract, contract.PkgPath(), path)
		}
	}
}

func TestSSAMethodReceiverConservesActualOperand(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, parameters, body, method string
		parameter                      int
	}{
		{"direct receiver", `r,other *sample`, `_=r.M(other)`, "M", 0},
		{"direct foreign operand stays distinct", `r,other *sample`, `_=other.M(r)`, "M", 1},
		{"bound receiver differs from explicit argument", `r,other *sample`, `bound:=other.M;_=bound(r)`, "M", 1},
		{"method expression receiver precedes arguments", `r,other *sample`, `_=(*sample).M(r,other)`, "M", 0},
		{"promoted expression lends outer operand", `r *outer,other *sample`, `_=(*outer).M(r,other)`, "M", 0},
		{"interface receiver", `r interface{M(*sample)int},other *sample`, `_=r.M(other)`, "M", 0},
		{"bound interface receiver", `r interface{M(*sample)int},other *sample`, `bound:=r.M;_=bound(other)`, "M", 0},
		{"value receiver", `r sample`, `_=r.V()`, "V", 0},
		{"builtin error receiver has no package", `r error`, `_=r.Error()`, "Error", 0},
		{"global receiver", `r *sample`, `_=global.M(r)`, "M", -1},
		{"ordinary function has no receiver", `r *sample`, `_=plain(r)`, "", 0},
		{"opaque function has no invented receiver", `r *sample,fn func(*sample)int`, `_=fn(r)`, "", 0},
		{"lexical capture has no method authority", `r *sample`, `fn:=func()int{return plain(r)};_=fn()`, "", 0},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeMethodReceiverDeclarations+`func Entry(`+row.parameters+`){`+row.body+`}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				call := nativeReceiverParameterLastCall(ctx, t, v, fn)
				got, queryErr := v.MethodReceiver(ctx, gotoolchain.SSAMethodReceiverRequest{Call: call})
				if queryErr != nil || got.Validate() != nil {
					t.Fatalf("method receiver=%+v,%v validation=%v, want native observation,nil", got, queryErr, got.Validate())
				}
				if row.method == "" {
					if got != (gotoolchain.SSAMethodReceiverObservation{}) {
						t.Fatalf("ordinary callable=%+v, want absent method and receiver", got)
					}
					return nil
				}
				var want ssa.Value
				if row.parameter < 0 {
					want = v.Package.Members["global"].(*ssa.Global)
				} else {
					want = fn.Params[row.parameter]
				}
				if got.Method == nil || got.Method.Name() != row.method || got.Receiver != want {
					t.Fatalf("method receiver=%+v, want method %s and actual operand %v", got, row.method, want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSAMethodReceiverRefusesDamagedNativeFacts(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		mutate func(*gotoolchain.SSAUnitView, *gotoolchain.SSAMethodReceiverRequest)
		want   error
	}{
		{"absent loan", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSAMethodReceiverRequest) {
			*v = gotoolchain.SSAUnitView{}
		}, core.ErrGoToolchainContract},
		{"absent selected call", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) { r.Call = nil }, core.ErrGoToolchainContract},
		{"parentless call", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) { r.Call = &ssa.Call{} }, core.ErrGoToolchainContract},
		{"detached selected call", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) {
			copy := *r.Call.(*ssa.Call)
			r.Call = &copy
		}, core.ErrGoToolchainContract},
		{"foreign call program", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) { r.Call.Parent().Prog = nil }, core.ErrGoToolchainContract},
		{"damaged block ordinal", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) { r.Call.Block().Index = -1 }, core.ErrGoToolchainOutput},
		{"missing callee value", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) { r.Call.Common().Value = nil }, core.ErrGoToolchainOutput},
		{"typed nil callee value", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) {
			r.Call.Common().Value = (*ssa.Function)(nil)
		}, core.ErrGoToolchainOutput},
		{"missing static signature", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) {
			r.Call.Common().StaticCallee().Signature = nil
		}, core.ErrGoToolchainOutput},
		{"missing receiver slot", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) { r.Call.Common().Args = nil }, core.ErrGoToolchainOutput},
		{"nil receiver", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) {
			r.Call.Common().Args[0] = nil
		}, core.ErrGoToolchainOutput},
		{"typed nil receiver", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) {
			r.Call.Common().Args[0] = (*ssa.Parameter)(nil)
		}, core.ErrGoToolchainOutput},
		{"foreign receiver", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) {
			r.Call.Common().Args[0] = v.Package.Func("Other").Params[0]
		}, core.ErrGoToolchainOutput},
		{"detached receiver", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) {
			copy := *r.Call.(*ssa.Call).Parent().Params[0]
			r.Call.Common().Args[0] = &copy
		}, core.ErrGoToolchainOutput},
		{"foreign package global", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSAMethodReceiverRequest) {
			g := v.Package.Members["global"].(*ssa.Global)
			g.Pkg = &ssa.Package{Prog: v.Program, Pkg: v.Source.Types, Members: map[string]ssa.Member{"global": g}}
			r.Call.Common().Args[0] = g
		}, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeMethodReceiverDeclarations+`func Entry(r *sample){_=r.M(r)};func Other(r *sample){}`)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				r := gotoolchain.SSAMethodReceiverRequest{Call: nativeReceiverParameterLastCall(ctx, t, v, v.Package.Func("Entry"))}
				row.mutate(&v, &r)
				got, queryErr := v.MethodReceiver(ctx, r)
				if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, row.want) || errors.Is(queryErr, core.ErrGoToolchainOutput) != errors.Is(row.want, core.ErrGoToolchainOutput) {
					t.Fatalf("method refusal=%+v,%v, want zero,%v", got, queryErr, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}

func TestSSAMethodReceiverPreservesCancellationAndObservationClosure(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeMethodReceiverDeclarations+`func Entry(r *sample){_=r.M(r)}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		r := gotoolchain.SSAMethodReceiverRequest{Call: nativeReceiverParameterLastCall(ctx, t, v, v.Package.Func("Entry"))}
		cancelled, cancel := context.WithCancelCause(ctx)
		cancel(io.ErrClosedPipe)
		for _, row := range []struct {
			ctx  context.Context
			want error
		}{{nil, core.ErrNilContext}, {cancelled, context.Canceled}} {
			got, queryErr := v.MethodReceiver(row.ctx, r)
			if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, row.want) || row.ctx == cancelled && !errors.Is(queryErr, io.ErrClosedPipe) {
				t.Fatalf("cancelled method receiver=%+v,%v, want zero,%v with cause", got, queryErr, row.want)
			}
		}
		parent, stop := context.WithCancelCause(ctx)
		defer stop(nil)
		late := &blockLinkTerminalContext{Context: parent, cancel: stop}
		got, queryErr := v.MethodReceiver(late, r)
		if got != (gotoolchain.SSAMethodReceiverObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
			t.Fatalf("late method receiver=%+v,%v, want zero,cancelled with cause", got, queryErr)
		}
		got, queryErr = v.MethodReceiver(ctx, r)
		if queryErr != nil || got.Validate() != nil {
			t.Fatalf("native method observation=%+v,%v, want valid observation,nil", got, queryErr)
		}
		for _, observation := range []gotoolchain.SSAMethodReceiverObservation{
			{Method: got.Method}, {Receiver: got.Receiver}, {Method: got.Method, Receiver: (*ssa.Parameter)(nil)},
			{Method: types.NewFunc(0, nil, "plain", types.NewSignatureType(nil, nil, nil, nil, nil, false)), Receiver: got.Receiver},
		} {
			if !errors.Is(observation.Validate(), core.ErrGoToolchainOutput) {
				t.Fatalf("damaged observation=%+v validation=%v, want output refusal", observation, observation.Validate())
			}
		}
		if err := (gotoolchain.SSAMethodReceiverObservation{}).Validate(); err != nil {
			t.Fatalf("absent observation error=%v, want nil", err)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit error=%v, want nil", err)
	}
}
