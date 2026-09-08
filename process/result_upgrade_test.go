package process

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestResultFactsLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		value    Result
		wantErr  error
		wantExit int64
	}{
		{name: "neutral/reaped zero counters remain valid facts", value: resultFactsFixture(0)},
		{name: "positive/nonzero normal exit remains exact", value: resultFactsFixture(7), wantExit: 7},
		{name: "boundary/largest signed wire code remains exact", value: resultFactsFixture(math.MaxInt32), wantExit: math.MaxInt32},
		{name: "boundary/first unsigned-only Windows code remains exact", value: resultFactsFixture(math.MaxInt32 + 1), wantExit: math.MaxInt32 + 1},
		{name: "boundary/largest Windows exit code remains exact", value: resultFactsFixture(math.MaxUint32), wantExit: math.MaxUint32},
		{name: "negative/unset result cannot publish an observation", wantErr: core.ErrProcessContract},
		{name: "negative/unset exit cannot be published", value: Result{set: true}, wantErr: core.ErrProcessContract},
		{name: "boundary/below signaled marker is invalid", value: resultFactsFixture(-2), wantErr: core.ErrProcessContract},
		{name: "boundary/one beyond native unsigned code is invalid", value: resultFactsFixture(math.MaxUint32 + 1), wantErr: core.ErrProcessContract},
		{name: "boundary/signaled marker without a reported signal is preserved", value: resultFactsFixture(-1), wantExit: -1},
		{name: "contradiction/normal exit cannot also have a signal", value: Result{set: true, exit: ExitCode{set: true, value: 0}, signalReported: true, signal: 1}, wantErr: core.ErrProcessContract},
		{name: "contradiction/unreported signal cannot contain a value", value: Result{set: true, exit: ExitCode{set: true, value: -1}, signal: 1}, wantErr: core.ErrProcessContract},
		{name: "negative/reported signal cannot be zero", value: Result{set: true, exit: ExitCode{set: true, value: -1}, signalReported: true}, wantErr: core.ErrProcessContract},
		{name: "negative/reported signal cannot be negative", value: Result{set: true, exit: ExitCode{set: true, value: -1}, signalReported: true, signal: -1}, wantErr: core.ErrProcessContract},
		{name: "positive/reported signal belongs to signaled exit", value: Result{set: true, exit: ExitCode{set: true, value: -1}, signalReported: true, signal: 1}, wantExit: -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			validationErr := tc.value.Validate()
			got, err := tc.value.Observation()
			if !errors.Is(validationErr, tc.wantErr) || !errors.Is(err, tc.wantErr) {
				t.Fatalf("result validation/projection = %v / %v, want %v", validationErr, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (ResultObservation{}) {
					t.Fatalf("refusal exposed partial observation: %+v", got)
				}
				return
			}
			if got.Validate() != nil || int64(got.ExitCode) != tc.wantExit || (got.TerminationSignal != nil) != tc.value.signalReported {
				t.Fatalf("result facts changed in projection: %+v", got)
			}
			if got.TerminationSignal != nil {
				if *got.TerminationSignal != tc.value.signal {
					t.Fatalf("projected signal = %v, want %v", *got.TerminationSignal, tc.value.signal)
				}
				*got.TerminationSignal = 0
				if _, err := tc.value.TerminationSignal(); err != nil {
					t.Fatalf("mutating observation damaged sealed result: %v", err)
				}
			}
		})
	}
}

func resultFactsFixture(exit int) Result {
	return Result{set: true, exit: ExitCode{set: true, value: exit}}
}
