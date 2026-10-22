package gotoolchain_test

import (
	"context"
	"errors"
	"go/types"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestReceiverMethodGateUsesValueGenericAndTypedCallbackOrigins(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, source string
		value        bool
	}{
		{name: "value receiver retains native operand", value: true, source: `package sample;type Record struct{};func(Record)Inspect()error{return nil};func(v Record)Apply()error{return v.Inspect()}`},
		{name: "generic method uses its native origin", source: `package sample;type Record[T ~int]struct{Count T};func(Record[T])Inspect()error{return nil};func(v *Record[T])Apply()error{return v.Inspect()};func Use(){var v Record[int];_=v.Apply()}`},
		{name: "callback input is native integer type", source: `package sample;type Record struct{Count int};type refusal uint8;const bad refusal=1;func(refusal)Error()string{return "refusal"};func(v Record)Inspect()error{if v.Count<0{return bad};return nil};func parse(input int)(Record,error){v:=Record{Count:input};if err:=v.Inspect();err!=nil{return v,err};return v,nil};func commit(dst *Record,input int,parse func(int)(Record,error))error{v,err:=parse(input);if err!=nil{return err};*dst=v;return nil};func(v *Record)Apply(input int)error{return commit(v,input,parse)}`},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			analysis := nativeSymbolSourceAnalysis(t, t.TempDir(), row.source)
			unit := analysis.Units[0]
			record := unit.Types.Scope().Lookup("Record").Type()
			receiver := record
			if !row.value {
				receiver = types.NewPointer(record)
			}
			method := types.NewMethodSet(receiver).Lookup(nil, "Apply").Obj().(*types.Func)
			target := types.NewMethodSet(record).Lookup(nil, "Inspect").Obj().(*types.Func)
			for _, selection := range unit.TypesInfo.Selections {
				if actual, ok := selection.Obj().(*types.Func); ok && actual.Name() == "Apply" {
					method = actual
				}
			}
			err := analysis.WithSSAUnit(t.Context(), gotoolchain.SSAUnitRequest{Unit: unit, SourceValues: true, Use: func(ctx context.Context, view gotoolchain.SSAUnitView) error {
				got, err := view.ReceiverMethodGate(ctx, gotoolchain.ReceiverMethodGateRequest{Analysis: analysis, Method: method, Target: target})
				if err != nil || got.Validate() != nil || got.Source != gotoolchain.FunctionSourcePresent || !got.Preserved {
					t.Fatalf("native method origins=%s/%s gate=%+v/%v,want preserved target", method.FullName(), target.FullName(), got, err)
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReceiverMethodGateObservationHasClosedSourceAuthority(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		observation gotoolchain.ReceiverMethodGateObservation
		valid       bool
	}{
		{observation: gotoolchain.ReceiverMethodGateObservation{Source: gotoolchain.FunctionSourcePresent}, valid: true},
		{observation: gotoolchain.ReceiverMethodGateObservation{Source: gotoolchain.FunctionSourcePresent, Preserved: true}, valid: true},
		{observation: gotoolchain.ReceiverMethodGateObservation{Source: gotoolchain.FunctionSourceAbsent}, valid: true},
		{observation: gotoolchain.ReceiverMethodGateObservation{Source: gotoolchain.FunctionSourceAbsent, Preserved: true}},
		{observation: gotoolchain.ReceiverMethodGateObservation{}},
		{observation: gotoolchain.ReceiverMethodGateObservation{Source: gotoolchain.FunctionSourcePresence(255)}},
	} {
		err := row.observation.Validate()
		if row.valid && err != nil || !row.valid && !errors.Is(err, core.ErrGoToolchainContract) {
			t.Errorf("source authority=%+v validation=%v,want valid=%t", row.observation, err, row.valid)
		}
	}
}
