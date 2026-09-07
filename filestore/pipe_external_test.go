package filestore_test

import (
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

func TestPipeCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	type contextKind uint8
	const (
		activeContext contextKind = iota
		absentContext
		typedNilContext
		cancelledContext
	)
	type endpointFault uint8
	const (
		intactEndpoints endpointFault = iota
		closedReader
		closedWriter
	)
	cases := []struct {
		name           string
		context        contextKind
		fault          endpointFault
		payload        string
		wantErr        error
		wantWriteErr   error
		wantWriteBytes int
		wantRead       string
		wantEndpoints  bool
	}{
		{name: "binary data survives the real OS pipe exactly", payload: "a\x00\xffz", wantEndpoints: true, wantWriteBytes: 4, wantRead: "a\x00\xffz"},
		{name: "empty completed pipe supplies EOF without a phantom byte", wantEndpoints: true},
		{name: "missing context creates neither endpoint", context: absentContext, wantErr: core.ErrContextStateContract},
		{name: "typed nil context creates neither endpoint", context: typedNilContext, wantErr: core.ErrContextStateContract},
		{name: "cancelled context creates neither endpoint", context: cancelledContext, wantErr: context.Canceled},
		{name: "closed read end preserves Go broken-pipe identity", fault: closedReader, payload: "ab", wantEndpoints: true, wantWriteErr: nativeClosedReaderCause(t)},
		{name: "closed write end preserves Go closed-file identity", fault: closedWriter, payload: "ab", wantEndpoints: true, wantWriteErr: fs.ErrClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var ctx context.Context = t.Context()
			switch tc.context {
			case activeContext:
			case absentContext:
				ctx = nil
			case typedNilContext:
				ctx = (*nilPipeContext)(nil)
			case cancelledContext:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			default:
				t.Fatalf("fixture context = %v, want a declared context", tc.context)
			}
			pipe, gotErr := filestore.OpenPipe(ctx)
			readerClosed, writerClosed := false, false
			t.Cleanup(func() {
				if pipe.Reader != nil && !readerClosed {
					if err := pipe.Reader.Close(); err != nil {
						t.Errorf("reader cleanup error = %v, want nil", err)
					}
				}
				if pipe.Writer != nil && pipe.Writer != pipe.Reader && !writerClosed {
					if err := pipe.Writer.Close(); err != nil {
						t.Errorf("writer cleanup error = %v, want nil", err)
					}
				}
			})
			if !errors.Is(gotErr, tc.wantErr) || (pipe.Reader != nil && pipe.Writer != nil) != tc.wantEndpoints {
				t.Fatalf("OpenPipe() = (%+v, %v), want endpoint presence %t and %v", pipe, gotErr, tc.wantEndpoints, tc.wantErr)
			}
			if !tc.wantEndpoints {
				if pipe != (filestore.Pipe{}) {
					t.Fatalf("refused pipe = %+v, want exact zero", pipe)
				}
				return
			}
			if err := pipe.Validate(); err != nil || pipe.Reader == pipe.Writer {
				t.Fatalf("produced endpoints/validation = (%+v, %v), want distinct valid custody", pipe, err)
			}
			switch tc.fault {
			case intactEndpoints:
			case closedReader:
				if err := pipe.Reader.Close(); err != nil {
					t.Fatalf("close-reader fixture error = %v, want nil", err)
				}
				readerClosed = true
			case closedWriter:
				if err := pipe.Writer.Close(); err != nil {
					t.Fatalf("close-writer fixture error = %v, want nil", err)
				}
				writerClosed = true
			default:
				t.Fatalf("fixture fault = %v, want declared fault", tc.fault)
			}
			// All payloads fit in one OS pipe buffer; no worker or timing rule
			// is involved in the exact-byte and native-failure observations.
			written, writeErr := io.WriteString(pipe.Writer, tc.payload)
			if written != tc.wantWriteBytes || !errors.Is(writeErr, tc.wantWriteErr) {
				t.Fatalf("native write = (%d, %v), want (%d, %v)", written, writeErr, tc.wantWriteBytes, tc.wantWriteErr)
			}
			if !writerClosed {
				if err := pipe.Writer.Close(); err != nil {
					t.Fatalf("writer finish error = %v, want nil", err)
				}
				writerClosed = true
			}
			if !readerClosed {
				data, err := io.ReadAll(io.LimitReader(pipe.Reader, 8))
				if err != nil || string(data) != tc.wantRead {
					t.Fatalf("native read = (%q, %v), want (%q, nil)", data, err, tc.wantRead)
				}
			}
		})
	}
}

func TestPipeEndpointAdmissionExhaustsPresenceAndAliasing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		omitReader bool
		omitWriter bool
		alias      bool
		wantErr    error
	}{
		{name: "distinct endpoints are admitted"},
		{name: "missing reader is refused", omitReader: true, wantErr: core.ErrFilestoreContract},
		{name: "missing writer is refused", omitWriter: true, wantErr: core.ErrFilestoreContract},
		{name: "two missing endpoints do not cancel the defect", omitReader: true, omitWriter: true, wantErr: core.ErrFilestoreContract},
		{name: "one endpoint cannot stand in for both directions", alias: true, wantErr: core.ErrFilestoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			owned, err := filestore.OpenPipe(t.Context())
			if err != nil {
				t.Fatalf("pipe fixture admission error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := errors.Join(owned.Reader.Close(), owned.Writer.Close()); err != nil {
					t.Errorf("pipe fixture close error = %v, want nil", err)
				}
			})
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
			if err := candidate.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// The standard-library control supplies the platform's strongest native cause.
// This is fixture setup, not a call back into Filestore under test.
func nativeClosedReaderCause(t testing.TB) error {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Go pipe control setup error = %v, want nil", err)
	}
	defer func() {
		if err := writer.Close(); err != nil {
			t.Errorf("Go pipe control writer close = %v, want nil", err)
		}
	}()
	if err := reader.Close(); err != nil {
		t.Fatalf("Go pipe control reader close = %v, want nil", err)
	}
	written, err := writer.Write([]byte{0x00})
	pathErr, ok := errors.AsType[*os.PathError](err)
	if written != 0 || !ok || pathErr.Err == nil {
		t.Fatalf("Go closed-reader control = (%d, %v), want zero and a native path cause", written, err)
	}
	return pathErr.Err
}
