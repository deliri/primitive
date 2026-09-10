package runworkspace_test

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/runworkspace"
	"github.com/deliri/primitive/v2026/temporal"
)

// These finite fixture widths cross native directory batches. Scrub must drain
// the complete owned namespace while its callback removes entries during Walk.
func TestScrubNativeBatchContinuationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name              string
		width             int
		nested, cancelled bool
		wantErr           error
	}{
		{name: "empty owned root remains empty"},
		{name: "one below batch preserves complete removal", width: 63},
		{name: "exact batch preserves complete removal", width: 64},
		{name: "first entry after batch cannot survive", width: 65},
		{name: "third partial batch cannot survive", width: 129},
		{name: "removing nested trees cannot hide the next root batch", width: 65, nested: true},
		{name: "cancellation preserves every entry across multiple batches", width: 129, cancelled: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path, err := core.ParseAbsolutePath(directory)
			if err != nil {
				t.Fatal(err)
			}
			root, err := filestore.OpenRoot(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			manager, err := runworkspace.Open(t.Context(), runworkspace.Configuration{RunParent: path})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := manager.Close(); err != nil {
					t.Error(err)
				}
			})
			payload := []byte{0, 255, 1, '\n'}
			var scratch [4]byte
			paths := make([]core.RelativePath, 0, tc.width)
			for index := range tc.width {
				entry, err := core.ParseRelativePath("entry-" + strconv.Itoa(index))
				if err != nil {
					t.Fatal(err)
				}
				if tc.nested {
					if err := filestore.EnsureDirectory(t.Context(), filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: entry}, Mode: 0o700}); err != nil {
						t.Fatal(err)
					}
					entry, err = entry.Resolve("child")
					if err != nil {
						t.Fatal(err)
					}
				}
				temporary, err := core.ParseRelativePath(entry.String() + ".temporary")
				if err != nil {
					t.Fatal(err)
				}
				recovery, err := filestore.Write(t.Context(), filestore.WriteRequest{Source: bytes.NewReader(payload), Buffer: scratch[:], Location: filestore.Location{Root: root, Path: entry}, Temporary: temporary, Mode: 0o600, Install: filestore.InstallCreate})
				if err != nil || recovery != (filestore.CommitRequest{}) {
					t.Fatalf("fixture write = (%v,%v), want settled exact file", recovery, err)
				}
				paths = append(paths, entry)
			}
			wantBefore := uint32(tc.width)
			if tc.nested {
				wantBefore *= 2
			}
			before, err := manager.Observe(t.Context(), temporal.InstantFromNanoseconds(1), runworkspace.Residue{})
			if err != nil || before.Entries != wantBefore {
				t.Fatalf("before scrub = (%v,%v), want %d entries", before, err, wantBefore)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			gotErr := manager.Scrub(ctx)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Scrub = %v, want %v", gotErr, tc.wantErr)
			}
			wantAfter := uint32(0)
			if tc.cancelled {
				wantAfter = wantBefore
			}
			after, err := manager.Observe(t.Context(), temporal.InstantFromNanoseconds(2), runworkspace.Residue{})
			if err != nil || after.Entries != wantAfter || after.IsClean() != (wantAfter == 0) {
				t.Fatalf("after scrub = (%v,%v), want %d entries with matching clean state", after, err, wantAfter)
			}
			if tc.cancelled {
				for _, entry := range paths {
					var received bytes.Buffer
					count, err := filestore.Read(t.Context(), filestore.ReadRequest{Location: filestore.Location{Root: root, Path: entry}, Destination: &received, Buffer: scratch[:]})
					if err != nil || count.Uint64() != uint64(len(payload)) || !bytes.Equal(received.Bytes(), payload) {
						t.Fatalf("cancelled scrub retained file = (%d,%v,%v), want exact %v", count.Uint64(), received.Bytes(), err, payload)
					}
				}
			}
		})
	}
}
