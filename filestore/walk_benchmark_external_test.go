package filestore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func BenchmarkWalkNativeSparseDirectory(b *testing.B) {
	directory := b.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "entry"), nil, 0o600); err != nil {
		b.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			b.Error(closeErr)
		}
	})
	path, err := core.ParseRelativePath(".")
	if err != nil {
		b.Fatal(err)
	}
	request := filestore.WalkRequest{
		Location: filestore.Location{Root: root, Path: path},
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if err := entry.Validate(); err != nil {
				return filestore.WalkDirectiveUnknown, err
			}
			return filestore.WalkContinue, nil
		},
	}
	var wantErr error
	b.ReportAllocs()
	var visits int
	for b.Loop() {
		visits = 0
		request.Visit = func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if err := entry.Validate(); err != nil {
				return filestore.WalkDirectiveUnknown, err
			}
			visits++
			return filestore.WalkContinue, nil
		}
		err := filestore.Walk(b.Context(), request)
		if !errors.Is(err, wantErr) || visits != 1 {
			b.Fatalf("filestore.Walk() = visits %d error %v, want one sparse entry and %v", visits, err, wantErr)
		}
	}
}
