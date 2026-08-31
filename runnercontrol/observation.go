package runnercontrol

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/temporal"
)

// ExperimentObservationRequest is the typed handoff from domain-blind
// execution into Primitive-owned evidence policy.
type ExperimentObservationRequest struct {
	Capability   ExperimentCapability
	BeganAt      temporal.Instant
	CompletedAt  temporal.Instant
	Process      *process.ResultObservation
	Failure      error
	Artifacts    []projectstandards.ArtifactReference
	Measurements projectstandards.ExperimentMeasurements
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
	return ValidateExperimentMeasurements(r.Capability, r.Failure, r.Process != nil, r.Measurements)
}

func ValidateExperimentMeasurements(capability ExperimentCapability, failure error, started bool, measurements projectstandards.ExperimentMeasurements) error {
	if err := errors.Join(capability.Validate(), measurements.Validate()); err != nil {
		return err
	}
	policy := capability.Execution.Observation
	if policy.Format == ObservationOpaque {
		return validateOpaqueMeasurements(capability.Execution.Artifacts, measurements)
	}
	if policy.Format == ObservationJUnitXML {
		return validateJUnitMeasurements(capability, failure, started, measurements)
	}
	if policy.Format != ObservationGoTestJSON {
		return core.ErrPrimitiveContract
	}
	return validateGoMeasurements(capability, failure, started, measurements)
}

