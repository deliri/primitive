package gotoolchain_test

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

// Native source generation varies call representation independently of which
// actual parameter owns storage. Receiver equality must conserve that identity.
func FuzzSSAReceiverStorageConservesParameterIdentity(f *testing.F) {
	f.Add(uint8(0), uint8(0), false)
	f.Add(uint8(1), uint8(2), true)
	f.Add(uint8(2), uint8(1), false)
	f.Fuzz(func(t *testing.T, left, right uint8, different bool) {
		receiver := "a"
		if different {
			receiver = "b"
		}
		first, second := "a.First();", receiver+".Second();"
		switch left % 3 {
		case 0:
		case 1:
			first = "first:=a.First;first();"
		case 2:
			first = "(*Box).First(a);"
		}
		switch right % 3 {
		case 0:
		case 1:
			second = "second:=" + receiver + ".Second;second();"
		case 2:
			second = "(*Box).Second(" + receiver + ");"
		}
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Cell struct{};func(*Cell)First(){};func(*Cell)Second(){};type Box struct{Cell};func Entry(a,b *Box){`+first+second+`}`)
		err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, v gotoolchain.SSAUnitView) error {
			var calls [2]*ssa.Call
			count := 0
			for _, block := range v.Package.Func("Entry").Blocks {
				for _, instruction := range block.Instrs {
					if c, ok := instruction.(*ssa.Call); ok {
						if count < len(calls) {
							calls[count] = c
						}
						count++
					}
				}
			}
			if count != 2 {
				t.Fatalf("native fixture calls=%d, want 2", count)
			}
			got, queryErr := v.SameReceiverStorage(ctx, gotoolchain.SSACallReceiverStorageRequest{Left: calls[0], Right: calls[1]})
			if queryErr != nil || got.Validate() != nil || got.Proven == different {
				t.Fatalf("receiver conservation=(%+v,%v) representations=%d/%d different=%t, want proven=%t and nil", got, queryErr, left%3, right%3, different, !different)
			}
			return nil
		}})
		if err != nil {
			t.Fatalf("WithSSAUnit error=%v, want nil", err)
		}
	})
}
