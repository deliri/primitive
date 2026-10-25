package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSSACaptureFramesUsesPrimitiveCatalogOwner(t *testing.T) {
	t.Parallel()
	owner, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatal(err)
	}
	path, err := owner.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSACaptureFramesRequest]().PkgPath() != path {
		t.Fatalf("capture frame owner=%s,%v, want compiler request owner", path, err)
	}
}

func nativeCaptureFramesMode(t testing.TB, directory string, mode uint8, foreign bool) {
	t.Helper()
	mode %= 12
	form := mode % 3
	depth := 1 + (mode/3)%2
	selected := mode/6 == 0
	value := "a"
	if !selected {
		value = "b"
	}
	inner := ""
	if depth == 2 {
		inner = `g:=func(){_=` + value + `};_=g;`
	}
	body := `f:=func(){_=` + value + `;` + inner + `};_=f`
	if form == 1 {
		body = `func(){_=` + value + `;` + inner + `}()`
	}
	if form == 2 {
		body = `sink(func(callback *int){_=callback;_=` + value + `;` + inner + `})`
	}
	a := nativeSymbolSourceAnalysis(t, directory, `package sample;func sink(func(*int)){};func Other(a *int){};func Entry(a,b *int){`+body+`}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		fn := v.Package.Func("Entry")
		if fn == nil || len(fn.Params) != 2 || len(fn.AnonFuncs) != 1 || len(fn.AnonFuncs[0].FreeVars) != 1 {
			t.Fatal("fixture does not own the requested root and one captured value")
		}
		r := gotoolchain.SSACaptureFramesRequest{Function: fn, Argument: fn.Params[0]}
		if foreign {
			r.Argument = v.Package.Func("Other").Params[0]
			if r.Argument == fn.Params[0] || r.Argument.Parent() == fn {
				t.Fatal("foreign-owner mutation did not change native custody")
			}
		}
		count, refusals := 0, 0
		for got, queryErr := range v.CaptureFrames(ctx, r) {
			if foreign {
				if got != (gotoolchain.SSAArgumentBindingObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainContract) {
					t.Fatalf("capture mode%d foreign loan=%+v,%v, want zero and contract refusal", mode, got, queryErr)
				}
				refusals++
				continue
			}
			if queryErr != nil || got.Validate() != nil {
				t.Fatalf("capture mode%d loan=%+v,%v, want native frame", mode, got, queryErr)
			}
			if count == 0 {
				if got.Function != fn || got.Local != fn.Params[0] {
					t.Fatalf("capture root=%+v, want exact selected parameter", got)
				}
			} else {
				want := fn.AnonFuncs[0]
				if count == 2 {
					if len(want.AnonFuncs) != 1 {
						t.Fatal("fixture lost its nested native capture")
					}
					want = want.AnonFuncs[0]
				}
				if got.Function != want || got.Local != want.FreeVars[0] {
					t.Fatalf("capture mode%d ordinal%d loan=%+v, want exact native free variable in %v", mode, count, got, want)
				}
			}
			count++
		}
		want, wantRefusals := 1, 0
		if selected {
			want += int(depth)
		}
		if foreign {
			want, wantRefusals = 0, 1
		}
		if count != want || refusals != wantRefusals {
			t.Fatalf("capture mode%d selected=%t depth%d foreign=%t observations/refusals=%d/%d, want%d/%d", mode, selected, depth, foreign, count, refusals, want, wantRefusals)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSSACaptureFramesFiniteNativeSourceDomain(t *testing.T) {
	t.Parallel()
	for mode := uint8(0); mode < 12; mode++ {
		for _, foreign := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/foreign=%t", mode, foreign), func(t *testing.T) {
				t.Parallel()
				nativeCaptureFramesMode(t, t.TempDir(), mode, foreign)
			})
		}
	}
}

func FuzzSSACaptureFramesConservesNativeValueCustody(f *testing.F) {
	f.Add(uint8(0), false)
	f.Fuzz(func(t *testing.T, mode uint8, foreign bool) {
		nativeCaptureFramesMode(t, t.TempDir(), mode, foreign)
	})
}

func TestSSACaptureFramesAcceptsSelectedNativeFreeVariable(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;func Entry(a *int){f:=func(){_=a;g:=func(){_=a};_=g};_=f}`)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		parent := v.Package.Func("Entry").AnonFuncs[0]
		if len(parent.FreeVars) != 1 || len(parent.AnonFuncs) != 1 || len(parent.AnonFuncs[0].FreeVars) != 1 {
			t.Fatal("fixture lost its native free-variable chain")
		}
		count := 0
		for got, queryErr := range v.CaptureFrames(ctx, gotoolchain.SSACaptureFramesRequest{Function: parent, Argument: parent.FreeVars[0]}) {
			want := parent
			if count == 1 {
				want = parent.AnonFuncs[0]
			}
			if queryErr != nil || got.Function != want || got.Local != want.FreeVars[0] {
				t.Fatalf("selected free variable ordinal%d got=%+v,%v, want native %v", count, got, queryErr, want)
			}
			count++
		}
		if count != 2 {
			t.Fatalf("selected free-variable frames=%d, want2", count)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
