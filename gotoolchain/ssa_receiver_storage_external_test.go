package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

func TestSSAReceiverStorageUsesNativeBindings(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body string
		want       bool
	}{
		{"same direct pointer", `a.First();a.Second()`, true},
		{"different direct pointers", `a.First();b.Second()`, false},
		{"copied pointer", `copy:=a;copy.First();a.Second()`, true},
		{"bound acquisition", `first:=a.First;first();a.Second()`, true},
		{"bound release", `a.First();second:=a.Second;second()`, true},
		{"two bound methods", `first:=a.First;second:=a.Second;first();second()`, true},
		{"different bound receivers", `first:=a.First;second:=b.Second;first();second()`, false},
		{"binding survives pointer reassignment", `first:=a.First;a=b;first();a.Second()`, false},
		{"plain functions have no receiver", `plain(a);plain(a)`, false},
		{"value receivers supply no storage fact", `v.Value();v.Value()`, false},
		{"same interface operand", `i.First();i.Second()`, true},
		{"different interface operands", `i.First();j.Second()`, false},
		{"direct method expressions", `(*Cell).First(a);(*Cell).Second(a)`, true},
		{"promoted direct receiver", `x.First();x.Second()`, true},
		{"promoted bound receiver", `first:=x.First;second:=x.Second;first();second()`, true},
		{"promoted expression receiver", `(*Box).First(x);(*Box).Second(x)`, true},
		{"mixed promoted expression receiver", `x.First();(*Box).Second(x)`, true},
		{"different promoted instances", `(*Box).First(x);(*Box).Second(y)`, false},
		{"different native field addresses", `p.Left.First();p.Right.Second()`, false},
		{"nil pointer is no storage authority", `(*Cell)(nil).First();(*Cell)(nil).Second()`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := `package sample;type Cell struct{};func(*Cell)First(){};func(*Cell)Second(){};func(Cell)Value(){};type Box struct{Cell};type Pair struct{Left,Right Cell};type Interface interface{First();Second()};func plain(*Cell){};func Entry(a,b *Cell,x,y *Box,p *Pair,i,j Interface,v Cell){` + row.body + `}`
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
				fn := v.Package.Func("Entry")
				var calls [2]*ssa.Call
				count := 0
				for _, block := range fn.Blocks {
					for _, instruction := range block.Instrs {
						if call, ok := instruction.(*ssa.Call); ok {
							if count < len(calls) {
								calls[count] = call
							}
							count++
						}
					}
				}
				if count != 2 {
					t.Fatalf("native receiver fixture calls=%d, want 2", count)
				}
				observation, queryErr := v.SameReceiverStorage(ctx, gotoolchain.SSACallReceiverStorageRequest{Left: calls[0], Right: calls[1]})
				if queryErr != nil || observation.Proven != row.want || observation.Validate() != nil {
					t.Fatalf("receiver storage=(%+v,%v), want proven=%t and nil", observation, queryErr, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatalf("WithSSAUnit error=%v, want nil", err)
			}
		})
	}
}
