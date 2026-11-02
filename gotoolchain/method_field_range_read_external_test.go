package gotoolchain_test

import (
	"context"
	"fmt"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/gotoolchain"
)

// Each combination changes a native call binding, receiver origin or field
// slot. The expectation follows source identity, not the query's output.
func nativeRangeFieldReadCase(t testing.TB, mode uint8) {
	t.Helper()
	mode %= 50
	form, origin, field := mode/10, (mode/2)%5, uint64(mode%2)
	forms := [...]struct{ name, declaration, invocation string }{
		{"value receiver", `func(x Record)iterate()func(func(int)bool){return func(yield func(int)bool){yield(x.A)}}`, `(%s).iterate()`},
		{"free helper", `func iterate(x Record)func(func(int)bool){return func(yield func(int)bool){yield(x.A)}}`, `iterate(%s)`},
		{"method expression", `func(x Record)iterate()func(func(int)bool){return func(yield func(int)bool){yield(x.A)}}`, `Record.iterate(%s)`},
		{"pointer receiver", `func(x *Record)iterate()func(func(int)bool){return func(yield func(int)bool){yield(x.A)}}`, `(&%s).iterate()`},
		{"converted receiver", `type Shared Record;func(x Shared)iterate()func(func(int)bool){return func(yield func(int)bool){yield(x.A)}}`, `Shared(%s).iterate()`},
	}
	origins := [...]string{"actual receiver", "foreign value", "shadowed receiver", "discarded iterator", "untaken range"}
	operand := "v"
	if origin == 1 {
		operand = "Record{}"
	}
	invocation := fmt.Sprintf(forms[form].invocation, operand)
	body := "for range " + invocation + "{}"
	switch origin {
	case 2:
		body = "{v:=Record{};" + body + "}"
	case 3:
		body = "_=" + invocation
	case 4:
		body = "if false{" + body + "}"
	}
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample;type Record struct{A,B int};"+forms[form].declaration+";func(v Record)Inspect(){"+body+"}")
	unit := a.Units[0]
	method := types.NewMethodSet(unit.Types.Scope().Lookup("Record").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
	err := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		got, err := view.MethodFieldRead(ctx, gotoolchain.MethodFieldReadRequest{Analysis: a, Method: method, Field: field})
		want := origin == 0 && field == 0
		if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Read != want {
			t.Fatalf("range read form=%s origin=%s field=%d got=%+v/%v, want present source and read=%t", forms[form].name, origins[origin], field, got, err, want)
		}
		return nil
	}})
	if err != nil {
		t.Fatalf("range read unit error=%v, want nil", err)
	}
}

func TestMethodFieldReadReturnedIteratorBindings(t *testing.T) {
	t.Parallel()
	forms := [...]string{"value receiver", "free helper", "method expression", "pointer receiver", "converted receiver"}
	origins := [...]string{"actual receiver", "foreign value", "shadowed receiver", "discarded iterator", "untaken range"}
	fields := [...]string{"read field", "unread sibling"}
	for mode := uint8(0); mode < 50; mode++ {
		t.Run(forms[mode/10]+"/"+origins[(mode/2)%5]+"/"+fields[mode%2], func(t *testing.T) {
			t.Parallel()
			nativeRangeFieldReadCase(t, mode)
		})
	}
}

func FuzzMethodFieldReadReturnedIteratorBindings(f *testing.F) {
	for _, seed := range []uint8{0, 1, 2, 6, 8, 10} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, mode uint8) { nativeRangeFieldReadCase(t, mode) })
}
