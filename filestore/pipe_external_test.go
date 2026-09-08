package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type nilPipeContext struct{ context.Context }
type safeNilPipeContext struct{ context.Context }

func (*safeNilPipeContext) Err() error { return nil }

type pipeContextKind uint8

const (
	pipeContextActive pipeContextKind = iota
	pipeContextMissing
	pipeContextPanickingNil
	pipeContextSafeNil
	pipeContextCanceled
	pipeContextExpired
	pipeContextCancelAfterAcquisition
	pipeContextLimit
)

type pipeEndpointFault uint8

const (
	pipeEndpointsIntact pipeEndpointFault = iota
	pipeReaderClosed
	pipeWriterClosed
	pipeBothClosed
	pipeEndpointFaultLimit
)

func TestPipeCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                  string
		context               pipeContextKind
		fault                 pipeEndpointFault
		payload               []byte
		wantErr, wantWriteErr error
		wantRead              []byte
		wantEndpoints         bool
	}{
		{name: "binary bytes survive exact native pipe custody", payload: []byte{0, 255, 1, 127}, wantRead: []byte{0, 255, 1, 127}, wantEndpoints: true},
		{name: "empty completed pipe gives EOF without invented data", wantEndpoints: true},
		{name: "fixed 512-byte transfer cannot be truncated", payload: bytes.Repeat([]byte{0, 255, 1, 127}, 128), wantRead: bytes.Repeat([]byte{0, 255, 1, 127}, 128), wantEndpoints: true},
		{name: "missing context creates neither endpoint", context: pipeContextMissing, wantErr: core.ErrNilContext},
		{name: "panicking typed nil context keeps observation identity", context: pipeContextPanickingNil, wantErr: core.ErrContextObservation},
		{name: "nil-safe context is admitted by its actual method contract", context: pipeContextSafeNil, payload: []byte{0, 255}, wantRead: []byte{0, 255}, wantEndpoints: true},
		{name: "canceled context creates neither endpoint", context: pipeContextCanceled, wantErr: context.Canceled},
		{name: "expired context keeps deadline identity", context: pipeContextExpired, wantErr: context.DeadlineExceeded},
		{name: "later cancellation does not revoke transferred Go handles", context: pipeContextCancelAfterAcquisition, payload: []byte{0, 255}, wantRead: []byte{0, 255}, wantEndpoints: true},
		{name: "closed reader retains Go broken-pipe cause", fault: pipeReaderClosed, payload: []byte{0, 255}, wantEndpoints: true},
		{name: "zero-byte write retains native closed-reader behavior", fault: pipeReaderClosed, wantEndpoints: true},
		{name: "closed writer retains Go closed-file cause", fault: pipeWriterClosed, payload: []byte{0, 255}, wantWriteErr: fs.ErrClosed, wantEndpoints: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			var cancel context.CancelFunc
			switch tc.context {
			case pipeContextActive:
			case pipeContextMissing:
				ctx = nil
			case pipeContextPanickingNil:
				ctx = (*nilPipeContext)(nil)
			case pipeContextSafeNil:
				ctx = (*safeNilPipeContext)(nil)
			case pipeContextCanceled:
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case pipeContextExpired:
				ctx, cancel = newFilesystemBackstop(ctx, t, 0)
				defer cancel()
			case pipeContextCancelAfterAcquisition:
				ctx, cancel = context.WithCancel(ctx)
				defer cancel()
			default:
				t.Fatalf("context = %v, want declared fixture", tc.context)
			}
			wantWriteErr := tc.wantWriteErr
			if tc.fault == pipeReaderClosed {
				wantWriteErr = nativeClosedReaderCause(t, tc.payload)
			}
			pipe, gotErr := filestore.OpenPipe(ctx)
			readerClosed, writerClosed := false, false
			t.Cleanup(func() {
				if pipe.Reader != nil && !readerClosed {
					if err := pipe.Reader.Close(); err != nil {
						t.Error(err)
					}
				}
				if pipe.Writer != nil && pipe.Writer != pipe.Reader && !writerClosed {
					if err := pipe.Writer.Close(); err != nil {
						t.Error(err)
					}
				}
			})
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) {
				t.Fatalf("OpenPipe() error = %v, want %v without invented source failure", gotErr, tc.wantErr)
			}
			if !tc.wantEndpoints {
				if pipe != (filestore.Pipe{}) {
					t.Fatalf("refused pipe = %+v, want zero", pipe)
				}
				return
			}
			if pipe.Validate() != nil || pipe.Reader == pipe.Writer {
				t.Fatalf("returned pipe = %+v, want distinct admitted endpoints", pipe)
			}
			finish := newPipeBackstop(t, pipe)
			defer func() {
				if err := finish(); err != nil {
					t.Errorf("pipe required backstop cancellation: %v", err)
				}
			}()
			readerInfo, err := pipe.Reader.Stat()
			if err != nil || readerInfo.Mode()&fs.ModeNamedPipe == 0 {
				t.Fatalf("reader = (%v,%v), want native pipe", readerInfo, err)
			}
			writerInfo, err := pipe.Writer.Stat()
			if err != nil || writerInfo.Mode()&fs.ModeNamedPipe == 0 {
				t.Fatalf("writer = (%v,%v), want native pipe", writerInfo, err)
			}
			if tc.context == pipeContextCancelAfterAcquisition {
				cancel()
			}
			switch tc.fault {
			case pipeEndpointsIntact:
			case pipeReaderClosed:
				if err := pipe.Reader.Close(); err != nil {
					t.Fatal(err)
				}
				readerClosed = true
			case pipeWriterClosed:
				if err := pipe.Writer.Close(); err != nil {
					t.Fatal(err)
				}
				writerClosed = true
			default:
				t.Fatalf("fault = %v, want declared fixture", tc.fault)
			}
			written, writeErr := pipe.Writer.Write(tc.payload)
			wantWritten := len(tc.payload)
			if wantWriteErr != nil {
				wantWritten = 0
			}
			if written != wantWritten || !errors.Is(writeErr, wantWriteErr) {
				t.Fatalf("native write = (%d,%v), want (%d,%v)", written, writeErr, wantWritten, wantWriteErr)
			}
			if !writerClosed {
				if err := pipe.Writer.Close(); err != nil {
					t.Fatal(err)
				}
				writerClosed = true
			}
			if !readerClosed {
				data := make([]byte, len(tc.wantRead))
				n, err := io.ReadFull(pipe.Reader, data)
				if err != nil || n != len(data) || !bytes.Equal(data, tc.wantRead) {
					t.Fatalf("native read = (%v,%d,%v), want %v", data, n, err, tc.wantRead)
				}
				var extra [1]byte
				if n, err := pipe.Reader.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
					t.Fatalf("completed stream = (%d,%v), want exact EOF", n, err)
				}
			}
		})
	}
}

