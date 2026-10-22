package gotoolchain_test

import (
	"context"
	"fmt"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzMethodFieldReadPreservesTargetArgumentAndField(f *testing.F) {
	f.Add(uint8(0), uint8(0), false, false)
	f.Add(uint8(1), uint8(0), false, false)
	f.Add(uint8(1), uint8(1), true, false)
	f.Add(uint8(0), uint8(1), true, true)
	f.Fuzz(func(t *testing.T, field, readField uint8, wrongArgument, replaced bool) {
		field %= 2
		readField %= 2
		member := "A"
		if readField == 1 {
			member = "B"
		}
		argument := "v,Record{}"
		if wrongArgument {
			argument = "Record{},v"
		}
		body := "call:=read;"
		if replaced {
			body += "call=ignore;"
		}
		body += "call(" + argument + ")"
		source := fmt.Sprintf("package sample\ntype Record struct{A,B int}\nfunc read(a,b Record){_=a.%s}\nfunc ignore(a,b Record){}\nfunc(v Record)Inspect(){%s}", member, body)
		analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		unit := analysis.Units[0]
		method := types.NewMethodSet(unit.Types.Scope().Lookup("Record").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
		want := field == readField && !wrongArgument && !replaced
		err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			got, err := view.MethodFieldRead(ctx, gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: method, Field: uint64(field)})
			if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Read != want {
				t.Fatalf("native field=%d target field=%d wrong argument=%t replaced target=%t: observation=%+v/%v,want read=%t", field, readField, wrongArgument, replaced, got, err, want)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestMethodFieldReadUsesPointerAndGenericOrigins(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, source string
		pointer      bool
	}{
		{name: "pointer receiver", source: "package sample;type Record struct{A int};func(v *Record)Inspect(){_=(*v).A}", pointer: true},
		{name: "generic receiver origin", source: "package sample;type Record[T any]struct{A T};func(v Record[T])Inspect(){_=v.A};func Use(){Record[int]{}.Inspect()}"},
		{name: "generic helper preserves field slot", source: "package sample;type Record[T any]struct{A T};func read[T any](v Record[T]){_=v.A};func(v Record[T])Inspect(){read(v)};func Use(){Record[int]{}.Inspect()}"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), row.source)
			unit := analysis.Units[0]
			record := unit.Types.Scope().Lookup("Record").Type()
			if row.pointer {
				record = types.NewPointer(record)
			}
			method := types.NewMethodSet(record).Lookup(nil, "Inspect").Obj().(*types.Func)
			for _, selection := range unit.TypesInfo.Selections {
				if selected, ok := selection.Obj().(*types.Func); ok && selected.Name() == "Inspect" {
					method = selected
				}
			}
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.MethodFieldRead(ctx, gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: method})
				if err != nil || !got.Read || got.Source != gotoolchain.FunctionSourcePresent {
					t.Fatalf("native method origin=%s observation=%+v/%v,want original field read", method.FullName(), got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
