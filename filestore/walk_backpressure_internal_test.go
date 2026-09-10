package filestore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Closing the native producer after first delivery proves Walk did not read
// the whole directory before invoking its consumer. The observed next read
// must retain the closed-handle cause; cancellation stops inside a batch.
func TestWalkBatchBackpressureLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                 string
		count, closeAt, cancelAt, wantVisits int
		closed, cancelled                    bool
		wantErr                              error
	}{
		{name: "empty producer emits nothing"},
		{name: "partial second batch delivers every entry", count: walkDirectoryBatchEntries + 1, wantVisits: walkDirectoryBatchEntries + 1},
		{name: "closed producer retains native failure without delivery", count: 1, closed: true, wantErr: core.ErrFilestoreSource},
		{name: "cancelled producer is never read", count: 1, cancelled: true, wantErr: context.Canceled},
		{name: "first delivery precedes reading the second batch", count: walkDirectoryBatchEntries + 1, closeAt: 1, wantVisits: walkDirectoryBatchEntries, wantErr: core.ErrFilestoreSource},
		{name: "closing at a batch boundary cannot deliver a later prefix", count: 2*walkDirectoryBatchEntries + 1, closeAt: walkDirectoryBatchEntries, wantVisits: walkDirectoryBatchEntries, wantErr: core.ErrFilestoreSource},
		{name: "cancellation stops delivery inside the already-read batch", count: walkDirectoryBatchEntries + 1, cancelAt: 1, wantVisits: 1, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			for i := range tc.count {
				if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("entry-%03d", i)), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			file, err := os.Open(directory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
					t.Error(err)
				}
			})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			if tc.closed {
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			path, err := core.ParseRelativePath(".")
			if err != nil {
				t.Fatal(err)
			}
			var got []core.RelativePath
			request := WalkRequest{Visit: func(entry WalkEntry) (WalkDirective, error) {
				if err := entry.Validate(); err != nil {
					return WalkDirectiveUnknown, err
				}
				if slices.Contains(got, entry.Path) {
					return WalkDirectiveUnknown, core.ErrFilestoreContract
				}
				got = append(got, entry.Path)
				if len(got) == tc.closeAt {
					if err := file.Close(); err != nil {
						return WalkDirectiveUnknown, err
					}
				}
				if len(got) == tc.cancelAt {
					cancel()
				}
				return WalkContinue, nil
			}}
			gotErr := readDirectoryEntries(readDirectoryInput{ctx: ctx, directory: file, directoryPath: path, request: request})
			if !errors.Is(gotErr, tc.wantErr) || len(got) != tc.wantVisits {
				t.Fatalf("delivery = (%d,%v), want (%d,%v)", len(got), gotErr, tc.wantVisits, tc.wantErr)
			}
			if errors.Is(tc.wantErr, core.ErrFilestoreSource) {
				_, nativeErr := file.ReadDir(1)
				native, nativeOK := errors.AsType[*fs.PathError](nativeErr)
				observed, observedOK := errors.AsType[*fs.PathError](gotErr)
				if !nativeOK || !observedOK || !errors.Is(gotErr, native.Err) || observed.Op != native.Op || observed.Path != native.Path {
					t.Fatalf("source error = %v, want exact native closed-directory cause %v", gotErr, nativeErr)
				}
			}
			for _, path := range got {
				info, err := os.Stat(filepath.Join(directory, path.String()))
				if err != nil || !info.Mode().IsRegular() || info.Size() != 0 {
					t.Fatalf("delivered path = (%v,%v), want retained empty fixture", info, err)
				}
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != tc.count {
				t.Fatalf("namespace = (%d,%v), want %d retained entries", len(entries), err, tc.count)
			}
		})
	}
}