func TestPipeEndpointAdmissionExhaustsPresenceAndAliasing(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                   string
		omitReader, omitWriter, alias, reverse bool
		fault                                  pipeEndpointFault
		wantErr                                error
	}{
		{name: "distinct endpoints preserve buffered native bytes"},
		{name: "missing reader cannot transfer a complete capability", omitReader: true, wantErr: core.ErrFilestoreContract},
		{name: "missing writer cannot transfer a complete capability", omitWriter: true, wantErr: core.ErrFilestoreContract},
		{name: "two missing endpoints do not cancel the defect", omitReader: true, omitWriter: true, wantErr: core.ErrFilestoreContract},
		{name: "one endpoint cannot stand in for both directions", alias: true, wantErr: core.ErrFilestoreContract},
		{name: "reversed real handles are shape-admitted without I/O policy", reverse: true},
		{name: "closed reader retains structurally valid custody", fault: pipeReaderClosed},
		{name: "closed writer retains structurally valid custody", fault: pipeWriterClosed},
		{name: "both closed handles remain structurally distinct", fault: pipeBothClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			owned := filestore.Pipe{Reader: reader, Writer: writer}
			readerClosed, writerClosed := false, false
			t.Cleanup(func() {
				if !readerClosed {
					if err := reader.Close(); err != nil {
						t.Error(err)
					}
				}
				if !writerClosed {
					if err := writer.Close(); err != nil {
						t.Error(err)
					}
				}
			})
			finish := newPipeBackstop(t, owned)
			defer func() {
				if err := finish(); err != nil {
					t.Error(err)
				}
			}()
			payload := []byte{0, 255, 1, 127}
			if n, err := writer.Write(payload); err != nil || n != len(payload) {
				t.Fatalf("queued native bytes = (%d,%v), want %d", n, err, len(payload))
			}
			if tc.fault == pipeReaderClosed || tc.fault == pipeBothClosed {
				if err := reader.Close(); err != nil {
					t.Fatal(err)
				}
				readerClosed = true
			}
			if tc.fault == pipeWriterClosed || tc.fault == pipeBothClosed {
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				writerClosed = true
			}
			candidate := owned
			if tc.omitReader {
				candidate.Reader = nil
			}
			if tc.omitWriter {
				candidate.Writer = nil
			}
			if tc.alias {
				candidate.Writer = candidate.Reader
			}
			if tc.reverse {
				candidate.Reader, candidate.Writer = candidate.Writer, candidate.Reader
			}
			before := candidate
			gotErr := candidate.Validate()
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) || candidate != before {
				t.Fatalf("Validate() = (%+v,%v), want preserved %+v and %v", candidate, gotErr, before, tc.wantErr)
			}
			if !writerClosed {
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				writerClosed = true
			}
			if readerClosed {
				var data [1]byte
				if n, err := reader.Read(data[:]); n != 0 || !errors.Is(err, fs.ErrClosed) {
					t.Fatalf("closed reader = (%d,%v), want native closed identity", n, err)
				}
			} else {
				data := make([]byte, len(payload))
				if n, err := io.ReadFull(reader, data); err != nil || n != len(payload) || !bytes.Equal(data, payload) {
					t.Fatalf("retained queued bytes = (%v,%d,%v), want %v", data, n, err, payload)
				}
				var extra [1]byte
				if n, err := reader.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
					t.Fatalf("completed stream = (%d,%v), want exact EOF", n, err)
				}
			}
		})
	}
}

// A native control exposes the platform's broken-pipe cause; it never invokes
// production under test and returns the cause for direct comparisons there.
func nativeClosedReaderCause(t testing.TB, payload []byte) error {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	n, err := writer.Write(payload)
	if len(payload) == 0 && n == 0 && err == nil {
		return nil
	}
	cause, ok := errors.AsType[*os.PathError](err)
	if n != 0 || !ok || cause.Err == nil {
		t.Fatalf("native closed-reader control = (%d,%v), want zero and PathError cause", n, err)
	}
	return cause.Err
}
