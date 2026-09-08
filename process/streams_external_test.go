package process_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// The finite writer outcomes include every legal count/error combination and
// both impossible count directions. Refused routes must never reach a writer.
func TestStreamsWriteOutputLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		stream     process.Stream
		payload    []byte
		nilStream  process.Stream
		count      int
		cause      error
		wantCount  uint64
		wantErr    error
		wantNative error
		wantCalls  int
	}{
		{name: "positive/stdout preserves binary payload", stream: process.StreamStdout, payload: []byte{0, 255, 'x'}, count: 3, wantCount: 3, wantCalls: 1},
		{name: "positive/stderr preserves binary payload", stream: process.StreamStderr, payload: []byte{0, 255, 'x'}, count: 3, wantCount: 3, wantCalls: 1},
		{name: "neutral/nil stdout payload never calls destination", stream: process.StreamStdout},
		{name: "neutral/empty stderr payload never calls destination", stream: process.StreamStderr, payload: []byte{}},
		{name: "negative/stdin cannot be written", stream: process.StreamStdin, payload: []byte("x"), wantErr: core.ErrProcessContract},
		{name: "negative/unknown stream cannot be routed", stream: process.StreamUnknown, payload: []byte("x"), wantErr: core.ErrProcessContract},
		{name: "negative/off-domain stream cannot be routed", stream: process.Stream(255), payload: []byte("x"), wantErr: core.ErrProcessContract},
		{name: "negative/empty input still validates routing", stream: process.StreamStdin, wantErr: core.ErrProcessContract},
		{name: "negative/nil stdin refuses before output", stream: process.StreamStdout, nilStream: process.StreamStdin, payload: []byte("x"), wantErr: core.ErrProcessContract},
		{name: "negative/nil selected stdout refuses", stream: process.StreamStdout, nilStream: process.StreamStdout, payload: []byte("x"), wantErr: core.ErrProcessContract},
		{name: "negative/nil unselected stderr refuses", stream: process.StreamStdout, nilStream: process.StreamStderr, payload: []byte("x"), wantErr: core.ErrProcessContract},
		{name: "negative/zero progress is a short write", stream: process.StreamStdout, payload: []byte("abc"), wantErr: core.ErrProcessStream, wantNative: io.ErrShortWrite, wantCalls: 1},
		{name: "negative/partial nil error cannot claim completion", stream: process.StreamStdout, payload: []byte("abc"), count: 2, wantCount: 2, wantErr: core.ErrProcessStream, wantNative: io.ErrShortWrite, wantCalls: 1},
		{name: "negative/partial native failure preserves count", stream: process.StreamStderr, payload: []byte("abc"), count: 2, cause: io.ErrClosedPipe, wantCount: 2, wantErr: core.ErrProcessStream, wantNative: io.ErrClosedPipe, wantCalls: 1},
		{name: "negative/full count cannot erase native failure", stream: process.StreamStdout, payload: []byte("abc"), count: 3, cause: io.ErrClosedPipe, wantCount: 3, wantErr: core.ErrProcessStream, wantNative: io.ErrClosedPipe, wantCalls: 1},
		{name: "negative/negative count cannot wrap byte counter", stream: process.StreamStdout, payload: []byte("abc"), count: -1, wantErr: core.ErrProcessStream, wantNative: io.ErrShortWrite, wantCalls: 1},
		{name: "negative/excess count cannot invent retained bytes", stream: process.StreamStderr, payload: []byte("abc"), count: 4, wantErr: core.ErrProcessStream, wantNative: io.ErrShortWrite, wantCalls: 1},
		{name: "negative/impossible count preserves independent cause", stream: process.StreamStdout, payload: []byte("abc"), count: -1, cause: io.ErrClosedPipe, wantErr: core.ErrProcessStream, wantNative: io.ErrShortWrite, wantCalls: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stdout := &outputOutcomeWriter{count: tc.count, cause: tc.cause}
			stderr := &outputOutcomeWriter{count: tc.count, cause: tc.cause}
			streams := process.Streams{Stdin: bytes.NewReader(nil), Stdout: stdout, Stderr: stderr}
			switch tc.nilStream {
			case process.StreamStdin:
				streams.Stdin = nil
			case process.StreamStdout:
				streams.Stdout = nil
			case process.StreamStderr:
				streams.Stderr = nil
			}
			length, err := streams.WriteOutput(tc.stream, tc.payload)
			if length.Uint64() != tc.wantCount || !errors.Is(err, tc.wantErr) || tc.wantNative != nil && !errors.Is(err, tc.wantNative) || tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("write=%d, %v; want %d, %v with native %v and cause %v", length.Uint64(), err, tc.wantCount, tc.wantErr, tc.wantNative, tc.cause)
			}
			selected, other := stdout, stderr
			if tc.stream == process.StreamStderr {
				selected, other = stderr, stdout
			}
			if selected.calls != tc.wantCalls || other.calls != 0 || other.retained.Len() != 0 || !bytes.Equal(selected.retained.Bytes(), tc.payload[:tc.wantCount]) {
				t.Fatalf("effects selected=%d/%q other=%d/%q; want calls=%d exact prefix=%q and no other output", selected.calls, selected.retained.Bytes(), other.calls, other.retained.Bytes(), tc.wantCalls, tc.payload[:tc.wantCount])
			}
		})
	}
}

