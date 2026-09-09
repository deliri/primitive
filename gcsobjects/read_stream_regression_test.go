package gcsobjects

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
)

// A direct local ratchet for SDK read-error versus destination-error ownership.
// The public SDK/HTTP integration matrix independently exercises both callers.
func TestGCSExactCopyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name               string
		source             func() io.Reader
		length             uint64
		writerErr          error
		fullWrite          bool
		wantBytes          string
		wantErr, wantCause error
	}{
		{name: "neutral empty stream proves EOF", source: func() io.Reader { return bytes.NewReader(nil) }},
		{name: "positive exact byte waits for EOF", source: func() io.Reader { return bytes.NewBufferString("a") }, length: 1, wantBytes: "a"},
		{name: "positive bytes and bare EOF remain accepted", source: func() io.Reader { return iotest.DataErrReader(bytes.NewBufferString("ab")) }, length: 2, wantBytes: "ab"},
		{name: "negative short stream preserves prefix and source refusal", source: func() io.Reader { return bytes.NewBufferString("a") }, length: 2, wantBytes: "a", wantErr: core.ErrObjectStoreIntegrity, wantCause: io.EOF},
		{name: "negative extra byte never reaches destination", source: func() io.Reader { return bytes.NewBufferString("ab") }, length: 1, wantBytes: "a", wantErr: core.ErrObjectStoreIntegrity},
		{name: "negative empty expectation cannot hide a byte", source: func() io.Reader { return bytes.NewBufferString("a") }, wantErr: core.ErrObjectStoreIntegrity},
		{name: "negative cancellation at empty EOF boundary survives", source: func() io.Reader { return iotest.ErrReader(context.Canceled) }, wantErr: core.ErrObjectStoreIntegrity, wantCause: context.Canceled},
		{name: "negative cancellation after exact bytes survives", source: func() io.Reader {
			return io.MultiReader(bytes.NewBufferString("a"), iotest.ErrReader(context.Canceled))
		}, length: 1, wantBytes: "a", wantErr: core.ErrObjectStoreIntegrity, wantCause: context.Canceled},
		{name: "negative final bytes beside deadline cannot become copy success", source: func() io.Reader { return &gcsTerminalReader{data: []byte("a"), terminal: context.DeadlineExceeded} }, length: 1, wantBytes: "a", wantErr: core.ErrObjectStoreIntegrity, wantCause: context.DeadlineExceeded},
		{name: "negative joined EOF preserves final byte failure", source: func() io.Reader {
			return &gcsTerminalReader{data: []byte("a"), terminal: errors.Join(io.EOF, context.Canceled)}
		}, length: 1, wantBytes: "a", wantErr: core.ErrObjectStoreIntegrity, wantCause: context.Canceled},
		{name: "negative probe byte cannot erase accompanying deadline", source: func() io.Reader { return &gcsTerminalReader{data: []byte("a"), terminal: context.DeadlineExceeded} }, wantErr: core.ErrObjectStoreIntegrity, wantCause: context.DeadlineExceeded},
		{name: "negative complete write cannot erase destination failure", source: func() io.Reader { return bytes.NewBufferString("a") }, length: 1, fullWrite: true, writerErr: io.ErrClosedPipe, wantBytes: "a", wantErr: core.ErrObjectStoreDestination, wantCause: io.ErrClosedPipe},
		{name: "negative destination refusal retains separate ownership", source: func() io.Reader { return bytes.NewBufferString("a") }, length: 1, writerErr: io.ErrClosedPipe, wantErr: core.ErrObjectStoreDestination, wantCause: io.ErrClosedPipe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			length, err := core.NewByteLength(tc.length)
			if err != nil {
				t.Fatalf("NewByteLength error = %v, want nil", err)
			}
			var destination bytes.Buffer
			var writer io.Writer = &destination
			if tc.writerErr != nil {
				writer = gcsRefusingWriter{err: tc.writerErr}
				if tc.fullWrite {
					writer = gcsFullErrorWriter{destination: &destination, err: tc.writerErr}
				}
			}
			gotErr := copyGCSExact(tc.source(), writer, length)
			if !errors.Is(gotErr, tc.wantErr) || tc.wantCause != nil && !errors.Is(gotErr, tc.wantCause) || destination.String() != tc.wantBytes {
				t.Fatalf("copyGCSExact = (%q, %v), want (%q, %v) preserving %v", destination.String(), gotErr, tc.wantBytes, tc.wantErr, tc.wantCause)
			}
			if tc.writerErr != nil && errors.Is(gotErr, core.ErrObjectStoreSource) {
				t.Fatalf("writer refusal error = %v, want no source misclassification", gotErr)
			}
		})
	}
}

type gcsTerminalReader struct {
	data     []byte
	terminal error
}

func (r *gcsTerminalReader) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, r.terminal
	}
	return n, nil
}

type gcsRefusingWriter struct{ err error }

func (w gcsRefusingWriter) Write([]byte) (int, error) { return 0, w.err }

type gcsFullErrorWriter struct {
	destination *bytes.Buffer
	err         error
}

func (w gcsFullErrorWriter) Write(p []byte) (int, error) {
	n, err := w.destination.Write(p)
	return n, errors.Join(w.err, err)
}
