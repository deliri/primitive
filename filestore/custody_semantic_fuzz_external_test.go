package filestore_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzCustodyNamespaceAndTimestampSemanticClosure(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted, int32(0), false, true)
	for _, seed := range []struct {
		payload []byte
		seconds int32
		link    bool
		touch   bool
	}{
		{payload: nil, touch: true},
		{payload: []byte{0, 255}, seconds: 1, touch: true},
		{payload: []byte{0, 255}, seconds: -1, touch: true},
		{payload: []byte{0, 255}, seconds: 1, link: true, touch: true},
		{payload: []byte{0, 255}, link: true},
		{payload: []byte{0, 255}},
	} {
		f.Add(seed.payload, seed.seconds, seed.link, seed.touch)
	}
	f.Fuzz(func(t *testing.T, payload []byte, seconds int32, link, touch bool) {
		payload = payload[:min(len(payload), 4096)]
		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		if err := os.WriteFile(filepath.Join(directory, "source"), payload, 0o600); err != nil {
			t.Fatal(err)
		}
		before, err := root.Lstat("source")
		if err != nil {
			t.Fatal(err)
		}
		name := "source"
		if link {
			name = "link"
			if err := root.Symlink("source", name); err != nil {
				t.Fatal(err)
			}
		}
		location := filestore.Location{Root: root, Path: mustRelativePath(t, name)}
		// Whole seconds remain exactly observable on filesystems with coarser
		// timestamp precision, while signed input pressures both sides of epoch.
		instant := temporal.InstantFromNanoseconds(int64(seconds) * 1_000_000_000)
		var gotErr error
		if touch {
			gotErr = filestore.Touch(t.Context(), filestore.TouchRequest{Location: location, ModifiedAt: instant})
		} else {
			gotErr = filestore.ConfirmDurable(t.Context(), filestore.DurabilityRequest{Location: location})
		}
		if link {
			if !errors.Is(gotErr, core.ErrFilestoreSource) || !errors.Is(gotErr, fs.ErrInvalid) {
				t.Fatalf("link custody = %v, want source/invalid entry", gotErr)
			}
		} else if gotErr != nil {
			t.Fatalf("regular custody = %v, want nil", gotErr)
		}
		after, err := root.Lstat("source")
		if err != nil {
			t.Fatal(err)
		}
		wantStamp := before.ModTime().UnixNano()
		if touch && !link {
			wantStamp = int64(seconds) * 1_000_000_000
		}
		if !os.SameFile(before, after) || after.Mode() != before.Mode() || after.Size() != before.Size() || after.ModTime().UnixNano() != wantStamp {
			t.Fatalf("source = (%v,%d,%d), want same inode/mode/size and stamp %d", after.Mode(), after.Size(), after.ModTime().UnixNano(), wantStamp)
		}
		got, err := os.ReadFile(filepath.Join(directory, "source"))
		if err != nil || !bytes.Equal(got, payload) {
			t.Fatalf("bytes = (%v,%v), want exact %v", got, err, payload)
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatal(err)
		}
		wantEntries := 1
		if link {
			wantEntries++
			info, err := root.Lstat(name)
			if err != nil || info.Mode()&fs.ModeSymlink == 0 {
				t.Fatalf("refused link = (%v,%v), want retained symlink", info, err)
			}
			target, err := root.Readlink(name)
			if err != nil || target != "source" {
				t.Fatalf("link target = (%q,%v), want original source", target, err)
			}
		}
		if len(entries) != wantEntries {
			t.Fatalf("namespace = %v, want %d exact entries", entries, wantEntries)
		}
	})
}
