package runnercontrol

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
)

type GoTestObservation struct {
	Accounting runprotocol.ExecutionAccounting    `json:"accounting"`
	Benchmarks []runprotocol.BenchmarkMeasurement `json:"benchmarks"`
}

func (o GoTestObservation) Validate() error {
	if err := o.Accounting.Validate(); err != nil {
		return err
	}
	if len(o.Benchmarks) > runprotocol.BenchmarkMeasurementMaximum {
		return core.ErrPrimitiveContract
	}
	for index := range o.Benchmarks {
		if err := o.Benchmarks[index].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// GoTestObservationCompiler consumes the exact stdout emitted by go test
// -json. It retains bounded state only for planned package units.
type GoTestObservationCompiler struct {
	failure     error
	seen        map[[32]byte]struct{}
	terminal    map[[32]byte]GoEventAction
	stream      goJSONStream
	benchmarks  []runprotocol.BenchmarkMeasurement
	policy      ObservationPolicy
	buildFailed bool
	sealed      bool
}

func NewGoTestObservationCompiler(policy ObservationPolicy) (*GoTestObservationCompiler, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if policy.Format != ObservationGoTestJSON {
		return nil, observationFailure("go test observation compiler requires go-test-json format", core.ErrPrimitiveContract)
	}
	return &GoTestObservationCompiler{policy: policy, seen: make(map[[32]byte]struct{}), terminal: make(map[[32]byte]GoEventAction), benchmarks: []runprotocol.BenchmarkMeasurement{}}, nil
}

func (c *GoTestObservationCompiler) Write(data []byte) (int, error) {
	if c == nil || c.seen == nil || c.terminal == nil || c.sealed {
		return 0, observationFailure(goTestObservationCompilerNilDiagnostic, core.ErrPrimitiveContract)
	}
	if c.failure != nil {
		return 0, c.failure
	}
	for _, value := range data {
		if err := c.stream.consume(value, c.consumeProjection); err != nil {
			c.failure = err
			return len(data), nil
		}
	}
	return len(data), nil
}

func (c *GoTestObservationCompiler) consumeProjection(event goJSONProjection) error {
	build := event.action == GoEventActionBuildOutput || event.action == GoEventActionBuildFail
	if build {
		allowed := uint16(1)<<GoEventFieldAction | uint16(1)<<GoEventFieldImportPath | uint16(1)<<GoEventFieldOutput
		if event.fields & ^allowed != 0 {
			return goJSONFailure()
		}
		if event.action == GoEventActionBuildOutput {
			if !event.outputPresent {
				return goJSONFailure()
			}
		} else {
			if event.outputPresent {
				return goJSONFailure()
			}
			c.buildFailed = true
		}
		return nil
	}
	if event.fields&(uint16(1)<<GoEventFieldImportPath) != 0 || event.action == GoEventActionUnknown || !event.packagePresent {
		return goJSONFailure()
	}
	if event.outputType != GoEventOutputOrdinary && event.action != GoEventActionOutput {
		return goJSONFailure()
	}
	if err := c.observePackage(event); err != nil {
		return err
	}
	if event.action == GoEventActionOutput {
		measurement, present, err := event.benchmark.result()
		if err != nil {
			return err
		}
		if present {
			if len(c.benchmarks) >= runprotocol.BenchmarkMeasurementMaximum {
				return observationFailure("go benchmark aggregate exceeds its nominal capacity", core.ErrPrimitiveContract)
			}
			c.benchmarks = append(c.benchmarks, measurement)
		}
	}
	return nil
}

func (c *GoTestObservationCompiler) observePackage(event goJSONProjection) error {
	if _, terminal := c.terminal[event.packageID]; terminal {
		return observationFailure("go test JSON event follows a terminal package event", core.ErrJSONContract)
	}
	c.seen[event.packageID] = struct{}{}
	seen, _, err := c.packageCounts()
	if err != nil || seen > c.policy.ExpectedUnits {
		return observationFailure("go test JSON stream names more packages than planned", core.ErrPrimitiveContract)
	}
	if event.testPresent || !goTestTerminalAction(event.action) {
		return nil
	}
	c.terminal[event.packageID] = event.action
	return nil
}

func (c *GoTestObservationCompiler) Seal(executionErr error) (GoTestObservation, error) {
	if c == nil || c.seen == nil || c.terminal == nil || c.sealed {
		return GoTestObservation{}, observationFailure(goTestObservationCompilerNilDiagnostic, core.ErrPrimitiveContract)
	}
	c.sealed = true
	if c.failure != nil {
		result, err := c.unavailableObservation()
		return result, errors.Join(c.failure, err)
	}
	if err := c.stream.finish(c.consumeProjection); err != nil {
		c.failure = err
		result, validationErr := c.unavailableObservation()
		return result, errors.Join(err, validationErr)
	}
	accounting, err := c.compileAccounting(executionErr)
	if err != nil {
		result, validationErr := c.unavailableObservation()
		return result, errors.Join(err, validationErr)
	}
	result := GoTestObservation{Accounting: accounting, Benchmarks: append([]runprotocol.BenchmarkMeasurement(nil), c.benchmarks...)}
	return result, result.Validate()
}

func (c *GoTestObservationCompiler) unavailableObservation() (GoTestObservation, error) {
	attempt := newExecutionAttempt(c.policy)
	for _, action := range c.terminal {
		switch action {
		case GoEventActionPass:
			attempt.Passed++
		case GoEventActionFail:
			attempt.Failed++
		case GoEventActionSkip:
			attempt.Skipped++
		default:
			return GoTestObservation{}, goJSONFailure()
		}
	}
	_, terminal, countErr := c.packageCounts()
	if countErr != nil {
		return GoTestObservation{}, observationFailure("go test terminal count exceeds its numeric ceiling", core.ErrPrimitiveContract, countErr)
	}
	if terminal > c.policy.ExpectedUnits {
		return GoTestObservation{}, observationFailure("go test terminal count exceeds planned units", core.ErrPrimitiveContract)
	}
	attempt.Unavailable = c.policy.ExpectedUnits - terminal
	accounting := runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{attempt}}
	result := GoTestObservation{Accounting: accounting, Benchmarks: append([]runprotocol.BenchmarkMeasurement(nil), c.benchmarks...)}
	return result, result.Validate()
}

func (c *GoTestObservationCompiler) compileAccounting(executionErr error) (runprotocol.ExecutionAccounting, error) {
	attempt := newExecutionAttempt(c.policy)
	for _, action := range c.terminal {
		switch action {
		case GoEventActionPass:
			attempt.Passed++
		case GoEventActionFail:
			attempt.Failed++
		case GoEventActionSkip:
			attempt.Skipped++
		default:
			return runprotocol.ExecutionAccounting{}, goJSONFailure()
		}
	}
	seen, terminal, countErr := c.packageCounts()
	if countErr != nil || terminal > seen {
		return runprotocol.ExecutionAccounting{}, observationFailure("go test package accounting exceeds its numeric ceiling", core.ErrPrimitiveContract, countErr)
	}
	active := seen - terminal
	observed := terminal + active
	if err := c.validateObservedAccounting(observed, terminal, executionErr); err != nil {
		return runprotocol.ExecutionAccounting{}, err
	}
	c.classifyInterrupted(&attempt, active, executionErr)
	attempt.NotRun = c.policy.ExpectedUnits - observed
	if executionErr != nil && observed == 0 {
		attempt.NotRun--
		c.classifyInterrupted(&attempt, 1, executionErr)
	}
	accounting := runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{attempt}}
	return accounting, accounting.Validate()
}

