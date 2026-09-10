package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type readBoundaryFault uint8

const (
	readBoundaryRegular readBoundaryFault = iota
	readBoundaryMissing
	readBoundaryMissingParent
	readBoundaryDirectory
	readBoundaryConfinedLink
	readBoundaryEscape
	readBoundaryClosedRoot
	readBoundaryNilContext
	readBoundaryCanceled
	readBoundaryNilDestination
	readBoundaryNilRoot
	readBoundaryZeroPath

	readBoundaryWriter
	readBoundaryClosedWriter
)

type readBoundarySink struct {
	data         []byte
	count, calls int
	err          error
}

func (w *readBoundarySink) Write(data []byte) (int, error) {
	w.calls++
	if w.count >= 0 && w.count <= len(data) {
		w.data = append(w.data, data[:w.count]...)
	}
	return w.count, w.err
}

// One public reader matrix replaces separate polite triads. Every row proves
// the exact returned count, delivered bytes, error class and retained namespace.
func TestReadNativeBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	binary := []byte{0, 255, 1, 127}
	for _, tc := range []struct {
		name    string
		fault   readBoundaryFault
		payload []byte

		writerCount         int
		writerErr           error
		want                []byte
		wantCount           uint64
		wantErr, wantNative error
		wantWrites          int
	}{
		{name: "regular source preserves every opaque binary byte", payload: binary, want: binary, wantCount: 4},
		{name: "empty file emits nothing to a rejecting writer", fault: readBoundaryWriter, writerErr: io.ErrClosedPipe},
		{name: "missing leaf cannot fabricate a byte receipt", fault: readBoundaryMissing, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrNotExist},
		{name: "missing ancestor retains native path refusal", fault: readBoundaryMissingParent, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrNotExist},
		{name: "directory is refused before writer execution", fault: readBoundaryDirectory, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrInvalid},
		{name: "confined link uses Go root resolution", fault: readBoundaryConfinedLink, payload: binary, want: binary, wantCount: 4},
		{name: "escaping ancestor cannot deliver outside bytes", fault: readBoundaryEscape, payload: binary, wantErr: core.ErrFilestoreSource},
		{name: "closed root preserves native closed identity", fault: readBoundaryClosedRoot, payload: binary, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrClosed},
		{name: "nil context refuses before source acquisition", fault: readBoundaryNilContext, payload: binary, wantErr: core.ErrNilContext},
		{name: "canceled context refuses before source acquisition", fault: readBoundaryCanceled, payload: binary, wantErr: context.Canceled},
		{name: "nil destination cannot consume the source", fault: readBoundaryNilDestination, payload: binary, wantErr: core.ErrFilestoreContract},
		{name: "nil root cannot fall back to the working directory", fault: readBoundaryNilRoot, payload: binary, wantErr: core.ErrFilestoreContract},
		{name: "zero path cannot observe the root directory", fault: readBoundaryZeroPath, payload: binary, wantErr: core.ErrFilestoreContract},
		{name: "short nil write is not retried into success", fault: readBoundaryWriter, payload: binary, writerCount: 2, want: binary[:2], wantCount: 2, wantErr: core.ErrFilestoreDestination, wantNative: io.ErrShortWrite, wantWrites: 1},
		{name: "partial writer failure keeps exact acknowledgment and cause", fault: readBoundaryWriter, payload: binary, writerCount: 2, writerErr: io.ErrClosedPipe, want: binary[:2], wantCount: 2, wantErr: core.ErrFilestoreDestination, wantNative: io.ErrClosedPipe, wantWrites: 1},
		{name: "full write with an error still retains that error", fault: readBoundaryWriter, payload: binary, writerCount: 4, writerErr: io.ErrClosedPipe, want: binary, wantCount: 4, wantErr: core.ErrFilestoreDestination, wantNative: io.ErrClosedPipe, wantWrites: 1},
		{name: "zero progress writer cannot claim consumed source bytes", fault: readBoundaryWriter, payload: binary, wantErr: core.ErrFilestoreDestination, wantNative: io.ErrShortWrite, wantWrites: 1},
		{name: "negative writer count cannot wrap the receipt", fault: readBoundaryWriter, payload: binary, writerCount: -1, wantErr: core.ErrFilestoreDestination, wantNative: io.ErrShortWrite, wantWrites: 1},
		{name: "overreported writer count cannot escape its input extent", fault: readBoundaryWriter, payload: binary, writerCount: 5, wantErr: core.ErrFilestoreDestination, wantNative: io.ErrShortWrite, wantWrites: 1},
		{name: "closed Go destination preserves native failure", fault: readBoundaryClosedWriter, payload: binary, wantErr: core.ErrFilestoreDestination, wantNative: fs.ErrClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			container := t.TempDir()
			directory := filepath.Join(container, "root")
			outside := filepath.Join(container, "outside")
			for _, path := range []string{directory, outside} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(directory, "source"), tc.payload, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(outside, "secret"), binary, 0o640); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			closed := false
			t.Cleanup(func() {
				if !closed {
					if err := root.Close(); err != nil {
						t.Error(err)
					}
				}
			})
			var output bytes.Buffer
			writer := readBoundarySink{count: tc.writerCount, err: tc.writerErr}
			request := filestore.ReadRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "source")}, Destination: &output}
			ctx := t.Context()
			switch tc.fault {
			case readBoundaryRegular:
			case readBoundaryMissing:
				request.Location.Path = mustRelativePath(t, "missing")
			case readBoundaryMissingParent:
				request.Location.Path = mustRelativePath(t, filepath.Join("missing", "leaf"))
			case readBoundaryDirectory:
				request.Location.Path = mustRelativePath(t, ".")
			case readBoundaryConfinedLink:
				if err := root.Symlink("source", "alias"); err != nil {
					t.Fatal(err)
				}
				request.Location.Path = mustRelativePath(t, "alias")
			case readBoundaryEscape:
				if err := root.Symlink(outside, "escape"); err != nil {
					t.Fatal(err)
				}
				request.Location.Path = mustRelativePath(t, filepath.Join("escape", "secret"))
			case readBoundaryClosedRoot:
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
				closed = true
			case readBoundaryNilContext:
				ctx = nil
			case readBoundaryCanceled:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case readBoundaryNilDestination:
				request.Destination = nil
			case readBoundaryNilRoot:
				request.Location.Root = nil
			case readBoundaryZeroPath:
				request.Location.Path = core.RelativePath{}
			case readBoundaryWriter:
				request.Destination = &writer
			case readBoundaryClosedWriter:
				destination, err := os.Create(filepath.Join(directory, "destination"))
				if err != nil {
					t.Fatal(err)
				}
				if err := destination.Close(); err != nil {
					t.Fatal(err)
				}
				request.Destination = destination
			default:
				t.Fatalf("fault = %v, want declared fixture", tc.fault)
			}
			before, err := removalFixtureSnapshot(container)
			if err != nil {
				t.Fatal(err)
			}
			sourceBefore, err := os.Stat(filepath.Join(directory, "source"))
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := filestore.Read(ctx, request)
			if got.Uint64() != tc.wantCount || !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("Read = (%d,%v), want (%d,%v) and native %v", got.Uint64(), gotErr, tc.wantCount, tc.wantErr, tc.wantNative)
			}
			for _, class := range []error{core.ErrFilestoreSource, core.ErrFilestoreDestination, core.ErrFilestoreSize, core.ErrFilestoreActivation, core.ErrFilestoreCleanup, core.ErrFilestoreActivationIndeterminate} {
				if errors.Is(gotErr, class) != errors.Is(tc.wantErr, class) {
					t.Fatalf("error %v class %v = %t, want %t", gotErr, class, errors.Is(gotErr, class), errors.Is(tc.wantErr, class))
				}
			}
			data := output.Bytes()
			if tc.fault == readBoundaryWriter {
				data = writer.data
			}
			if !bytes.Equal(data, tc.want) || writer.calls != tc.wantWrites {
				t.Fatalf("destination = (%v,%d calls), want (%v,%d)", data, writer.calls, tc.want, tc.wantWrites)
			}
			after, err := removalFixtureSnapshot(container)
			if err != nil || len(after) != len(before) {
				t.Fatalf("namespace = (%v,%v), want exact original entries", after, err)
			}
			for i, entry := range before {
				if after[i].name != entry.name || after[i].mode != entry.mode || after[i].target != entry.target || !bytes.Equal(after[i].data, entry.data) {
					t.Fatalf("namespace entry %d = %+v, want %+v", i, after[i], entry)
				}
			}
			sourceAfter, err := os.Stat(filepath.Join(directory, "source"))
			if err != nil || !os.SameFile(sourceBefore, sourceAfter) || sourceBefore.ModTime().UnixNano() != sourceAfter.ModTime().UnixNano() {
				t.Fatalf("source custody = (%v,%v), want original inode/timestamp", sourceAfter, err)
			}
			// Go root escape errors lack an exported sentinel. Preserve Source plus
			// the native PathError structure without comparing its prose.
			if tc.fault == readBoundaryEscape || tc.fault == readBoundaryMissing || tc.fault == readBoundaryMissingParent || tc.fault == readBoundaryClosedRoot || tc.fault == readBoundaryClosedWriter {
				var native *fs.PathError
				if !errors.As(gotErr, &native) || native.Err == nil {
					t.Fatalf("native refusal = %v, want retained PathError and cause", gotErr)
				}
			}
		})
	}
}
