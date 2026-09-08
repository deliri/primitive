package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Each operation performs exactly three identity observations against one held
// inode: original, foreign same-content file, and absent name. No data is copied
// by production; a 128-byte fixture remains untouched until the final oracle.
func BenchmarkHeldStandingNativeIdentityTriad(b *testing.B) {
	directory := b.TempDir()
	payload := bytes.Repeat([]byte{0, 255, 1, 127}, 32)
	for _, name := range []string{"owned", "foreign"} {
		if err := os.WriteFile(filepath.Join(directory, name), payload, 0o600); err != nil {
			b.Fatal(err)
		}
	}
	held, err := os.Open(filepath.Join(directory, "owned"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := held.Close(); err != nil {
			b.Error(err)
		}
	})
	before, err := held.Stat()
	if err != nil {
		b.Fatal(err)
	}
	foreign, err := os.Stat(filepath.Join(directory, "foreign"))
	if err != nil || os.SameFile(before, foreign) {
		b.Fatalf("foreign fixture = (%v,%v), want different inode", foreign, err)
	}
	queries := []struct {
		path core.AbsolutePath
		want filestore.HeldStanding
	}{
		{want: filestore.HeldStandingSame}, {want: filestore.HeldStandingReplaced}, {want: filestore.HeldStandingAbsent},
	}
	for i, name := range []string{"owned", "foreign", "missing"} {
		queries[i].path, err = core.ParseAbsolutePath(filepath.Join(directory, name))
		if err != nil {
			b.Fatal(err)
		}
	}
	if info, err := os.Lstat(queries[2].path.String()); !errors.Is(err, os.ErrNotExist) {
		b.Fatalf("absent fixture = (%v,%v), want native absence", info, err)
	}
	b.ReportAllocs()
	for b.Loop() {
		for _, query := range queries {
			got, err := filestore.ObserveHeldStanding(b.Context(), held, query.path)
			if err != nil || got != query.want || got.Validate() != nil {
				b.Fatalf("standing = (%v,%v), want %v", got, err, query.want)
			}
		}
	}
	after, err := held.Stat()
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
		b.Fatalf("retained held inode = (%v,%v), want %v", after, err, before)
	}
	data := make([]byte, len(payload))
	if n, err := io.ReadFull(held, data); err != nil || n != len(payload) || !bytes.Equal(data, payload) {
		b.Fatalf("retained held cursor/bytes = (%v,%d,%v), want %v", data, n, err, payload)
	}
	var extra [1]byte
	if n, err := held.Read(extra[:]); n != 0 || !errors.Is(err, io.EOF) {
		b.Fatalf("held completion = (%d,%v), want exact EOF", n, err)
	}
	foreignAfter, err := os.Stat(queries[1].path.String())
	if err != nil || !os.SameFile(foreign, foreignAfter) || foreign.Mode() != foreignAfter.Mode() || foreign.ModTime().UnixNano() != foreignAfter.ModTime().UnixNano() {
		b.Fatalf("retained foreign inode = (%v,%v), want %v", foreignAfter, err, foreign)
	}
	data, err = os.ReadFile(queries[1].path.String())
	if err != nil || !bytes.Equal(data, payload) {
		b.Fatalf("retained foreign bytes = (%v,%v), want %v", data, err, payload)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 2 || entries[0].Name() != "foreign" || entries[1].Name() != "owned" {
		b.Fatalf("retained namespace = (%v,%v), want foreign and owned", entries, err)
	}
}
