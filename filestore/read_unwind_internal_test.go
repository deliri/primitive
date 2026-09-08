package filestore

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type unwindReadDestination struct {
	bytes.Buffer
	terminal          error
	panicAtWrite      bool
	effectBeforePanic bool
	writes            int
}

func (w *unwindReadDestination) Write(p []byte) (int, error) {
	w.writes++
	if w.panicAtWrite && !w.effectBeforePanic {
		panic(w.terminal)
	}
	n, err := w.Buffer.Write(p)
	if w.panicAtWrite {
		panic(w.terminal)
	}
	return n, errors.Join(err, w.terminal)
}

// The internal seam receives the actual Go handle that public Read transfers
// into it. File.Stat proves that exact handle was closed, without process-wide
// descriptor counts or relying on eventual Go finalizers.
func TestReadCallerUnwindCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name              string
		payload           []byte
		panicAtWrite      bool
		effectBeforePanic bool
		returnFailure     bool
		wantWrites        int
		wantBytes         []byte
		wantErr           error
	}{
		{name: "empty source never invokes a hostile destination", panicAtWrite: true},
		{name: "binary source preserves exact returned receipt", payload: []byte{0, 255}, wantWrites: 1, wantBytes: []byte{0, 255}},
		{name: "native write failure retains acknowledged bytes and closes source", payload: []byte{0, 255}, returnFailure: true, wantWrites: 1, wantBytes: []byte{0, 255}, wantErr: core.ErrFilestoreDestination},
		{name: "panic before write effect still closes source", payload: []byte{0, 255}, panicAtWrite: true, wantWrites: 1},
		{name: "panic after physical write does not invent a returned receipt", payload: []byte{0, 255}, panicAtWrite: true, effectBeforePanic: true, wantWrites: 1, wantBytes: []byte{0, 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path := directory + "/source"
			if err := os.WriteFile(path, tc.payload, 0o600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
					t.Error(err)
				}
			})
			before, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			maximum, err := core.NewByteCount(uint64(max(1, len(tc.payload))))
			if err != nil {
				t.Fatal(err)
			}
			native := &fs.PathError{Op: "write", Path: "caller-destination", Err: fs.ErrPermission}
			destination := &unwindReadDestination{panicAtWrite: tc.panicAtWrite, effectBeforePanic: tc.effectBeforePanic}
			if tc.panicAtWrite || tc.returnFailure {
				destination.terminal = native
			}
			var got core.ByteLength
			var gotErr error
			var gotPanic any
			returned := false
			func() {
				defer func() { gotPanic = recover() }()
				got, gotErr = readOwnedRegularFile(t.Context(), ReadRequest{Destination: destination, MaximumBytes: maximum}, file, before)
				returned = true
			}()
			wantPanic := tc.panicAtWrite && tc.wantWrites > 0
			if wantPanic {
				if gotPanic != native || returned || got != (core.ByteLength{}) || gotErr != nil {
					t.Errorf("unwind = (%v,%t,%+v,%v), want original panic with no returned receipt", gotPanic, returned, got, gotErr)
				}
			} else if gotPanic != nil || !returned || got.Uint64() != uint64(len(tc.wantBytes)) || (gotErr == nil) != (tc.wantErr == nil) || tc.wantErr != nil && (!errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, native)) {
				t.Errorf("return = (%v,%t,%d,%v), want (%d,%v) with native identity", gotPanic, returned, got.Uint64(), gotErr, len(tc.wantBytes), tc.wantErr)
			}
			if _, err := file.Stat(); !errors.Is(err, fs.ErrClosed) {
				t.Errorf("owned source Stat = %v, want closed before return or unwind", err)
			}
			if destination.writes != tc.wantWrites || !bytes.Equal(destination.Bytes(), tc.wantBytes) {
				t.Errorf("destination = (%d,%v), want (%d,%v)", destination.writes, destination.Bytes(), tc.wantWrites, tc.wantBytes)
			}
			after, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			retained, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(retained, tc.payload) || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
				t.Errorf("source = (%v,%v,%v), want original inode, metadata and %v", after, retained, err, tc.payload)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 {
				t.Errorf("namespace = (%v,%v), want only original source", entries, err)
			}
		})
	}
}
