package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestIndexNativeIOFailuresDoNotBecomeMalformedRecords(t *testing.T) {
	t.Parallel()
	for _, closed := range []bool{false, true} {
		t.Run(indexFileLifetimeName(closed), func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			source, err := os.Create(filepath.Join(directory, "source"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if !closed {
					if err := source.Close(); err != nil {
						t.Error(err)
					}
				}
			})
			entry := filestore.ContentIndexEntry{Digest: core.SHA256Of(nil)}
			if err := filestore.WriteContentIndexEntry(source, entry); err != nil {
				t.Fatal(err)
			}
			if _, err := source.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			if closed {
				if err := source.Close(); err != nil {
					t.Fatal(err)
				}
			}
			got, readErr := filestore.ReadContentIndexEntry(source)
			if closed {
				if got != (filestore.ContentIndexEntry{}) || !errors.Is(readErr, core.ErrFilestoreSource) || !errors.Is(readErr, fs.ErrClosed) || errors.Is(readErr, core.ErrFilestoreContract) {
					t.Fatalf("read = (%+v,%v), want zero and source/native identity only", got, readErr)
				}
			} else if readErr != nil || got != entry {
				t.Fatalf("read = (%+v,%v), want exact entry", got, readErr)
			}
			writeErr := filestore.WriteContentIndexEntry(source, entry)
			if closed && (!errors.Is(writeErr, core.ErrFilestoreDestination) || !errors.Is(writeErr, fs.ErrClosed) || errors.Is(writeErr, core.ErrFilestoreContract)) {
				t.Fatalf("write = %v, want destination/native identity only", writeErr)
			}
			if !closed && writeErr != nil {
				t.Fatal(writeErr)
			}
			length, err := core.NewByteLength(1)
			if err != nil {
				t.Fatal(err)
			}
			rangeErr := filestore.WriteRecordRangeIndex(source, lineio.RecordRange{Bytes: length, Framing: lineio.RecordFramingEOF})
			if closed && (!errors.Is(rangeErr, core.ErrFilestoreDestination) || !errors.Is(rangeErr, fs.ErrClosed) || errors.Is(rangeErr, core.ErrFilestoreContract)) {
				t.Fatalf("range write = %v, want destination/native identity only", rangeErr)
			}
			if !closed && rangeErr != nil {
				t.Fatal(rangeErr)
			}
		})
	}
	// No native operation occurs for malformed contracts or a clean empty stream.
	if _, err := filestore.ReadContentIndexEntry(bytes.NewReader(nil)); err != io.EOF {
		t.Fatalf("empty record = %v, want clean EOF", err)
	}
	if err := filestore.WriteContentIndexEntry(nil, filestore.ContentIndexEntry{}); !errors.Is(err, core.ErrFilestoreContract) || errors.Is(err, core.ErrFilestoreDestination) {
		t.Fatalf("nil writer = %v, want request refusal only", err)
	}
}

func indexFileLifetimeName(closed bool) string {
	if closed {
		return "closed native file retains read and write causes"
	}
	return "live native file conserves records"
}
