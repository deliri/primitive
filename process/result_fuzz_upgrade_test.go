package process_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzResultObservationExternalIngress(f *testing.F) {
	request := processRequest(f, "silent", process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
	result, err := process.Run(f.Context(), request)
	if err != nil {
		f.Fatal(err)
	}
	seed, err := result.Observation()
	if err != nil {
		f.Fatal(err)
	}
	maximum, err := core.NewByteLength(math.MaxInt64)
	if err != nil {
		f.Fatal(err)
	}
	duration, err := temporal.DurationFromNanoseconds(math.MaxInt64)
	if err != nil {
		f.Fatal(err)
	}
	zero, err := core.NewByteLength(0)
	if err != nil {
		f.Fatal(err)
	}
	signal := process.SignalNumber(1)
	for _, change := range []func(*process.ResultObservation){
		func(*process.ResultObservation) {},
		func(o *process.ResultObservation) { o.PeakMemoryBytes = nil },
		func(o *process.ResultObservation) { o.PeakMemoryBytes = &zero },
		func(o *process.ResultObservation) { o.PeakMemoryBytes = &maximum },
		func(o *process.ResultObservation) { o.ExitCode = core.ProcessExitCodeMaximum },
		func(o *process.ResultObservation) {
			o.ExitCode = core.ProcessExitCodeSignaled
			o.TerminationSignal = &signal
		},
		func(o *process.ResultObservation) {
			o.ExitCode = core.ProcessExitCodeSignaled
			o.TerminationSignal = nil
		},
		func(o *process.ResultObservation) {
			o.CPUTime = duration
			o.StdinBytes = maximum
			o.StdoutBytes = maximum
			o.StderrBytes = maximum
		},
	} {
		observation := seed
		change(&observation)
		if err := observation.Validate(); err != nil {
			f.Fatal(err)
		}
		encoded, err := core.MarshalCanonicalJSONDocument(observation)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(encoded)
	}
	for _, exit := range []int64{core.ProcessExitCodeSignaled - 1, core.ProcessExitCodeMaximum + 1, math.MinInt64, math.MaxInt64} {
		hostile := seed
		hostile.ExitCode = exit
		encoded, err := json.Marshal(hostile)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(encoded)
	}
	for _, data := range [][]byte{nil, []byte("null"), []byte("[]"), []byte(`{"unowned":1}`)} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		wire, syntaxErr := core.DecodeStrictJSONStructure[process.ResultObservation](input, core.DefaultStrictJSONLimits())
		trimmed := bytes.TrimSpace(input)
		wantAccepted := syntaxErr == nil && len(trimmed) != 0 && trimmed[0] == '{' && wire.ExitCode >= core.ProcessExitCodeSignaled && wire.ExitCode <= core.ProcessExitCodeMaximum
		wantAccepted = wantAccepted && (wire.TerminationSignal == nil || wire.ExitCode == core.ProcessExitCodeSignaled && *wire.TerminationSignal > 0)
		before := bytes.Clone(input)
		got, err := core.DecodeStrictJSON[process.ResultObservation](bytes.NewReader(input), core.DefaultStrictJSONLimits())
		if !bytes.Equal(input, before) {
			t.Fatalf("decoder changed caller bytes: got=%x want=%x", input, before)
		}
		if !wantAccepted {
			if !errors.Is(err, core.ErrJSONContract) || got != (process.ResultObservation{}) {
				t.Fatalf("result refusal lost identity or exposed facts: %+v, %v", got, err)
			}
			if syntaxErr == nil && len(trimmed) != 0 && trimmed[0] == '{' && !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("process contradiction lost its owner: %v", err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.ExitCode != wire.ExitCode || got.CPUTime != wire.CPUTime || got.StdinBytes != wire.StdinBytes || got.StdoutBytes != wire.StdoutBytes || got.StderrBytes != wire.StderrBytes || (got.PeakMemoryBytes == nil) != (wire.PeakMemoryBytes == nil) || got.PeakMemoryBytes != nil && wire.PeakMemoryBytes != nil && *got.PeakMemoryBytes != *wire.PeakMemoryBytes || (got.TerminationSignal == nil) != (wire.TerminationSignal == nil) || got.TerminationSignal != nil && wire.TerminationSignal != nil && *got.TerminationSignal != *wire.TerminationSignal {
			t.Fatalf("result admission changed exact observations: %v", err)
		}
		encoded, err := core.MarshalCanonicalJSONDocument(got)
		if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
			t.Fatalf("accepted result cannot publish: %v", err)
		}
		next, err := core.DecodeStrictJSON[process.ResultObservation](bytes.NewReader(encoded), core.DefaultStrictJSONLimits())
		if err != nil || next.ExitCode != got.ExitCode || next.CPUTime != got.CPUTime || next.StdinBytes != got.StdinBytes || next.StdoutBytes != got.StdoutBytes || next.StderrBytes != got.StderrBytes || (next.PeakMemoryBytes == nil) != (got.PeakMemoryBytes == nil) || next.PeakMemoryBytes != nil && got.PeakMemoryBytes != nil && *next.PeakMemoryBytes != *got.PeakMemoryBytes || (next.TerminationSignal == nil) != (got.TerminationSignal == nil) || next.TerminationSignal != nil && got.TerminationSignal != nil && *next.TerminationSignal != *got.TerminationSignal {
			t.Fatalf("result round trip lost facts: %v", err)
		}
		again, err := core.MarshalCanonicalJSONDocument(next)
		if err != nil || !bytes.Equal(encoded, again) {
			t.Fatalf("result canonical bytes are unstable: %v", err)
		}
	})
}
