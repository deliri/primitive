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

// BenchmarkRequestValidateExactEnvironment64 measures the admission every Run
// and Begin performs: command, arguments, and a 64-variable exact environment
// including os/exec's duplicate-name check. No child is started.
func BenchmarkRequestValidateExactEnvironment64(b *testing.B) {
	request := processRequest(b, "silent", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
	})
	values := make([]string, 64)
	for index := range values {
		values[index] = "V" + strconv.Itoa(index) + "=payload"
	}
	environment, err := process.ParseExactEnvironment(values)
	if err != nil || len(environment.Variables) != len(values) {
		b.Fatalf("environment fixture = (%d variables, %v), want (%d, nil)", len(environment.Variables), err, len(values))
	}
	request.Environment = environment
	b.ReportAllocs()
	for b.Loop() {
		if err := request.Validate(); err != nil {
			b.Fatal(err)
		}
	}
	duplicate := request
	duplicate.Environment.Variables = append(append([]process.EnvironmentVariable(nil), environment.Variables...), environment.Variables[0])
	if err := duplicate.Validate(); err == nil {
		b.Fatal("duplicate environment admitted, want the measured check to be live")
	}
}

// BenchmarkTruncatingWriterBoundedCapture measures one bounded capture
// lifecycle: construction, one fully forwarded 32 KiB write, and one 32 KiB
// write split at the 48 KiB limit into forwarded and dropped bytes.
func BenchmarkTruncatingWriterBoundedCapture(b *testing.B) {
	const chunkBytes = 32 << 10
	limit := byteCount(b, 48<<10)
	chunk := bytes.Repeat([]byte{'x'}, chunkBytes)
	var destination benchmarkFilledWriter
	var writer *process.TruncatingWriter
	b.SetBytes(2 * chunkBytes)
	b.ReportAllocs()
	for b.Loop() {
		var err error
		writer, err = process.NewTruncatingWriter(&destination, limit)
		if err != nil {
			b.Fatal(err)
		}
		for range 2 {
			if count, err := writer.Write(chunk); err != nil || count != chunkBytes {
				b.Fatalf("Write() = (%d,%v), want (%d,nil)", count, err, chunkBytes)
			}
		}
	}
	retained, retainedErr := writer.RetainedBytes()
	dropped, droppedErr := writer.DroppedBytes()
	if retainedErr != nil || droppedErr != nil || retained.Uint64() != 48<<10 || dropped.Uint64() != 16<<10 || destination.count != uint64(b.N)*(48<<10) {
		b.Fatalf("capture = (retained %d,%v dropped %d,%v forwarded %d), want 48 KiB/16 KiB per operation", retained.Uint64(), retainedErr, dropped.Uint64(), droppedErr, destination.count)
	}
}
