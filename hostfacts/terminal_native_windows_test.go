// These proofs pin the Windows detached-versus-failure classification that
// previously existed only as a comment: a pipe and a regular file answer
// through the console API's invalid-handle family and are honest detachments,
// while a closed handle is a failed observation that keeps its native cause.
// The Darwin host cross-compiles this file; execution is a Windows runner's
// evidence. A console-attached case is deliberately absent, because a test
// runner owns no console this suite could bind deterministically.
//go:build windows

package hostfacts

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestObserveTerminalGeometryNamesWindowsDescriptorKinds(t *testing.T) {
	t.Parallel()

	t.Run("a pipe is not a console", func(t *testing.T) {
		t.Parallel()
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v, want nil", err)
		}
		t.Cleanup(func() { _ = read.Close(); _ = write.Close() })
		geometry, err := ObserveTerminalGeometry(TerminalGeometryRequest{File: read})
		if err != nil {
			t.Fatalf("ObserveTerminalGeometry(pipe) error = %v, want nil", err)
		}
		if attachment, err := geometry.Attachment(); err != nil || attachment != TerminalAttachmentNotTerminal {
			t.Fatalf("pipe Attachment() = (%v, %v), want (%v, nil)", attachment, err, TerminalAttachmentNotTerminal)
		}
	})

	t.Run("a regular file is not a console", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "plain")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v, want nil", err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatalf("Open() error = %v, want nil", err)
		}
		t.Cleanup(func() { _ = file.Close() })
		geometry, err := ObserveTerminalGeometry(TerminalGeometryRequest{File: file})
		if err != nil {
			t.Fatalf("ObserveTerminalGeometry(file) error = %v, want nil", err)
		}
		if attachment, err := geometry.Attachment(); err != nil || attachment != TerminalAttachmentNotTerminal {
			t.Fatalf("file Attachment() = (%v, %v), want (%v, nil)", attachment, err, TerminalAttachmentNotTerminal)
		}
	})

	t.Run("a closed handle is a failed observation, not a detachment", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "closed")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v, want nil", err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatalf("Open() error = %v, want nil", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("Close() error = %v, want nil", err)
		}
		_, err = ObserveTerminalGeometry(TerminalGeometryRequest{File: file})
		if !errors.Is(err, core.ErrHostFactsObservation) {
			t.Fatalf("ObserveTerminalGeometry(closed) error = %v, want %v", err, core.ErrHostFactsObservation)
		}
		failure, ok := errors.AsType[Failure](err)
		if !ok || failure.Operation != OperationTerminalGeometry {
			t.Fatalf("ObserveTerminalGeometry(closed) failure = (%+v, %t), want operation %v", failure, ok, OperationTerminalGeometry)
		}
		if failure.Cause == nil {
			t.Fatalf("ObserveTerminalGeometry(closed) failure carries no native cause; the platform refusal must stay reachable")
		}
	})
}
