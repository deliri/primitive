package runnercontrol

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
)

const GoTestJSONEventMaximumBytes = 1 << 20

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

type goTestEventWire struct {
	Time        string  `json:"Time"`
	Action      string  `json:"Action"`
	Package     string  `json:"Package"`
	Test        string  `json:"Test"`
	Output      string  `json:"Output"`
	FailedBuild string  `json:"FailedBuild"`
	Elapsed     float64 `json:"Elapsed"`
}

// GoTestObservationCompiler consumes the exact stdout emitted by go test
// -json. It retains bounded state only for planned package units.
type GoTestObservationCompiler struct {
	failure    error
	seen       map[string]struct{}
	terminal   map[string]string
	pending    []byte
	benchmarks []runprotocol.BenchmarkMeasurement
	policy     ObservationPolicy
}

func NewGoTestObservationCompiler(policy ObservationPolicy) (*GoTestObservationCompiler, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if policy.Format != ObservationGoTestJSON {
		return nil, observationFailure("go test observation compiler requires go-test-json format", core.ErrPrimitiveContract)
	}
	return &GoTestObservationCompiler{policy: policy, seen: make(map[string]struct{}), terminal: make(map[string]string), benchmarks: []runprotocol.BenchmarkMeasurement{}}, nil
}

func (c *GoTestObservationCompiler) Write(data []byte) (int, error) {
	if c == nil {
		return 0, observationFailure(goTestObservationCompilerNilDiagnostic, core.ErrPrimitiveContract)
	}
	if c.failure != nil {
		return 0, c.failure
	}
	written, extentFailure, err := writeBoundedLines(boundedLineWrite{
		pending: &c.pending,
		data:    data,
		maximum: GoTestJSONEventMaximumBytes,
		consume: c.consumeEvent,
	})
	if err != nil {
		c.failure = err
		c.pending = nil
		if !extentFailure {
			return len(data), nil
		}
	}
	return written, err
}

func (c *GoTestObservationCompiler) consumeEvent(line []byte) error {
	event, err := core.DecodeStrictJSONStructure[goTestEventWire](line, core.DefaultStrictJSONLimits())
	if err != nil {
		return observationFailure("go test JSON event cannot be decoded", core.ErrJSONContract, err)
	}
	if !goTestActionKnown(event.Action) {
		return observationFailure("go test JSON event has an unknown action", core.ErrJSONContract)
	}
	if event.Package == "" {
		return observationFailure("go test JSON event is missing its package identity", core.ErrJSONContract)
	}
	if err := c.observePackage(event); err != nil {
		return err
	}
	if event.Action == goTestOutputActionText && strings.Contains(event.Output, " ns/op") {
		measurement, measurementErr := parseGoBenchmarkMeasurement(event.Output)
		if measurementErr != nil {
			return measurementErr
		}
		if len(c.benchmarks) >= runprotocol.BenchmarkMeasurementMaximum {
			return observationFailure("go benchmark measurement count exceeds its ceiling", core.ErrPrimitiveContract)
		}
		c.benchmarks = append(c.benchmarks, measurement)
	}
	return nil
}

func (c *GoTestObservationCompiler) observePackage(event goTestEventWire) error {
	if _, terminal := c.terminal[event.Package]; terminal {
		return observationFailure("go test JSON event follows a terminal package event", core.ErrJSONContract)
	}
	c.seen[event.Package] = struct{}{}
	seen, _, err := c.packageCounts()
	if err != nil || seen > c.policy.ExpectedUnits {
		return observationFailure("go test JSON stream names more packages than planned", core.ErrPrimitiveContract)
	}
	if event.Test != "" || !goTestTerminalAction(event.Action) {
		return nil
	}
	c.terminal[event.Package] = event.Action
	return nil
}

