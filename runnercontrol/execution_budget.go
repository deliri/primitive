package runnercontrol

import (
	"errors"
	"fmt"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	ExecutionRepeatCount                uint16 = 1
	BudgetDivergenceReportMultiplier    uint64 = 10
	BudgetDivergenceViolationMultiplier uint64 = 100
)

// ExecutionBudget preserves both the configured per-wave timeout and the
// effective worst-case budget produced by package waves. The effective value
// is checked rather than trusted so orchestration cannot silently amplify it.
type ExecutionBudget struct {
	Configured      temporal.Duration `json:"configured"`
	Effective       temporal.Duration `json:"effective"`
	ExpectedUnits   uint32            `json:"expected_units"`
	PackageParallel uint16            `json:"package_parallel"`
	RepeatCount     uint16            `json:"repeat_count"`
}

func NewExecutionBudget(configured temporal.Duration, expectedUnits uint32, packageParallel uint16) (ExecutionBudget, error) {
	if err := configured.Validate(); err != nil || configured.IsZero() || expectedUnits == 0 || expectedUnits > ExecutionAccountingUnitMaximum || packageParallel == 0 {
		return ExecutionBudget{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	waves := executionBudgetWaves(expectedUnits, packageParallel)
	effective, err := configured.Multiply(waves * uint64(ExecutionRepeatCount))
	if err != nil {
		return ExecutionBudget{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	budget := ExecutionBudget{Configured: configured, Effective: effective, ExpectedUnits: expectedUnits, PackageParallel: packageParallel, RepeatCount: ExecutionRepeatCount}
	return budget, budget.Validate()
}

func (b ExecutionBudget) Validate() error {
	if err := b.validateShape(); err != nil {
		return err
	}
	multiplier := executionBudgetWaves(b.ExpectedUnits, b.PackageParallel) * uint64(b.RepeatCount)
	wantEffective, err := b.Configured.Multiply(multiplier)
	if err != nil || wantEffective != b.Effective {
		return errors.Join(core.ErrPrimitiveContract, fmt.Errorf("execution effective budget %d ns does not equal configured budget %d ns multiplied by %d bounded waves", b.Effective.Nanoseconds(), b.Configured.Nanoseconds(), multiplier), err)
	}
	if multiplier > BudgetDivergenceViolationMultiplier {
		return errors.Join(core.ErrPrimitiveContract, fmt.Errorf("execution budget divergence %dx exceeds infrastructure ceiling %dx", multiplier, BudgetDivergenceViolationMultiplier))
	}
	return nil
}

func (b ExecutionBudget) validateShape() error {
	if err := errors.Join(b.Configured.Validate(), b.Effective.Validate()); err != nil {
		return err
	}
	validBounds := !b.Configured.IsZero() && !b.Effective.IsZero() && b.ExpectedUnits > 0 && b.ExpectedUnits <= ExecutionAccountingUnitMaximum
	validAmplification := b.PackageParallel > 0 && b.RepeatCount == ExecutionRepeatCount
	if !validBounds || !validAmplification {
		return errors.Join(core.ErrPrimitiveContract, errors.New("execution budget contains an invalid configured bound, unit count, package parallelism, or repeat policy"))
	}
	return nil
}

func (b ExecutionBudget) DivergenceMultiplier() (uint64, error) {
	if err := b.Validate(); err != nil {
		return 0, err
	}
	return executionBudgetWaves(b.ExpectedUnits, b.PackageParallel) * uint64(b.RepeatCount), nil
}

func (b ExecutionBudget) ReportDivergence() (bool, error) {
	multiplier, err := b.DivergenceMultiplier()
	return multiplier > BudgetDivergenceReportMultiplier, err
}

func executionBudgetWaves(units uint32, parallel uint16) uint64 {
	width := uint64(parallel)
	return (uint64(units) + width - 1) / width
}

var _ core.Validatable = ExecutionBudget{}
