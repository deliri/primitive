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

const nativeTupleSource = `package sample;
func pair(flag bool)(int,bool){return 7,flag};func consume(int,bool){};func Entry(flag bool){n,ok:=pair(flag);consume(n,ok)};
func Other(flag bool){n,ok:=pair(flag);consume(n,ok)};
func Lookup(m map[int]int,key int){n,ok:=m[key];consume(n,ok)};
func Assert(value interface{}){n,ok:=value.(int);consume(n,ok)};
func Receive(ch <-chan int){n,ok:=<-ch;consume(n,ok)};
func Select(a,b <-chan int){select{case n,ok:=<-a:consume(n,ok);case n,ok:=<-b:consume(n,ok)}};
func Iterate(m map[int]int){for k,v:=range m{consume(k,true);consume(v,true)}};
func Closure(flag bool){_=func(){n,ok:=pair(flag);consume(n,ok)}};
func Untaken(flag bool){if false{n,ok:=pair(flag);consume(n,ok)}};`

func TestSSATupleElementUsesCatalogOwner(t *testing.T) {
	t.Parallel()
	match, err := capabilities.Resolve(capabilities.ForPackage(capabilities.ScopeProduction, core.PackageGoToolchain))
	if err != nil {
		t.Fatalf("tuple owner got=%+v,%v, want compiled capability,nil", match, err)
	}
	path, err := match.Capability.ImportPath()
	if err != nil || reflect.TypeFor[gotoolchain.SSATupleElementRequest]().PkgPath() != path || reflect.TypeFor[gotoolchain.SSATupleElementObservation]().PkgPath() != path {
		t.Fatalf("tuple contracts got=%s,%s,%v, want catalog path %s,nil", reflect.TypeFor[gotoolchain.SSATupleElementRequest]().PkgPath(), reflect.TypeFor[gotoolchain.SSATupleElementObservation]().PkgPath(), err, path)
	}
}

// Source declarations independently specify each virtual tuple's complete
// layout. Compare every emitted Extract, not merely the first convenient value.
func TestSSATupleElementConservesNativeProducerLayouts(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeTupleSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		for _, row := range []struct {
			name         string
			layout       []types.Type
			wantOrdinals uint8
		}{
			{"Entry", []types.Type{types.Typ[types.Int], types.Typ[types.Bool]}, 3},
			{"Lookup", []types.Type{types.Typ[types.Int], types.Typ[types.Bool]}, 3},
			{"Assert", []types.Type{types.Typ[types.Int], types.Typ[types.Bool]}, 3},
			{"Receive", []types.Type{types.Typ[types.Int], types.Typ[types.Bool]}, 3},
			{"Select", []types.Type{types.Typ[types.Int], types.Typ[types.Bool], types.Typ[types.Int], types.Typ[types.Int]}, 15},
			{"Iterate", []types.Type{types.Typ[types.Bool], types.Typ[types.Int], types.Typ[types.Int]}, 7},
			{"Closure", []types.Type{types.Typ[types.Int], types.Typ[types.Bool]}, 3},
		} {
			fn := v.Package.Func(row.name)
			if row.name == "Closure" {
				fn = fn.AnonFuncs[0]
			}
			var ordinals uint8
			for instruction, queryErr := range v.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
				if queryErr != nil {
					return queryErr
				}
				element, ok := instruction.(*ssa.Extract)
				if !ok {
					continue
				}
				got, queryErr := v.TupleElement(ctx, gotoolchain.SSATupleElementRequest{Element: element})
				if queryErr != nil || got.Tuple != element.Tuple || got.Index != element.Index || got.Validate() != nil {
					t.Fatalf("%s element got=%+v,%v, want original tuple and ordinal=%d,nil", row.name, got, queryErr, element.Index)
				}
				if got.Index < 0 || got.Index >= len(row.layout) || !types.Identical(element.Type(), row.layout[got.Index]) {
					t.Fatalf("%s index=%d type=%v, want source layout %v", row.name, got.Index, element.Type(), row.layout)
				}
				ordinals |= 1 << got.Index
			}
			if ordinals != row.wantOrdinals {
				t.Fatalf("%s extracted ordinal mask got=%04b, want %04b from complete source tuple", row.name, ordinals, row.wantOrdinals)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSATupleElementRefusesBrokenNativeBoundaries(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		mutate  func(*gotoolchain.SSAUnitView, *gotoolchain.SSATupleElementRequest)
		wantErr error
	}{
		{"absent view", func(v *gotoolchain.SSAUnitView, _ *gotoolchain.SSATupleElementRequest) {
			*v = gotoolchain.SSAUnitView{}
		}, core.ErrGoToolchainContract},
		{"absent element", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) { r.Element = nil }, core.ErrGoToolchainContract},
		{"parentless element", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) { r.Element = &ssa.Extract{} }, core.ErrGoToolchainContract},
		{"detached element", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) {
			copy := *r.Element
			r.Element = &copy
		}, core.ErrGoToolchainContract},
		{"missing producer", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) { r.Element.Tuple = nil }, core.ErrGoToolchainOutput},
		{"typed nil producer", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) {
			r.Element.Tuple = (*ssa.Call)(nil)
		}, core.ErrGoToolchainOutput},
		{"empty producer", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) { r.Element.Tuple = &ssa.Call{} }, core.ErrGoToolchainOutput},
		{"scalar interface producer", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) {
			r.Element.Tuple = foreignConditionalScalar{}
		}, core.ErrGoToolchainOutput},
		{"wrong tuple type", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) {
			r.Element.Tuple = r.Element.Parent().Params[0]
		}, core.ErrGoToolchainOutput},
		{"foreign frame producer", func(v *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) {
			for _, instruction := range v.Package.Func("Other").Blocks[0].Instrs {
				if call, ok := instruction.(*ssa.Call); ok {
					r.Element.Tuple = call
					break
				}
			}
		}, core.ErrGoToolchainOutput},
		{"detached producer", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) {
			copy := *r.Element.Tuple.(*ssa.Call)
			r.Element.Tuple = &copy
		}, core.ErrGoToolchainOutput},
		{"negative ordinal", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) { r.Element.Index = -1 }, core.ErrGoToolchainOutput},
		{"ordinal at width", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) { r.Element.Index = 2 }, core.ErrGoToolchainOutput},
		{"ordinal above width", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) { r.Element.Index = 3 }, core.ErrGoToolchainOutput},
		{"result type mismatch", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) { r.Element.Index = 0 }, core.ErrGoToolchainOutput},
		{"self producer", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) { r.Element.Tuple = r.Element }, core.ErrGoToolchainOutput},
		{"producer after use", func(_ *gotoolchain.SSAUnitView, r *gotoolchain.SSATupleElementRequest) {
			block := r.Element.Block()
			source := r.Element.Tuple.(ssa.Instruction)
			before, after := -1, -1
			for i, instruction := range block.Instrs {
				if instruction == source {
					before = i
				}
				if instruction == r.Element {
					after = i
				}
			}
			block.Instrs[before], block.Instrs[after] = block.Instrs[after], block.Instrs[before]
		}, core.ErrGoToolchainOutput},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeTupleSource)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				var element *ssa.Extract
				for _, instruction := range v.Package.Func("Entry").Blocks[0].Instrs {
					if current, ok := instruction.(*ssa.Extract); ok && current.Index == 1 {
						element = current
					}
				}
				if element == nil {
					t.Fatalf("tuple fixture element got=%v, want actual bool result", element)
				}
				r := gotoolchain.SSATupleElementRequest{Element: element}
				row.mutate(&v, &r)
				got, queryErr := v.TupleElement(ctx, r)
				if got != (gotoolchain.SSATupleElementObservation{}) || !errors.Is(queryErr, row.wantErr) {
					t.Fatalf("tuple custody got=%+v,%v, want zero,%v", got, queryErr, row.wantErr)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit got=%v, want nil", err)
			}
		})
	}
}