func (c *GoTestObservationCompiler) Seal(executionErr error) (GoTestObservation, error) {
	if c == nil {
		return GoTestObservation{}, observationFailure(goTestObservationCompilerNilDiagnostic, core.ErrPrimitiveContract)
	}
	if c.failure != nil {
		result, err := c.unavailableObservation()
		return result, errors.Join(c.failure, err)
	}
	if len(c.pending) > 0 {
		if len(c.pending) > GoTestJSONEventMaximumBytes {
			c.failure = observationFailure("go test JSON event exceeds the byte ceiling", core.ErrJSONContract)
			result, err := c.unavailableObservation()
			return result, errors.Join(c.failure, err)
		}
		if err := c.consumeEvent(c.pending); err != nil {
			c.failure = err
			result, validationErr := c.unavailableObservation()
			return result, errors.Join(err, validationErr)
		}
		c.pending = nil
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
		case goTestPassActionText:
			attempt.Passed++
		case goTestFailActionText:
			attempt.Failed++
		case goTestSkipActionText:
			attempt.Skipped++
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
		case goTestPassActionText:
			attempt.Passed++
		case goTestFailActionText:
			attempt.Failed++
		case goTestSkipActionText:
			attempt.Skipped++
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
	if observed > c.policy.ExpectedUnits {
		return observationFailure("go test observed package count exceeds planned units", core.ErrPrimitiveContract)
	}
	if executionErr != nil {
		return nil
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

func goTestActionKnown(action string) bool {
	switch action {
	case "start", "run", "pause", "cont", goTestPassActionText, "bench", goTestFailActionText, goTestOutputActionText, goTestSkipActionText:
		return true
	default:
		return false
	}
}

func goTestTerminalAction(action string) bool {
	return action == goTestPassActionText || action == goTestFailActionText || action == goTestSkipActionText
}

func parseGoBenchmarkMeasurement(output string) (runprotocol.BenchmarkMeasurement, error) {
	fields := strings.Fields(output)
	if len(fields) < 8 {
		return runprotocol.BenchmarkMeasurement{}, observationFailure("go benchmark result is incomplete", core.ErrJSONContract)
	}
	name, err := runprotocol.NewName(fields[0])
	if err != nil {
		return runprotocol.BenchmarkMeasurement{}, observationFailure("go benchmark name is invalid", core.ErrJSONContract, err)
	}
	iterations, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil || iterations == 0 {
		return runprotocol.BenchmarkMeasurement{}, observationFailure("go benchmark iteration count is invalid", core.ErrJSONContract, err)
	}
	nanoseconds, nanosecondsErr := benchmarkDecimal(fields, "ns/op")
	bytesPerOp, bytesErr := benchmarkInteger(fields, "B/op")
	allocations, allocationsErr := benchmarkInteger(fields, "allocs/op")
	if err := errors.Join(nanosecondsErr, bytesErr, allocationsErr); err != nil {
		return runprotocol.BenchmarkMeasurement{}, err
	}
	measurement := runprotocol.BenchmarkMeasurement{Name: name, Iterations: iterations, NanosecondsPerOp: nanoseconds, BytesPerOp: bytesPerOp, AllocationsPerOp: allocations}
	return measurement, measurement.Validate()
}

func benchmarkDecimal(fields []string, unit string) (runprotocol.DecimalMeasurement, error) {
	value, ok := benchmarkField(fields, unit)
	if !ok {
		return runprotocol.DecimalMeasurement{}, observationFailure("go benchmark result is missing "+unit, core.ErrJSONContract)
	}
	coefficient, scale, err := parseBenchmarkDecimal(value)
	if err != nil {
		return runprotocol.DecimalMeasurement{}, err
	}
	for scale > 0 && coefficient%10 == 0 {
		coefficient /= 10
		scale--
	}
	result := runprotocol.DecimalMeasurement{Coefficient: coefficient, Scale: scale}
	return result, result.Validate()
}

func parseBenchmarkDecimal(value string) (uint64, uint8, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
		return 0, 0, observationFailure("go benchmark decimal is invalid", core.ErrJSONContract)
	}
	scale := uint8(0)
	if len(parts) == 2 {
		if len(parts[1]) > int(runprotocol.DecimalMeasurementScaleMaximum) {
			return 0, 0, observationFailure("go benchmark decimal precision exceeds its ceiling", core.ErrJSONContract)
		}
		parsedScale, scaleErr := core.CheckedUint8FromInt(len(parts[1]))
		if scaleErr != nil {
			return 0, 0, observationFailure("go benchmark decimal precision exceeds its numeric ceiling", core.ErrJSONContract, scaleErr)
		}
		scale = parsedScale
	}
	coefficient, err := strconv.ParseUint(strings.Join(parts, ""), 10, 64)
	if err != nil {
		return 0, 0, observationFailure("go benchmark decimal exceeds its numeric ceiling", core.ErrJSONContract, err)
	}
	return coefficient, scale, nil
}

func benchmarkInteger(fields []string, unit string) (uint64, error) {
	value, ok := benchmarkField(fields, unit)
	if !ok {
		return 0, observationFailure("go benchmark result is missing "+unit, core.ErrJSONContract)
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, observationFailure("go benchmark integer is invalid for "+unit, core.ErrJSONContract, err)
	}
	return parsed, nil
}

func benchmarkField(fields []string, unit string) (string, bool) {
	for index := 2; index < len(fields); index++ {
		if fields[index] == unit && index > 0 {
			return fields[index-1], true
		}
	}
	return "", false
}

func observationFailure(message string, causes ...error) error {
	joined := []error{core.ErrPrimitiveContract, errors.New(message)}
	joined = append(joined, causes...)
	return errors.Join(joined...)
}

var _ io.Writer = (*GoTestObservationCompiler)(nil)
