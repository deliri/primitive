package filestore_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Exactly two mode changes per iteration, each observed independently through
// Go Stat. The same 128-byte inode alternates read-only and owner read/write;
// the final byte and identity checks exclude replacement or truncation.
func BenchmarkPermissionTwoObservedChanges(b *testing.B) {
	directory := b.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			b.Error(err)
		}
	}()
	path, err := core.ParseRelativePath("subject")
	if err != nil {
		b.Fatal(err)
	}
	payload := bytes.Repeat([]byte{0, 255, 7, 31}, 32)
	if err := os.WriteFile(directory+"/"+path.String(), payload, 0o600); err != nil {
		b.Fatal(err)
	}
	before, err := root.Stat(path.String())
	if err != nil {
		b.Fatal(err)
	}
	requests := [2]filestore.PermissionRequest{
		{Location: filestore.Location{Root: root, Path: path}, Mode: 0o400},
		{Location: filestore.Location{Root: root, Path: path}, Mode: 0o600},
	}
	for _, request := range requests {
		if err := request.Validate(); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		for _, request := range requests {
			if err := filestore.SetPermissions(b.Context(), request); err != nil {
				b.Fatal(err)
			}
			got, err := root.Stat(path.String())
			if err != nil {
				b.Fatal(err)
			}
			if got.Mode().Perm() != request.Mode || !os.SameFile(before, got) || got.Size() != int64(len(payload)) {
				b.Fatalf("permission observation = %v, want same inode, exact extent and mode %v", got, request.Mode)
			}
		}
	}
	b.ReportMetric(float64(len(requests)), "changes/op")
	retained, err := os.ReadFile(directory + "/" + path.String())
	if err != nil || !bytes.Equal(retained, payload) {
		b.Fatalf("retained bytes = (%v,%v), want %v", retained, err, payload)
	}
}