func newExecutionAttempt(policy ObservationPolicy) runprotocol.ExecutionAttempt {
	return runprotocol.ExecutionAttempt{Sequence: 1, Planned: policy.ExpectedUnits, Cache: runprotocol.CacheDisabled, Filtered: policy.Filtered}
}

func (c *GoTestObservationCompiler) validateObservedAccounting(observed, terminal uint32, executionErr error) error {
	if c.buildFailed && executionErr == nil {
		return observationFailure("go test exited successfully after a build failure", core.ErrPrimitiveContract)
	}
	if observed > c.policy.ExpectedUnits {
		return observationFailure("go test observed package count exceeds planned units", core.ErrPrimitiveContract)
	}
	if executionErr != nil {
		return nil
	}
	for _, action := range c.terminal {
		if action == GoEventActionFail {
			return observationFailure("go test exited successfully after a failed package", core.ErrPrimitiveContract)
		}
	}
	if observed != terminal {
		return observationFailure("go test exited successfully with an unterminated package", core.ErrPrimitiveContract)
	}
	if terminal != c.policy.ExpectedUnits {
		return observationFailure("go test exited successfully without terminal evidence for every planned package", core.ErrPrimitiveContract)
	}
	return nil
}

func (c *GoTestObservationCompiler) packageCounts() (uint32, uint32, error) {
	seen, seenErr := core.CheckedUint32FromInt(len(c.seen))
	terminal, terminalErr := core.CheckedUint32FromInt(len(c.terminal))
	return seen, terminal, errors.Join(seenErr, terminalErr)
}

func (c *GoTestObservationCompiler) classifyInterrupted(accounting *runprotocol.ExecutionAttempt, count uint32, executionErr error) {
	if count == 0 {
		return
	}
	switch {
	case errors.Is(executionErr, context.Canceled):
		accounting.Cancelled += count
	case errors.Is(executionErr, context.DeadlineExceeded):
		accounting.Expired += count
	default:
		accounting.Failed += count
	}
}

func goTestTerminalAction(action GoEventAction) bool {
	return action == GoEventActionPass || action == GoEventActionFail || action == GoEventActionSkip
}

func observationFailure(message string, causes ...error) error {
	joined := []error{core.ErrPrimitiveContract, errors.New(message)}
	joined = append(joined, causes...)
	return errors.Join(joined...)
}

var _ io.Writer = (*GoTestObservationCompiler)(nil)
