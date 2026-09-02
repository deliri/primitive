package runnercontrol_test

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

type budgetMutation uint8

const (
	budgetUnchanged budgetMutation = iota
	budgetZeroConfigured
	budgetZeroEffective
	budgetChangedEffective
	budgetZeroUnits
	budgetExcessUnits
	budgetZeroParallel
	budgetZeroRepeat
	budgetRepeatedPhase
)

type executionBudgetCase struct {
	wantErr        error
	name           string
	class          string
	configuredNs   int64
	wantMultiplier uint64
	units          uint32
	parallel       uint16
	mutation       budgetMutation
	wantReport     bool
}

func TestExecutionBudgetHostileEvidenceMatrix(t *testing.T) {
	t.Parallel()

	cases := executionBudgetCases()
	gotClasses := make(map[string]int)
	for index := range cases {
		gotClasses[cases[index].class]++
	}
	wantClasses := map[string]int{"valid": 10, "rejection": 10, "boundary": 20}
	if len(cases) != 40 || gotClasses["valid"] != 10 || gotClasses["rejection"] != 10 || gotClasses["boundary"] != 20 {
		t.Fatalf("execution budget matrix classes = %v across %d cases, want %v across 40 earned cases", gotClasses, len(cases), wantClasses)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := compileBudgetCase(tc)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("ExecutionBudget(%s) error = %v, want errors.Is(..., %v) for the named budget boundary", tc.name, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ExecutionBudget(%s) error = %v, want nil", tc.name, gotErr)
			}
			gotMultiplier, multiplierErr := got.DivergenceMultiplier()
			gotReport, reportErr := got.ReportDivergence()
			if multiplierErr != nil || reportErr != nil || gotMultiplier != tc.wantMultiplier || gotReport != tc.wantReport {
				t.Fatalf("ExecutionBudget(%s) divergence = (multiplier %d, report %t, errors %v/%v), want (%d, %t, nil/nil)", tc.name, gotMultiplier, gotReport, multiplierErr, reportErr, tc.wantMultiplier, tc.wantReport)
			}
		})
	}
}

func compileBudgetCase(tc executionBudgetCase) (runnercontrol.ExecutionBudget, error) {
	configured, err := temporal.DurationFromNanoseconds(tc.configuredNs)
	if err != nil {
		return runnercontrol.ExecutionBudget{}, err
	}
	budget, err := runnercontrol.NewExecutionBudget(configured, tc.units, tc.parallel)
	if err != nil {
		return runnercontrol.ExecutionBudget{}, err
	}
	switch tc.mutation {
	case budgetUnchanged:
	case budgetZeroConfigured:
		budget.Configured, _ = temporal.DurationFromNanoseconds(0)
	case budgetZeroEffective:
		budget.Effective, _ = temporal.DurationFromNanoseconds(0)
	case budgetChangedEffective:
		one, _ := temporal.DurationFromNanoseconds(1)
		budget.Effective, _ = budget.Effective.Add(one)
	case budgetZeroUnits:
		budget.ExpectedUnits = 0
	case budgetExcessUnits:
		budget.ExpectedUnits = runnercontrol.ExecutionAccountingUnitMaximum + 1
	case budgetZeroParallel:
		budget.PackageParallel = 0
	case budgetZeroRepeat:
		budget.RepeatCount = 0
	case budgetRepeatedPhase:
		budget.RepeatCount = 2
	}
	return budget, budget.Validate()
}

