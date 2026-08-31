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

func TestStreamsWriteOutputLayerTriad(t *testing.T) {
	t.Parallel()

	payload := []byte("proof\n")
	for _, stream := range []process.Stream{process.StreamStdout, process.StreamStderr} {
		t.Run("positive exact write to "+stream.String(), func(t *testing.T) {
			t.Parallel()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			streams := process.Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}
			length, err := streams.WriteOutput(stream, payload)
			if err != nil || length.Uint64() != uint64(len(payload)) {
				t.Fatalf("WriteOutput() = (%v, %v), want exact payload length", length, err)
			}
			if stream == process.StreamStdout && !bytes.Equal(stdout.Bytes(), payload) || stream == process.StreamStderr && !bytes.Equal(stderr.Bytes(), payload) {
				t.Fatalf("WriteOutput(%s) = stdout %q/stderr %q, want payload %q only at the selected destination", stream, stdout.Bytes(), stderr.Bytes(), payload)
			}
		})
	}

	short := &shortWriter{}
	streams := process.Streams{Stdin: strings.NewReader(""), Stdout: short, Stderr: io.Discard}
	if length, err := streams.WriteOutput(process.StreamStdin, []byte("x")); !errors.Is(err, core.ErrProcessContract) || length.Uint64() != 0 || short.retained.Len() != 0 {
		t.Fatalf("WriteOutput(stdin) = (length %d, retained %d, %v), want zero, zero, and %v", length.Uint64(), short.retained.Len(), err, core.ErrProcessContract)
	}
	if length, err := streams.WriteOutput(process.StreamStdout, []byte("proof")); !errors.Is(err, core.ErrProcessStream) || !errors.Is(err, io.ErrShortWrite) || length.Uint64() != 4 || short.retained.String() != "proo" {
		t.Fatalf("WriteOutput(short) = (length %d, retained %q, %v), want 4, %q, stream and short-write identities", length.Uint64(), short.retained.String(), err, "proo")
	}

	untouched := &outputCallWriter{}
	neutral := process.Streams{Stdin: strings.NewReader(""), Stdout: untouched, Stderr: io.Discard}
	length, err := neutral.WriteOutput(process.StreamStdout, nil)
	if err != nil || length.Uint64() != 0 || untouched.calls != 0 {
		t.Fatalf("WriteOutput(empty) = (length %d, calls %d, %v), want zero, zero, nil", length.Uint64(), untouched.calls, err)
	}
}

type shortWriter struct{ retained bytes.Buffer }

func (w *shortWriter) Write(payload []byte) (int, error) {
	count := len(payload) - 1
	_, _ = w.retained.Write(payload[:count])
	return count, nil
}

type outputCallWriter struct{ calls uint64 }

func (w *outputCallWriter) Write(payload []byte) (int, error) {
	w.calls++
	return len(payload), nil
}

func FuzzStreamsWriteOutputSemanticClosure(f *testing.F) {
	f.Add(uint8(process.StreamStdout), []byte("proof\n"))
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
