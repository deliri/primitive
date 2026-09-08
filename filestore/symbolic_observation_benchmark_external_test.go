package filestore_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// One exact opaque target observation; the 128-byte binary referent is never
// read in the timed operation. Fixture and native oracle construction are untimed.
func BenchmarkReadSymbolicLinkExactOpaqueTarget(b *testing.B) {
	directory := b.TempDir()
	payload := bytes.Repeat([]byte{0, 255, 1, 127}, 32)
	if err := os.WriteFile(filepath.Join(directory, "file"), payload, 0o600); err != nil {
		b.Fatal(err)
	}
	want := "./file"
	if err := os.Symlink(want, filepath.Join(directory, "link")); err != nil {
		b.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := root.Close(); err != nil {
			b.Error(err)
		}
	})
	relative, err := core.ParseRelativePath("link")
	if err != nil {
		b.Fatal(err)
	}
	location := filestore.Location{Root: root, Path: relative}
	if err := location.Validate(); err != nil {
		b.Fatal(err)
	}
	native, err := root.Readlink(relative.String())
	if err != nil || native != want {
		b.Fatalf("native fixture = (%q,%v), want %q", native, err, want)
	}
	before, err := root.Lstat(relative.String())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := filestore.ReadSymbolicLink(b.Context(), location)
		if err != nil || got.Validate() != nil || got.String() != want {
			b.Fatalf("target = (%q,%v), want %q", got.String(), err, want)
		}
	}
	after, err := root.Lstat(relative.String())
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
		b.Fatalf("retained link = (%v,%v), want %v", after, err, before)
	}
	got, err := os.ReadFile(filepath.Join(directory, "file"))
	if err != nil || !bytes.Equal(got, payload) {
		b.Fatalf("retained referent = (%v,%v), want %v", got, err, payload)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 2 || entries[0].Name() != "file" || entries[1].Name() != "link" {
		b.Fatalf("namespace = (%v,%v), want file and link only", entries, err)
	}
}

// One resolution traverses an ancestor link and a final link to a fixed binary
// file. Exact nominal output is checked every operation against untimed Go data.
func BenchmarkCanonicalizeAncestorAndFinalLink(b *testing.B) {
	directory := b.TempDir()
	payload := bytes.Repeat([]byte{0, 255, 1, 127}, 32)
	if err := os.Mkdir(filepath.Join(directory, "real"), 0o700); err != nil {
		b.Fatal(err)
	}
	name := filepath.Join(directory, "real", "file")
	if err := os.WriteFile(name, payload, 0o600); err != nil {
		b.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(directory, "parent")); err != nil {
		b.Fatal(err)
	}
	if err := os.Symlink("file", filepath.Join(directory, "real", "link")); err != nil {
		b.Fatal(err)
	}
	path, err := core.ParseAbsolutePath(filepath.Join(directory, "parent", "link"))
	if err != nil {
		b.Fatal(err)
	}
	native, err := filepath.EvalSymlinks(path.String())
	if err != nil {
		b.Fatal(err)
	}
	want, err := core.ParseAbsolutePath(native)
	if err != nil || want == path {
		b.Fatalf("native canonical fixture = (%v,%v), want resolved spelling distinct from input", want, err)
	}
	before, err := os.Stat(name)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := filestore.Canonicalize(b.Context(), path)
		if err != nil || got != want || got.Validate() != nil {
			b.Fatalf("canonical path = (%v,%v), want %v", got, err, want)
		}
	}
	after, err := os.Stat(want.String())
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
		b.Fatalf("retained referent = (%v,%v), want %v", after, err, before)
	}
	got, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(got, payload) {
		b.Fatalf("retained bytes = (%v,%v), want %v", got, err, payload)
	}
	for _, link := range []struct{ name, want string }{{"parent", "real"}, {filepath.Join("real", "link"), "file"}} {
		got, err := os.Readlink(filepath.Join(directory, link.name))
		if err != nil || got != link.want {
			b.Fatalf("retained target = (%q,%v), want %q", got, err, link.want)
		}
	}
}
