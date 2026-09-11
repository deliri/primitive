package runnercontrol_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestExperimentObservationOwnsCoverageAndNestedScalingSamples(t *testing.T) {
	t.Parallel()
	request := experimentObservationRequestFixture(t)
	request.Capability.Execution.Observation = runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationGoTestJSON, ExpectedUnits: 1}
	limit, err := core.NewByteCount(1024)
	if err != nil {
		t.Fatal(err)
	}
	request.Capability.Execution.Artifacts = []runnercontrol.ArtifactExpectation{{Kind: runnercontrol.ArtifactCoverage, Path: mustProfileSourcePath(t, "coverage.out"), MediaType: core.HTTPMediaTypeOctetStream(), MaximumBytes: limit}}
	coverage := uint16(5000)
	request.Measurements.CoverageBasisPoints = &coverage
	request.Measurements.Accounting = &runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{{Sequence: 1, Planned: 1, Passed: 1, Cache: runprotocol.CacheDisabled}}}
	identity := mustProfileIdentifier(t, "scaling")
	samples := []runprotocol.ScalingSample{{InputSize: 1, Iterations: 1, NanosecondsPerOp: 10}, {InputSize: 2, Iterations: 1, NanosecondsPerOp: 20}, {InputSize: 3, Iterations: 1, NanosecondsPerOp: 30}}
	request.Measurements.Scaling = []runprotocol.ScalingCapture{{Measurement: identity, Profile: runprotocol.ProfileIdentity{Name: identity, Version: 1}, Samples: samples, BudgetSeconds: 30, EnvironmentFingerprint: core.SHA256Of([]byte("machine"))}}
	got, err := runnercontrol.CompileExperimentObservation(request)
	if err != nil {
		t.Fatalf("CompileExperimentObservation() error = %v, want nil", err)
	}
	coverage = 7000
	samples[0].NanosecondsPerOp = 99
	if *got.Measurements.CoverageBasisPoints != 5000 || got.Measurements.Scaling[0].Samples[0].NanosecondsPerOp != 10 {
		t.Fatalf("retained measurements = %+v, want original coverage and samples", got.Measurements)
	}
	*got.Measurements.CoverageBasisPoints = 6000
	got.Measurements.Scaling[0].Samples[1].NanosecondsPerOp = 88
	if coverage != 7000 || samples[1].NanosecondsPerOp != 20 {
		t.Fatalf("source measurements = %d/%+v, want 7000/original second sample", coverage, samples)
	}
}
