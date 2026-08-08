//go:build unix

package hostfacts

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// requireDetached proves the complete detached contract on one observation:
// the attachment names detachment and the column accessor refuses.
func requireDetached(t *testing.T, geometry TerminalGeometry) {
	t.Helper()
	attachment, err := geometry.Attachment()
	if err != nil || attachment != TerminalAttachmentNotTerminal {
		t.Fatalf("Attachment() = (%v, %v), want (%v, nil)", attachment, err, TerminalAttachmentNotTerminal)
	}
	if columns, err := geometry.Columns(); !errors.Is(err, core.ErrHostFactsContract) || columns != 0 {
		t.Fatalf("Columns() = (%v, %v), want (0, %v)", columns, err, core.ErrHostFactsContract)
	}
}

func TestObserveTerminalGeometryNamesEveryRealDescriptorKind(t *testing.T) {
	t.Parallel()

	t.Run("a pipe has no terminal geometry", func(t *testing.T) {
		t.Parallel()
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		defer func() {
			if err := errors.Join(reader.Close(), writer.Close()); err != nil {
				t.Fatalf("close pipe: %v", err)
			}
		}()
		geometry, err := ObserveTerminalGeometry(TerminalGeometryRequest{File: reader})
		if err != nil {
			t.Fatalf("ObserveTerminalGeometry(pipe) error = %v, want nil", err)
		}
		requireDetached(t, geometry)
	})

	t.Run("the null device has no terminal geometry", func(t *testing.T) {
		t.Parallel()
		null, err := os.Open(os.DevNull)
		if err != nil {
			t.Fatalf("open %s: %v", os.DevNull, err)
		}
		defer func() {
			if err := null.Close(); err != nil {
				t.Fatalf("close %s: %v", os.DevNull, err)
			}
		}()
		geometry, err := ObserveTerminalGeometry(TerminalGeometryRequest{File: null})
		if err != nil {
			t.Fatalf("ObserveTerminalGeometry(%s) error = %v, want nil", os.DevNull, err)
		}
		requireDetached(t, geometry)
	})

	t.Run("a regular file has no terminal geometry", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file, err := os.Create(filepath.Join(dir, "not-a-terminal"))
		if err != nil {
			t.Fatalf("create regular file: %v", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Fatalf("close regular file: %v", err)
			}
		}()
		geometry, err := ObserveTerminalGeometry(TerminalGeometryRequest{File: file})
		if err != nil {
			t.Fatalf("ObserveTerminalGeometry(regular file) error = %v, want nil", err)
		}
		requireDetached(t, geometry)
	})

	t.Run("a closed descriptor is a failed observation, not a detachment", func(t *testing.T) {
		t.Parallel()
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		if err := errors.Join(reader.Close(), writer.Close()); err != nil {
			t.Fatalf("close pipe: %v", err)
		}
		_, err = ObserveTerminalGeometry(TerminalGeometryRequest{File: reader})
		if !errors.Is(err, core.ErrHostFactsObservation) {
			t.Fatalf("ObserveTerminalGeometry(closed) error = %v, want %v", err, core.ErrHostFactsObservation)
		}
		failure, ok := errors.AsType[Failure](err)
		if !ok || failure.Operation != OperationTerminalGeometry {
			t.Fatalf("ObserveTerminalGeometry(closed) failure = (%+v, %t), want operation %v", failure, ok, OperationTerminalGeometry)
		}
		// The strongest assertable contract here is cause presence. The real
		// substrate answers this path with internal/poll's unexported
		// file-closing sentinel, returned untranslated by SyscallConn.Control,
		// so os.ErrClosed is genuinely unreachable and asserting it fails
		// against correct production code.
		if failure.Cause == nil {
			t.Fatalf("ObserveTerminalGeometry(closed) failure carries no native cause; the platform refusal must stay reachable")
		}
	})
}
