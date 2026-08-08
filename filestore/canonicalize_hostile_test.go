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

func canonicalizeAbsolutePath(t *testing.T, value string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", value, err)
	}
	return path
}

// TestCanonicalizeResolvesEverySymbolicLinkComponent proves the door's whole
// claim: a path reached through a link chain canonicalizes to the same
// spelling the direct real path does, so an integrity decision over the two
// compares one location with itself.
func TestCanonicalizeResolvesEverySymbolicLinkComponent(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	realDirectory := filepath.Join(base, "real")
	if err := os.Mkdir(realDirectory, 0o755); err != nil {
		t.Fatalf("os.Mkdir(real) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(realDirectory, "entry"), []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(entry) error = %v, want nil", err)
	}
	linkDirectory := filepath.Join(base, "link")
	if err := os.Symlink(realDirectory, linkDirectory); err != nil {
		t.Fatalf("os.Symlink(directory) error = %v, want nil", err)
	}

	direct, err := filestore.Canonicalize(t.Context(), canonicalizeAbsolutePath(t, filepath.Join(realDirectory, "entry")))
	if err != nil {
		t.Fatalf("Canonicalize(direct) error = %v, want nil", err)
	}
	linked, err := filestore.Canonicalize(t.Context(), canonicalizeAbsolutePath(t, filepath.Join(linkDirectory, "entry")))
	if err != nil {
		t.Fatalf("Canonicalize(through link) error = %v, want nil", err)
	}
	if linked != direct {
		t.Fatalf("Canonicalize(through link) = %s, want the direct spelling %s", linked.String(), direct.String())
	}
}

// TestCanonicalizeRefusesAnAbsentPathWithTheNativeCause holds the door to
// existing paths: an absent name has no canonical spelling, and the native
// absence identity stays reachable for the caller's own decision.
func TestCanonicalizeRefusesAnAbsentPathWithTheNativeCause(t *testing.T) {
	t.Parallel()

	absent := canonicalizeAbsolutePath(t, filepath.Join(t.TempDir(), "never-created"))
	_, err := filestore.Canonicalize(t.Context(), absent)
	if !errors.Is(err, core.ErrFilestoreSource) {
		t.Fatalf("Canonicalize(absent) error = %v, want errors.Is %v", err, core.ErrFilestoreSource)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Canonicalize(absent) error = %v, want the native %v to stay reachable", err, os.ErrNotExist)
	}
}

// TestCanonicalizeRefusesALinkLoopAsASourceFailure proves a cycle is a loud
// typed refusal, never a hang or an unbounded walk.
func TestCanonicalizeRefusesALinkLoopAsASourceFailure(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	first := filepath.Join(base, "first")
	second := filepath.Join(base, "second")
	if err := os.Symlink(second, first); err != nil {
		t.Fatalf("os.Symlink(first) error = %v, want nil", err)
	}
	if err := os.Symlink(first, second); err != nil {
		t.Fatalf("os.Symlink(second) error = %v, want nil", err)
	}
	if _, err := filestore.Canonicalize(t.Context(), canonicalizeAbsolutePath(t, first)); !errors.Is(err, core.ErrFilestoreSource) {
		t.Fatalf("Canonicalize(link loop) error = %v, want errors.Is %v", err, core.ErrFilestoreSource)
	}
}

// TestCanonicalizeRefusesContractViolations holds the two ingress gates: a
// terminal context and the unset zero path.
func TestCanonicalizeRefusesContractViolations(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := filestore.Canonicalize(cancelled, canonicalizeAbsolutePath(t, t.TempDir())); !errors.Is(err, context.Canceled) {
		t.Fatalf("Canonicalize(cancelled context) error = %v, want errors.Is %v", err, context.Canceled)
	}
	if _, err := filestore.Canonicalize(t.Context(), core.AbsolutePath{}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("Canonicalize(zero path) error = %v, want errors.Is %v", err, core.ErrFilestoreContract)
	}
}

// TestCanonicalizeOfARealPathIsItself pins the neutral case: a path with no
// links canonicalizes to its own already-canonical spelling.
func TestCanonicalizeOfARealPathIsItself(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	want, err := filestore.Canonicalize(t.Context(), canonicalizeAbsolutePath(t, directory))
	if err != nil {
		t.Fatalf("Canonicalize(real directory) error = %v, want nil", err)
	}
	again, err := filestore.Canonicalize(t.Context(), want)
	if err != nil {
		t.Fatalf("Canonicalize(canonical answer) error = %v, want nil", err)
	}
	if again != want {
		t.Fatalf("Canonicalize(Canonicalize(x)) = %s, want the fixed point %s", again.String(), want.String())
	}
}
