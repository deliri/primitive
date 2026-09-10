package filestore_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

type scratchObservingReader struct {
	source      *bytes.Reader
	scratch     []byte
	calls       int
	wrongWindow bool
}

func (r *scratchObservingReader) Read(p []byte) (int, error) {
	r.calls++
	if len(r.scratch) > 0 && (len(p) != len(r.scratch) || &p[0] != &r.scratch[0]) {
		r.wrongWindow = true
	}
	return r.source.Read(p)
}

type scratchObservingWriter struct {
	destination bytes.Buffer
	scratch     []byte
	wrongWindow bool
}

func (w *scratchObservingWriter) Write(p []byte) (int, error) {
	if len(w.scratch) > 0 && (len(p) == 0 || len(p) > len(w.scratch) || &p[0] != &w.scratch[0]) {
		w.wrongWindow = true
	}
	return w.destination.Write(p)
}

// Every row crosses the real public effect boundary. Pointer and canary checks
// reject ignoring the borrowed buffer, widening its length to capacity, or
// treating its size as an accepted extent. Test fixtures own their allocations.
func TestTransferScratchOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	for _, operation := range []struct {
		name string
		door transferExtentDoor
	}{
		{name: "read", door: transferExtentRead},
		{name: "stage", door: transferExtentStage},
		{name: "write", door: transferExtentWrite},
	} {
		for _, tc := range []struct {
			name           string
			window, extent int
			nilWindow      bool
		}{
			{name: "nil delegates allocation to Go", nilWindow: true, extent: 257},
			{name: "empty nonnil buffer cannot panic or consume spare capacity", extent: 257},
			{name: "one byte window carries multiple bytes", window: 1, extent: 5},
			{name: "partial final window keeps exact opaque bytes", window: 7, extent: 23},
			{name: "caller window is reused beyond twice its extent", window: 128, extent: 257},
			{name: "large fixed window carries full chunks and final byte", window: 32 << 10, extent: (64 << 10) + 1},
			{name: "empty source produces an exact empty effect", window: 128},
		} {
			t.Run(operation.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				const guard byte = 0x5a
				storage := bytes.Repeat([]byte{guard}, tc.window+2)
				scratch := storage[1 : 1+tc.window]
				if tc.nilWindow {
					scratch = nil
				}
				payload := deterministicPayload(tc.extent)
				source := scratchObservingReader{source: bytes.NewReader(payload), scratch: scratch}
				root := requireTestRoot(t, t.TempDir())
				location := filestore.Location{Root: root, Path: mustRelativePath(t, "target")}
				stagePath := mustRelativePath(t, "stage")
				switch operation.door {
				case transferExtentRead:
					file, err := root.OpenFile(location.Path.String(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
					if err != nil {
						t.Fatal(err)
					}
					count, copyErr := io.Copy(file, bytes.NewReader(payload))
					closeErr := file.Close()
					if copyErr != nil || closeErr != nil || count != int64(len(payload)) {
						t.Fatalf("fixture = (%d,%v,%v), want exact %d bytes", count, copyErr, closeErr, len(payload))
					}
				case transferExtentStage:
					staged, err := filestore.Stage(t.Context(), filestore.StageRequest{Source: &source, Temporary: filestore.Location{Root: root, Path: stagePath}, Mode: 0o600, Buffer: scratch})
					if err != nil || staged.Validate() != nil || staged.BytesWritten().Uint64() != uint64(len(payload)) {
						t.Fatalf("Stage = (%v,%v), want exact %d-byte receipt", staged, err, len(payload))
					}
					location.Path = staged.Path()
				case transferExtentWrite:
					recovery, err := filestore.Write(t.Context(), filestore.WriteRequest{Source: &source, Location: location, Temporary: stagePath, Mode: 0o600, Install: filestore.InstallCreate, Buffer: scratch})
					if err != nil || recovery != (filestore.CommitRequest{}) {
						t.Fatalf("Write = (%v,%v), want settled exact effect", recovery, err)
					}
				}
				if source.wrongWindow || operation.door != transferExtentRead && (source.calls == 0 || source.source.Len() != 0) {
					t.Fatalf("source = (wrong window %t,calls %d,remaining %d), want complete transfer through borrowed window", source.wrongWindow, source.calls, source.source.Len())
				}
				destination := scratchObservingWriter{scratch: scratch}
				count, err := filestore.Read(t.Context(), filestore.ReadRequest{Location: location, Destination: &destination, Buffer: scratch})
				if err != nil || count.Uint64() != uint64(len(payload)) || !bytes.Equal(destination.destination.Bytes(), payload) || destination.wrongWindow {
					t.Fatalf("Read = (%d,%v,%d bytes,wrong window %t), want exact %d bytes through borrowed window", count.Uint64(), err, destination.destination.Len(), destination.wrongWindow, len(payload))
				}
				if storage[0] != guard || storage[len(storage)-1] != guard {
					t.Fatalf("scratch guards = (%d,%d), want (%d,%d) unchanged outside borrowed length", storage[0], storage[len(storage)-1], guard, guard)
				}
				// The receipt/file owns its bytes after return; reusing scratch cannot
				// change the effect already delivered to either side.
				clear(scratch)
				retained, err := root.ReadFile(location.Path.String())
				if err != nil || !bytes.Equal(retained, payload) || !bytes.Equal(destination.destination.Bytes(), payload) {
					t.Fatalf("retained transfer = (%d,%v), want independent exact %d-byte effect", len(retained), err, len(payload))
				}
			})
		}
	}
}
