package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestAppendDestinationConservesBytesAcrossFlushSyncRotationAndClose(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 4095, 4096, 4097, 32769, 4<<20 + 17} {
		t.Run(fmt.Sprintf("bytes_%d", size), func(t *testing.T) {
			t.Parallel()
			proveAppendDestinationBytes(t, bytes.Repeat([]byte{0, 255, 17}, size/3), size%3)
		})
	}
}

// Each independently read file is the oracle, rather than the owner's counts.
func proveAppendDestinationBytes(t *testing.T, payload []byte, suffix int) {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	first := filestore.AppendRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "first")}, Mode: 0o600, Append: filestore.AppendCreate}
	d, err := filestore.OpenAppendDestination(t.Context(), first)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	inspection, err := d.Inspect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	extent, err := inspection.SizeBytes()
	if err != nil || extent.Uint64() != 0 {
		t.Fatalf("initial extent = %v,%v", extent, err)
	}
	absolute, err := core.ParseAbsolutePath(dir + "/first")
	if err != nil {
		t.Fatal(err)
	}
	if standing, err := d.ObserveStanding(t.Context(), absolute); err != nil || standing != filestore.HeldStandingSame {
		t.Fatalf("initial standing = %v,%v", standing, err)
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	// Acquisition borrows the root; the live destination owns its native inode.
	if _, err := d.Write([]byte("discard")); err != nil {
		t.Fatal(err)
	}
	if n, err := d.Buffered(); err != nil || n != 7 {
		t.Fatalf("buffered = %d,%v, want 7", n, err)
	}
	if n, err := d.DiscardBuffered(); err != nil || n != 7 {
		t.Fatalf("discarded = %d,%v, want 7", n, err)
	}
	if _, err := d.Write([]byte("filler")); err != nil {
		t.Fatal(err)
	}
	if err := d.Truncate(t.Context(), core.ByteLength{}); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(dir + "/first"); err != nil || len(got) != 0 {
		t.Fatalf("truncated bytes = %q,%v", got, err)
	}
	for _, chunk := range [][]byte{payload, bytes.Repeat([]byte{29}, suffix)} {
		if n, err := d.Write(chunk); err != nil || n != len(chunk) {
			t.Fatalf("write = %d,%v, want %d", n, err, len(chunk))
		}
	}
	if err := d.Flush(); err != nil {
		t.Fatal(err)
	}
	want := append(bytes.Clone(payload), bytes.Repeat([]byte{29}, suffix)...)
	got, err := os.ReadFile(dir + "/first")
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("flushed file = %d bytes,%v, want %d exact bytes", len(got), err, len(want))
	}
	if err := d.Sync(); err != nil {
		t.Fatal(err)
	}
	root, err = os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	second := first
	second.Location = filestore.Location{Root: root, Path: mustRelativePath(t, "second")}
	if _, err := d.Write([]byte("tail")); err != nil {
		t.Fatal(err)
	}
	if err := d.Rotate(t.Context(), second); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write([]byte("incoming")); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write([]byte("zombie")); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("settled write = %v", err)
	}
	got, err = os.ReadFile(dir + "/first")
	if err != nil || !bytes.Equal(got, append(want, []byte("tail")...)) {
		t.Fatalf("outgoing lost buffered tail: %d bytes,%v", len(got), err)
	}
	got, err = os.ReadFile(dir + "/second")
	if err != nil || string(got) != "incoming" {
		t.Fatalf("incoming = %q,%v", got, err)
	}
}

func TestAppendDestinationObservesReplacedNamesAndNativeExtent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	d, err := filestore.OpenAppendDestination(t.Context(), filestore.AppendRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "subject")}, Mode: 0o600, Append: filestore.AppendCreate})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.Write([]byte("owned")); err != nil {
		t.Fatal(err)
	}
	if err := d.Sync(); err != nil {
		t.Fatal(err)
	}
	i, err := d.Inspect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	n, err := i.SizeBytes()
	if err != nil || n.Uint64() != 5 {
		t.Fatalf("native extent=%v,%v", n, err)
	}
	if err := os.Rename(dir+"/subject", dir+"/old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/subject", []byte("foreign"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := core.ParseAbsolutePath(dir + "/subject")
	if err != nil {
		t.Fatal(err)
	}
	standing, err := d.ObserveStanding(t.Context(), path)
	if err != nil || standing == filestore.HeldStandingSame {
		t.Fatalf("replaced name claimed owned=%v,%v", standing, err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Inspect(t.Context()); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("settled inspection=%v", err)
	}
	if got, err := d.ObserveStanding(t.Context(), path); got != filestore.HeldStandingUnknown || !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("settled standing=%v,%v", got, err)
	}
}

func TestAppendDestinationRefusalPreservesOrSettlesTheCorrectOwner(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		invalid   bool
		canceled  bool
		collision bool
	}{
		{name: "invalid intent", invalid: true}, {name: "caller cancellation", canceled: true}, {name: "native create collision", collision: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			request := filestore.AppendRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "out")}, Mode: 0o600, Append: filestore.AppendCreate}
			d, err := filestore.OpenAppendDestination(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			defer d.Close()
			if _, err := d.Write([]byte("outgoing")); err != nil {
				t.Fatal(err)
			}
			request.Location.Path = mustRelativePath(t, "in")
			wantErr := error(core.ErrFilestoreContract)
			if tc.invalid {
				request.Append = filestore.AppendExisting
			}
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			if tc.canceled {
				cancel(io.ErrUnexpectedEOF)
				wantErr = io.ErrUnexpectedEOF
			}
			if tc.collision {
				if err := os.WriteFile(dir+"/in", []byte("foreign"), 0o600); err != nil {
					t.Fatal(err)
				}
				wantErr = fs.ErrExist
			}
			if err := d.Rotate(ctx, request); !errors.Is(err, wantErr) {
				t.Fatalf("rotation=%v,want %v", err, wantErr)
			}
			if tc.collision {
				if d.Validate() == nil {
					t.Fatal("execution failure retained live owner")
				}
				if _, err := d.Write([]byte("zombie")); !errors.Is(err, core.ErrFilestoreContract) {
					t.Fatal(err)
				}
				got, err := os.ReadFile(dir + "/in")
				if err != nil || string(got) != "foreign" {
					t.Fatalf("foreign=%q,%v", got, err)
				}
			} else {
				if _, err := d.Write([]byte("still live")); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(dir + "/out")
			want := "outgoing"
			if !tc.collision {
				want += "still live"
			}
			if err != nil || string(got) != want {
				t.Fatalf("outgoing=%q,%v,want %q", got, err, want)
			}
		})
	}
}

func FuzzAppendDestinationExactBytesAndSettlement(f *testing.F) {
	for _, size := range []int{0, 1, 4095, 4096, 4097, 32769} {
		f.Add(bytes.Repeat([]byte{37}, size), uint8(size%3))
	}
	f.Fuzz(func(t *testing.T, data []byte, suffix uint8) { proveAppendDestinationBytes(t, data, int(suffix)) })
}
