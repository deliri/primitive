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
	namedPipeReadHelperEnvironment    = "PRIMITIVE_FILESTORE_NAMED_PIPE_READ_HELPER"
	namedPipeInspectHelperEnvironment = "PRIMITIVE_FILESTORE_NAMED_PIPE_INSPECT_HELPER"
	// namedPipeReadBackstop only separates "returned" from "wedged forever".
	// Both cases below spawn a helper process, and under a full-repository run
	// that helper competes with every other package's tests for the disk, so a
	// tight bound here would be asserting scheduling rather than behaviour. The
	// failure it guards against is an unbounded block, which no amount of load
	// turns into a pass.
	namedPipeReadBackstop = 120 * time.Second
)

// TestInspectReportsUnreachableThroughANamedPipeParentWithoutBlocking is the
// sibling of the Read case below, and it exists because Inspect reintroduced
// the hazard Read already defends against.
//
// Inspect opens the parent with os.OpenRoot in order to Lstat the final
// component. An ordinary open of a named pipe blocks until a writer arrives,
// and no cancellation reaches a goroutine parked in that syscall, so a FIFO
// anywhere in a configured path used to stop the caller permanently. A
// non-directory parent is exactly the case Inspect documents as an
// observation rather than a failure, so the answer must be Unreachable.
//
// The observation runs in a helper process because a wedged call cannot be
// abandoned: nothing in the parent process could reclaim the parked goroutine.
func TestInspectReportsUnreachableThroughANamedPipeParentWithoutBlocking(t *testing.T) {
	t.Parallel()

	if rootDirectory := os.Getenv(namedPipeInspectHelperEnvironment); rootDirectory != "" {
		runNamedPipeInspectHelper(t, rootDirectory)
		return
	}

	rootDirectory := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(rootDirectory, "parent"), 0o600); err != nil {
		t.Fatalf("Mkfifo(parent) error = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), namedPipeReadBackstop)
	defer cancel()
	command := exec.CommandContext(
		ctx,
		os.Args[0],
		"-test.run=^TestInspectReportsUnreachableThroughANamedPipeParentWithoutBlocking$",
	)
	command.Env = append(
		os.Environ(),
		namedPipeInspectHelperEnvironment+"="+rootDirectory,
	)
	output, err := command.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		t.Fatalf("Inspect(under a named pipe) did not return before the deadlock backstop")
	}
	if err != nil {
		t.Fatalf("named-pipe inspect helper error = %v output:%q, want nil", err, output)
	}
}

func runNamedPipeInspectHelper(t *testing.T, rootDirectory string) {
	t.Helper()

	path, err := core.ParseAbsolutePath(filepath.Join(rootDirectory, "parent", "entry"))
	if err != nil {
		t.Fatalf("ParseAbsolutePath() error = %v, want nil", err)
	}
	inspection, err := filestore.Inspect(t.Context(), path)
	if err != nil {
		t.Fatalf("Inspect(under a named pipe) error = %v, want nil", err)
	}
	gotKind, err := inspection.Kind()
	if err != nil {
		t.Fatalf("Kind() error = %v, want nil", err)
	}
	if gotKind != filestore.PathKindUnreachable {
		t.Fatalf("Inspect(under a named pipe) kind = %v, want %v", gotKind, filestore.PathKindUnreachable)
	}
}

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
