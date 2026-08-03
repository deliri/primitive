//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

const (
	namedPipeReadHelperEnvironment = "PRIMITIVE_FILESTORE_NAMED_PIPE_READ_HELPER"
	namedPipeReadBackstop          = 5 * time.Second
)

func TestReadRejectsNamedPipeBeforeBlockingOpen(t *testing.T) {
	t.Parallel()

	if rootDirectory := os.Getenv(namedPipeReadHelperEnvironment); rootDirectory != "" {
		runNamedPipeReadHelper(t, rootDirectory)
		return
	}

	rootDirectory := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(rootDirectory, "source"), 0o600); err != nil {
		t.Fatalf("Mkfifo(source) error = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), namedPipeReadBackstop)
	defer cancel()
	command := exec.CommandContext(
		ctx,
		os.Args[0],
		"-test.run=^TestReadRejectsNamedPipeBeforeBlockingOpen$",
	)
	command.Env = append(
		os.Environ(),
		namedPipeReadHelperEnvironment+"="+rootDirectory,
	)
	output, err := command.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("Read(named pipe) did not return before the deadlock backstop")
	}
	if err != nil {
		t.Fatalf("named-pipe helper error = %v output:%q, want nil", err, output)
	}
}

func runNamedPipeReadHelper(t *testing.T, rootDirectory string) {
	t.Helper()

	root := requireTestRoot(t, rootDirectory)
	var destination bytes.Buffer
	gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
		Destination: &destination,
		Location: filestore.Location{
			Root: root,
			Path: mustRelativePath(t, "source"),
		},
		MaximumBytes: mustByteCount(t, 1),
	})
	if !errors.Is(gotErr, core.ErrFilestoreSource) ||
		!errors.Is(gotErr, fs.ErrInvalid) {
		t.Fatalf(
			"Read(named pipe) error = %v, want %v and %v",
			gotErr,
			core.ErrFilestoreSource,
			fs.ErrInvalid,
		)
	}
	if gotCount.Uint64() != 0 || destination.Len() != 0 {
		t.Fatalf(
			"Read(named pipe) = count:%d destination:%d, want 0/0",
			gotCount.Uint64(),
			destination.Len(),
		)
	}
}
