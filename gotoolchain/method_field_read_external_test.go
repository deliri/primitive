package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestMethodFieldReadBorrowsNativeReceiverAndHelperSlots(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, declarations, body string
		field                    uint64
		want                     bool
	}{
		{name: "direct first field", body: `_ = v.A`, want: true},
		{name: "unread first field"},
		{name: "read other field does not cover first", body: `_ = v.B`},
		{name: "last field has its own slot", body: `_ = v.B`, field: 1, want: true},
		{name: "shadowed receiver cannot supply read", body: `{v:=Record{}; _=v.A}`},
		{name: "whole comparable record reads every slot", body: `_ = v == (Record{})`, field: 1, want: true},
		{name: "called helper lends actual receiver", declarations: `func read(x Record){_=x.A}`, body: `read(v)`, want: true},
		{name: "helper receives foreign record", declarations: `func read(x Record){_=x.A}`, body: `read(Record{})`},
		{name: "helper receives wrong argument slot", declarations: `func read(a,b Record){_=a.A}`, body: `read(Record{},v)`},
		{name: "nonfirst helper argument preserves slot", declarations: `func read(a,b Record){_=b.A}`, body: `read(Record{},v)`, want: true},
		{name: "helper alias uses native target", declarations: `func read(x Record){_=x.A}`, body: `call:=read;call(v)`, want: true},
		{name: "replaced helper alias cannot supply read", declarations: `func read(x Record){_=x.A};func ignore(x Record){}`, body: `call:=read;call=ignore;call(v)`},
		{name: "whole record conversion preserves slots", declarations: `type Shared Record;func read(x Shared){_=x.A}`, body: `read(Shared(v))`, want: true},
		{name: "constructed conversion does not borrow receiver", declarations: `type Shared Record;func read(x Shared){_=x.A}`, body: `read(Shared(Record{}))`},
		{name: "method expression borrows receiver", declarations: `func(x Record)read(){_=x.A}`, body: `Record.read(v)`, want: true},
		{name: "invoked closure borrows actual parameter", body: `func(x Record){_=x.A}(v)`, want: true},
		{name: "invoked closure receives another record", body: `func(x Record){_=x.A}(Record{})`},
		{name: "uninvoked closure supplies no read", body: `_ = func(){_=v.A}`},
		{name: "recursive helper supplies no invented read", declarations: `func again(x Record,done bool){if done{return};again(x,true)}`, body: `again(v,false)`},
		{name: "untaken helper invocation supplies no read", declarations: `func read(x Record){_=x.A}`, body: `if false{read(v)}`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), "package sample\ntype Record struct{A,B int}\n"+row.declarations+"\nfunc(v Record)Inspect(){"+row.body+"}")
			unit := analysis.Units[0]
			record := unit.Types.Scope().Lookup("Record").Type()
			method := types.NewMethodSet(record).Lookup(nil, "Inspect").Obj().(*types.Func)
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.MethodFieldRead(ctx, gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: method, Field: row.field})
				if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || got.Read != row.want {
					t.Fatalf("method field observation=%+v/%v, want present native source and read=%t", got, err, row.want)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMethodFieldReadRefusesInvalidCapabilityWithoutObservation(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), `package sample;type Record struct{A int};func(v Record)Inspect(){_=v.A};func Plain(){}`)
	unit := analysis.Units[0]
	method := types.NewMethodSet(unit.Types.Scope().Lookup("Record").Type()).Lookup(nil, "Inspect").Obj().(*types.Func)
	err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		for _, row := range []struct {
			name    string
			ctx     context.Context
			view    gotoolchain.SSAUnitView
			request gotoolchain.MethodFieldReadRequest
			want    error
		}{
			{name: "nil method", ctx: ctx, view: view, request: gotoolchain.MethodFieldReadRequest{Analysis: analysis}, want: core.ErrGoToolchainContract},
			{name: "field exactly beyond record", ctx: ctx, view: view, request: gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: method, Field: 1}, want: core.ErrGoToolchainContract},
			{name: "field maximum cannot truncate", ctx: ctx, view: view, request: gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: method, Field: ^uint64(0)}, want: core.ErrGoToolchainContract},
			{name: "free function has no record receiver", ctx: ctx, view: view, request: gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: unit.Types.Scope().Lookup("Plain").(*types.Func)}, want: core.ErrGoToolchainContract},
			{name: "missing native loan", ctx: ctx, request: gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: method}, want: core.ErrGoToolchainContract},
			{name: "nil context", view: view, request: gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: method}, want: core.ErrNilContext},
			{name: "canceled context", ctx: canceled, view: view, request: gotoolchain.MethodFieldReadRequest{Analysis: analysis, Method: method}, want: context.Canceled},
		} {
			t.Run(row.name, func(t *testing.T) {
				got, err := row.view.MethodFieldRead(row.ctx, row.request)
				if !errors.Is(err, row.want) || got != (gotoolchain.MethodFieldReadObservation{}) {
					t.Fatalf("refused method query=%+v/%v,want zero observation and %v", got, err, row.want)
				}
			})
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}
