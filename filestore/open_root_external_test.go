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

const openRootDirectoryPermissions = 0o700

// openRootAbsolute parses one absolute path or fails the test. A malformed
// fixture path would otherwise be reported as an OpenRoot refusal and hide
// whichever contract the case meant to exercise.
func openRootAbsolute(tb testing.TB, path string) core.AbsolutePath {
	tb.Helper()
	parsed, err := core.ParseAbsolutePath(path)
	if err != nil {
		tb.Fatalf("ParseAbsolutePath(%s) error = %v, want nil", path, err)
	}
	return parsed
}

// openRootClose closes a root and reports a close failure as a test failure.
// The handle is the whole product of OpenRoot, so a root that cannot be closed
// is a leak the caller inherited.
func openRootClose(tb testing.TB, root *os.Root) {
	tb.Helper()
	if err := root.Close(); err != nil {
		tb.Errorf("Close() error = %v, want nil", err)
	}
}

// TestOpenRootConfinesEveryPathToTheDirectoryItOpened proves the property the
// handle exists for. Confinement is the reason a product holds a root instead
// of a directory string: without it, every later operation would have to
// re-check that a caller-supplied name stayed inside the store. A root that
// opened the directory but let ".." walk out would pass a naive "did it open"
// test and still hand the product a path traversal.
func TestOpenRootConfinesEveryPathToTheDirectoryItOpened(t *testing.T) {
	t.Parallel()

	enclosing := t.TempDir()
	store := filepath.Join(enclosing, "store")
	if err := os.MkdirAll(filepath.Join(store, "nested", "deeper"), openRootDirectoryPermissions); err != nil {
		t.Fatalf("MkdirAll() error = %v, want nil", err)
	}
	for _, name := range []string{"entry", "nested/child", "nested/deeper/leaf"} {
		if err := os.WriteFile(filepath.Join(store, name), []byte(name), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v, want nil", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(enclosing, "secret"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("WriteFile(secret) error = %v, want nil", err)
	}

	root, err := filestore.OpenRoot(t.Context(), openRootAbsolute(t, store))
	if err != nil {
		t.Fatalf("OpenRoot() error = %v, want nil", err)
	}
	// Cleanup, not defer: the parallel subtests below run after this function
	// body returns, so a deferred close would shut the root before the first
	// case ever opened anything through it.
	t.Cleanup(func() { openRootClose(t, root) })

	cases := []struct {
		name       string
		target     string
		wantOpened bool
	}{
		{name: "entry directly inside the root opens", target: "entry", wantOpened: true},
		{name: "entry one level down opens", target: "nested/child", wantOpened: true},
		{name: "entry two levels down opens", target: "nested/deeper/leaf", wantOpened: true},
		{name: "the root names itself", target: ".", wantOpened: true},
		{name: "descend and return inside the root opens", target: "nested/../entry", wantOpened: true},
		{name: "parent of the root is refused", target: "..", wantOpened: false},
		{name: "sibling of the root is refused", target: "../secret", wantOpened: false},
		{name: "grandparent of the root is refused", target: "../..", wantOpened: false},
		{name: "descend then escape past the root is refused", target: "nested/../../secret", wantOpened: false},
		{name: "escape deeper than the enclosing directory is refused", target: "../../../../secret", wantOpened: false},
		{name: "absolute path outside the root is refused", target: "/etc/passwd", wantOpened: false},
		{name: "absolute path naming the root itself is refused", target: store, wantOpened: false},
		{name: "absent entry inside the root is refused", target: "missing", wantOpened: false},
		{name: "absent entry below an absent directory is refused", target: "missing/deeper", wantOpened: false},
		{name: "entry below a regular file is refused", target: "entry/child", wantOpened: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opened, err := root.Open(tc.target)
			if err == nil {
				defer func() {
					if closeErr := opened.Close(); closeErr != nil {
						t.Errorf("Close() error = %v, want nil", closeErr)
					}
				}()
			}
			gotOpened := err == nil
			if gotOpened != tc.wantOpened {
				t.Fatalf("root.Open(%q) opened = %t (err = %v), want opened = %t", tc.target, gotOpened, err, tc.wantOpened)
			}
		})
	}
}

