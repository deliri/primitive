package process_test

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"testing"

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

	request := processRequest(
		b,
		"output:"+strconv.FormatUint(output, 10),
		process.Streams{
			Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
		},
	)
	request.OutputLimit = byteCount(b, output)

	for b.Loop() {
		got, gotErr := process.Run(context.Background(), request)
		if gotErr != nil {
			b.Fatalf("process.Run(%d streamed bytes) error = %v, want nil", output, gotErr)
		}
		if gotBytes := resultByteLength(b, got.StdoutBytes); gotBytes != output {
			b.Fatalf(
				"process.Run(%d streamed bytes) stdout = %d, want %d",
				output,
				gotBytes,
				output,
			)
		}
	}
}

// benchmarkRunStreamingStdin measures the input path, which the output
// benchmarks never exercise. A reader is constructed per iteration because a
// consumed reader cannot be replayed; that allocation is the fixed setup cost
// and does not grow with the streamed extent.
func benchmarkRunStreamingStdin(b *testing.B, input uint64) {
	b.Helper()

	request := processRequest(b, "stdin-count", process.Streams{
		Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard,
	})

	for b.Loop() {
		request.Streams.Stdin = io.LimitReader(filledReader{}, int64(input))
		got, gotErr := process.Run(context.Background(), request)
		if gotErr != nil {
			b.Fatalf("process.Run(%d stdin bytes) error = %v, want nil", input, gotErr)
		}
		if gotBytes := resultByteLength(b, got.StdinBytes); gotBytes != input {
			b.Fatalf(
				"process.Run(%d stdin bytes) stdin = %d, want %d",
				input,
				gotBytes,
				input,
			)
		}
	}
}
