package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Two independently encoded records plus a replay pressure the typed seam.
// The oracle compares exact records; it does not call the sorter's comparator.
func FuzzContentSortExactUnion(f *testing.F) {
	f.Add([]byte("first"), []byte("second"), uint64(1), uint64(2), false)
	f.Add([]byte("same"), []byte("same"), uint64(3), uint64(4), false)
	f.Add([]byte("same"), []byte("same"), uint64(3), uint64(3), true)
	f.Add([]byte("first"), []byte("second"), uint64(math.MaxInt64), uint64(1), true)
	f.Fuzz(func(t *testing.T, a, b []byte, firstSize, secondSize uint64, reverse bool) {
		dir := t.TempDir()
		var files [2]*os.File
		for i, name := range [2]string{"input", "scratch"} {
			file, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			files[i] = file
			t.Cleanup(func() {
				if err := file.Close(); err != nil {
					t.Errorf("Close() error = %v, want nil", err)
				}
			})
		}
		firstExtent, err := core.NewByteLength(firstSize)
		if err != nil {
			if !errors.Is(err, core.ErrNumericOverflow) || firstSize <= math.MaxInt64 || firstExtent != (core.ByteLength{}) {
				t.Fatalf("first extent = (%v,%v), want zero refusal outside signed file extent", firstExtent, err)
			}
			return
		}
		secondExtent, err := core.NewByteLength(secondSize)
		if err != nil {
			if !errors.Is(err, core.ErrNumericOverflow) || secondSize <= math.MaxInt64 || secondExtent != (core.ByteLength{}) {
				t.Fatalf("second extent = (%v,%v), want zero refusal outside signed file extent", secondExtent, err)
			}
			return
		}
		entries := [2]filestore.ContentIndexEntry{
			{Digest: core.SHA256Of(a), Extent: firstExtent},
			{Digest: core.SHA256Of(b), Extent: secondExtent},
		}
		// A zero extent with a nonempty content digest is refused by the record
		// producer and must not enter sorting as a made-up admitted fact.
		for _, entry := range entries {
			if err := entry.Validate(); err != nil {
				var rejected bytes.Buffer
				gotErr := filestore.WriteContentIndexEntry(&rejected, entry)
				if !errors.Is(gotErr, core.ErrFilestoreContract) || rejected.Len() != 0 {
					t.Fatalf("WriteContentIndexEntry(invalid) = (%v,%d bytes), want typed refusal and no bytes", gotErr, rejected.Len())
				}
				return
			}
		}
		order := [3]int{0, 1, 0}
		if reverse {
			order = [3]int{1, 0, 1}
		}
		for _, i := range order {
			if err := filestore.WriteContentIndexEntry(files[0], entries[i]); err != nil {
				t.Fatal(err)
			}
		}
		var output bytes.Buffer
		got, gotErr := filestore.SortContentIndex(t.Context(), filestore.ContentSortRequest{Files: files, Destination: &output})
		same := entries[0].Digest == entries[1].Digest
		if same && firstSize != secondSize {
			var conflict filestore.ContentIndexConflictError
			if !errors.As(gotErr, &conflict) || conflict.Digest != entries[0].Digest || conflict.First.Uint64() != min(firstSize, secondSize) || conflict.Second.Uint64() != max(firstSize, secondSize) || conflict.Validate() != nil || !errors.Is(conflict.Unwrap(), core.ErrFilestoreContract) || conflict.Error() == "" || got != (filestore.ContentIndexSummary{}) || output.Len() != 0 {
				t.Fatalf("conflicting union = (%+v,%v,%d bytes), want exact typed conflict and no output", got, gotErr, output.Len())
			}
			return
		}
		if !same && firstSize > math.MaxInt64-secondSize {
			if !errors.Is(gotErr, core.ErrFilestoreContract) || got != (filestore.ContentIndexSummary{}) || output.Len() != 0 {
				t.Fatalf("overflow union = (%+v,%v,%d bytes), want typed refusal and no output", got, gotErr, output.Len())
			}
			return
		}
		wantCount, wantSize := uint64(2), firstSize+secondSize
		if same {
			wantCount, wantSize = 1, firstSize
		}
		if gotErr != nil || got.Entries != wantCount || got.Extent.Uint64() != wantSize {
			t.Fatalf("union = (%+v,%v), want %d entries/%d bytes and nil", got, gotErr, wantCount, wantSize)
		}
		left, err := entries[0].Digest.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		right, err := entries[1].Digest.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Compare(left[:], right[:]) > 0 {
			entries[0], entries[1] = entries[1], entries[0]
		}
		for i := uint64(0); i < wantCount; i++ {
			entry, err := filestore.ReadContentIndexEntry(&output)
			if err != nil || entry != entries[i] {
				t.Fatalf("union record %d = (%+v,%v), want (%+v,nil)", i, entry, err, entries[i])
			}
		}
		entry, err := filestore.ReadContentIndexEntry(&output)
		if !errors.Is(err, io.EOF) || entry != (filestore.ContentIndexEntry{}) {
			t.Fatalf("union suffix = (%+v,%v), want zero and EOF", entry, err)
		}
	})
}