// TestOpenRootAcceptsTheDirectoriesProductsKeep proves the accepted spectrum is
// the real one: a store root is whatever directory the operator configured, so
// the contract must not quietly depend on the name being plain ASCII, shallow,
// non-empty, or free of characters a shell would need to quote.
func TestOpenRootAcceptsTheDirectoriesProductsKeep(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	cases := []struct {
		name    string
		suffix  string
		entries []string
	}{
		{name: "plain directory name", suffix: "store"},
		{name: "directory two levels down", suffix: "one/two"},
		{name: "directory six levels down", suffix: "a/b/c/d/e/f"},
		{name: "empty directory is a usable root", suffix: "empty"},
		{name: "directory holding many entries", suffix: "populated", entries: []string{"1", "2", "3", "4", "5"}},
		{name: "dot prefixed directory name", suffix: ".hidden"},
		{name: "directory name containing a space", suffix: "with space"},
		{name: "directory name containing non ascii runes", suffix: "café"},
		{name: "directory name containing a dot", suffix: "store.v2"},
		{name: "directory name that looks like a dotted parent", suffix: "..store"},
		{name: "directory name containing a dash", suffix: "store-2026"},
		{name: "single character directory name", suffix: "s"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(base, tc.suffix)
			if err := os.MkdirAll(path, openRootDirectoryPermissions); err != nil {
				t.Fatalf("MkdirAll(%s) error = %v, want nil", tc.suffix, err)
			}
			for _, entry := range tc.entries {
				if err := os.WriteFile(filepath.Join(path, entry), []byte(entry), 0o600); err != nil {
					t.Fatalf("WriteFile(%s) error = %v, want nil", entry, err)
				}
			}

			root, err := filestore.OpenRoot(t.Context(), openRootAbsolute(t, path))
			if err != nil {
				t.Fatalf("OpenRoot(%s) error = %v, want nil", tc.suffix, err)
			}
			defer openRootClose(t, root)

			if root.Name() != path {
				t.Fatalf("OpenRoot(%s) name = %q, want %q", tc.suffix, root.Name(), path)
			}
			// A root that opened but cannot serve an operation would satisfy a
			// nil-error check and fail the caller on first use.
			location := filestore.Location{Root: root, Path: openRootRelative(t, "probe")}
			if err := location.Validate(); err != nil {
				t.Fatalf("Location.Validate() error = %v, want nil", err)
			}
		})
	}
}

// openRootRelative parses one relative path or fails the test.
func openRootRelative(tb testing.TB, path string) core.RelativePath {
	tb.Helper()
	parsed, err := core.ParseRelativePath(path)
	if err != nil {
		tb.Fatalf("ParseRelativePath(%s) error = %v, want nil", path, err)
	}
	return parsed
}

// TestOpenRootRefusesWhatCannotBeARoot proves each refusal carries the identity
// that tells the caller whose fault it is. A product that cannot separate "you
// handed me an unset path" from "the disk said no" retries the wrong one.
func TestOpenRootRefusesWhatCannotBeARoot(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "regular"), []byte("regular"), 0o600); err != nil {
		t.Fatalf("WriteFile(regular) error = %v, want nil", err)
	}
	if err := os.MkdirAll(filepath.Join(base, "removed"), openRootDirectoryPermissions); err != nil {
		t.Fatalf("MkdirAll(removed) error = %v, want nil", err)
	}
	if err := os.Remove(filepath.Join(base, "removed")); err != nil {
		t.Fatalf("Remove(removed) error = %v, want nil", err)
	}

	// wantExcluded is load-bearing. core.ErrFilestoreSource reports true for
	// errors.Is(err, core.ErrFilestoreContract) because Source sits under
	// Contract in the identity hierarchy, so asserting only the wanted identity
	// would let a source refusal satisfy a contract case and vice versa. Each
	// case therefore names the identity it must NOT carry.
	cases := []struct {
		name         string
		path         func(tb testing.TB) core.AbsolutePath
		context      func(t *testing.T) context.Context
		wantErr      error
		wantExcluded error
		whyExist     string
	}{
		{
			name:         "unset absolute path is a contract refusal",
			path:         func(testing.TB) core.AbsolutePath { return core.AbsolutePath{} },
			wantErr:      core.ErrFilestoreContract,
			wantExcluded: core.ErrFilestoreSource,
			whyExist:     "the zero value is what an uninitialised config field hands over",
		},
		{
			name: "cancelled context is refused before the disk is touched",
			path: func(tb testing.TB) core.AbsolutePath { return openRootAbsolute(tb, base) },
			context: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			},
			wantErr:      context.Canceled,
			wantExcluded: core.ErrFilestoreContract,
		},
		{
			name: "expired deadline is refused before the disk is touched",
			path: func(tb testing.TB) core.AbsolutePath { return openRootAbsolute(tb, base) },
			context: func(t *testing.T) context.Context {
				// A zero timeout is already expired when it is created, so the
				// case proves the deadline branch without reading the clock.
				ctx, cancel := context.WithTimeout(t.Context(), 0)
				t.Cleanup(cancel)
				return ctx
			},
			wantErr:      context.DeadlineExceeded,
			wantExcluded: core.ErrFilestoreContract,
		},
		{
			name:     "absent directory is a source refusal",
			path:     func(tb testing.TB) core.AbsolutePath { return openRootAbsolute(tb, filepath.Join(base, "absent")) },
			wantErr:  core.ErrFilestoreSource,
			whyExist: "the configured store directory was never created",
		},
		{
			name:    "directory removed after it was configured is a source refusal",
			path:    func(tb testing.TB) core.AbsolutePath { return openRootAbsolute(tb, filepath.Join(base, "removed")) },
			wantErr: core.ErrFilestoreSource,
		},
		{
			name:     "regular file is a source refusal",
			path:     func(tb testing.TB) core.AbsolutePath { return openRootAbsolute(tb, filepath.Join(base, "regular")) },
			wantErr:  core.ErrFilestoreSource,
			whyExist: "a file where a directory was expected must not become a root",
		},
		{
			name: "path below a regular file is a source refusal",
			path: func(tb testing.TB) core.AbsolutePath {
				return openRootAbsolute(tb, filepath.Join(base, "regular", "child"))
			},
			wantErr: core.ErrFilestoreSource,
		},
		{
			name: "absent directory below an absent parent is a source refusal",
			path: func(tb testing.TB) core.AbsolutePath {
				return openRootAbsolute(tb, filepath.Join(base, "absent", "deeper"))
			},
			wantErr: core.ErrFilestoreSource,
		},
		{
			name: "absent directory many levels below an absent parent is a source refusal",
			path: func(tb testing.TB) core.AbsolutePath {
				return openRootAbsolute(tb, filepath.Join(base, "no", "such", "chain", "at", "all"))
			},
			wantErr: core.ErrFilestoreSource,
		},
		{
			name:    "absent directory whose name is only a dot segment is a source refusal",
			path:    func(tb testing.TB) core.AbsolutePath { return openRootAbsolute(tb, filepath.Join(base, "..absent")) },
			wantErr: core.ErrFilestoreSource,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			if tc.context != nil {
				ctx = tc.context(t)
			}
			root, err := filestore.OpenRoot(ctx, tc.path(t))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("OpenRoot() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantExcluded != nil && errors.Is(err, tc.wantExcluded) {
				t.Fatalf("OpenRoot() error = %v, want an error that is not %v", err, tc.wantExcluded)
			}
			// A refusal that still returned a handle would leak it: the caller
			// checked the error and never learned there was anything to close.
			if root != nil {
				openRootClose(t, root)
				t.Fatalf("OpenRoot() root = %v, want nil on refusal", root)
			}
		})
	}
}

