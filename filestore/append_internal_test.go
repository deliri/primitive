package filestore

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestValidateAppendFileRejectsAndClosesRealDirectoryHandle(t *testing.T) {
	t.Parallel()

	directory, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gotErr := validateAppendFile(directory)
	if !errors.Is(gotErr, core.ErrFilestoreActivation) ||
		!errors.Is(gotErr, fs.ErrInvalid) {
		t.Fatalf("validateAppendFile(directory) error = %v, want %v and %v", gotErr, core.ErrFilestoreActivation, fs.ErrInvalid)
	}
	if _, err := directory.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("directory.Stat() after rejection error = %v, want %v", err, os.ErrClosed)
	}
}
