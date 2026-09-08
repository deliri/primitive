package filestore

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAppendAndLockRegularityGatesSettleOnlyRefusedHandles(t *testing.T) {
	t.Parallel()
	for _, operation := range []struct {
		name         string
		run          func(*os.File) error
		wantBoundary error
	}{
		{name: "append", run: validateAppendFile, wantBoundary: core.ErrFilestoreActivation},
		{name: "lock", run: validateLockFile, wantBoundary: core.ErrFilestoreDestination},
	} {
		t.Run(operation.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name       string
				fixture    custodySyncFixture
				wantNative error
				wantLive   bool
			}{
				{name: "empty regular file remains live", fixture: custodySyncEmpty, wantLive: true},
				{name: "binary regular file remains live and unchanged", fixture: custodySyncWritten, wantLive: true},
				{name: "regularity gate does not invent access-mode policy", fixture: custodySyncReadOnly, wantLive: true},
				{name: "directory is closed before refusal escapes", fixture: custodySyncDirectory, wantNative: fs.ErrInvalid},
				{name: "pipe reader is closed without consuming its peer", fixture: custodySyncPipeRead, wantNative: fs.ErrInvalid},
				{name: "pipe writer is closed without consuming its peer", fixture: custodySyncPipeWrite, wantNative: fs.ErrInvalid},
				{name: "closed file retains native closed cause", fixture: custodySyncClosed, wantNative: fs.ErrClosed},
				{name: "nil file retains native invalid cause", fixture: custodySyncNil, wantNative: fs.ErrInvalid},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					directory := t.TempDir()
					payload := []byte{0, 255, 7}
					var file, peer *os.File
					var err error
					switch tc.fixture {
					case custodySyncEmpty, custodySyncWritten, custodySyncClosed:
						file, err = os.OpenFile(directory+"/file", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
					case custodySyncReadOnly:
						if err := os.WriteFile(directory+"/file", payload, 0o600); err != nil {
							t.Fatal(err)
						}
						file, err = os.Open(directory + "/file")
					case custodySyncDirectory:
						file, err = os.Open(directory)
					case custodySyncPipeRead:
						file, peer, err = os.Pipe()
					case custodySyncPipeWrite:
						peer, file, err = os.Pipe()
					case custodySyncNil:
					default:
						t.Fatalf("fixture = %d, want declared native handle", tc.fixture)
					}
					if err != nil {
						t.Fatal(err)
					}
					if file != nil {
						t.Cleanup(func() { _ = file.Close() })
					}
					if peer != nil {
						t.Cleanup(func() { _ = peer.Close() })
					}
					if tc.fixture == custodySyncWritten {
						if n, err := file.Write(payload); err != nil || n != len(payload) {
							t.Fatalf("native write = (%d,%v), want %d", n, err, len(payload))
						}
					}
					var before fs.FileInfo
					if file != nil {
						before, err = file.Stat()
						if err != nil {
							t.Fatal(err)
						}
					}
					if tc.fixture == custodySyncClosed {
						if err := file.Close(); err != nil {
							t.Fatal(err)
						}
					}
					gotErr := operation.run(file)
					if tc.wantNative == nil {
						if gotErr != nil {
							t.Fatalf("regularity admission = %v, want nil", gotErr)
						}
					} else if !errors.Is(gotErr, operation.wantBoundary) || !errors.Is(gotErr, tc.wantNative) {
						t.Fatalf("regularity refusal = %v, want %v and native %v", gotErr, operation.wantBoundary, tc.wantNative)
					}
					after, afterErr := file.Stat()
					if tc.wantLive {
						if afterErr != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
							t.Fatalf("retained handle = (%v,%v), want original metadata %v", after, afterErr, before)
						}
					} else {
						wantClosed := fs.ErrClosed
						if file == nil {
							wantClosed = fs.ErrInvalid
						}
						if !errors.Is(afterErr, wantClosed) {
							t.Fatalf("refused handle = %v, want %v", afterErr, wantClosed)
						}
					}
					if peer != nil {
						if _, err := peer.Stat(); err != nil {
							t.Fatalf("unowned peer = %v, want still live", err)
						}
					}
					if tc.fixture == custodySyncWritten || tc.fixture == custodySyncReadOnly {
						got, err := os.ReadFile(directory + "/file")
						if err != nil || !bytes.Equal(got, payload) {
							t.Fatalf("retained bytes = (%v,%v), want %v", got, err, payload)
						}
					}
				})
			}
		})
	}
}
