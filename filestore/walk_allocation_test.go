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

// Allocation bound regression: a sparse directory must not reserve its entire
// permitted population. Existing public walk tests prove ordering and refusal
// at the population ceiling; this test pins the previously unbounded slack.
func TestLexicalDirectoryAllocationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name              string
		count             int
		closed, cancelled bool
		wantErr           error
	}{
		{name: "neutral empty directory reserves only bounded slack"},
		{name: "positive sparse directory does not reserve the configured ceiling", count: 1},
		{name: "one below initial batch preserves every entry", count: walkDirectoryBatchEntries - 1},
		{name: "exact initial batch preserves every entry", count: walkDirectoryBatchEntries},
		{name: "one above initial batch grows with actual entries", count: walkDirectoryBatchEntries + 1},
		{name: "multiple full batches retain every distinct source entry", count: 3 * walkDirectoryBatchEntries},
		{name: "closed input retains source identity with no entries", count: 1, closed: true, wantErr: core.ErrFilestoreSource},
		{name: "cancelled input returns no entries", count: 1, cancelled: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			var wantNames []string
			for index := range tc.count {
				name := fmt.Sprintf("file_%03d", index)
				wantNames = append(wantNames, name)
				if err := os.WriteFile(filepath.Join(directory, name), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			input, err := os.Open(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if tc.closed {
					return
				}
				if err := input.Close(); err != nil {
					t.Error(err)
				}
			}()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			var nativeRefusal *fs.PathError
			if tc.closed {
				if err := input.Close(); err != nil {
					t.Fatal(err)
				}
				_, nativeErr := input.ReadDir(1)
				if !errors.As(nativeErr, &nativeRefusal) || nativeRefusal.Err == nil {
					t.Fatalf("closed standard-library read = %T/%v, want native path refusal", nativeErr, nativeErr)
				}
			}
			entries, err := readLexicalDirectoryBatch(readDirectoryInput{ctx: ctx, directory: input}, int(DirectoryEntryMaximumLimit))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("directory error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if entries != nil {
					t.Fatalf("rejected entries = %v, want nil", entries)
				}
				var pathError *fs.PathError
				if tc.closed && (!errors.As(err, &pathError) || !errors.Is(err, nativeRefusal.Err) || pathError.Path != nativeRefusal.Path || pathError.Op != nativeRefusal.Op) {
					t.Fatalf("closed input error = %v, want preserved filesystem path error", err)
				}
				return
			}
			if len(entries) != tc.count {
				t.Fatalf("directory entries = %d, want %d", len(entries), tc.count)
			}
			var gotNames []string
			for _, entry := range entries {
				gotNames = append(gotNames, entry.Name())
			}
			slices.Sort(gotNames)
			if !slices.Equal(gotNames, wantNames) {
				t.Fatalf("directory identities = %q, want exact fixture permutation %q", gotNames, wantNames)
			}
			// Allow Go's append growth and allocator size-class rounding. The
			// bound follows actual entries, never the configured 65,536 ceiling.
			if cap(entries) > 4*max(tc.count, walkDirectoryBatchEntries) {
				t.Fatalf("directory capacity = %d for %d entries, want bounded growth", cap(entries), tc.count)
			}
		})
	}
}

func BenchmarkLexicalSparseDirectory(b *testing.B) {
	directory := b.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "entry"), nil, 0o600); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		input, err := os.Open(directory)
		if err != nil {
			b.Fatal(err)
		}
		entries, err := readLexicalDirectoryBatch(readDirectoryInput{ctx: b.Context(), directory: input}, int(DirectoryEntryMaximumLimit))
		closeErr := input.Close()
		if err != nil || closeErr != nil || len(entries) != 1 || entries[0].Name() != "entry" || !entries[0].Type().IsRegular() {
			b.Fatalf("directory = %d/%v/%v, want one entry", len(entries), err, closeErr)
		}
	}
}

// Public Walk must not expose the smaller I/O batches as sorted prefixes.
// A later batch can contain the lexical first name or exceed the request's
// ceiling. Both require the entire directory decision before any delivery.
func TestLexicalBatchDeliveryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		count   int
		maximum uint32
		wantErr error
	}{
		{name: "empty input emits no invented entry", maximum: DirectoryEntryMaximumLimit},
		{name: "last native batch can contain lexical first entry", count: walkDirectoryBatchEntries + 1, maximum: DirectoryEntryMaximumLimit},
		{name: "one below two-batch ceiling emits complete order", count: 2*walkDirectoryBatchEntries - 1, maximum: 2 * walkDirectoryBatchEntries},
		{name: "exact two-batch ceiling is admitted without requiring spare capacity", count: 2 * walkDirectoryBatchEntries, maximum: 2 * walkDirectoryBatchEntries},
		{name: "one above two-batch ceiling rejects before delivering any prefix", count: 2*walkDirectoryBatchEntries + 1, maximum: 2 * walkDirectoryBatchEntries, wantErr: core.ErrFilestoreContract},
		{name: "caller ceiling smaller than I/O batch cannot leak a prefix", count: 2, maximum: 1, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			// Reverse creation order is adversarial where the OS preserves it;
			// the oracle never assumes native enumeration order on any OS.
			for index := range tc.count {
				name := fmt.Sprintf("entry-%03d", tc.count-1-index)
				if err := os.WriteFile(filepath.Join(directory, name), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			rootPath, err := core.ParseRelativePath(".")
			if err != nil {
				t.Fatal(err)
			}
			var want []core.RelativePath
			for index := range tc.count {
				path, err := core.ParseRelativePath(fmt.Sprintf("entry-%03d", index))
				if err != nil {
					t.Fatal(err)
				}
				want = append(want, path)
			}
			ceiling, err := NewDirectoryEntryMaximum(DirectoryEntryMaximumLimit)
			if err != nil {
				t.Fatal(err)
			}
			var got []core.RelativePath
			request := WalkRequest{Location: Location{Root: root, Path: rootPath}, Order: WalkOrderLexical, DirectoryEntryMaximum: ceiling, Visit: func(entry WalkEntry) (WalkDirective, error) {
				if err := entry.Validate(); err != nil {
					return WalkDirectiveUnknown, err
				}
				got = append(got, entry.Path)
				return WalkContinue, nil
			}}
			if err := Walk(t.Context(), request); err != nil || !slices.Equal(got, want) {
				t.Fatalf("baseline directory = %v/%v, want exact ordered identities %v", got, err, want)
			}
			request.DirectoryEntryMaximum, err = NewDirectoryEntryMaximum(tc.maximum)
			if err != nil {
				t.Fatal(err)
			}
			got = nil
			gotErr := Walk(t.Context(), request)
			if tc.wantErr != nil {
				want = nil
			}
			if !errors.Is(gotErr, tc.wantErr) || !slices.Equal(got, want) {
				t.Fatalf("directory at ceiling %d = %v/%v, want %v/%v", tc.maximum, got, gotErr, want, tc.wantErr)
			}
		})
	}
}