func validateJUnitMeasurements(capability ExperimentCapability, failure error, started bool, measurements projectstandards.ExperimentMeasurements) error {
	if err := validateGoAccounting(capability.Execution.Observation, failure, started, measurements); err != nil {
		return err
	}
	if len(measurements.Benchmarks) != 0 || measurements.CoverageBasisPoints != nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

func validateOpaqueMeasurements(artifacts []ArtifactExpectation, measurements projectstandards.ExperimentMeasurements) error {
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

func validateGoMeasurements(capability ExperimentCapability, failure error, started bool, measurements projectstandards.ExperimentMeasurements) error {
	policy := capability.Execution.Observation
	if err := validateGoAccounting(policy, failure, started, measurements); err != nil {
		return err
	}
	if measurements.Accounting == nil {
		return nil
	}
	if err := validateGoBenchmarks(capability.Probe.Kind, failure, measurements.Benchmarks); err != nil {
		return err
	}
	return validateGoCoverage(capability.Execution.Artifacts, failure, measurements.CoverageBasisPoints)
}

func validateGoAccounting(policy ObservationPolicy, failure error, started bool, measurements projectstandards.ExperimentMeasurements) error {
	if measurements.Accounting == nil {
		if !started && failure != nil && len(measurements.Benchmarks) == 0 {
			return nil
		}
		return core.ErrPrimitiveContract
	}
	accounting := measurements.Accounting
	latest, ok := accounting.Latest()
	if !ok || latest.Planned != policy.ExpectedUnits || latest.Filtered != policy.Filtered || latest.Cache != projectstandards.CacheDisabled {
		return core.ErrPrimitiveContract
	}
	return nil
}

func validateGoBenchmarks(kind projectstandards.ProbeKind, failure error, benchmarks []projectstandards.BenchmarkMeasurement) error {
	if kind != projectstandards.ProbeKindGoBenchmark && len(benchmarks) != 0 {
		return core.ErrPrimitiveContract
	}
	if kind == projectstandards.ProbeKindGoBenchmark && failure == nil && len(benchmarks) == 0 {
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

func CompileExperimentObservation(request ExperimentObservationRequest) (projectstandards.ExperimentObservation, error) {
	if err := request.Validate(); err != nil {
		return projectstandards.ExperimentObservation{}, err
	}
	execution, err := request.Capability.Digest()
	if err != nil {
		return projectstandards.ExperimentObservation{}, err
	}
	measurements, err := compileExperimentMeasurements(request)
	if err != nil {
		return projectstandards.ExperimentObservation{}, err
	}
	observation := projectstandards.ExperimentObservation{
		Experiment: request.Capability.Experiment, Started: request.Process != nil,
		Outcome:                compileExperimentOutcome(request.Process, request.Failure),
		EnvironmentFingerprint: request.Capability.Probe.Environment.EnvironmentFingerprint,
		ExecutionFingerprint:   execution, MachineSheetDigest: request.Capability.Probe.Environment.MachineSheetDigest,
		Measurements: measurements, Artifacts: append([]projectstandards.ArtifactReference(nil), request.Artifacts...),
	}
	return observation, observation.Validate()
}

func compileExperimentMeasurements(request ExperimentObservationRequest) (projectstandards.ExperimentMeasurements, error) {
	measurements := request.Measurements
	measurements.Benchmarks = append([]projectstandards.BenchmarkMeasurement(nil), request.Measurements.Benchmarks...)
	measurements.Complexity = append([]projectstandards.ComplexityCapture(nil), request.Measurements.Complexity...)
	if request.Process == nil {
		if (request.Capability.Execution.Observation.Format == ObservationGoTestJSON || request.Capability.Execution.Observation.Format == ObservationJUnitXML) && measurements.Accounting == nil {
			accounting := compileUnstartedAccounting(request.Capability.Execution.Observation, request.Failure)
			measurements.Accounting = &accounting
		}
		return measurements, nil
	}
	duration, err := request.CompletedAt.Since(request.BeganAt)
	if err != nil || duration.Nanoseconds() <= 0 {
		return projectstandards.ExperimentMeasurements{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	measurements.DurationNs = uint64(duration.Nanoseconds())
	measurements.PeakMemoryBytes = request.Process.PeakMemoryBytes.Uint64()
	return measurements, nil
}

func compileUnstartedAccounting(policy ObservationPolicy, failure error) projectstandards.ExecutionAccounting {
	attempt := projectstandards.ExecutionAttempt{Sequence: 1, Planned: policy.ExpectedUnits, Cache: projectstandards.CacheDisabled, Filtered: policy.Filtered, NotRun: policy.ExpectedUnits - 1}
	switch {
	case errors.Is(failure, context.Canceled):
		attempt.Cancelled = 1
	case errors.Is(failure, context.DeadlineExceeded):
		attempt.TimedOut = 1
	default:
		attempt.Failed = 1
	}
	return projectstandards.ExecutionAccounting{Attempts: []projectstandards.ExecutionAttempt{attempt}}
}

func compileExperimentOutcome(result *process.ResultObservation, failure error) projectstandards.Outcome {
	if errors.Is(failure, context.Canceled) {
		return projectstandards.OutcomeCancelled
	}
	if errors.Is(failure, context.DeadlineExceeded) {
		return projectstandards.OutcomeTimedOut
	}
	if result == nil {
		return projectstandards.OutcomeSetupFailed
	}
	if failure != nil {
		return projectstandards.OutcomeFailed
	}
	if result.ExitCode == 0 {
		return projectstandards.OutcomePassed
	}
	return projectstandards.OutcomeFailed
}

func CompileSelectionObservation(manifest ExpansionManifest, approval ExpansionApproval, executed uint16) (projectstandards.SelectionObservation, error) {
	if err := errors.Join(manifest.Validate(), approval.Validate()); err != nil {
		return projectstandards.SelectionObservation{}, err
	}
	if approval.Run != manifest.Run || approval.ManifestDigest != manifest.Identity {
		return projectstandards.SelectionObservation{}, core.ErrPrimitiveContract
	}
	planned := uint32(manifest.Admitted) + uint32(manifest.Refused)
	admitted := uint32(manifest.Admitted)
	refused := uint32(manifest.Refused)
	if !approval.Approved {
		admitted = 0
		refused = planned
	}
	if uint32(executed) > admitted {
		return projectstandards.SelectionObservation{}, core.ErrPrimitiveContract
	}
	observation := projectstandards.SelectionObservation{
		ExpansionDigest: manifest.Identity, Planned: uint16(planned), Admitted: uint16(admitted),
		Refused: uint16(refused), Executed: executed, NotRun: uint16(admitted - uint32(executed)),
	}
	return observation, observation.Validate()
}
