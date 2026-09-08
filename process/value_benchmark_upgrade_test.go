package process_test

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/process"
)

// BenchmarkResultObservation measures checked projection of one already reaped
// child. Starting the fixture child is outside the timed loop.
func BenchmarkResultObservation(b *testing.B) {
	request := processRequest(b, "silent", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
	})
	result, err := process.Run(b.Context(), request)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := result.Observation()
		if err != nil || got.Validate() != nil || got.ExitCode != 0 ||
			got.StdinBytes.Uint64() != 0 || got.StdoutBytes.Uint64() != 0 || got.StderrBytes.Uint64() != 0 {
			b.Fatalf("silent child lost its exact observation: %+v, %v", got, err)
		}
	}
}

// BenchmarkPlanRoundTrip measures canonical encoding, strict decoding, and
// re-encoding. Fixed admitted paths are data only; no file is opened.
func BenchmarkPlanRoundTrip(b *testing.B) {
	request := processRequest(b, "silent", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
	})
	root := filepath.VolumeName(request.Command.String()) + string(filepath.Separator)
	request.Command = absolutePath(b, filepath.Join(root, "primitive-benchmark", "command"))
	request.WorkingDirectory = absolutePath(b, filepath.Join(root, "primitive-benchmark"))
	plan := planFromRequest(request)
	b.ReportAllocs()
	for b.Loop() {
		encoded, encodeErr := plan.MarshalJSON()
		if encodeErr != nil {
			b.Fatal(encodeErr)
		}
		var decoded process.Plan
		if err := decoded.UnmarshalJSON(encoded); err != nil {
			b.Fatal(err)
		}
		reencoded, err := decoded.MarshalJSON()
		if err != nil || !bytes.Equal(encoded, reencoded) {
			b.Fatalf("plan canonical closure failed: %v", err)
		}
	}
}
