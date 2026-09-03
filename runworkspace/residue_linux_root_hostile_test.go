//go:build linux

package runworkspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestLinuxResidueProcessObservationCannotLeaveConfiguredProcRoot(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	procDirectory := filepath.Join(parent, "proc")
	processDirectory := filepath.Join(procDirectory, "123")
	if err := os.MkdirAll(processDirectory, 0o700); err != nil {
		t.Fatalf("os.MkdirAll(process directory) error = %v, want nil", err)
	}
	outsideStatus := filepath.Join(parent, "outside-status")
	if err := os.WriteFile(outsideStatus, []byte("Uid:\t4242\t4242\t4242\t4242\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(outside status) error = %v, want nil", err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "outside-status"), filepath.Join(processDirectory, "status")); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}
	procRoot, err := core.ParseAbsolutePath(procDirectory)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(proc root) error = %v, want nil", err)
	}
	source := LinuxResidueSource{configuration: LinuxResidueConfiguration{ProcRoot: procRoot, ProcessUserID: 4242}}
	got, gotErr := source.observeSubjectProcesses(t.Context())
	if got != (residueCounts{}) || !errors.Is(gotErr, core.ErrFilestoreSource) {
		t.Fatalf("observeSubjectProcesses(escaping status link) = (%+v, %v), want zero and %v", got, gotErr, core.ErrFilestoreSource)
	}
}
