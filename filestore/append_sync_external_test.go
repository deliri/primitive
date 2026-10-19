package filestore_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzAppendSynchronizationReceiptsMatchNativeBytes(f *testing.F) {
	f.Add([]byte{}, []byte{}, false)
	f.Add([]byte("first\n"), []byte("second\n"), false)
	f.Add([]byte("discarded\n"), []byte("replacement\n"), true)
	f.Fuzz(func(t *testing.T, first, second []byte, truncate bool) {
		dir := t.TempDir()
		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		path, err := core.ParseRelativePath("native")
		if err != nil {
			t.Fatal(err)
		}
		d, err := filestore.OpenAppendDestination(t.Context(), filestore.AppendRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o600, Append: filestore.AppendCreate})
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		if n, err := d.Write(first); n != len(first) || err != nil {
			t.Fatalf("first write=%d/%v", n, err)
		}
		one, err := filestore.SyncAppend(t.Context(), filestore.AppendSyncRequest{Destination: d})
		if err != nil || one.Validate() != nil || one.AcceptedBytes().Uint64() != uint64(len(first)) {
			t.Fatalf("first receipt=%+v/%v", one, err)
		}
		observed, err := os.ReadFile(filepath.Join(dir, "native"))
		if err != nil || !bytes.Equal(observed, first) {
			t.Fatalf("first native bytes=%q/%v, want %q", observed, err, first)
		}
		if truncate {
			if err := d.Truncate(t.Context(), core.ByteLength{}); err != nil {
				t.Fatal(err)
			}
		}
		if n, err := d.Write(second); n != len(second) || err != nil {
			t.Fatalf("second write=%d/%v", n, err)
		}
		two, err := filestore.SyncAppend(t.Context(), filestore.AppendSyncRequest{Destination: d})
		if err != nil || two.Validate() != nil || two.AcceptedBytes().Uint64() != uint64(len(first))+uint64(len(second)) {
			t.Fatalf("second receipt=%+v/%v", two, err)
		}
		want := append(bytes.Clone(first), second...)
		if truncate {
			want = bytes.Clone(second)
		}
		observed, err = os.ReadFile(filepath.Join(dir, "native"))
		if err != nil || !bytes.Equal(observed, want) {
			t.Fatalf("second native bytes=%q/%v, want %q", observed, err, want)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		if one.Validate() != nil || one.AcceptedBytes().Uint64() != uint64(len(first)) {
			t.Fatalf("later effects changed immutable first receipt=%+v", one)
		}
	})
}
