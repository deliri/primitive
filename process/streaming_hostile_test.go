package process

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// A real child is stopped after the first output failure. This owned writer
// seam deterministically checks failures already delivered to both writers.
func TestCommandStreamsRetainIndependentLimitFailuresLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name           string
		stdout, stderr int
	}{
		{name: "neutral/empty streams fabricate no failure"},
		{name: "positive/both streams admit their complete bounds", stdout: 8, stderr: 8},
		{name: "negative/stdout overflow preserves stderr success", stdout: 9, stderr: 8},
		{name: "negative/stderr overflow preserves stdout success", stdout: 8, stderr: 9},
		{name: "negative/two overflows retain two independent causes", stdout: 9, stderr: 9},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			limit, err := core.NewByteCount(8)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			failures := &streamFailures{cancel: cancel}
			var stdout, stderr bytes.Buffer
			streams := newCommandStreams(Request{Streams: Streams{Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: &stderr}, OutputPolicy: OutputPolicy{Mode: OutputModeBounded, Maximum: limit}}, failures)
			outputs := []struct {
				stream      Stream
				writer      *observedWriter
				destination *bytes.Buffer
				size        int
			}{
				{StreamStdout, streams.stdout, &stdout, tc.stdout},
				{StreamStderr, streams.stderr, &stderr, tc.stderr},
			}
			for _, output := range outputs {
				payload := bytes.Repeat([]byte{byte(output.stream)}, output.size)
				count, err := output.writer.Write(payload)
				want := min(output.size, 8)
				if count != want || output.writer.count != uint64(want) || !bytes.Equal(output.destination.Bytes(), payload[:want]) {
					t.Fatalf("%v write=%d counter=%d payload=%q; want exact %d-byte prefix", output.stream, count, output.writer.count, output.destination.Bytes(), want)
				}
				if output.size <= 8 {
					if err != nil || failures.values[output.stream] != nil {
						t.Fatalf("in-bound %v fabricated failure: %v/%v", output.stream, err, failures.values[output.stream])
					}
				} else {
					var exceeded OutputLimitExceeded
					if !errors.Is(err, core.ErrProcessOutputLimit) || !errors.As(err, &exceeded) || exceeded.Stream() != output.stream || exceeded.Limit() != limit || !errors.Is(failures.values[output.stream], err) || !errors.Is(failures.joined(), err) {
						t.Fatalf("%v overflow lost typed detail or joined cause: %v / %v", output.stream, err, failures.joined())
					}
				}
			}
			overflow := tc.stdout > 8 || tc.stderr > 8
			if overflow {
				if !errors.Is(context.Cause(ctx), core.ErrProcessOutputLimit) {
					t.Fatalf("overflow cancellation=%v", context.Cause(ctx))
				}
			} else if context.Cause(ctx) != nil || failures.joined() != nil {
				t.Fatalf("successful writes fabricated cancellation=%v or failure=%v", context.Cause(ctx), failures.joined())
			}
		})
	}
}

type emptyWriteRejectingDestination struct{ calls int }

func (w *emptyWriteRejectingDestination) Write(payload []byte) (int, error) {
	w.calls++
	if len(payload) == 0 {
		return 0, io.ErrClosedPipe
	}
	return len(payload), nil
}

func TestBoundedWriterEmptyPrefixLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                  string
		first, second         []byte
		wantSecond, wantCalls int
		wantErr               error
	}{
		{name: "neutral/empty first and second writes never touch caller"},
		{name: "positive/remaining byte reaches caller", first: []byte("ab"), second: []byte("c"), wantSecond: 1, wantCalls: 2},
		{name: "neutral/empty write at exact capacity remains empty", first: []byte("abc"), wantCalls: 1},
		{name: "negative/overflow at capacity cannot call empty destination", first: []byte("abc"), second: []byte("d"), wantCalls: 1, wantErr: core.ErrProcessOutputLimit},
		{name: "negative/partial prefix forwards only remaining capacity", first: []byte("ab"), second: []byte("cd"), wantSecond: 1, wantCalls: 2, wantErr: core.ErrProcessOutputLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			limit, err := core.NewByteCount(3)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			destination := &emptyWriteRejectingDestination{}
			failures := &streamFailures{cancel: cancel}
			streams := newCommandStreams(Request{Streams: Streams{Stdin: bytes.NewReader(nil), Stdout: destination, Stderr: io.Discard}, OutputPolicy: OutputPolicy{Mode: OutputModeBounded, Maximum: limit}}, failures)
			count, err := streams.stdout.Write(tc.first)
			if count != len(tc.first) || err != nil {
				t.Fatalf("initial write=%d, %v; want %d, nil", count, err, len(tc.first))
			}
			count, err = streams.stdout.Write(tc.second)
			if count != tc.wantSecond || !errors.Is(err, tc.wantErr) || destination.calls != tc.wantCalls || streams.stdout.count != uint64(len(tc.first)+tc.wantSecond) || errors.Is(err, io.ErrClosedPipe) || errors.Is(context.Cause(ctx), io.ErrClosedPipe) {
				t.Fatalf("later write=%d, %v; calls=%d counter=%d; want %d, %v calls=%d exact counter", count, err, destination.calls, streams.stdout.count, tc.wantSecond, tc.wantErr, tc.wantCalls)
			}
		})
	}
}

// The schedule controls consecutive empty reads; a single actual byte must
// reset that budget, and EOF must never count as no progress.
type progressScheduleReader struct {
	empty    int
	data     bool
	terminal error
}

func (r *progressScheduleReader) Read(payload []byte) (int, error) {
	if r.empty > 0 {
		r.empty--
		return 0, nil
	}
	if r.data {
		r.data = false
		payload[0] = 'x'
		return 1, nil
	}
	return 0, r.terminal
}
func TestObservedReaderNoProgressLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		empty     int
		data      bool
		terminal  error
		calls     int
		wantCount uint64
		wantErr   error
	}{
		{name: "neutral/immediate EOF is not a failure", terminal: io.EOF, calls: 1, wantErr: io.EOF},
		{name: "boundary/EOF below empty-read limit remains EOF", empty: core.ReaderConsecutiveEmptyReadMaximum - 1, terminal: io.EOF, calls: core.ReaderConsecutiveEmptyReadMaximum, wantErr: io.EOF},
		{name: "positive/byte at last admissible read resets budget", empty: core.ReaderConsecutiveEmptyReadMaximum - 1, data: true, calls: 2*core.ReaderConsecutiveEmptyReadMaximum - 1, wantCount: 1},
		{name: "negative/exact empty-read limit cancels", empty: core.ReaderConsecutiveEmptyReadMaximum, calls: core.ReaderConsecutiveEmptyReadMaximum, wantErr: io.ErrNoProgress},
		{name: "negative/read failure remains original cause", terminal: io.ErrClosedPipe, calls: 1, wantErr: io.ErrClosedPipe},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			source := &progressScheduleReader{empty: tc.empty, data: tc.data, terminal: tc.terminal}
			failures := &streamFailures{cancel: cancel}
			reader := &observedReader{source: source, failures: failures}
			var err error
			var payload [1]byte
			observed := uint64(0)
			for i := range tc.calls {
				var count int
				count, err = reader.Read(payload[:])
				if count < 0 || count > 1 {
					t.Fatalf("impossible observed count=%d", count)
				}
				observed += uint64(count)
				if count == 1 && payload[0] != 'x' {
					t.Fatalf("read changed source byte=%x", payload[0])
				}
				if i < tc.calls-1 && err != nil {
					t.Fatalf("premature read %d error=%v", i, err)
				}
			}
			if !errors.Is(err, tc.wantErr) || reader.count != tc.wantCount || observed != tc.wantCount {
				t.Fatalf("read=%v observed=%d counter=%d; want %v and %d", err, observed, reader.count, tc.wantErr, tc.wantCount)
			}
			if tc.wantErr == nil || errors.Is(tc.wantErr, io.EOF) {
				if context.Cause(ctx) != nil || failures.joined() != nil {
					t.Fatalf("admitted progress fabricated failure=%v/%v", context.Cause(ctx), failures.joined())
				}
			} else if !errors.Is(context.Cause(ctx), tc.wantErr) || !errors.Is(failures.joined(), tc.wantErr) {
				t.Fatalf("read failure lost cancellation/join identity=%v/%v", context.Cause(ctx), failures.joined())
			}
		})
	}
}
