package filestore_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func BenchmarkInspectRegularFileObservation(b *testing.B) {
	directory := b.TempDir()
	name := filepath.Join(directory, "source")
	payload := []byte{0, 255, 1, '\n'}
	if err := os.WriteFile(name, payload, 0o600); err != nil {
		b.Fatal(err)
	}
	path, err := core.ParseAbsolutePath(name)
	if err != nil {
		b.Fatal(err)
	}
	want, err := os.Lstat(name)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := filestore.Inspect(b.Context(), path)
		if err != nil {
			b.Fatal(err)
		}
		kind, kindErr := got.Kind()
		size, sizeErr := got.SizeBytes()
		modified, modifiedErr := got.ModifiedAt()
		nanoseconds, nanosecondsErr := modified.Nanoseconds()
		permissions, permissionsErr := got.Permissions()
		mode, modeErr := permissions.FileMode()
		if kindErr != nil || sizeErr != nil || modifiedErr != nil || nanosecondsErr != nil || permissionsErr != nil || modeErr != nil || kind != filestore.PathKindRegularFile || size.Uint64() != uint64(len(payload)) || nanoseconds != want.ModTime().UnixNano() || mode != want.Mode().Perm() {
			b.Fatalf("observation = (%v,%v,%v,%v), errors (%v,%v,%v,%v,%v,%v), want original kind/extent/time/mode", kind, size, modified, mode, kindErr, sizeErr, modifiedErr, nanosecondsErr, permissionsErr, modeErr)
		}
	}
}

func BenchmarkInspectionValidationBatch(b *testing.B) {
	directory := b.TempDir()
	name := filepath.Join(directory, "source")
	if err := os.WriteFile(name, []byte{0, 255}, 0o600); err != nil {
		b.Fatal(err)
	}
	path, err := core.ParseAbsolutePath(name)
	if err != nil {
		b.Fatal(err)
	}
	observation, err := filestore.Inspect(b.Context(), path)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		for range 64 {
			if err := observation.Validate(); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.ReportMetric(64, "validations/op")
}
