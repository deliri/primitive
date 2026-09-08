package filestore_test

import (
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Acquisition, an actual operation through the acquired capability, exact inode
// comparison, and owned Close are the fixed workload. Directory setup is untimed.
func BenchmarkRootDirectoryAcquisition(b *testing.B) {
	directory := b.TempDir()
	path, err := core.ParseAbsolutePath(directory)
	if err != nil {
		b.Fatal(err)
	}
	want, err := os.Stat(directory)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		root, err := filestore.OpenRoot(b.Context(), path)
		if err != nil {
			b.Fatal(err)
		}
		got, statErr := root.Stat(".")
		closeErr := root.Close()
		if statErr != nil || closeErr != nil || !os.SameFile(want, got) {
			b.Fatalf("root stat/close = (%v,%v,%v), want exact directory and nil errors", got, statErr, closeErr)
		}
	}
}

func BenchmarkHeldDirectoryAcquisition(b *testing.B) {
	directory := b.TempDir()
	path, err := core.ParseAbsolutePath(directory)
	if err != nil {
		b.Fatal(err)
	}
	want, err := os.Stat(directory)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		held, err := filestore.OpenDirectory(b.Context(), path)
		if err != nil {
			b.Fatal(err)
		}
		file, fileErr := held.File()
		filesystem, filesystemErr := held.Filesystem()
		if fileErr != nil || filesystemErr != nil || filesystem.Validate() != nil {
			closeErr := held.Close()
			b.Fatalf("directory capability = (%v,%v,%v), close = %v", fileErr, filesystemErr, filesystem.Validate(), closeErr)
		}
		got, statErr := file.Stat()
		closeErr := held.Close()
		if statErr != nil || closeErr != nil || !os.SameFile(want, got) {
			b.Fatalf("held stat/close = (%v,%v,%v), want exact directory and nil errors", got, statErr, closeErr)
		}
	}
}
