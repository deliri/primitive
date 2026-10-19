package filestore_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzNativeFileHandleCustody(f *testing.F) {
	f.Add(uint8(0), []byte("native bytes"))
	f.Add(uint8(1), []byte{})
	f.Add(uint8(2), []byte{0, 255})
	f.Fuzz(func(t *testing.T, mode uint8, body []byte) {
		file, err := os.CreateTemp(t.TempDir(), "native")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := file.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
				t.Error(err)
			}
		}()
		if _, err := file.Write(body); err != nil {
			t.Fatal(err)
		}
		request := filestore.FileHandleRequest{File: file}
		switch mode % 3 {
		case 0:
			if err := filestore.SyncFile(t.Context(), request); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Stat(); err != nil {
				t.Fatalf("synchronized handle=%v, want retained native ownership", err)
			}
		case 1:
			if err := filestore.CloseFile(request); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Stat(); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("closed handle=%v, want ErrClosed", err)
			}
		case 2:
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			if err := filestore.SyncFile(t.Context(), request); !errors.Is(err, fs.ErrClosed) || !errors.Is(err, core.ErrFilestoreDestination) {
				t.Fatalf("closed sync=%v, want native refusal", err)
			}
		}
		got, err := os.ReadFile(file.Name())
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("native retained bytes=(%d,%v), want exact %d-byte body", len(got), err, len(body))
		}
	})
}
