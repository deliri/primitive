package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"golang.org/x/tools/go/ssa"
)

const cellWriterDeclarations = `type Target struct{Name string};func write(any){};func other(any){};func sink(Target){};`

func TestSSACellWriterPreservesNativeAddressUseContracts(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, declarations, body, callee string
		want                             bool
	}{
		{name: "boxed address has sole opaque call", body: `var target Target;write(&target);sink(target)`, callee: "write", want: true},
		{name: "local address alias is same native cell", body: `var target Target;p:=&target;write(p);sink(target)`, callee: "write", want: true},
		{name: "copied interface is same native argument", body: `var target Target;p:=any(&target);q:=p;write(q);sink(target)`, callee: "write", want: true},
		{name: "conditional read follows dominating call", body: `var target Target;write(&target);if flag{sink(target)}`, callee: "write", want: true},
		{name: "typed address call needs no boxing", declarations: `func typed(*Target){}`, body: `var target Target;typed(&target);sink(target)`, callee: "typed", want: true},
		{name: "parent store before call stays unproved", body: `var target Target;target=Target{Name:"before"};write(&target);sink(target)`},
		{name: "parent store after call stays unproved", body: `var target Target;write(&target);target=Target{Name:"after"};sink(target)`},
		{name: "field write after call stays unproved", body: `var target Target;write(&target);target.Name="other";sink(target)`},
		{name: "alias field write stays unproved", body: `var target Target;write(&target);p:=&target;p.Name="other";sink(target)`},
		{name: "second opaque call stays unproved", body: `var target Target;write(&target);other(&target);sink(target)`},
		{name: "same call argument repeated stays unproved", declarations: `func twice(*Target,*Target){}`, body: `var target Target;twice(&target,&target);sink(target)`},
		{name: "competing interface boxes stay unproved", body: `var target Target;p:=any(&target);q:=any(&target);write(p);other(q);sink(target)`},
		{name: "sibling closure write stays unproved", body: `var target Target;write(&target);func(){target.Name="other"}();sink(target)`},
		{name: "readonly capture remains conservatively unproved", body: `var target Target;write(&target);defer func(){sink(target)}();sink(target)`},
		{name: "deferred call is not a prior writer", body: `var target Target;defer write(&target);sink(target)`},
		{name: "asynchronous call is not a prior writer", body: `var target Target;go write(&target);sink(target)`},
		{name: "stored address escape stays unproved", declarations: `var escaped *Target`, body: `var target Target;write(&target);escaped=&target;sink(target)`},
		{name: "conditional call does not dominate read", body: `var target Target;if flag{write(&target)};sink(target)`},
		{name: "loop may omit writer", body: `var target Target;for flag{write(&target);flag=false};sink(target)`},
		{name: "read preceding sole call stays unproved", body: `var target Target;sink(target);write(&target)`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			source := `package sample;` + cellWriterDeclarations + row.declarations + "\n" + `func Entry(flag bool){` + row.body + `}`
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
			loanErr := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				load := cellWriterSinkLoad(ctx, t, view, view.Package.Func("Entry"))
				got, err := view.CellWriter(ctx, gotoolchain.SSACellWriterRequest{Load: load})
				if err != nil {
					return err
				}
				if err := got.Validate(); err != nil {
					return err
				}
				if (got.Call != nil) != row.want {
					t.Fatalf("cell writer=%v, want presence %t", got.Call, row.want)
				}
				if row.want && got.Call.Common().StaticCallee() != view.Package.Func(row.callee) {
					t.Fatalf("cell writer=%v, want exact native %s", got.Call, row.callee)
				}
				return nil
			}})
			if loanErr != nil {
				t.Fatal(loanErr)
			}
		})
	}
}