type outputOutcomeWriter struct {
	retained bytes.Buffer
	count    int
	cause    error
	calls    int
}

func (w *outputOutcomeWriter) Write(payload []byte) (int, error) {
	w.calls++
	if w.count >= 0 && w.count <= len(payload) {
		_, _ = w.retained.Write(payload[:w.count])
	}
	return w.count, w.cause
}

func FuzzStreamsWriteOutputSemanticClosure(f *testing.F) {
	var canonical bytes.Buffer
	seedStreams := process.Streams{Stdin: bytes.NewReader(nil), Stdout: &canonical, Stderr: io.Discard}
	seed := []byte{0, 255, 'p', '\n'}
	count, err := seedStreams.WriteOutput(process.StreamStdout, seed)
	if err != nil || count.Uint64() != uint64(len(seed)) || !bytes.Equal(canonical.Bytes(), seed) {
		f.Fatalf("production stream seed=%q, %v, %v", canonical.Bytes(), count, err)
	}
	f.Add(uint8(process.StreamStdout), canonical.Bytes())
	f.Add(uint8(process.StreamStderr), []byte{})
	f.Add(uint8(process.StreamStdin), []byte("refused"))
	f.Add(uint8(255), []byte{0x00, 0xff})

	f.Fuzz(func(t *testing.T, raw uint8, payload []byte) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		streams := process.Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}
		stream := process.Stream(raw)
		length, gotErr := streams.WriteOutput(stream, payload)
		if stream != process.StreamStdout && stream != process.StreamStderr {
			if !errors.Is(gotErr, core.ErrProcessContract) || length.Uint64() != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("WriteOutput(%d, rejected) = (length %d, stdout %d, stderr %d, %v), want zero effects and %v", raw, length.Uint64(), stdout.Len(), stderr.Len(), gotErr, core.ErrProcessContract)
			}
			return
		}
		if gotErr != nil || length.Uint64() != uint64(len(payload)) {
			t.Fatalf("WriteOutput(%s) = (length %d, %v), want %d and nil", stream, length.Uint64(), gotErr, len(payload))
		}
		if stream == process.StreamStdout && (!bytes.Equal(stdout.Bytes(), payload) || stderr.Len() != 0) || stream == process.StreamStderr && (!bytes.Equal(stderr.Bytes(), payload) || stdout.Len() != 0) {
			t.Fatalf("WriteOutput(%s) = stdout %q/stderr %q, want exact payload %q at one destination", stream, stdout.Bytes(), stderr.Bytes(), payload)
		}
	})
}
