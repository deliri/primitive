package filestore_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func requireHeldAbsolute(t *testing.T, raw string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(raw)
	if err != nil {
		t.Fatalf("ParseAbsolutePath(%q) error = %v, want nil", raw, err)
	}
	return path
}

func requireHeldOpen(t *testing.T, raw string) *os.File {
	t.Helper()
	held, err := os.Open(raw)
	if err != nil {
		t.Fatalf("Open(%q) error = %v, want nil", raw, err)
	}
	t.Cleanup(func() { _ = held.Close() })
	return held
}

func requireHeldWrite(t *testing.T, raw string) {
	t.Helper()
	if err := os.WriteFile(raw, []byte("held fixture"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v, want nil", raw, err)
	}
}

func requireHeldRemove(t *testing.T, raw string) {
	t.Helper()
	if err := os.Remove(raw); err != nil {
		t.Fatalf("Remove(%q) error = %v, want nil", raw, err)
	}
}

func requireHeldRename(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Rename(from, to); err != nil {
		t.Fatalf("Rename(%q, %q) error = %v, want nil", from, to, err)
	}
}

// TestObserveHeldStandingAnswersEveryOccupancy drives the door across every
// way a name can stand relative to the entry a handle holds: still the held
// entry, the held entry under another of its names, taken by an imposter of
// every kind, and gone by unlink, rename, or a destroyed parent.
func TestObserveHeldStandingAnswersEveryOccupancy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		setup func(t *testing.T, dir string) (*os.File, core.AbsolutePath)
		name  string
		want  filestore.HeldStanding
	}{
		{
			name: "path still names the held regular file",
			setup: func(t *testing.T, dir string) (*os.File, core.AbsolutePath) {
				t.Helper()
				name := filepath.Join(dir, "held")
				requireHeldWrite(t, name)
				return requireHeldOpen(t, name), requireHeldAbsolute(t, name)
			},
			want: filestore.HeldStandingSame,
		},
		{
			name: "hard link to the held entry is the held entry",
			setup: func(t *testing.T, dir string) (*os.File, core.AbsolutePath) {
				t.Helper()
				name := filepath.Join(dir, "held")
				alias := filepath.Join(dir, "alias")
				requireHeldWrite(t, name)
				if err := os.Link(name, alias); err != nil {
					t.Fatalf("Link(%q, %q) error = %v, want nil", name, alias, err)
				}
				return requireHeldOpen(t, name), requireHeldAbsolute(t, alias)
			},
			want: filestore.HeldStandingSame,
		},
		{
			name: "held directory still occupies its name",
			setup: func(t *testing.T, dir string) (*os.File, core.AbsolutePath) {
				t.Helper()
				name := filepath.Join(dir, "held")
				if err := os.Mkdir(name, 0o700); err != nil {
					t.Fatalf("Mkdir(%q) error = %v, want nil", name, err)
				}
				return requireHeldOpen(t, name), requireHeldAbsolute(t, name)
			},
			want: filestore.HeldStandingSame,
		},
		{
			name: "recreated name with identical bytes is a different entry",
			setup: func(t *testing.T, dir string) (*os.File, core.AbsolutePath) {
				t.Helper()
				name := filepath.Join(dir, "held")
				requireHeldWrite(t, name)
				held := requireHeldOpen(t, name)
				requireHeldRemove(t, name)
				requireHeldWrite(t, name)
				return held, requireHeldAbsolute(t, name)
			},
			want: filestore.HeldStandingReplaced,
		},
		{
			name: "another file renamed onto the held name",
			setup: func(t *testing.T, dir string) (*os.File, core.AbsolutePath) {
				t.Helper()
				name := filepath.Join(dir, "held")
				intruder := filepath.Join(dir, "intruder")
				requireHeldWrite(t, name)
				requireHeldWrite(t, intruder)
				held := requireHeldOpen(t, name)
				requireHeldRename(t, intruder, name)
				return held, requireHeldAbsolute(t, name)
			},
			want: filestore.HeldStandingReplaced,
		},
		{
			name: "directory now occupies the held name",
			setup: func(t *testing.T, dir string) (*os.File, core.AbsolutePath) {
				t.Helper()
				name := filepath.Join(dir, "held")
				requireHeldWrite(t, name)
				held := requireHeldOpen(t, name)
				requireHeldRemove(t, name)
				if err := os.Mkdir(name, 0o700); err != nil {
					t.Fatalf("Mkdir(%q) error = %v, want nil", name, err)
				}
				return held, requireHeldAbsolute(t, name)
			},
			want: filestore.HeldStandingReplaced,
		},
		{
			name: "symbolic link at the name pointing at the held entry",
			setup: func(t *testing.T, dir string) (*os.File, core.AbsolutePath) {
				t.Helper()
				name := filepath.Join(dir, "held")
				target := filepath.Join(dir, "target")
				requireHeldWrite(t, name)
				held := requireHeldOpen(t, name)
				requireHeldRename(t, name, target)
				if err := os.Symlink(target, name); err != nil {
					t.Fatalf("Symlink(%q, %q) error = %v, want nil", target, name, err)
				}
				return held, requireHeldAbsolute(t, name)
			},
			want: filestore.HeldStandingReplaced,
		},
		{
			name: "unlinked name is absent",
			setup: func(t *testing.T, dir string) (*os.File, core.AbsolutePath) {
				t.Helper()
				name := filepath.Join(dir, "held")
				requireHeldWrite(t, name)
				held := requireHeldOpen(t, name)
				requireHeldRemove(t, name)
				return held, requireHeldAbsolute(t, name)
			},
			want: filestore.HeldStandingAbsent,
		},
		{
			name: "renamed away name is absent",
			setup: func(t *testing.T, dir string) (*os.File, core.AbsolutePath) {
				t.Helper()
				name := filepath.Join(dir, "held")
				elsewhere := filepath.Join(dir, "elsewhere")
				requireHeldWrite(t, name)
				held := requireHeldOpen(t, name)
				requireHeldRename(t, name, elsewhere)
				return held, requireHeldAbsolute(t, name)
			},
			want: filestore.HeldStandingAbsent,
		},
		{
			name: "removed parent leaves the name absent",
			setup: func(t *testing.T, dir string) (*os.File, core.AbsolutePath) {
				t.Helper()
				parent := filepath.Join(dir, "parent")
				if err := os.Mkdir(parent, 0o700); err != nil {
					t.Fatalf("Mkdir(%q) error = %v, want nil", parent, err)
				}
				name := filepath.Join(parent, "held")
				requireHeldWrite(t, name)
				held := requireHeldOpen(t, name)
				requireHeldRemove(t, name)
				requireHeldRemove(t, parent)
				return held, requireHeldAbsolute(t, name)
			},
			want: filestore.HeldStandingAbsent,
		},
		{
			name: "parent replaced by a regular file leaves the name absent",
			setup: func(t *testing.T, dir string) (*os.File, core.AbsolutePath) {
				t.Helper()
				parent := filepath.Join(dir, "parent")
				if err := os.Mkdir(parent, 0o700); err != nil {
					t.Fatalf("Mkdir(%q) error = %v, want nil", parent, err)
				}
				name := filepath.Join(parent, "held")
				requireHeldWrite(t, name)
				held := requireHeldOpen(t, name)
				requireHeldRemove(t, name)
				requireHeldRemove(t, parent)
				requireHeldWrite(t, parent)
				return held, requireHeldAbsolute(t, name)
			},
			want: filestore.HeldStandingAbsent,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			held, path := tc.setup(t, dir)
			got, err := filestore.ObserveHeldStanding(t.Context(), held, path)
			if err != nil {
				t.Fatalf("ObserveHeldStanding() error = %v, want nil", err)
			}
			if got != tc.want {
				t.Fatalf("ObserveHeldStanding() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestObserveHeldStandingHoldsItsContractGates proves refusal comes before
// any filesystem effect, carries the strongest stable identity, and never
// hands back an observation beside an error.
func TestObserveHeldStandingHoldsItsContractGates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	name := filepath.Join(dir, "held")
	requireHeldWrite(t, name)
	held := requireHeldOpen(t, name)
	path := requireHeldAbsolute(t, name)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := filestore.ObserveHeldStanding(cancelled, held, path)
	if !errors.Is(err, context.Canceled) || got != filestore.HeldStandingUnknown {
		t.Fatalf("ObserveHeldStanding(cancelled context) = (%v, %v), want (%v, errors.Is %v)",
			got, err, filestore.HeldStandingUnknown, context.Canceled)
	}

	got, err = filestore.ObserveHeldStanding(t.Context(), nil, path)
	if !errors.Is(err, core.ErrFilestoreContract) || got != filestore.HeldStandingUnknown {
		t.Fatalf("ObserveHeldStanding(nil handle) = (%v, %v), want (%v, errors.Is %v)",
			got, err, filestore.HeldStandingUnknown, core.ErrFilestoreContract)
	}

	got, err = filestore.ObserveHeldStanding(t.Context(), held, core.AbsolutePath{})
	if !errors.Is(err, core.ErrFilestoreContract) || got != filestore.HeldStandingUnknown {
		t.Fatalf("ObserveHeldStanding(zero path) = (%v, %v), want (%v, errors.Is %v)",
			got, err, filestore.HeldStandingUnknown, core.ErrFilestoreContract)
	}

	closedName := filepath.Join(dir, "closed")
	requireHeldWrite(t, closedName)
	closed := requireHeldOpen(t, closedName)
	if err := closed.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	got, err = filestore.ObserveHeldStanding(t.Context(), closed, path)
	if !errors.Is(err, core.ErrFilestoreContract) || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("ObserveHeldStanding(closed handle) error = %v, want errors.Is %v and %v",
			err, core.ErrFilestoreContract, os.ErrClosed)
	}
	if got != filestore.HeldStandingUnknown {
		t.Fatalf("ObserveHeldStanding(closed handle) = %v, want %v", got, filestore.HeldStandingUnknown)
	}
}

// TestHeldStandingAdmitsOnlyTheClosedDomain sweeps the whole uint8 space the
// way every filestore off-wire enum is proven.
func TestHeldStandingAdmitsOnlyTheClosedDomain(t *testing.T) {
	t.Parallel()

	proveFilestoreOffWireEnum(t, func(raw uint8) filestore.HeldStanding { return filestore.HeldStanding(raw) }, []filestore.HeldStanding{
		filestore.HeldStandingSame,
		filestore.HeldStandingReplaced,
		filestore.HeldStandingAbsent,
	})
}