func TestSSATupleElementProducerLayerTriad(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeTupleSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		var element *ssa.Extract
		for _, instruction := range v.Package.Func("Entry").Blocks[0].Instrs {
			if current, ok := instruction.(*ssa.Extract); ok && current.Index == 1 {
				element = current
			}
		}
		if element == nil {
			t.Fatalf("positive element got=%v, want actual bool extraction", element)
		}
		r := gotoolchain.SSATupleElementRequest{Element: element}
		original := element.Tuple
		got, queryErr := v.TupleElement(ctx, r)
		if queryErr != nil || got.Tuple != original || got.Index != 1 {
			t.Fatalf("positive tuple got=%+v,%v, want original producer,index 1,nil", got, queryErr)
		}
		element.Tuple = (*ssa.Call)(nil)
		got, queryErr = v.TupleElement(ctx, r)
		if got != (gotoolchain.SSATupleElementObservation{}) || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
			t.Fatalf("negative tuple got=%+v,%v, want zero,output refusal", got, queryErr)
		}
		var untaken *ssa.Extract
		for _, block := range v.Package.Func("Untaken").Blocks {
			for _, instruction := range block.Instrs {
				if current, ok := instruction.(*ssa.Extract); ok {
					untaken = current
				}
			}
		}
		if untaken == nil {
			t.Fatalf("untaken extraction got=%v, want actual unreachable native element", untaken)
		}
		got, queryErr = v.TupleElement(ctx, gotoolchain.SSATupleElementRequest{Element: untaken})
		if got != (gotoolchain.SSATupleElementObservation{}) || queryErr != nil {
			t.Fatalf("neutral untaken tuple got=%+v,%v, want zero,nil", got, queryErr)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}

func TestSSATupleElementPreservesFinalCancellation(t *testing.T) {
	t.Parallel()
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), nativeTupleSource)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
		var element *ssa.Extract
		for _, instruction := range v.Package.Func("Entry").Blocks[0].Instrs {
			if current, ok := instruction.(*ssa.Extract); ok && current.Index == 1 {
				element = current
			}
		}
		if element == nil {
			t.Fatalf("cancel fixture got=%v, want actual bool extraction", element)
		}
		for at := 1; at <= 4; at++ {
			parent, cancel := context.WithCancelCause(ctx)
			controlled := &nativeConditionalContext{Context: parent, cancel: cancel, failAt: at}
			got, queryErr := v.TupleElement(controlled, gotoolchain.SSATupleElementRequest{Element: element})
			cancel(nil)
			if got != (gotoolchain.SSATupleElementObservation{}) || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, io.ErrClosedPipe) {
				t.Fatalf("tuple observation=%d got=%+v,%v, want zero,cancelled with native cause", at, got, queryErr)
			}
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("WithSSAUnit got=%v, want nil", err)
	}
}
