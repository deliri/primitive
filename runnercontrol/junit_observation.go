package runnercontrol

import (
	"encoding/xml"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
)

type JUnitObservation struct {
	Accounting runprotocol.ExecutionAccounting `json:"accounting"`
}

func (o JUnitObservation) Validate() error { return o.Accounting.Validate() }

// JUnitObservationCompiler projects observed cases against an independently
// known plan. ExpectedUnits is never inferred from a report or a test-file count.
type JUnitObservationCompiler struct {
	compiler *JUnitSummaryCompiler
	policy   ObservationPolicy
}

func NewJUnitObservationCompiler(policy ObservationPolicy) (*JUnitObservationCompiler, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if policy.Format != ObservationJUnitXML {
		return nil, observationFailure("JUnit observation compiler requires junit-xml format", core.ErrPrimitiveContract)
	}
	return &JUnitObservationCompiler{policy: policy, compiler: NewJUnitSummaryCompiler()}, nil
}

func (c *JUnitObservationCompiler) Write(data []byte) (int, error) {
	if c == nil {
		return 0, observationFailure("JUnit observation compiler is not writable", core.ErrPrimitiveContract)
	}
	return c.compiler.Write(data)
}

func (c *JUnitObservationCompiler) Seal(executionErr error) (JUnitObservation, error) {
	if c == nil || c.compiler == nil || c.compiler.sealed {
		return JUnitObservation{}, observationFailure("JUnit observation compiler cannot be sealed", core.ErrPrimitiveContract)
	}
	summary, err := c.compiler.Seal(executionErr)
	if err != nil {
		return unavailableJUnitObservation(c.policy), err
	}
	cases, err := summary.Cases()
	if err != nil || cases > uint64(c.policy.ExpectedUnits) {
		return unavailableJUnitObservation(c.policy), observationFailure("JUnit XML names more test cases than planned", core.ErrPrimitiveContract, err)
	}
	attempt := runprotocol.ExecutionAttempt{
		Sequence: 1, Planned: c.policy.ExpectedUnits, Cache: runprotocol.CacheDisabled,
		Filtered: c.policy.Filtered, Passed: uint32(summary.Passed), Failed: uint32(summary.Failed),
		Skipped: uint32(summary.Skipped), NotRun: c.policy.ExpectedUnits - uint32(cases),
	}
	observation := JUnitObservation{Accounting: runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{attempt}}}
	return observation, observation.Validate()
}

// Abort releases an unsealed compiler and joins its parser. It is idempotent.
func (c *JUnitObservationCompiler) Abort() {
	if c != nil {
		c.compiler.Abort()
	}
}

func duplicateJUnitAttribute(attributes []xml.Attr) bool {
	for index := range attributes {
		for previous := range index {
			if attributes[previous].Name == attributes[index].Name {
				return true
			}
		}
	}
	return false
}

func unavailableJUnitObservation(policy ObservationPolicy) JUnitObservation {
	return JUnitObservation{Accounting: runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{{Sequence: 1, Planned: policy.ExpectedUnits, Unavailable: policy.ExpectedUnits, Cache: runprotocol.CacheDisabled, Filtered: policy.Filtered}}}}
}

var (
	_ core.Validatable = JUnitObservation{}
	_ io.Writer        = (*JUnitObservationCompiler)(nil)
)

const junitErrorElement = "error"
