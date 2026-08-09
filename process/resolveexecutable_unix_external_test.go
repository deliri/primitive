//go:build unix

package process_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// TestResolveExecutableKeepsTheExactPOSIXRefusalIdentityReachable pins the
// errno a POSIX host names for each shape a path can be that a process
// cannot run. The portable table beside this file proves the refusals whose
// identity every platform shares; these rows prove the exact native cause
// stays reachable where the platform commits to one, so a caller walking a
// candidate chain can distinguish a directory wearing the name from a link
// cycle from a path that descends through a regular file.
func TestResolveExecutableKeepsTheExactPOSIXRefusalIdentityReachable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		build     func(t *testing.T) string
		name      string
		wantErrno syscall.Errno
	}{
		{
			name:      "a directory wearing the path refuses as a directory",
			wantErrno: syscall.EISDIR,
			build: func(t *testing.T) string {
				return t.TempDir()
			},
		},
		{
			name:      "a symbolic link to a directory refuses as a directory",
			wantErrno: syscall.EISDIR,
			build: func(t *testing.T) string {
				directory := t.TempDir()
				real := filepath.Join(directory, "real")
				if err := os.Mkdir(real, 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
				link := filepath.Join(directory, "link")
				if err := os.Symlink(real, link); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
				return link
			},
		},
		{
			name:      "a symbolic link loop refuses as a link cycle",
			wantErrno: syscall.ELOOP,
			build: func(t *testing.T) string {
				loop := filepath.Join(t.TempDir(), "loop")
				if err := os.Symlink(loop, loop); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
				return loop
			},
		},
		{
			name:      "a path descending through a regular file refuses as not a directory",
			wantErrno: syscall.ENOTDIR,
			build: func(t *testing.T) string {
				occupant := filepath.Join(t.TempDir(), "occupant")
				if err := os.WriteFile(occupant, []byte("not a directory"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
				return filepath.Join(occupant, "candidate-tool")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := tc.build(t)
			got, err := process.ResolveExecutable(t.Context(), absolutePath(t, path))
			if !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("ResolveExecutable(%q) error = %v, want %v", path, err, core.ErrProcessContract)
			}
			if !errors.Is(err, tc.wantErrno) {
				t.Fatalf("ResolveExecutable(%q) error = %v, want errors.Is(_, %v)", path, err, tc.wantErrno)
			}
			if got.String() != "" {
				t.Fatalf("ResolveExecutable(%q) = %q, want the zero path on refusal", path, got.String())
			}
		})
	}
}