func cellWriterSinkLoad(ctx context.Context, t testing.TB, view gotoolchain.SSAUnitView, fn *ssa.Function) *ssa.UnOp {
	t.Helper()
	var load *ssa.UnOp
	for instruction, err := range view.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
		if err != nil {
			t.Fatal(err)
		}
		call, ok := instruction.(*ssa.Call)
		if !ok || call.Common().StaticCallee() != view.Package.Func("sink") {
			continue
		}
		if len(call.Common().Args) != 1 || load != nil {
			t.Fatalf("sink operands=%v/prior load=%v, want one selected load", call.Common().Args, load)
		}
		load, ok = call.Common().Args[0].(*ssa.UnOp)
		if !ok {
			t.Fatalf("sink operand=%T, want native dereference", call.Common().Args[0])
		}
	}
	if load == nil {
		t.Fatalf("sink load=%v, want one native dereference", load)
	}
	return load
}

func TestSSACellWriterNativeErrorsAsAndRefusals(t *testing.T) {
	t.Parallel()
	source := `package sample;import "errors";type Target struct{Name string};func(Target)Error()string{return "detail"};func sink(Target){};func Entry(err error,foreign *int){var target Target;if !errors.As(err,&target){return};sink(target)}`
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
	loanErr := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		fn := view.Package.Func("Entry")
		load := cellWriterSinkLoad(ctx, t, view, fn)
		got, err := view.CellWriter(ctx, gotoolchain.SSACellWriterRequest{Load: load})
		if err != nil || got.Call == nil {
			t.Fatalf("native As writer=%v/%v, want native call", got.Call, err)
		}
		object := got.Call.Common().StaticCallee().Object()
		if object == nil || object.Pkg() == nil || object.Pkg().Path() != "errors" || object.Name() != "As" {
			t.Fatalf("native writer object=%v, want errors.As", object)
		}
		cause := errors.New("cell observation canceled")
		canceled, cancel := context.WithCancelCause(ctx)
		cancel(cause)
		refused, queryErr := view.CellWriter(canceled, gotoolchain.SSACellWriterRequest{Load: load})
		if refused.Call != nil || !errors.Is(queryErr, context.Canceled) || !errors.Is(queryErr, cause) {
			t.Fatalf("canceled writer=%v/%v, want zero and both causes", refused.Call, queryErr)
		}
		for _, request := range []gotoolchain.SSACellWriterRequest{{}, {Load: new(ssa.UnOp)}} {
			refused, queryErr = view.CellWriter(ctx, request)
			if refused.Call != nil || !errors.Is(queryErr, core.ErrGoToolchainContract) {
				t.Fatalf("invalid request=%v/%v, want zero contract refusal", refused.Call, queryErr)
			}
		}
		clone := *load
		refused, queryErr = view.CellWriter(ctx, gotoolchain.SSACellWriterRequest{Load: &clone})
		if refused.Call != nil || !errors.Is(queryErr, core.ErrGoToolchainContract) {
			t.Fatalf("foreign instruction=%v/%v, want zero contract refusal", refused.Call, queryErr)
		}
		for _, address := range []ssa.Value{nil, fn.Params[1]} {
			original := load.X
			load.X = address
			refused, queryErr = view.CellWriter(ctx, gotoolchain.SSACellWriterRequest{Load: load})
			load.X = original
			if refused.Call != nil || !errors.Is(queryErr, core.ErrGoToolchainOutput) {
				t.Fatalf("damaged address=%v/%v, want zero output refusal", refused.Call, queryErr)
			}
		}
		refused, queryErr = (gotoolchain.SSAUnitView{}).CellWriter(ctx, gotoolchain.SSACellWriterRequest{Load: load})
		if refused.Call != nil || !errors.Is(queryErr, core.ErrGoToolchainContract) {
			t.Fatalf("missing unit=%v/%v, want zero contract refusal", refused.Call, queryErr)
		}
		return nil
	}})
	if loanErr != nil {
		t.Fatal(loanErr)
	}
}

