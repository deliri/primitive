package process_test

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func BenchmarkRunStreamingStdout64KiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkRunStreamingStdout(b, 64<<10)
}

func BenchmarkRunStreamingStdout1MiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkRunStreamingStdout(b, 1<<20)
}

func BenchmarkRunStreamingStdin64KiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkRunStreamingStdin(b, 64<<10)
}

func BenchmarkRunStreamingStdin1MiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkRunStreamingStdin(b, 1<<20)
}

// benchmarkRunStreamingStdout measures the output path. Allocation must stay
// flat as the streamed extent grows, because output is forwarded rather than
// retained.
func benchmarkRunStreamingStdout(b *testing.B, output uint64) {
	b.Helper()
	var destination benchmarkFilledWriter

	request := processRequest(
		b,
		"output:"+strconv.FormatUint(output, 10),
		process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: &destination, Stderr: io.Discard,
		},
	)
	request.OutputPolicy = process.OutputPolicy{Mode: process.OutputModeStreaming}
	b.SetBytes(int64(output))

	for b.Loop() {
		destination.count = 0
		got, gotErr := process.Run(context.Background(), request)
		if gotErr != nil {
			b.Fatalf("process.Run(%d streamed bytes) error = %v, want nil", output, gotErr)
		}
		facts, err := got.Observation()
		if err != nil || int64(facts.ExitCode) != core.ProcessExitCodeSuccess || facts.StdoutBytes.Uint64() != output || facts.StdinBytes.Uint64() != 0 || facts.StderrBytes.Uint64() != 0 || destination.count != output {
			b.Fatalf("streamed output lost child exit, content, or exact counts: %+v, writer=%d, error=%v", facts, destination.count, err)
		}
	}
}

// benchmarkRunStreamingStdin measures the input path, which the output
// benchmarks never exercise. A reader is constructed per iteration because a
// consumed reader cannot be replayed; that allocation is the fixed setup cost
// and does not grow with the streamed extent.
func benchmarkRunStreamingStdin(b *testing.B, input uint64) {
	b.Helper()
	var output bytes.Buffer
	want := strconv.FormatUint(input, 10)

	request := processRequest(b, "stdin-count", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: &output, Stderr: io.Discard,
	})
	request.OutputPolicy.Maximum = byteCount(b, uint64(len(want)))
	b.SetBytes(int64(input))

	for b.Loop() {
		output.Reset()
		request.Streams.Stdin = io.LimitReader(filledReader{}, int64(input))
		got, gotErr := process.Run(context.Background(), request)
		if gotErr != nil {
			b.Fatalf("process.Run(%d stdin bytes) error = %v, want nil", input, gotErr)
		}
		facts, err := got.Observation()
		if err != nil || int64(facts.ExitCode) != core.ProcessExitCodeSuccess || facts.StdinBytes.Uint64() != input || facts.StdoutBytes.Uint64() != uint64(len(want)) || facts.StderrBytes.Uint64() != 0 || output.String() != want {
			b.Fatalf("streamed stdin lost child receipt or exact counts: %+v, receipt=%q, error=%v", facts, output.String(), err)
		}
	}
}

// Validate content without retaining streamed output. The byte scan is part
// of both sides of the measured workload, not an unmeasured success assumption.
type benchmarkFilledWriter struct{ count uint64 }

func (w *benchmarkFilledWriter) Write(payload []byte) (int, error) {
	if bytes.Count(payload, []byte{'x'}) != len(payload) {
		return 0, core.ErrProcessContract
	}
	w.count += uint64(len(payload))
	return len(payload), nil
}
