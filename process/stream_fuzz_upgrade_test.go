package process_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzTruncatingWriterExternalIngress(f *testing.F) {
	limit, err := core.NewByteCount(3)
	if err != nil {
		f.Fatal(err)
	}
	var seed bytes.Buffer
	writer, err := process.NewTruncatingWriter(&seed, limit)
	if err != nil || writer.Validate() != nil {
		f.Fatalf("seed writer: %v", err)
	}
	if _, err := writer.Write([]byte{'x', 0, 0xff}); err != nil {
		f.Fatal(err)
	}
	f.Add(seed.Bytes(), uint64(3), uint16(1), false)
	f.Add([]byte{}, uint64(1), uint16(0), false)
	f.Add([]byte{0, 0xff}, uint64(1), uint16(1), false)
	f.Add([]byte{0xff}, uint64(0), uint16(0), false)
	f.Add([]byte{0xff}, uint64(math.MaxInt64)+1, uint16(0), false)
	f.Add([]byte{0xff}, uint64(1), uint16(0), true)
	f.Fuzz(func(t *testing.T, payload []byte, rawLimit uint64, split uint16, nilDestination bool) {
		limit, limitErr := core.NewByteCount(rawLimit)
		if (limitErr != nil) != (rawLimit == 0) {
			t.Fatalf("byte-count fixture error=%v for %d", limitErr, rawLimit)
		}
		var output bytes.Buffer
		var destination io.Writer = &output
		if nilDestination {
			destination = nil
		}
		writer, err := process.NewTruncatingWriter(destination, limit)
		if nilDestination || rawLimit == 0 || rawLimit > math.MaxInt64 {
			if !errors.Is(err, core.ErrProcessContract) || writer != nil || output.Len() != 0 {
				t.Fatalf("writer construction refusal exposed capability or output: %v", err)
			}
			return
		}
		if err != nil || writer.Validate() != nil {
			t.Fatalf("admitted writer refused: %v", err)
		}
		before := bytes.Clone(payload)
		cut := min(int(split), len(payload))
		for _, part := range [][]byte{payload[:cut], nil, payload[cut:]} {
			count, err := writer.Write(part)
			if err != nil || count != len(part) {
				t.Fatalf("truncation failed to consume exact source: %d of %d, %v", count, len(part), err)
			}
		}
		want := min(uint64(len(payload)), rawLimit)
		retained, retainedErr := writer.RetainedBytes()
		dropped, droppedErr := writer.DroppedBytes()
		if errors.Join(retainedErr, droppedErr) != nil || retained.Uint64() != want || dropped.Uint64() != uint64(len(payload))-want || !bytes.Equal(output.Bytes(), payload[:want]) || !bytes.Equal(payload, before) {
			t.Fatalf("truncation broke prefix or byte conservation: retained=%d dropped=%d errors=%v", retained.Uint64(), dropped.Uint64(), errors.Join(retainedErr, droppedErr))
		}
	})
}

// Only the owned test executable is started. Payload bytes never name a
// command, path, argument, environment, or external service.
func FuzzRunAndBeginStreamingExternalIngress(f *testing.F) {
	var seed bytes.Buffer
	streams := process.Streams{Stdin: bytes.NewReader(nil), Stdout: &seed, Stderr: io.Discard}
	if err := streams.Validate(); err != nil {
		f.Fatal(err)
	}
	if _, err := streams.WriteOutput(process.StreamStdout, []byte{'x', 0, 0xff}); err != nil {
		f.Fatal(err)
	}
	f.Add(seed.Bytes(), uint32(3))
	f.Add([]byte{}, uint32(1))
	f.Add([]byte{0xff}, uint32(0))
	f.Add([]byte{0, 0xff}, uint32(1))
	f.Fuzz(func(t *testing.T, payload []byte, rawLimit uint32) {
		limit, limitErr := core.NewByteCount(uint64(rawLimit))
		if (limitErr != nil) != (rawLimit == 0) {
			t.Fatalf("byte-count fixture error=%v for %d", limitErr, rawLimit)
		}
		// Keep oracle output bounded even if mutation chooses a huge budget.
		// The caller's actual request receives this same visible budget.
		if rawLimit > core.JSONDocumentMaximumBytes {
			limit = byteCount(t, core.JSONDocumentMaximumBytes)
		}
		for _, supervised := range []bool{false, true} {
			var stdout, stderr bytes.Buffer
			request := processRequest(t, "copy", process.Streams{Stdin: bytes.NewReader(payload), Stdout: &stdout, Stderr: &stderr})
			request.OutputPolicy.Maximum = limit
			backstop, err := temporal.DurationFromNanoseconds(int64(processTestBackstop))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: backstop})
			if err != nil {
				t.Fatal(err)
			}
			var result process.Result
			var runErr error
			if supervised {
				execution, beginErr := process.Begin(ctx, request)
				runErr = beginErr
				if beginErr == nil {
					result, runErr = execution.Wait()
				} else if execution != nil {
					cancel()
					t.Fatalf("Begin refusal exposed execution=%v error=%v", execution, beginErr)
				}
			} else {
				result, runErr = process.Run(ctx, request)
			}
			cancel()
			if rawLimit == 0 {
				if !errors.Is(runErr, core.ErrProcessContract) || result != (process.Result{}) || stdout.Len() != 0 || stderr.Len() != 0 {
					t.Fatalf("invalid execution budget exposed facts or output: %v", runErr)
				}
				continue
			}
			facts, err := result.Observation()
			if err != nil {
				t.Fatalf("reaped child lost observations: %v / %v", runErr, err)
			}
			maximum, err := limit.Uint64()
			if err != nil {
				t.Fatal(err)
			}
			want := min(uint64(len(payload)), maximum)
			if !bytes.Equal(stdout.Bytes(), payload[:want]) || stderr.Len() != 0 || facts.StdoutBytes.Uint64() != want || facts.StderrBytes.Uint64() != 0 || facts.StdinBytes.Uint64() < want || facts.StdinBytes.Uint64() > uint64(len(payload)) {
				t.Fatalf("native stream handoff changed bytes or accounting: %+v, %v", facts, runErr)
			}
			if uint64(len(payload)) > maximum {
				var exceeded process.OutputLimitExceeded
				if !errors.Is(runErr, core.ErrProcessOutputLimit) || !errors.As(runErr, &exceeded) || exceeded.Stream() != process.StreamStdout || exceeded.Limit() != limit {
					t.Fatalf("overrun lost typed stream and budget: %v", runErr)
				}
			} else if runErr != nil || facts.ExitCode != core.ProcessExitCodeSuccess || facts.StdinBytes.Uint64() != uint64(len(payload)) {
				t.Fatalf("finite copy failed or lost input: %+v, %v", facts, runErr)
			}
		}
	})
}
