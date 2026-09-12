package filestore

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func contentEntryForTest(t testing.TB, ordinal, size uint64) ContentIndexEntry {
	t.Helper()
	var raw [core.SHA256DigestBytes]byte
	binary.BigEndian.PutUint64(raw[len(raw)-8:], ordinal)
	extent, err := core.NewByteLength(size)
	if err != nil {
		t.Fatalf("NewByteLength(%d) error = %v, want nil", size, err)
	}
	return ContentIndexEntry{Digest: core.NewSHA256Digest(raw), Extent: extent}
}

func contentScratchForTest(t testing.TB, dir string) [2]*os.File {
	t.Helper()
	var files [2]*os.File
	for i, name := range [2]string{"first", "second"} {
		file, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("Create(%s) error = %v, want nil", name, err)
		}
		files[i] = file
		t.Cleanup(func() {
			if err := file.Close(); err != nil {
				t.Errorf("Close(%s) error = %v, want nil", name, err)
			}
		})
	}
	return files
}

func TestContentSortLayerTriadExactStream(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		count    int
		conflict bool
	}{
		{name: "empty input emits no record"},
		{name: "one record survives", count: 1},
		{name: "129 records exceed former product ceiling", count: 129},
		{name: "exact initial run boundary", count: contentSortRunEntries},
		{name: "one beyond initial run requires merge", count: contentSortRunEntries + 1},
		{name: "three runs require successive merge passes", count: 2*contentSortRunEntries + 1},
		{name: "conflicting duplicate refuses before emission", count: 2, conflict: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			files := contentScratchForTest(t, dir)
			for i := tc.count; i > 0; i-- {
				entry := contentEntryForTest(t, uint64(i), 1)
				if tc.conflict {
					entry = contentEntryForTest(t, 1, uint64(i))
				}
				if err := WriteContentIndexEntry(files[0], entry); err != nil {
					t.Fatalf("WriteContentIndexEntry() error = %v, want nil", err)
				}
				if err := WriteContentIndexEntry(files[0], entry); err != nil {
					t.Fatalf("WriteContentIndexEntry(duplicate) error = %v, want nil", err)
				}
			}
			var output bytes.Buffer
			got, err := SortContentIndex(t.Context(), ContentSortRequest{Files: files, Destination: &output})
			if tc.conflict {
				var conflict ContentIndexConflictError
				if !errors.As(err, &conflict) || conflict.First.Uint64() != 1 || conflict.Second.Uint64() != 2 || got != (ContentIndexSummary{}) || output.Len() != 0 {
					t.Fatalf("SortContentIndex(conflict) = (%+v, %v, %d bytes), want zero summary, typed 1/2 conflict and no output", got, err, output.Len())
				}
				return
			}
			if err != nil || got.Entries != uint64(tc.count) || got.Extent.Uint64() != uint64(tc.count) {
				t.Fatalf("SortContentIndex() = (%+v,%v), want %d unique one-byte entries and nil", got, err, tc.count)
			}
			for i := 1; i <= tc.count; i++ {
				entry, err := ReadContentIndexEntry(&output)
				want := contentEntryForTest(t, uint64(i), 1)
				if err != nil || entry != want {
					t.Fatalf("ReadContentIndexEntry(%d) = (%+v,%v), want (%+v,nil)", i, entry, err, want)
				}
			}
			entry, err := ReadContentIndexEntry(&output)
			if !errors.Is(err, io.EOF) || entry != (ContentIndexEntry{}) {
				t.Fatalf("ReadContentIndexEntry(end) = (%+v,%v), want zero and EOF", entry, err)
			}
			for _, file := range files {
				if _, err := file.Stat(); err != nil {
					t.Fatalf("borrowed scratch Stat() error = %v, want nil", err)
				}
			}
		})
	}
}

func TestContentMergeRefusesMissingCompleteRecord(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	files := contentScratchForTest(t, dir)
	if err := WriteContentIndexEntry(files[0], contentEntryForTest(t, 1, 1)); err != nil {
		t.Fatal(err)
	}
	err := mergeContentRuns(t.Context(), files[0], files[1], 2, 1)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("mergeContentRuns(missing record) error = %v, want %v", err, io.ErrUnexpectedEOF)
	}
}

func BenchmarkContentIndexRecordAdmission(b *testing.B) {
	entry := contentEntryForTest(b, 17, 23)
	var encoded bytes.Buffer
	if err := WriteContentIndexEntry(&encoded, entry); err != nil {
		b.Fatal(err)
	}
	data := encoded.Bytes()
	if len(data) != ContentIndexRecordBytes {
		b.Fatalf("encoded record bytes = %d, want %d", len(data), ContentIndexRecordBytes)
	}
	var reader bytes.Reader
	var got ContentIndexEntry
	b.ReportAllocs()
	b.SetBytes(ContentIndexRecordBytes)
	for b.Loop() {
		reader.Reset(data)
		var err error
		got, err = ReadContentIndexEntry(&reader)
		if err != nil {
			b.Fatalf("ReadContentIndexEntry() error = %v, want nil", err)
		}
	}
	if got != entry {
		b.Fatalf("ReadContentIndexEntry() = %+v, want %+v", got, entry)
	}
}
