package runnercontrol

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/standard"
	"github.com/deliri/primitive/v2026/temporal"
)

// ExperimentObservationRequest is the typed handoff from domain-blind
// execution into Primitive-owned evidence policy.
type ExperimentObservationRequest struct {
	Failure      error
	Process      *process.ResultObservation
	Artifacts    []standard.ArtifactReference
	Measurements standard.ExperimentMeasurements
	Capability   ExperimentCapability
	BeganAt      temporal.Instant
	CompletedAt  temporal.Instant
}

func (r ExperimentObservationRequest) Validate() error {
	if err := errors.Join(r.Capability.Validate(), r.BeganAt.Validate(), r.CompletedAt.Validate()); err != nil {
		return err
	}
	comparison, err := r.BeganAt.Compare(r.CompletedAt)
	if err != nil || comparison == core.ComparisonGreater {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if r.Process == nil && r.Failure == nil {
		return core.ErrPrimitiveContract
	}
	if r.Process != nil {
		if err := r.Process.Validate(); err != nil {
			return err
		}
	}
	for index := range r.Artifacts {
		if err := r.Artifacts[index].Validate(); err != nil {
			return err
		}
	}
	return validateExperimentMeasurements(measurementValidation{capability: r.Capability, failure: r.Failure, started: r.Process != nil, measurements: r.Measurements})
}

type measurementValidation struct {
	failure      error
	measurements standard.ExperimentMeasurements
	capability   ExperimentCapability
	started      bool
}

func validateExperimentMeasurements(request measurementValidation) error {
	if err := errors.Join(request.capability.Validate(), request.measurements.Validate()); err != nil {
		return err
	}
	policy := request.capability.Execution.Observation
	if policy.Format == ObservationOpaque {
		return validateOpaqueMeasurements(request.capability.Execution.Artifacts, request.measurements)
	}
	if policy.Format == ObservationJUnitXML {
		return validateJUnitMeasurements(request)
	}
	if policy.Format != ObservationGoTestJSON {
		return core.ErrPrimitiveContract
	}
	return validateGoMeasurements(request)
}

func validateJUnitMeasurements(request measurementValidation) error {
	if err := validateGoAccounting(request); err != nil {
		return err
	}
	if len(request.measurements.Benchmarks) != 0 || request.measurements.CoverageBasisPoints != nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

func validateOpaqueMeasurements(artifacts []ArtifactExpectation, measurements standard.ExperimentMeasurements) error {
	if measurements.Accounting != nil || len(measurements.Benchmarks) != 0 || measurements.CoverageBasisPoints != nil {
		return core.ErrPrimitiveContract
	}
	for index := range artifacts {
		if artifacts[index].Kind == ArtifactCoverage {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func validateGoMeasurements(request measurementValidation) error {
	if err := validateGoAccounting(request); err != nil {
		return err
	}
	if request.measurements.Accounting == nil {
		return nil
	}
	if err := validateGoBenchmarks(request.capability.Probe.Kind, request.failure, request.measurements.Benchmarks); err != nil {
		return err
	}
	return validateGoCoverage(request.capability.Execution.Artifacts, request.failure, request.measurements.CoverageBasisPoints)
}

func validateGoAccounting(request measurementValidation) error {
	if request.measurements.Accounting == nil {
		if !request.started && request.failure != nil && len(request.measurements.Benchmarks) == 0 {
			return nil
		}
		return core.ErrPrimitiveContract
	}
	accounting := request.measurements.Accounting
	latest, ok := accounting.Latest()
	policy := request.capability.Execution.Observation
	if !ok || latest.Planned != policy.ExpectedUnits || latest.Filtered != policy.Filtered || latest.Cache != standard.CacheDisabled {
		return core.ErrPrimitiveContract
	}
	return nil
}

func validateGoBenchmarks(kind standard.ProbeKind, failure error, benchmarks []standard.BenchmarkMeasurement) error {
	if kind != standard.ProbeKindGoBenchmark && len(benchmarks) != 0 {
		return core.ErrPrimitiveContract
	}
	if kind == standard.ProbeKindGoBenchmark && failure == nil && len(benchmarks) == 0 {
		return core.ErrPrimitiveContract
	}
	return nil
}

func validateGoCoverage(artifacts []ArtifactExpectation, failure error, coverage *uint16) error {
	coverageCount := artifactKindCount(artifacts, ArtifactCoverage)
	if coverageCount > 1 {
		return core.ErrPrimitiveContract
	}
	coverageExpected := coverageCount == 1
	if (!coverageExpected && coverage != nil) || (coverageExpected && failure == nil && coverage == nil) {
		return core.ErrPrimitiveContract
	}
	return nil
}

func artifactKindCount(artifacts []ArtifactExpectation, kind ArtifactKind) uint16 {
	var count uint16
	for index := range artifacts {
		if artifacts[index].Kind == kind {
			count++
		}
	}
	return count
}

func CompileExperimentObservation(request ExperimentObservationRequest) (standard.ExperimentObservation, error) {
	if err := request.Validate(); err != nil {
		return standard.ExperimentObservation{}, err
	}
	execution, err := request.Capability.Digest()
	if err != nil {
		return standard.ExperimentObservation{}, err
	}
	measurements, err := compileExperimentMeasurements(request)
	if err != nil {
		return standard.ExperimentObservation{}, err
	}
	observation := standard.ExperimentObservation{
		Experiment: request.Capability.Experiment, Started: request.Process != nil,
		Outcome:                compileExperimentOutcome(request.Process, request.Failure),
		EnvironmentFingerprint: request.Capability.Probe.Environment.EnvironmentFingerprint,
		ExecutionFingerprint:   execution, MachineSheetDigest: request.Capability.Probe.Environment.MachineSheetDigest,
		Measurements: measurements, Artifacts: append([]standard.ArtifactReference(nil), request.Artifacts...),
	}
	return observation, observation.Validate()
}

func compileExperimentMeasurements(request ExperimentObservationRequest) (standard.ExperimentMeasurements, error) {
	measurements := request.Measurements
	measurements.Benchmarks = append([]standard.BenchmarkMeasurement(nil), request.Measurements.Benchmarks...)
	measurements.Complexity = append([]standard.ComplexityCapture(nil), request.Measurements.Complexity...)
	if request.Process == nil {
		if (request.Capability.Execution.Observation.Format == ObservationGoTestJSON || request.Capability.Execution.Observation.Format == ObservationJUnitXML) && measurements.Accounting == nil {
			accounting := compileUnstartedAccounting(request.Capability.Execution.Observation, request.Failure)
			measurements.Accounting = &accounting
		}
		return measurements, nil
	}
	duration, err := request.CompletedAt.Since(request.BeganAt)
	if err != nil || duration.Nanoseconds() <= 0 {
		return standard.ExperimentMeasurements{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	durationNanoseconds, err := core.CheckedUint64FromInt64(duration.Nanoseconds())
	if err != nil {
		return standard.ExperimentMeasurements{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	measurements.DurationNs = durationNanoseconds
	measurements.PeakMemoryBytes = request.Process.PeakMemoryBytes.Uint64()
	return measurements, nil
}

func compileUnstartedAccounting(policy ObservationPolicy, failure error) standard.ExecutionAccounting {
	attempt := standard.ExecutionAttempt{Sequence: 1, Planned: policy.ExpectedUnits, Cache: standard.CacheDisabled, Filtered: policy.Filtered, NotRun: policy.ExpectedUnits - 1}
	switch {
	case errors.Is(failure, context.Canceled):
		attempt.Cancelled = 1
	case errors.Is(failure, context.DeadlineExceeded):
		attempt.Expired = 1
	default:
		attempt.Failed = 1
	}
	return standard.ExecutionAccounting{Attempts: []standard.ExecutionAttempt{attempt}}
}

func compileExperimentOutcome(result *process.ResultObservation, failure error) standard.Outcome {
	if errors.Is(failure, context.Canceled) {
		return standard.OutcomeCancelled
	}
	if errors.Is(failure, context.DeadlineExceeded) {
		return standard.OutcomeTimedOut
	}
	if result == nil {
		return standard.OutcomeSetupFailed
	}
	if failure != nil {
		return standard.OutcomeFailed
	}
	if result.ExitCode == 0 {
		return standard.OutcomePassed
	}
	return standard.OutcomeFailed
}

func CompileSelectionObservation(manifest ExpansionManifest, approval ExpansionApproval, executed uint16) (standard.SelectionObservation, error) {
	if err := errors.Join(manifest.Validate(), approval.Validate()); err != nil {
		return standard.SelectionObservation{}, err
	}
	manifestDigest, err := manifest.Digest()
	if err != nil || approval.Run != manifest.Run || approval.ManifestDigest != manifestDigest {
		return standard.SelectionObservation{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	planned := uint32(manifest.Admitted) + uint32(manifest.Refused)
	admitted := uint32(manifest.Admitted)
	refused := uint32(manifest.Refused)
	if !approval.Approved {
		admitted = 0
		refused = planned
	}
	if uint32(executed) > admitted {
		return standard.SelectionObservation{}, core.ErrPrimitiveContract
	}
	plannedCount, plannedErr := checkedUint16FromUint64(uint64(planned))
	admittedCount, admittedErr := checkedUint16FromUint64(uint64(admitted))
	refusedCount, refusedErr := checkedUint16FromUint64(uint64(refused))
	notRunCount, notRunErr := checkedUint16FromUint64(uint64(admitted - uint32(executed)))
	if err := errors.Join(plannedErr, admittedErr, refusedErr, notRunErr); err != nil {
		return standard.SelectionObservation{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	observation := standard.SelectionObservation{
		ExpansionIdentity: manifest.Identity, ManifestDigest: manifestDigest, Planned: plannedCount, Admitted: admittedCount,
		Refused: refusedCount, Executed: executed, NotRun: notRunCount,
	}
	return observation, observation.Validate()
}
