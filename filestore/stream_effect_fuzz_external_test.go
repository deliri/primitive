package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// The callback reaches the real public writer, stage producer, activation and
// read consumer. It checks exact on-disk effects and refusal cleanup, including
// an unrelated sentinel file whose bytes must survive every callback.
func FuzzFragmentedWriteActivationSemanticCustody(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted, uint16(len(emitted)-1), false, false)
	for _, seed := range []struct {
		payload []byte
		maximum uint16
		failure bool
		replace bool
	}{
		{payload: nil},
		{payload: []byte{0, 255}, maximum: 1},
		{payload: []byte{0, 255, 7}, maximum: 1},
		{payload: []byte{0, 255}, maximum: 1, failure: true},
		{payload: []byte{0, 255}, maximum: 2, replace: true},
		{payload: nil, failure: true, replace: true},
	} {
		f.Add(seed.payload, seed.maximum, seed.failure, seed.replace)
	}
	f.Fuzz(func(t *testing.T, payload []byte, rawMaximum uint16, failure, replace bool) {
		payload = payload[:min(len(payload), 4096)]
		maximum, err := core.NewByteCount(uint64(rawMaximum%4097) + 1)
		if err != nil {
			t.Fatal(err)
		}
		ceiling, err := maximum.Uint64()
		if err != nil {
			t.Fatal(err)
		}
		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		targetPath := mustRelativePath(t, "target")
		stagePath := mustRelativePath(t, "stage")
		unrelated := []byte{31, 0, 255, 9}
		if err := os.WriteFile(directory+"/unrelated", unrelated, 0o600); err != nil {
			t.Fatal(err)
		}
		install := filestore.InstallCreate
		if replace {
			install = filestore.InstallReplace
			if err := os.WriteFile(directory+"/target", unrelated, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		var source io.Reader = iotest.OneByteReader(bytes.NewReader(payload))
		native := &fs.PathError{Op: "read", Path: "caller-source", Err: fs.ErrPermission}
		if failure {
			source = io.MultiReader(source, iotest.ErrReader(errors.Join(io.EOF, native)))
		}
		got, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{Source: source, Location: filestore.Location{Root: root, Path: targetPath}, Temporary: stagePath, Mode: 0o600, Install: install, MaximumBytes: maximum})
		var wantErr error
		if uint64(len(payload)) > ceiling {
			wantErr = core.ErrFilestoreSize
		} else if failure {
			wantErr = core.ErrFilestoreSource
		}
		if (gotErr == nil) != (wantErr == nil) || wantErr != nil && !errors.Is(gotErr, wantErr) {
			t.Fatalf("Write = %v, want %v", gotErr, wantErr)
		}
		if failure && uint64(len(payload)) <= ceiling {
			var gotNative *fs.PathError
			if !errors.As(gotErr, &gotNative) || gotNative != native || !errors.Is(gotErr, fs.ErrPermission) {
				t.Fatalf("Write native error = %v, want exact %v", gotErr, native)
			}
		}
		if _, err := root.Lstat(stagePath.String()); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("temporary entry error = %v, want absent after either outcome", err)
		}
		wantEntries := 2
		if wantErr != nil {
			if got != (filestore.CommitRequest{}) {
				t.Fatalf("refusal receipt = %+v, want zero", got)
			}
			if !replace {
				wantEntries = 1
				if _, err := root.Lstat(targetPath.String()); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("refused target = %v, want absent", err)
				}
			} else {
				retained, err := os.ReadFile(directory + "/target")
				if err != nil || !bytes.Equal(retained, unrelated) {
					t.Fatalf("refused replacement = (%v,%v), want exact previous bytes", retained, err)
				}
			}
		} else {
			if got != (filestore.CommitRequest{}) {
				t.Fatalf("resolved Write recovery request = %+v, want zero after completed activation", got)
			}
			var received bytes.Buffer
			count, err := filestore.Read(t.Context(), filestore.ReadRequest{Destination: &received, Location: filestore.Location{Root: root, Path: targetPath}, MaximumBytes: maximum})
			if err != nil || count.Uint64() != uint64(len(payload)) || !bytes.Equal(received.Bytes(), payload) {
				t.Fatalf("read-back = (%d,%v,%v), want exact %v", count.Uint64(), received.Bytes(), err, payload)
			}
			info, err := root.Lstat(targetPath.String())
			if err != nil {
				t.Fatal(err)
			}
			if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() != int64(len(payload)) {
				t.Fatalf("target = (%v,%d), want exact regular file", info.Mode(), info.Size())
			}
		}
		retained, err := os.ReadFile(directory + "/unrelated")
		if err != nil || !bytes.Equal(retained, unrelated) {
			t.Fatalf("unrelated file = (%v,%v), want preserved %v", retained, err, unrelated)
		}
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != wantEntries {
			t.Fatalf("entries = (%v,%v), want %d exact owned effects", entries, err, wantEntries)
		}
	})
}
