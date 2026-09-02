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

type mutateAfterWrite struct {
	mutate      func() error
	destination bytes.Buffer
	mutated     bool
}

func (w *mutateAfterWrite) Write(data []byte) (int, error) {
	count, err := w.destination.Write(data)
	if err != nil || w.mutated {
		return count, err
	}
	w.mutated = true
	return count, w.mutate()
}

func TestReadRefusesAFileThatChangesAfterItsExtentObservation(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		wantErr      error
		mutate       func(path string) error
		name         string
		initialBytes int
		maximumBytes int
		wantCopied   int
		wantSource   bool
	}{
		{
			name:         "growth at the caller ceiling is detected by an eof probe",
			initialBytes: 64, maximumBytes: 64, wantErr: core.ErrFilestoreSize, wantCopied: 64,
			mutate: func(path string) error {
				file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					return err
				}
				_, writeErr := file.Write([]byte("x"))
				return errors.Join(writeErr, file.Close())
			},
		},
		{
			name:         "shrink below the observed extent is a typed short source",
			initialBytes: 64 << 10, maximumBytes: 64 << 10, wantErr: io.ErrUnexpectedEOF, wantCopied: 32 << 10, wantSource: true,
			mutate: func(path string) error { return os.Truncate(path, 0) },
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			path := filepath.Join(directory, "source")
			content := bytes.Repeat([]byte("a"), testCase.initialBytes)
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
			}
			root, rootErr := filestore.OpenRoot(t.Context(), openRootAbsolute(t, directory))
			if rootErr != nil {
				t.Fatalf("filestore.OpenRoot() error = %v, want nil", rootErr)
			}
			t.Cleanup(func() { openRootClose(t, root) })
			destination := &mutateAfterWrite{mutate: func() error { return testCase.mutate(path) }}

			gotLength, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
				Location:    filestore.Location{Root: root, Path: mustRelativePath(t, "source")},
				Destination: destination, MaximumBytes: mustByteCount(t, uint64(testCase.maximumBytes)),
			})
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("filestore.Read(changing source) error = %v, want %v", gotErr, testCase.wantErr)
			}
			if testCase.wantSource && !errors.Is(gotErr, core.ErrFilestoreSource) {
				t.Fatalf("filestore.Read(changing source) error = %v, want %v", gotErr, core.ErrFilestoreSource)
			}
			if gotLength.Uint64() != uint64(testCase.wantCopied) || destination.destination.Len() != testCase.wantCopied {
				t.Fatalf("filestore.Read(changing source) = (length %d, bytes %d), want (%d, %d)", gotLength.Uint64(), destination.destination.Len(), testCase.wantCopied, testCase.wantCopied)
			}
			if !destination.mutated {
				t.Fatal("filestore.Read(changing source) mutation executed = false, want true")
			}
		})
	}
}
