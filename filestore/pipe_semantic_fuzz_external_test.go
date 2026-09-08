package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzPipeNativeCustodyAndBytes(f *testing.F) {
	seed, err := filestore.OpenPipe(f.Context())
	if err != nil || seed.Validate() != nil {
		f.Fatalf("pipe seed = (%+v,%v), want admitted endpoints", seed, err)
	}
	input := []byte{0, 255, 1, 127}
	if n, err := seed.Writer.Write(input); err != nil || n != len(input) {
		f.Fatalf("seed write = (%d,%v), want %d", n, err, len(input))
	}
	if err := seed.Writer.Close(); err != nil {
		f.Fatal(err)
	}
	emitted := make([]byte, len(input))
	if n, err := io.ReadFull(seed.Reader, emitted); err != nil || n != len(input) || !bytes.Equal(emitted, input) {
		f.Fatalf("seed read = (%v,%d,%v), want %v", emitted, n, err, input)
	}
	if err := seed.Reader.Close(); err != nil {
		f.Fatal(err)
	}
	f.Add(emitted, uint8(pipeContextActive), uint8(pipeEndpointsIntact), uint8(1))
	f.Add([]byte{}, uint8(pipeContextActive), uint8(pipeEndpointsIntact), uint8(1))
	f.Add(bytes.Repeat(emitted, 128), uint8(pipeContextActive), uint8(pipeEndpointsIntact), uint8(64))
	for kind := pipeContextMissing; kind < pipeContextLimit; kind++ {
		f.Add(emitted, uint8(kind), uint8(pipeEndpointsIntact), uint8(1))
	}
	for _, fault := range []pipeEndpointFault{pipeReaderClosed, pipeWriterClosed, pipeBothClosed} {
		f.Add(emitted, uint8(pipeContextActive), uint8(fault), uint8(1))
		f.Add([]byte{}, uint8(pipeContextActive), uint8(fault), uint8(1))
	}
	f.Fuzz(func(t *testing.T, payload []byte, rawContext, rawFault, rawFragment uint8) {
		payload = payload[:min(len(payload), 512)]
		kind := pipeContextKind(rawContext % uint8(pipeContextLimit))
		fault := pipeEndpointFault(rawFault % uint8(pipeEndpointFaultLimit))
		fragment := int(rawFragment%64) + 1
		ctx := t.Context()
		var cancel context.CancelFunc
		var wantErr error
		switch kind {
		case pipeContextActive:
		case pipeContextMissing:
			ctx = nil
			wantErr = core.ErrNilContext
		case pipeContextPanickingNil:
			ctx = (*nilPipeContext)(nil)
			wantErr = core.ErrContextObservation
		case pipeContextSafeNil:
			ctx = (*safeNilPipeContext)(nil)
		case pipeContextCanceled:
			ctx, cancel = context.WithCancel(ctx)
			cancel()
			wantErr = context.Canceled
		case pipeContextExpired:
			ctx, cancel = newFilesystemBackstop(ctx, t, 0)
			defer cancel()
			wantErr = context.DeadlineExceeded
		case pipeContextCancelAfterAcquisition:
			ctx, cancel = context.WithCancel(ctx)
			defer cancel()
		default:
			t.Fatalf("context = %v, want declared fixture", kind)
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
		if !errors.Is(gotErr, wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) {
			t.Fatalf("OpenPipe = (%+v,%v), want exact %v", pipe, gotErr, wantErr)
		}
		if wantErr != nil {
			if pipe != (filestore.Pipe{}) {
				t.Fatalf("refused pipe = %+v, want zero", pipe)
			}
			return
		}
		if pipe.Validate() != nil || pipe.Reader == pipe.Writer {
			t.Fatalf("pipe = %+v, want distinct admitted endpoints", pipe)
		}
		finish := newPipeBackstop(t, pipe)
		defer func() {
			if err := finish(); err != nil {
				t.Errorf("pipe required backstop: %v", err)
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
		if kind == pipeContextCancelAfterAcquisition {
			cancel()
		}
		if fault == pipeReaderClosed || fault == pipeBothClosed {
			if err := pipe.Reader.Close(); err != nil {
				t.Fatal(err)
			}
			readerClosed = true
		}
		if fault == pipeWriterClosed || fault == pipeBothClosed {
			if err := pipe.Writer.Close(); err != nil {
				t.Fatal(err)
			}
			writerClosed = true
		}
		var wantWriteErr error
		if writerClosed {
			wantWriteErr = fs.ErrClosed
		} else if readerClosed {
			wantWriteErr = nativeClosedReaderCause(t, payload[:min(len(payload), fragment)])
		}
		written := 0
		for {
			end := min(written+fragment, len(payload))
			n, err := pipe.Writer.Write(payload[written:end])
			if wantWriteErr != nil {
				if n != 0 || !errors.Is(err, wantWriteErr) {
					t.Fatalf("refused write = (%d,%v), want (0,%v)", n, err, wantWriteErr)
				}
				break
			}
			if err != nil || n != end-written {
				t.Fatalf("fragment write = (%d,%v), want %d", n, err, end-written)
			}
			written += n
			if written == len(payload) {
				break
			}
		}
		if !writerClosed {
			if err := pipe.Writer.Close(); err != nil {
				t.Fatal(err)
			}
			writerClosed = true
		}
		if !readerClosed {
			want := payload
			if wantWriteErr != nil {
				want = nil
			}
			got := make([]byte, len(want))
			if n, err := io.ReadFull(pipe.Reader, got); err != nil || n != len(want) || !bytes.Equal(got, want) {
				t.Fatalf("received = (%v,%d,%v), want %v", got, n, err, want)
			}
			var extra [1]byte
			if n, err := pipe.Reader.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
				t.Fatalf("stream completion = (%d,%v), want exact EOF", n, err)
			}
			if err := pipe.Reader.Close(); err != nil {
				t.Fatal(err)
			}
			readerClosed = true
		} else {
			var data [1]byte
			if n, err := pipe.Reader.Read(data[:]); n != 0 || !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("closed read = (%d,%v), want closed identity", n, err)
			}
		}
		if err := pipe.Validate(); err != nil {
			t.Fatalf("closed endpoint custody = %v, want admitted shape", err)
		}
	})
}
