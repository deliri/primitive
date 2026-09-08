package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzHeldStandingNativeIdentityAndCustody(f *testing.F) {
	directory := f.TempDir()
	payload := []byte{0, 255, 1, 127}
	if err := os.WriteFile(filepath.Join(directory, "held"), payload, 0o600); err != nil {
		f.Fatal(err)
	}
	absolute, err := core.ParseAbsolutePath(directory)
	if err != nil {
		f.Fatal(err)
	}
	root, err := filestore.OpenRoot(f.Context(), absolute)
	if err != nil {
		f.Fatal(err)
	}
	relative, err := core.ParseRelativePath("held")
	if err != nil {
		f.Fatal(err)
	}
	request := filestore.ReadHandleRequest{Location: filestore.Location{Root: root, Path: relative}}
	if err := request.Validate(); err != nil {
		f.Fatal(err)
	}
	held, err := filestore.OpenRead(f.Context(), request)
	if err != nil {
		f.Fatal(err)
	}
	path, err := core.ParseAbsolutePath(filepath.Join(directory, relative.String()))
	if err != nil {
		f.Fatal(err)
	}
	standing, err := filestore.ObserveHeldStanding(f.Context(), held, path)
	if err != nil || standing != filestore.HeldStandingSame || standing.Validate() != nil {
		f.Fatalf("seed standing = (%v,%v), want same native inode", standing, err)
	}
	emitted := make([]byte, len(payload))
	if n, err := io.ReadFull(held, emitted); err != nil || n != len(payload) || !bytes.Equal(emitted, payload) {
		f.Fatalf("seed native bytes = (%v,%d,%v), want %v", emitted, n, err, payload)
	}
	if err := errors.Join(held.Close(), root.Close()); err != nil {
		f.Fatal(err)
	}
	for mutation := range heldNativeMutationLimit {
		f.Add(emitted, uint8(mutation))
	}
	f.Add([]byte{}, uint8(heldNativeUnchanged))
	f.Add([]byte{}, uint8(heldNativeForeignSameBytes))
	f.Add([]byte{}, uint8(heldNativePipe))
	f.Fuzz(func(t *testing.T, payload []byte, rawMutation uint8) {
		payload = payload[:min(len(payload), 512)]
		mutation := heldNativeMutation(rawMutation % uint8(heldNativeMutationLimit))
		container := t.TempDir()
		fixture, err := createHeldNativeFixture(container, mutation, payload)
		if err != nil {
			t.Fatal(err)
		}
		if !fixture.closed {
			t.Cleanup(func() {
				if err := fixture.file.Close(); err != nil {
					t.Error(err)
				}
			})
		}
		path, err := core.ParseAbsolutePath(fixture.path)
		if err != nil {
			t.Fatal(err)
		}
		held := fixture.file
		ctx := t.Context()
		var wantErr error
		switch mutation {
		case heldNativeNilHandle:
			held = nil
			wantErr = core.ErrFilestoreContract
		case heldNativeClosedHandle:
			wantErr = core.ErrFilestoreContract
		case heldNativeZeroPath:
			path = core.AbsolutePath{}
			wantErr = core.ErrFilestoreContract
		case heldNativeNilContext:
			ctx = nil
			wantErr = core.ErrNilContext
		case heldNativeCanceledContext:
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
			wantErr = context.Canceled
		}
		heldBefore, heldNativeErr := fixture.file.Stat()
		native, nativeErr := os.Lstat(fixture.path)
		want := filestore.HeldStandingUnknown
		if wantErr == nil {
			if heldNativeErr != nil {
				t.Fatalf("native fixture handle = %v, want live capability", heldNativeErr)
			}
			switch {
			case errors.Is(nativeErr, fs.ErrNotExist) || errors.Is(nativeErr, syscall.ENOTDIR):
				want = filestore.HeldStandingAbsent
			case nativeErr != nil:
				wantErr = core.ErrFilestoreSource
			case os.SameFile(heldBefore, native):
				want = filestore.HeldStandingSame
			default:
				want = filestore.HeldStandingReplaced
			}
		}
		before, err := removalFixtureSnapshot(container)
		if err != nil {
			t.Fatal(err)
		}
		metadata := make([]fs.FileInfo, len(before))
		for i, entry := range before {
			metadata[i], err = os.Lstat(filepath.Join(container, entry.name))
			if err != nil {
				t.Fatal(err)
			}
		}
		got, gotErr := filestore.ObserveHeldStanding(ctx, held, path)
		if got != want || !errors.Is(gotErr, wantErr) {
			t.Fatalf("standing = (%v,%v), want (%v,%v) from native identity", got, gotErr, want, wantErr)
		}
		if !errors.Is(wantErr, core.ErrFilestoreSource) && errors.Is(gotErr, core.ErrFilestoreSource) {
			t.Fatalf("ingress refusal = %v, want no source classification", gotErr)
		}
		if mutation == heldNativeClosedHandle && !errors.Is(gotErr, os.ErrClosed) {
			t.Fatalf("closed handle refusal = %v, want native closed identity", gotErr)
		}
		if errors.Is(wantErr, core.ErrFilestoreSource) {
			var nativePath, gotPath *os.PathError
			if !errors.As(nativeErr, &nativePath) || !errors.As(gotErr, &gotPath) || !errors.Is(gotErr, nativePath.Err) || gotPath.Op != nativePath.Op || gotPath.Path != nativePath.Path {
				t.Fatalf("source refusal = (%v,%v), want exact native PathError", gotErr, nativeErr)
			}
		}
		if wantErr == nil && got.Validate() != nil {
			t.Fatalf("admitted standing = %v, want closed valid value", got)
		}
		if fixture.closed {
			var data [1]byte
			if n, err := fixture.file.Read(data[:]); n != 0 || !errors.Is(err, os.ErrClosed) {
				t.Fatalf("closed held read = (%d,%v), want native closed identity", n, err)
			}
		} else {
			heldAfter, err := fixture.file.Stat()
			if err != nil || !os.SameFile(heldBefore, heldAfter) || heldBefore.Mode() != heldAfter.Mode() || heldBefore.ModTime().UnixNano() != heldAfter.ModTime().UnixNano() {
				t.Fatalf("retained held identity = (%v,%v), want %v", heldAfter, err, heldBefore)
			}
			if mutation == heldNativeDirectory {
				children, err := fixture.file.ReadDir(1)
				if err != nil || len(children) != 1 || children[0].Name() != "child" {
					t.Fatalf("retained directory cursor = (%v,%v), want first child", children, err)
				}
				rest, err := fixture.file.ReadDir(1)
				if len(rest) != 0 || !errors.Is(err, io.EOF) {
					t.Fatalf("directory completion = (%v,%v), want exact EOF", rest, err)
				}
			} else {
				data := make([]byte, len(payload))
				if n, err := io.ReadFull(fixture.file, data); err != nil || n != len(payload) || !bytes.Equal(data, payload) {
					t.Fatalf("retained held stream = (%v,%d,%v), want %v", data, n, err, payload)
				}
				var extra [1]byte
				if n, err := fixture.file.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
					t.Fatalf("held stream completion = (%d,%v), want exact EOF", n, err)
				}
			}
		}
		after, err := removalFixtureSnapshot(container)
		if err != nil || len(after) != len(before) {
			t.Fatalf("namespace = (%v,%v), want %v", after, err, before)
		}
		for i, want := range before {
			got := after[i]
			if got.name != want.name || got.mode != want.mode || got.target != want.target || !bytes.Equal(got.data, want.data) {
				t.Fatalf("namespace entry = %+v, want %+v", got, want)
			}
			info, err := os.Lstat(filepath.Join(container, want.name))
			if err != nil || !os.SameFile(metadata[i], info) || metadata[i].ModTime().UnixNano() != info.ModTime().UnixNano() {
				t.Fatalf("retained namespace identity = (%v,%v), want %v", info, err, metadata[i])
			}
		}
	})
}
