package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Namespace substitution and receipt extent/mode are covered by their full
// cross-effect matrices. This table pins admission and the returned Go handle.
func TestOpenStagedReadLayerTriad(t *testing.T) {
	t.Parallel()
	type fault uint8
	const (
		none fault = iota
		nilContext
		canceled
		zeroReceipt
		closedRoot
	)
	for _, tc := range []struct {
		name                string
		size                int
		fault               fault
		wantErr, wantNative error
	}{
		{name: "empty receipt opens an actual empty inode"},
		{name: "buffer crossing returns every staged byte", size: (32 << 10) + 1},
		{name: "nil context cannot expose a handle", size: 1, fault: nilContext, wantErr: core.ErrNilContext},
		{name: "canceled context retains stage custody", size: 1, fault: canceled, wantErr: context.Canceled},
		{name: "zero receipt cannot name a file", size: 1, fault: zeroReceipt, wantErr: core.ErrFilestoreContract},
		{name: "closed root retains native custody refusal", size: 1, fault: closedRoot, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := deterministicPayload(tc.size)
			staged, err := filestore.Stage(t.Context(), filestore.StageRequest{Source: bytes.NewReader(payload), Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600, MaximumBytes: mustByteCount(t, uint64(max(1, len(payload))))})
			if err != nil {
				t.Fatal(err)
			}
			before, err := root.Lstat("stage")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch tc.fault {
			case nilContext:
				ctx = nil
			case canceled:
				cancel()
			case zeroReceipt:
				staged = filestore.StagedFile{}
			case closedRoot:
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			}
			file, gotErr := filestore.OpenStagedRead(ctx, staged)
			if file != nil {
				t.Cleanup(func() {
					if err := file.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) || (file != nil) != (tc.wantErr == nil) {
				t.Fatalf("OpenStagedRead = (%v,%v), want live=%t, %v and %v", file, gotErr, tc.wantErr == nil, tc.wantErr, tc.wantNative)
			}
			for _, class := range []error{core.ErrFilestoreContract, core.ErrFilestoreActivation, core.ErrFilestoreActivationIndeterminate, core.ErrFilestoreSource, core.ErrFilestoreCleanup} {
				if errors.Is(gotErr, class) != errors.Is(tc.wantErr, class) {
					t.Fatalf("error = %v, want exactly %v hierarchy", gotErr, tc.wantErr)
				}
			}
			if file != nil {
				info, err := file.Stat()
				if err != nil || !os.SameFile(before, info) {
					t.Fatalf("opened inode = (%v,%v), want exact staged inode", info, err)
				}
				got, err := io.ReadAll(file)
				if err != nil || !bytes.Equal(got, payload) {
					t.Fatalf("opened bytes = (%v,%v), want %v", got, err, payload)
				}
			}
			after, err := os.Lstat(filepath.Join(directory, "stage"))
			data, readErr := os.ReadFile(filepath.Join(directory, "stage"))
			entries, entriesErr := os.ReadDir(directory)
			if err != nil || readErr != nil || entriesErr != nil || len(entries) != 1 || entries[0].Name() != "stage" || !bytes.Equal(data, payload) || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("retained stage = (%v,%v,%v,%v), want exact original namespace, bytes and metadata", after, err, readErr, entriesErr)
			}
		})
	}
}
