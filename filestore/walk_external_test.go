package filestore_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestWalkStreamsFixedBatchesAndHonorsTypedDirectorySkip(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for index := range 129 {
		path := filepath.Join(directory, fmt.Sprintf("entry-%03d", index))
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
		}
	}
	skipped := filepath.Join(directory, "skipped")
	if err := os.Mkdir(skipped, 0o700); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", skipped, err)
	}
	if err := os.WriteFile(filepath.Join(skipped, "hidden"), []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(skipped child) error = %v, want nil", err)
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	rootPath, err := core.ParseRelativePath(".")
	if err != nil {
		t.Fatalf("core.ParseRelativePath(root) error = %v, want nil", err)
	}
	files := 0
	directories := 0
	err = filestore.Walk(t.Context(), filestore.WalkRequest{
		Location: filestore.Location{Root: root, Path: rootPath},
		Order:    filestore.WalkOrderNative,
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if entry.Entry.IsDir() {
				directories++
				return filestore.WalkSkipDirectory, nil
			}
			files++
			return filestore.WalkContinue, nil
		},
	})
	if err != nil {
		t.Fatalf("filestore.Walk() error = %v, want nil", err)
	}
	if files != 129 || directories != 1 {
		t.Fatalf("filestore.Walk() observations = files:%d directories:%d, want 129/1", files, directories)
	}
}

func TestWalkRejectsInvalidVisitorDirectiveAtTheRealDirectoryBoundary(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "entry"), []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(entry) error = %v, want nil", err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	rootPath, err := core.ParseRelativePath(".")
	if err != nil {
		t.Fatalf("core.ParseRelativePath(root) error = %v, want nil", err)
	}
	err = filestore.Walk(t.Context(), filestore.WalkRequest{
		Location: filestore.Location{Root: root, Path: rootPath},
		Order:    filestore.WalkOrderNative,
		Visit: func(filestore.WalkEntry) (filestore.WalkDirective, error) {
			return 0, nil
		},
	})
	if !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("filestore.Walk(invalid directive) error = %v, want %v", err, core.ErrFilestoreContract)
	}
}

func TestWalkLexicalOrderRequiresAndEnforcesExactDirectoryCeiling(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for _, name := range []string{"third", "first", "second"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v, want nil", name, err)
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	rootPath, err := core.ParseRelativePath(".")
	if err != nil {
		t.Fatalf("core.ParseRelativePath(root) error = %v, want nil", err)
	}
	exactMaximum, err := filestore.NewDirectoryEntryMaximum(3)
	if err != nil {
		t.Fatalf("filestore.NewDirectoryEntryMaximum(3) error = %v, want nil", err)
	}
	names := make([]string, 0, 3)
	err = filestore.Walk(t.Context(), filestore.WalkRequest{
		Location:              filestore.Location{Root: root, Path: rootPath},
		Order:                 filestore.WalkOrderLexical,
		DirectoryEntryMaximum: exactMaximum,
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			names = append(names, entry.Entry.Name())
			return filestore.WalkContinue, nil
		},
	})
	wantNames := []string{"first", "second", "third"}
	if err != nil || !slices.Equal(names, wantNames) {
		t.Fatalf("filestore.Walk(lexical) = (%q, %v), want (%q, nil)", names, err, wantNames)
	}
	belowMaximum, err := filestore.NewDirectoryEntryMaximum(2)
	if err != nil {
		t.Fatalf("filestore.NewDirectoryEntryMaximum(2) error = %v, want nil", err)
	}
	err = filestore.Walk(t.Context(), filestore.WalkRequest{
		Location:              filestore.Location{Root: root, Path: rootPath},
		Order:                 filestore.WalkOrderLexical,
		DirectoryEntryMaximum: belowMaximum,
		Visit: func(filestore.WalkEntry) (filestore.WalkDirective, error) {
			return filestore.WalkContinue, nil
		},
	})
	if !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("filestore.Walk(over ceiling) error = %v, want %v", err, core.ErrFilestoreContract)
	}
}

func TestDirectoryEntryMaximumRejectsAllocationAndConversionOverflow(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   uint32
	}{
		{name: "zero rejects", value: 0, wantErr: core.ErrFilestoreContract},
		{name: "one admits", value: 1},
		{name: "one below allocation ceiling admits", value: filestore.DirectoryEntryMaximumLimit - 1},
		{name: "exact allocation ceiling admits", value: filestore.DirectoryEntryMaximumLimit},
		{name: "one above allocation ceiling rejects", value: filestore.DirectoryEntryMaximumLimit + 1, wantErr: core.ErrFilestoreContract},
		{name: "maximum uint32 rejects before int conversion", value: math.MaxUint32, wantErr: core.ErrFilestoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := filestore.NewDirectoryEntryMaximum(tc.value)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("filestore.NewDirectoryEntryMaximum(%d) error = %v, want %v", tc.value, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != (filestore.DirectoryEntryMaximum{}) {
				t.Fatalf("filestore.NewDirectoryEntryMaximum(%d) = %v, want zero", tc.value, got)
			}
		})
	}
}

func TestWalkRejectsCancelledContextBeforeFilesystemObservation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := filestore.Walk(ctx, filestore.WalkRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("filestore.Walk(cancelled) error = %v, want %v", err, context.Canceled)
	}
}
