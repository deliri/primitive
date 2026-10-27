package gotoolchain_test

import (
	"context"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzMethodFieldIteratorErrorConservesCoordinateAndCause(f *testing.F) {
	f.Add(uint8(0), uint8(0), true, true, false, false, false)
	f.Add(uint8(1), uint8(1), false, false, true, false, false)
	f.Add(uint8(0), uint8(0), true, false, false, false, false)
	f.Add(uint8(0), uint8(1), false, false, true, false, false)
	f.Add(uint8(0), uint8(0), true, true, true, true, false)
	f.Add(uint8(0), uint8(0), false, false, false, false, true)
	f.Fuzz(func(t *testing.T, selected, used uint8, producerFirst, consumerFirst, helper, wrongMethod, discard bool) {
		selected %= 2
		used %= 2
		member := "A"
		if used == 1 {
			member = "B"
		}
		target := "Inspect"
		if wrongMethod {
			target = "Other"
		}
		call := "p." + member + "." + target + "()"
		first, second := call, "bad"
		if !producerFirst {
			first, second = second, first
		}
		producer := "yield(" + first + "," + second + ")"
		if helper {
			left, right := "err", "bad"
			if !producerFirst {
				left, right = right, left
			}
			producer = "refuse:=func(err error){yield(" + left + "," + right + ")};refuse(" + call + ")"
		}
		chosen := "first"
		if !consumerFirst {
			chosen = "second"
		}
		consumer := "for first,second:=range p.items(){_=first;_=second;err:=" + chosen + ";if err!=nil{return err}};return nil"
		if discard {
			consumer = "for range p.items(){};return nil"
		}
		source := `package sample
import "iter"
type refusal uint8
const bad refusal=1
func(refusal)Error()string{return "refusal"}
type Child struct{Count int}
func(Child)Inspect()error{return bad}
func(Child)Other()error{return bad}
type Parent struct{A,B Child}
func(p Parent)items()iter.Seq2[error,error]{return func(yield func(error,error)bool){` + producer + `}}
func(p Parent)Check()error{` + consumer + `}`
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		unit := analysis.Units[0]
		method := types.NewMethodSet(unit.Types.Scope().Lookup("Parent").Type()).Lookup(nil, "Check").Obj().(*types.Func)
		targetMethod := types.NewMethodSet(unit.Types.Scope().Lookup("Child").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
		want := selected == used && producerFirst == consumerFirst && !wrongMethod && !discard
		if err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			got, err := view.MethodFieldErrorPropagation(ctx, gotoolchain.MethodFieldErrorPropagationRequest{Analysis: analysis, Method: method, Field: uint64(selected), Target: targetMethod})
			if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Propagated != want {
				t.Fatalf("field=%d/%d coordinate=%t/%t helper=%t wrong=%t discard=%t: native=%+v/%v,want propagated=%t", selected, used, producerFirst, consumerFirst, helper, wrongMethod, discard, got, err, want)
			}
			return nil
		}}); err != nil {
			t.Fatalf("native SSA scope=%v,want nil", err)
		}
	})
}