func executionBudgetCases() []executionBudgetCase {
	valid := []executionBudgetCase{
		{name: "one unit occupies one exact execution wave", class: "valid", configuredNs: 1, units: 1, parallel: 1, wantMultiplier: 1},
		{name: "parallel width equal to two units remains one wave", class: "valid", configuredNs: 2, units: 2, parallel: 2, wantMultiplier: 1},
		{name: "parallel width wider than the unit set remains one wave", class: "valid", configuredNs: 3, units: 3, parallel: 4, wantMultiplier: 1},
		{name: "eight units close into two exact four-wide waves", class: "valid", configuredNs: 5, units: 8, parallel: 4, wantMultiplier: 2},
		{name: "nine units close into three exact three-wide waves", class: "valid", configuredNs: 7, units: 9, parallel: 3, wantMultiplier: 3},
		{name: "remainder unit creates the required final wave", class: "valid", configuredNs: 11, units: 5, parallel: 2, wantMultiplier: 3},
		{name: "maximum package parallelism bounds maximum admitted units", class: "valid", configuredNs: 13, units: runnercontrol.ExecutionAccountingUnitMaximum, parallel: math.MaxUint16, wantMultiplier: 2},
		{name: "ten waves meet the report threshold without crossing it", class: "valid", configuredNs: 17, units: 10, parallel: 1, wantMultiplier: 10},
		{name: "eleven waves retain visible divergence", class: "valid", configuredNs: 19, units: 11, parallel: 1, wantMultiplier: 11, wantReport: true},
		{name: "one hundred waves meet the infrastructure ceiling", class: "valid", configuredNs: 23, units: 100, parallel: 1, wantMultiplier: 100, wantReport: true},
	}
	rejections := []executionBudgetCase{
		{name: "zero configured timeout cannot define execution", class: "rejection", configuredNs: 0, units: 1, parallel: 1, wantErr: core.ErrPrimitiveContract},
		{name: "zero planned units cannot produce an evidence denominator", class: "rejection", configuredNs: 1, units: 0, parallel: 1, wantErr: core.ErrPrimitiveContract},
		{name: "unit count above accounting ceiling is refused", class: "rejection", configuredNs: 1, units: runnercontrol.ExecutionAccountingUnitMaximum + 1, parallel: 1, wantErr: core.ErrPrimitiveContract},
		{name: "zero package parallelism cannot form a wave", class: "rejection", configuredNs: 1, units: 1, parallel: 0, wantErr: core.ErrPrimitiveContract},
		{name: "persisted zero configured duration cannot validate", class: "rejection", configuredNs: 1, units: 1, parallel: 1, mutation: budgetZeroConfigured, wantErr: core.ErrPrimitiveContract},
		{name: "persisted zero effective duration cannot validate", class: "rejection", configuredNs: 1, units: 1, parallel: 1, mutation: budgetZeroEffective, wantErr: core.ErrPrimitiveContract},
		{name: "one nanosecond effective mutation exposes duplicated budget truth", class: "rejection", configuredNs: 1, units: 2, parallel: 1, mutation: budgetChangedEffective, wantErr: core.ErrPrimitiveContract},
		{name: "persisted zero repeat count cannot erase the phase", class: "rejection", configuredNs: 1, units: 1, parallel: 1, mutation: budgetZeroRepeat, wantErr: core.ErrPrimitiveContract},
		{name: "repeat-amplified execution is refused by the once-per-phase contract", class: "rejection", configuredNs: 1, units: 1, parallel: 1, mutation: budgetRepeatedPhase, wantErr: core.ErrPrimitiveContract},
		{name: "one hundred one waves exceed infrastructure divergence", class: "rejection", configuredNs: 1, units: 101, parallel: 1, wantErr: core.ErrPrimitiveContract},
	}
	boundaries := []executionBudgetCase{
		{name: "minimum positive duration is retained exactly", class: "boundary", configuredNs: 1, units: 1, parallel: 1, wantMultiplier: 1},
		{name: "duration multiplication by two is exact", class: "boundary", configuredNs: math.MaxInt32, units: 2, parallel: 1, wantMultiplier: 2},
		{name: "duration multiplication by ninety nine stays below infrastructure ceiling", class: "boundary", configuredNs: 1000, units: 99, parallel: 1, wantMultiplier: 99, wantReport: true},
		{name: "duration multiplication overflow is refused", class: "boundary", configuredNs: math.MaxInt64, units: 2, parallel: 1, wantErr: core.ErrPrimitiveContract},
		{name: "unit below four-wide wave boundary uses one wave", class: "boundary", configuredNs: 1, units: 3, parallel: 4, wantMultiplier: 1},
		{name: "unit at four-wide wave boundary uses one wave", class: "boundary", configuredNs: 1, units: 4, parallel: 4, wantMultiplier: 1},
		{name: "unit above four-wide wave boundary uses two waves", class: "boundary", configuredNs: 1, units: 5, parallel: 4, wantMultiplier: 2},
		{name: "extreme unit count stays bounded under maximum width", class: "boundary", configuredNs: 1, units: runnercontrol.ExecutionAccountingUnitMaximum, parallel: math.MaxUint16, wantMultiplier: 2},
		{name: "parallel one exposes every unit as a wave", class: "boundary", configuredNs: 1, units: 7, parallel: 1, wantMultiplier: 7},
		{name: "parallel width one below units requires two waves", class: "boundary", configuredNs: 1, units: 7, parallel: 6, wantMultiplier: 2},
		{name: "parallel width exact to units requires one wave", class: "boundary", configuredNs: 1, units: 7, parallel: 7, wantMultiplier: 1},
		{name: "parallel width one above units still requires one wave", class: "boundary", configuredNs: 1, units: 7, parallel: 8, wantMultiplier: 1},
		{name: "nine waves stay below divergence reporting", class: "boundary", configuredNs: 1, units: 9, parallel: 1, wantMultiplier: 9},
		{name: "ten waves are exactly the non-reporting ceiling", class: "boundary", configuredNs: 1, units: 20, parallel: 2, wantMultiplier: 10},
		{name: "eleven waves are the first reporting value", class: "boundary", configuredNs: 1, units: 22, parallel: 2, wantMultiplier: 11, wantReport: true},
		{name: "one hundred waves are the last admissible reporting value", class: "boundary", configuredNs: 1, units: 200, parallel: 2, wantMultiplier: 100, wantReport: true},
		{name: "one hundred one waves are the first infrastructure refusal", class: "boundary", configuredNs: 1, units: 202, parallel: 2, wantErr: core.ErrPrimitiveContract},
		{name: "zero persisted units cannot masquerade as neutral evidence", class: "boundary", configuredNs: 1, units: 1, parallel: 1, mutation: budgetZeroUnits, wantErr: core.ErrPrimitiveContract},
		{name: "excess persisted units cannot bypass constructor bounds", class: "boundary", configuredNs: 1, units: 1, parallel: 1, mutation: budgetExcessUnits, wantErr: core.ErrPrimitiveContract},
		{name: "zero persisted parallelism cannot bypass constructor bounds", class: "boundary", configuredNs: 1, units: 1, parallel: 1, mutation: budgetZeroParallel, wantErr: core.ErrPrimitiveContract},
	}
	return append(append(valid, rejections...), boundaries...)
}
