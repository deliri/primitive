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

const nativePhiInputsSource = `package sample;func touch(int)int{return 0};func Entry(flag bool,a,b int)int{count:=a;if flag{count=b};_=touch(count);return count};func Other(flag bool,a,b int)int{count:=a;if flag{count=b};return count}`

func nativeReturnedPhi(t testing.TB, fn *ssa.Function) *ssa.Phi {
	t.Helper()
	for _, block := range fn.Blocks {
		for _, instruction := range block.Instrs {
			if ret, ok := instruction.(*ssa.Return); ok && len(ret.Results) == 1 {
				if merge, ok := ret.Results[0].(*ssa.Phi); ok {
					return merge
				}
			}
		}
	}
	t.Fatalf("Phi fixture got no returned merge in %v, want native Phi", fn)
	return nil
}

func TestSSAPhiInputsUsesPrimitiveCatalogOwner(t *testing.T) {
	t.Parallel()
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("Phi owner got=%+v,%v, want compiler capability,nil", owner, err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSAPhiInputsRequest]().PkgPath() != path {
		t.Fatalf("Phi owner got=%s,%v, want %s,nil", reflect.TypeFor[gotoolchain.SSAPhiInputsRequest]().PkgPath(), err, path)
	}
}

func TestSSAPhiInputsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name             string
		hostile, neutral bool
	}{{"native source inputs", false, false}, {"hostile trailing input", true, false}, {"same block is not strict ancestor", false, true}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePhiInputsSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				merge := nativeReturnedPhi(t, fn)
				r := gotoolchain.SSAPhiInputsRequest{Function: fn, Merge: merge}
				if row.hostile {
					merge.Edges[1] = v.Package.Func("Other").Params[2]
				}
				if row.neutral {
					r.AncestorOf = merge.Block()
				}
				seen, refused := 0, 0
				for got, err := range v.PhiInputs(ctx, r) {
					if row.hostile {
						if got != (gotoolchain.SSAPhiInputObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
							t.Fatalf("Phi refusal got=%+v,%v, want zero,typed refusal", got, err)
						}
						refused++
						continue
					}
					if row.neutral {
						t.Fatalf("neutral Phi got=%+v,%v, want no observation", got, err)
					}
					if err != nil || got.Validate() != nil || got.Block != merge.Block() || seen >= len(merge.Edges) || got.Value != merge.Edges[seen] {
						t.Fatalf("Phi source index=%d got=%+v,%v, want actual native input,nil", seen, got, err)
					}
					seen++
				}
				wantSeen, wantRefused := 2, 0
				if row.hostile {
					wantSeen, wantRefused = 0, 1
				}
				if row.neutral {
					wantSeen = 0
				}
				if seen != wantSeen || refused != wantRefused {
					t.Fatalf("Phi stream got=%d,%d, want %d,%d", seen, refused, wantSeen, wantRefused)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSAPhiInputsRefusesBrokenNativeCustody(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		damage uint8
		want   error
	}{
		{"missing function", 0, core.ErrGoToolchainContract}, {"missing merge", 1, core.ErrGoToolchainContract},
		{"foreign function", 2, core.ErrGoToolchainContract}, {"cloned merge", 3, core.ErrGoToolchainContract},
		{"missing incoming values", 4, core.ErrGoToolchainOutput}, {"nil trailing input", 5, core.ErrGoToolchainOutput},
		{"typed nil trailing input", 6, core.ErrGoToolchainOutput}, {"foreign trailing input", 7, core.ErrGoToolchainOutput},
		{"missing predecessor", 8, core.ErrGoToolchainOutput}, {"nil predecessor", 9, core.ErrGoToolchainOutput},
		{"cloned predecessor", 10, core.ErrGoToolchainOutput}, {"nonincoming native block", 11, core.ErrGoToolchainOutput},
		{"later native definition", 12, core.ErrGoToolchainOutput}, {"foreign ancestor", 13, core.ErrGoToolchainContract},
		{"cloned ancestor", 14, core.ErrGoToolchainOutput}, {"missing view", 15, core.ErrGoToolchainContract},
		{"native cancellation cause", 16, io.ErrClosedPipe},
		{"predecessor without terminator", 17, core.ErrGoToolchainOutput},
		{"detached input identity", 18, core.ErrGoToolchainOutput},
		{"zero incoming layout", 19, core.ErrGoToolchainOutput},
		{"foreign predecessor", 20, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativePhiInputsSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				merge := nativeReturnedPhi(t, fn)
				r := gotoolchain.SSAPhiInputsRequest{Function: fn, Merge: merge}
				switch row.damage {
				case 0:
					r.Function = nil
				case 1:
					r.Merge = nil
				case 2:
					r.Function = v.Package.Func("Other")
				case 3:
					copied := *merge
					r.Merge = &copied
				case 4:
					merge.Edges = nil
				case 5:
					merge.Edges[1] = nil
				case 6:
					merge.Edges[1] = (*ssa.Parameter)(nil)
				case 7:
					merge.Edges[1] = v.Package.Func("Other").Params[2]
				case 8:
					merge.Block().Preds = merge.Block().Preds[:1]
				case 9:
					merge.Block().Preds[1] = nil
				case 10:
					copied := *merge.Block().Preds[1]
					merge.Block().Preds[1] = &copied
				case 11:
					merge.Block().Preds[1] = merge.Block()
				case 12:
					var later *ssa.Call
					for _, block := range fn.Blocks {
						for _, instruction := range block.Instrs {
							if call, ok := instruction.(*ssa.Call); ok {
								later = call
							}
						}
					}
					if later == nil {
						t.Fatalf("later input fixture got=%v, want actual native call", later)
					}
					merge.Edges[1] = later
				case 13:
					r.AncestorOf = v.Package.Func("Other").Blocks[0]
				case 14:
					copied := *merge.Block()
					r.AncestorOf = &copied
				case 15:
					v = gotoolchain.SSAUnitView{}
				case 16:
					var cancel context.CancelCauseFunc
					ctx, cancel = context.WithCancelCause(ctx)
					cancel(row.want)
				case 17:
					merge.Block().Preds[1].Instrs = nil
				case 18:
					copied := *fn.Params[2]
					merge.Edges[1] = &copied
				case 19:
					merge.Edges = nil
					merge.Block().Preds = nil
				case 20:
					merge.Block().Preds[1] = v.Package.Func("Other").Blocks[0]

				default:
					t.Fatalf("Phi damage got=%d, want declared case", row.damage)
				}
				refused := 0
				for got, err := range v.PhiInputs(ctx, r) {
					if got != (gotoolchain.SSAPhiInputObservation{}) || !errors.Is(err, row.want) || row.want == core.ErrGoToolchainContract && errors.Is(err, core.ErrGoToolchainOutput) {
						t.Fatalf("Phi custody got=%+v,%v, want zero,%v", got, err, row.want)
					}
					refused++
				}
				if refused != 1 {
					t.Fatalf("Phi refusal count got=%d, want1", refused)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}