func TestSSACellWriterDeclinesNonlocalCells(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, source string }{
		{name: "caller pointer is not a local zero cell", source: `package sample;type Target struct{Name string};func sink(Target){};func Entry(target *Target){sink(*target)}`},
		{name: "global address is not a local zero cell", source: `package sample;type Target struct{Name string};var target Target;func write(any){};func sink(Target){};func Entry(){write(&target);sink(target)}`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			a := nativeSymbolSourceAnalysis(t, t.TempDir(), row.source)
			loanErr := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				load := cellWriterSinkLoad(ctx, t, view, view.Package.Func("Entry"))
				got, err := view.CellWriter(ctx, gotoolchain.SSACellWriterRequest{Load: load})
				if err != nil || got.Call != nil {
					t.Fatalf("nonlocal writer=%v/%v, want absence without a refusal", got.Call, err)
				}
				return nil
			}})
			if loanErr != nil {
				t.Fatal(loanErr)
			}
		})
	}
}

func TestSSACellWriterRejectsForgedNativeUse(t *testing.T) {
	t.Parallel()
	source := `package sample;type Target struct{Name string};func write(any){};func other(int){};func sink(Target){};func Entry(){var target Target;other(1);write(&target);sink(target)}`
	a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
	loanErr := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
		fn := view.Package.Func("Entry")
		load := cellWriterSinkLoad(ctx, t, view, fn)
		var foreignUse *ssa.Call
		for instruction, err := range view.Instructions(ctx, gotoolchain.SSAInstructionRequest{Function: fn, Extent: gotoolchain.SSAInstructionExtentFunction}) {
			if err != nil {
				return err
			}
			if call, ok := instruction.(*ssa.Call); ok && call.Common().StaticCallee() == view.Package.Func("other") {
				foreignUse = call
			}
		}
		cell, ok := load.X.(*ssa.Alloc)
		if !ok || foreignUse == nil {
			t.Fatalf("native cell/use=%v/%v, want native allocation and unrelated call", load.X, foreignUse)
		}
		before, err := view.CellWriter(ctx, gotoolchain.SSACellWriterRequest{Load: load})
		if err != nil || before.Call == nil {
			t.Fatalf("native precondition=%v/%v, want proved writer", before.Call, err)
		}
		original := *cell.Referrers()
		*cell.Referrers() = []ssa.Instruction{foreignUse, load}
		got, err := view.CellWriter(ctx, gotoolchain.SSACellWriterRequest{Load: load})
		*cell.Referrers() = original
		if got.Call != nil || !errors.Is(err, core.ErrGoToolchainOutput) {
			t.Fatalf("forged use writer=%v/%v, want zero and typed output refusal", got.Call, err)
		}
		return nil
	}})
	if loanErr != nil {
		t.Fatal(loanErr)
	}
}

func FuzzSSACellWriterRejectsCompetingWrites(f *testing.F) {
	for mode := range uint8(5) {
		f.Add(mode, uint16(0))
	}
	f.Fuzz(func(t *testing.T, mode uint8, suffix uint16) {
		name := fmt.Sprintf("Cell%d", suffix)
		tails := [...]string{"", `target.Name="replacement"`, `p:=&target;p.Name="replacement"`, `other(&target)`, `func(){target.Name="replacement"}()`}
		index := int(mode) % len(tails)
		source := `package sample;type ` + name + ` struct{Name string};func write(any){};func other(any){};func sink(` + name + `){};func Entry(){var target ` + name + `;write(&target);` + tails[index] + `;sink(target)}`
		a := nativeSymbolSourceAnalysis(t, t.TempDir(), source)
		loanErr := a.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: a.Units[0], SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
			load := cellWriterSinkLoad(ctx, t, view, view.Package.Func("Entry"))
			got, err := view.CellWriter(ctx, gotoolchain.SSACellWriterRequest{Load: load})
			if err != nil {
				return err
			}
			if (got.Call != nil) != (index == 0) {
				t.Fatalf("mode=%d writer=%v, want presence %t", index, got.Call, index == 0)
			}
			return nil
		}})
		if loanErr != nil {
			t.Fatal(loanErr)
		}
	})
}
