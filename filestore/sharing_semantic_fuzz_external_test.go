package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Native fixtures are bounded. A real public reader emits the seed bytes;
// the observation must neither alter those bytes nor consume held custody.
func FuzzSharingNativeObservationAndCustody(f *testing.F) {
	directory := f.TempDir()
	name := filepath.Join(directory, "seed")
	input := []byte{0, 255, 1, 127}
	if err := os.WriteFile(name, input, 0o600); err != nil {
		f.Fatal(err)
	}
	rootPath, err := core.ParseAbsolutePath(directory)
	if err != nil {
		f.Fatal(err)
	}
	root, err := filestore.OpenRoot(f.Context(), rootPath)
	if err != nil {
		f.Fatal(err)
	}
	path, err := core.ParseRelativePath("seed")
	if err != nil {
		f.Fatal(err)
	}
	location := filestore.Location{Root: root, Path: path}
	if err := location.Validate(); err != nil {
		f.Fatal(err)
	}
	file, err := filestore.OpenRead(f.Context(), filestore.ReadHandleRequest{Location: location})
	if err != nil {
		f.Fatal(err)
	}
	emitted := make([]byte, len(input))
	n, readErr := io.ReadFull(file, emitted)
	closeErr := file.Close()
	rootErr := root.Close()
	if n != len(input) || readErr != nil || closeErr != nil || rootErr != nil || !bytes.Equal(emitted, input) {
		f.Fatalf("seed = (%v,%d,%v,%v,%v), want emitted %v", emitted, n, readErr, closeErr, rootErr, input)
	}
	for kind := range sharingIngressLimit {
		f.Add(emitted, uint8(kind), false, false)
	}
	f.Add(emitted, uint8(sharingIngressActive), true, false)
	f.Add(emitted, uint8(sharingIngressActive), false, true)
	f.Add([]byte{}, uint8(sharingIngressActive), true, false)
	f.Fuzz(func(t *testing.T, payload []byte, rawIngress uint8, held, missing bool) {
		payload = payload[:min(len(payload), 512)]
		directory := t.TempDir()
		name := filepath.Join(directory, "entry")
		if err := os.WriteFile(name, payload, 0o600); err != nil {
			t.Fatal(err)
		}
		before, err := os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		var file *os.File
		if held {
			file, err = os.Open(name)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			})
		}
		probe := name
		if missing {
			probe = filepath.Join(directory, "missing")
		}
		path := mustAbsolute(t, probe)
		ctx := t.Context()
		var wantErr error
		kind := sharingIngress(rawIngress % uint8(sharingIngressLimit))
		switch kind {
		case sharingIngressActive:
		case sharingIngressNil:
			ctx = nil
			wantErr = core.ErrNilContext
		case sharingIngressPanickingNil:
			ctx = (*sharingNilContext)(nil)
			wantErr = core.ErrContextObservation
		case sharingIngressSafeNil:
			ctx = (*sharingSafeNilContext)(nil)
		case sharingIngressCanceled:
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
			wantErr = context.Canceled
		case sharingIngressExpired:
			var cancel context.CancelFunc
			ctx, cancel = newFilesystemBackstop(ctx, t, 0)
			defer cancel()
			wantErr = context.DeadlineExceeded
		case sharingIngressZeroPath:
			path = core.AbsolutePath{}
			wantErr = core.ErrFilestoreContract
		default:
			t.Fatalf("ingress = %v, want declared fixture", kind)
		}
		want := filestore.SharingUnknown
		nativeFailure := false
		if wantErr == nil {
			want, wantErr = nativeSharingProbe(probe)
			nativeFailure = wantErr != nil && !errors.Is(wantErr, core.ErrFilestoreContract)
		}
		got, gotErr := filestore.ObserveSharing(ctx, path)
		if got != want || !errors.Is(gotErr, wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) != nativeFailure {
			t.Fatalf("ObserveSharing = (%v,%v), want (%v,%v), native failure %t", got, gotErr, want, wantErr, nativeFailure)
		}
		if gotErr == nil && got.Validate() != nil {
			t.Fatalf("accepted observation = %v, want admitted enum", got)
		}
		after, err := os.Stat(name)
		if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
			t.Fatalf("entry = (%v,%v), want original metadata and inode", after, err)
		}
		data, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(data, payload) {
			t.Fatalf("bytes = (%v,%v), want %v", data, err, payload)
		}
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != 1 || entries[0].Name() != "entry" {
			t.Fatalf("namespace = (%v,%v), want single original entry", entries, err)
		}
		if held {
			data := make([]byte, len(payload))
			n, err := io.ReadFull(file, data)
			if err != nil || n != len(payload) || !bytes.Equal(data, payload) {
				t.Fatalf("held cursor = (%v,%d,%v), want original bytes", data, n, err)
			}
			var extra [1]byte
			if n, err := file.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
				t.Fatalf("held EOF = (%d,%v), want (0,EOF)", n, err)
			}
		}
	})
}
