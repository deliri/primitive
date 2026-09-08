package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzAppendRotationNativeOwnership(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted, []byte{31}, uint8(filestore.AppendCreate), uint8(nativeHandleMissing), false, false, false)
	for _, seed := range []struct {
		payload, change               []byte
		intent                        filestore.AppendMode
		entry                         nativeHandleEntry
		canceled, closed, nilOutgoing bool
	}{
		{intent: filestore.AppendCreate, entry: nativeHandleMissing, change: []byte{0, 255}},
		{payload: []byte{0, 255}, change: []byte{31, 0}, intent: filestore.AppendCreate, entry: nativeHandleMissing},
		{payload: []byte{0, 255}, intent: filestore.AppendCreate, entry: nativeHandleRegular},
		{payload: []byte{0, 255}, intent: filestore.AppendCreate, entry: nativeHandleDirectory},
		{payload: []byte{0, 255}, intent: filestore.AppendCreate, entry: nativeHandleConfinedLink},
		{payload: []byte{0, 255}, intent: filestore.AppendCreate, entry: nativeHandleDanglingLink},
		{payload: []byte{0, 255}, intent: filestore.AppendCreate, entry: nativeHandleMissing, canceled: true},
		{payload: []byte{0, 255}, intent: filestore.AppendCreate, entry: nativeHandleMissing, closed: true},
		{payload: []byte{0, 255}, intent: filestore.AppendCreate, entry: nativeHandleMissing, nilOutgoing: true},
		{payload: []byte{0, 255}, intent: filestore.AppendExisting, entry: nativeHandleMissing},
		{payload: []byte{0, 255}, intent: filestore.AppendCreateOrOpen, entry: nativeHandleMissing},
	} {
		f.Add(seed.payload, seed.change, uint8(seed.intent), uint8(seed.entry), seed.canceled, seed.closed, seed.nilOutgoing)
	}
	f.Fuzz(func(t *testing.T, payload, change []byte, rawIntent, rawEntry uint8, canceled, closed, nilOutgoing bool) {
		payload = payload[:min(len(payload), 1024)]
		change = change[:min(len(change), 1024)]
		// Every represented entry is bounded and local. Outside confinement is
		// independently exercised by the handle ingress target and named tables.
		entry := nativeHandleEntry(rawEntry % uint8(nativeHandleOutsideLink))
		intent := filestore.AppendMode(rawIntent)
		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		neighbor := []byte{41, 0, 255}
		if err := os.WriteFile(filepath.Join(directory, "neighbor"), neighbor, 0o600); err != nil {
			t.Fatal(err)
		}
		outgoing, err := filestore.OpenAppend(t.Context(), filestore.AppendRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "current")}, Mode: 0o600, Append: filestore.AppendCreate})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = outgoing.Close() })
		if n, err := outgoing.Write(payload); err != nil || n != len(payload) {
			t.Fatalf("producer append = (%d,%v), want %d", n, err, len(payload))
		}
		original, err := outgoing.Stat()
		if err != nil {
			t.Fatal(err)
		}
		switch entry {
		case nativeHandleRegular:
			if err := os.WriteFile(filepath.Join(directory, "next"), neighbor, 0o600); err != nil {
				t.Fatal(err)
			}
		case nativeHandleMissing:
		case nativeHandleDirectory:
			if err := root.Mkdir("next", 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "next", "child"), neighbor, 0o600); err != nil {
				t.Fatal(err)
			}
		case nativeHandleConfinedLink:
			if err := root.Symlink("current", "next"); err != nil {
				t.Fatal(err)
			}
		case nativeHandleDanglingLink:
			if err := root.Symlink("absent", "next"); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("entry = %d, want closed bounded fixture", entry)
		}
		var occupied fs.FileInfo
		if entry != nativeHandleMissing {
			occupied, err = root.Lstat("next")
			if err != nil {
				t.Fatal(err)
			}
		}
		want, err := removalFixtureSnapshot(directory)
		if err != nil {
			t.Fatal(err)
		}
		request := filestore.RotationRequest{Outgoing: outgoing, Incoming: filestore.AppendRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "next")}, Mode: 0o640, Append: intent}}
		if nilOutgoing {
			request.Outgoing = nil
		}
		if closed {
			if err := outgoing.Close(); err != nil {
				t.Fatal(err)
			}
		}
		ctx := t.Context()
		if canceled {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
		}
		var wantErr, wantNative error
		admitted := !canceled && !nilOutgoing && intent == filestore.AppendCreate
		switch {
		case canceled:
			wantErr = context.Canceled
		case nilOutgoing || intent != filestore.AppendCreate:
			wantErr = core.ErrFilestoreContract
		case closed:
			wantErr = core.ErrFilestoreActivation
			wantNative = fs.ErrClosed
		case entry != nativeHandleMissing:
			wantErr = core.ErrFilestoreConflict
			wantNative = fs.ErrExist
		}
		incoming, gotErr := filestore.RotateAppend(ctx, request)
		if incoming != nil {
			t.Cleanup(func() { _ = incoming.Close() })
		}
		if !errors.Is(gotErr, wantErr) || wantNative != nil && !errors.Is(gotErr, wantNative) {
			t.Fatalf("rotation = (%v,%v), want %v/native %v", incoming, gotErr, wantErr, wantNative)
		}
		info, statErr := outgoing.Stat()
		if admitted || closed {
			if !errors.Is(statErr, fs.ErrClosed) {
				t.Fatalf("settled outgoing = %v, want native closed", statErr)
			}
		} else if statErr != nil || !os.SameFile(original, info) || info.Mode() != original.Mode() || info.Size() != int64(len(payload)) || !info.ModTime().Equal(original.ModTime()) {
			t.Fatalf("retained outgoing = (%v,%v), want original live metadata", info, statErr)
		}
		if wantErr != nil {
			if incoming != nil {
				t.Fatalf("refused incoming = %v, want nil", incoming)
			}
		} else {
			if incoming == nil {
				t.Fatal("accepted incoming = nil, want real Go file")
			}
			initial, err := incoming.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if !initial.Mode().IsRegular() || initial.Size() != 0 || initial.Mode().Perm() != request.Incoming.Mode || os.SameFile(initial, original) {
				t.Fatalf("incoming = %v, want new empty inode with requested mode", initial)
			}
			if n, err := incoming.Write(change); err != nil || n != len(change) {
				t.Fatalf("incoming write = (%d,%v), want %d", n, err, len(change))
			}
			if err := incoming.Close(); err != nil {
				t.Fatal(err)
			}
			want = append(want, removalFixtureEntry{name: "next", mode: request.Incoming.Mode, data: bytes.Clone(change)})
		}
		retained, err := root.Lstat("current")
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(original, retained) || retained.Mode() != original.Mode() || !retained.ModTime().Equal(original.ModTime()) {
			t.Fatalf("outgoing name = %v, want unchanged original inode %v", retained, original)
		}
		if occupied != nil {
			retained, err := root.Lstat("next")
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(occupied, retained) || occupied.Mode() != retained.Mode() || !occupied.ModTime().Equal(retained.ModTime()) {
				t.Fatalf("occupied incoming = %v, want unchanged %v", retained, occupied)
			}
		}
		got, err := removalFixtureSnapshot(directory)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatalf("namespace = %+v, want %+v", got, want)
		}
		for i, entry := range got {
			if entry.name != want[i].name || entry.mode != want[i].mode || entry.target != want[i].target || !bytes.Equal(entry.data, want[i].data) {
				t.Fatalf("entry %d = %+v, want %+v", i, entry, want[i])
			}
		}
	})
}