// TestOpenRootFollowsASymlinkedDirectoryButStillConfines proves the two facts a
// symlinked store root has to satisfy together. Operators do point a store at a
// symlink, so refusing one would be wrong, but the confinement must belong to
// the resolved directory: a root that kept the link's parent as its boundary
// would let the link's siblings be read through it.
func TestOpenRootFollowsASymlinkedDirectoryButStillConfines(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.MkdirAll(target, openRootDirectoryPermissions); err != nil {
		t.Fatalf("MkdirAll(target) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(target, "inside"), []byte("inside"), 0o600); err != nil {
		t.Fatalf("WriteFile(inside) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(base, "sibling"), []byte("sibling"), 0o600); err != nil {
		t.Fatalf("WriteFile(sibling) error = %v, want nil", err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("os.Symlink() unavailable: %v", err)
	}

	root, err := filestore.OpenRoot(t.Context(), openRootAbsolute(t, link))
	if err != nil {
		t.Fatalf("OpenRoot(link) error = %v, want nil", err)
	}
	defer openRootClose(t, root)

	opened, err := root.Open("inside")
	if err != nil {
		t.Fatalf("root.Open(inside) error = %v, want nil", err)
	}
	if closeErr := opened.Close(); closeErr != nil {
		t.Errorf("Close() error = %v, want nil", closeErr)
	}
	if _, err := root.Open("../sibling"); err == nil {
		t.Fatal("root.Open(../sibling) error = nil, want refusal through a symlinked root")
	}
}

// TestOpenRootRefusesASymlinkToARegularFile proves the link is resolved before
// the directory decision, not after. A root taken from a link that points at a
// file would fail on first use instead of at the boundary that owns the check.
func TestOpenRootRefusesASymlinkToARegularFile(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v, want nil", err)
	}
	cases := []struct {
		name   string
		target string
	}{
		{name: "link to a regular file", target: target},
		{name: "link to an absent path", target: filepath.Join(base, "absent")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			link := filepath.Join(t.TempDir(), "link")
			if err := os.Symlink(tc.target, link); err != nil {
				t.Skipf("os.Symlink() unavailable: %v", err)
			}
			root, err := filestore.OpenRoot(t.Context(), openRootAbsolute(t, link))
			if !errors.Is(err, core.ErrFilestoreSource) {
				t.Fatalf("OpenRoot(%s) error = %v, want %v", tc.name, err, core.ErrFilestoreSource)
			}
			if root != nil {
				openRootClose(t, root)
				t.Fatalf("OpenRoot(%s) root = %v, want nil on refusal", tc.name, root)
			}
		})
	}
}
