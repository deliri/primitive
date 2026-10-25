package filestore_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzReadAtMatchesBorrowedNativeFile(f *testing.F) {
	seed, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed, int64(0), uint16(1))
	f.Add([]byte{}, int64(0), uint16(1))
	f.Add([]byte{0, 255, 31}, int64(-1), uint16(4))
	f.Fuzz(func(t *testing.T, payload []byte, offset int64, window uint16) {
		directory := t.TempDir()
		path := filepath.Join(directory, "source")
		if err := os.WriteFile(path, payload, 0o600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := file.Seek(1, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		got := bytes.Repeat([]byte{19}, int(window))
		want := bytes.Clone(got)
		wantCount, wantErr := file.ReadAt(want, offset)
		gotCount, gotErr := filestore.ReadAt(t.Context(), filestore.ReadAtRequest{File: file, Data: got, Offset: offset})
		if gotCount != wantCount || !bytes.Equal(got, want) || !sameReadAtNativeError(gotErr, wantErr, offset) {
			t.Fatalf("ReadAt = (%d,%v,%v), want native (%d,%v,%v)", gotCount, got, gotErr, wantCount, want, wantErr)
		}
		position, err := file.Seek(0, io.SeekCurrent)
		if position != 1 || err != nil {
			t.Fatalf("borrowed cursor = (%d,%v), want (1,nil)", position, err)
		}
	})
}
