package process

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestResultMemoryOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		bytes    uint64
		reported bool
		unset    bool
		wantErr  error
	}{
		{name: "neutral/unreported memory remains unavailable"},
		{name: "boundary/reported zero is distinct from unreported", reported: true},
		{name: "positive/reported bytes remain exact", reported: true, bytes: 1},
		{name: "boundary/largest byte length remains exact", reported: true, bytes: math.MaxInt64},
		{name: "contradiction/unreported memory cannot carry bytes", bytes: 1, wantErr: core.ErrProcessContract},
		{name: "negative/unsealed result cannot expose reported memory", reported: true, unset: true, wantErr: core.ErrProcessContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			memory, err := core.NewByteLength(tc.bytes)
			if err != nil {
				t.Fatal(err)
			}
			result := resultFactsFixture(0)
			result.peakMemory = memory
			result.peakMemoryReported = tc.reported
			result.set = !tc.unset
			observation, observationErr := result.Observation()
			if !errors.Is(result.Validate(), tc.wantErr) || !errors.Is(observationErr, tc.wantErr) {
				t.Fatalf("memory validation/projection error = %v / %v, want %v", result.Validate(), observationErr, tc.wantErr)
			}
			got, gotErr := result.PeakMemoryBytes()
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got.Uint64() != 0 || observation != (ResultObservation{}) {
					t.Fatalf("invalid memory exposed an observation: %+v, %v", observation, gotErr)
				}
				return
			}
			if !tc.reported {
				if !errors.Is(gotErr, core.ErrProcessUnsupported) || got.Uint64() != 0 || observation.PeakMemoryBytes != nil {
					t.Fatalf("unavailable memory became measured data: %+v, %v", observation, gotErr)
				}
				return
			}
			if gotErr != nil || got.Uint64() != tc.bytes || observation.PeakMemoryBytes == nil || observation.PeakMemoryBytes.Uint64() != tc.bytes {
				t.Fatalf("reported memory changed: %+v, %v", observation, gotErr)
			}
			replacement, err := core.NewByteLength(2)
			if err != nil {
				t.Fatal(err)
			}
			*observation.PeakMemoryBytes = replacement
			after, afterErr := result.PeakMemoryBytes()
			if afterErr != nil || after.Uint64() != tc.bytes {
				t.Fatalf("caller mutation changed sealed memory: %v, %v", after, afterErr)
			}
		})
	}
}

func TestResultCounterProjectionKeepsEveryOwner(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                      string
		input, output, diagnostic uint64
		cpu                       int64
	}{
		{name: "all different counters cannot be swapped", input: 2, output: 3, diagnostic: 5, cpu: 7},
		{name: "stdin-only observation cannot become output", input: 1},
		{name: "stdout-only observation cannot become stderr", output: 1},
		{name: "stderr-only observation cannot become stdout", diagnostic: 1},
		{name: "cpu-only observation cannot become a byte count", cpu: 1},
		{name: "maximum stdin is not narrowed", input: math.MaxInt64},
		{name: "maximum stdout is not narrowed", output: math.MaxInt64},
		{name: "maximum stderr is not narrowed", diagnostic: math.MaxInt64},
		{name: "maximum cpu is not narrowed", cpu: math.MaxInt64},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input, a := core.NewByteLength(tc.input)
			output, b := core.NewByteLength(tc.output)
			diagnostic, c := core.NewByteLength(tc.diagnostic)
			cpu, d := temporal.DurationFromNanoseconds(tc.cpu)
			if err := errors.Join(a, b, c, d); err != nil {
				t.Fatal(err)
			}
			r := resultFactsFixture(0)
			r.stdinBytes = input
			r.stdoutBytes = output
			r.stderrBytes = diagnostic
			r.cpu = cpu
			got, err := r.Observation()
			if err != nil || got.StdinBytes.Uint64() != tc.input || got.StdoutBytes.Uint64() != tc.output || got.StderrBytes.Uint64() != tc.diagnostic || got.CPUTime.Nanoseconds() != tc.cpu {
				t.Fatalf("observation changed source counters: %+v, %v", got, err)
			}
			in, ie := r.StdinBytes()
			out, oe := r.StdoutBytes()
			diag, de := r.StderrBytes()
			duration, ce := r.CPUTime()
			if err := errors.Join(ie, oe, de, ce); err != nil || in != input || out != output || diag != diagnostic || duration != cpu {
				t.Fatalf("accessors changed source counters: %v", err)
			}
		})
	}
}
