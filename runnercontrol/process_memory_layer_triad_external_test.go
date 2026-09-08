package runnercontrol_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestProcessMemoryReceiverLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		bytes       uint64
		reported    bool
		invalidExit bool
		wantErr     error
	}{
		{name: "positive/reported memory replaces stale measurement", bytes: 31, reported: true},
		{name: "boundary/reported zero replaces stale measurement", reported: true},
		{name: "neutral/unreported memory clears stale measurement"},
		{name: "negative/invalid process facts cannot become measurements", invalidExit: true, wantErr: core.ErrProcessContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := experimentObservationRequestFixture(t)
			request.Measurements.PeakMemoryBytes = 97
			if tc.reported {
				value, err := core.NewByteLength(tc.bytes)
				if err != nil {
					t.Fatal(err)
				}
				request.Process.PeakMemoryBytes = &value
			}
			if tc.invalidExit {
				request.Process.ExitCode = core.ProcessExitCodeMaximum + 1
			}
			got, err := runnercontrol.CompileExperimentObservation(request)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("receiver error = %v, want %v", err, tc.wantErr)
			}
			if request.Measurements.PeakMemoryBytes != 97 {
				t.Fatalf("receiver changed source memory=%d; want 97", request.Measurements.PeakMemoryBytes)
			}
			if tc.wantErr != nil {
				if got.Artifacts != nil || got.Measurements.CoverageBasisPoints != nil || got.Measurements.Accounting != nil || got.Measurements.Benchmarks != nil || got.Measurements.Scaling != nil || got.Measurements.DurationNs != 0 || got.Measurements.PeakMemoryBytes != 0 || got.EnvironmentFingerprint != (core.SHA256Digest{}) || got.ExecutionFingerprint != (core.SHA256Digest{}) || got.MachineSheetDigest != (core.SHA256Digest{}) || got.Experiment != (runprotocol.ExperimentID{}) || got.Started || got.Outcome != runprotocol.OutcomeUnknown {
					t.Fatalf("receiver exposed partial measurement: %+v", got)
				}
				return
			}
			if !got.Started || got.Outcome != runprotocol.OutcomePassed || got.Measurements.PeakMemoryBytes != tc.bytes {
				t.Fatalf("receiver misreported memory/exit: %+v", got)
			}
			if (request.Process.PeakMemoryBytes != nil) != tc.reported {
				t.Fatalf("receiver changed source availability=%t; want %t", request.Process.PeakMemoryBytes != nil, tc.reported)
			}
		})
	}
}
