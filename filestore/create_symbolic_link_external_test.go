package filestore_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestCreateSymbolicLinkNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, target string
	}{
		{name: "positive relative target bytes survive redundant components", target: "./referent/../referent"},
		{name: "positive absolute target stays outside the capability", target: "absolute"},
		{name: "neutral dangling target needs no referent", target: "missing/../still-missing"},
		{name: "positive opaque punctuation is not a URL or path grammar", target: "opaque:+:@ [1]"},
		{name: "positive outside relative target is not traversed", target: "../outside"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			rootDirectory := filepath.Join(directory, "root")
			if err := os.Mkdir(rootDirectory, 0o700); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(directory, "outside")
			wantBytes := []byte{0, 255, 47, 8}
			if err := os.WriteFile(outside, wantBytes, 0o600); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(rootDirectory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = root.Close() })
			raw := tc.target
			if raw == "absolute" {
				raw = outside
			}
			target, err := filestore.ParseSymbolicLinkTarget(raw)
			if err != nil {
				t.Fatal(err)
			}
			request := filestore.SymbolicLinkRequest{Location: symbolicLinkFixtureLocation(t, root, "link"), Target: target}
			if err := filestore.CreateSymbolicLink(t.Context(), request); err != nil {
				t.Fatalf("CreateSymbolicLink(%q) = %v, want nil", raw, err)
			}
			got, err := os.Readlink(filepath.Join(rootDirectory, "link"))
			info, statErr := os.Lstat(filepath.Join(rootDirectory, "link"))
			if err != nil || statErr != nil || got != raw || info.Mode()&fs.ModeSymlink == 0 {
				t.Fatalf("native link = (%q,%v,%v), want exact %q and symbolic-link mode", got, err, statErr, raw)
			}
			body, err := os.ReadFile(outside)
			if err != nil || string(body) != string(wantBytes) {
				t.Fatalf("outside bytes = (%v,%v), want unchanged %v", body, err, wantBytes)
			}
			entries, err := os.ReadDir(rootDirectory)
			if err != nil || len(entries) != 1 || entries[0].Name() != "link" {
				t.Fatalf("created entries = (%v,%v), want only link", entries, err)
			}
		})
	}
}

func TestCreateSymbolicLinkRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		setup  func(*testing.T, string, *os.Root, *filestore.SymbolicLinkRequest) context.Context
		want   error
		native error
	}{
		{name: "negative occupied file preserves bytes and native conflict", setup: func(t *testing.T, directory string, _ *os.Root, _ *filestore.SymbolicLinkRequest) context.Context {
			if err := os.WriteFile(filepath.Join(directory, "link"), []byte("kept"), 0o600); err != nil {
				t.Fatal(err)
			}
			return t.Context()
		}, want: core.ErrFilestoreConflict, native: fs.ErrExist},
		{name: "negative occupied dangling link preserves old target", setup: func(t *testing.T, directory string, _ *os.Root, _ *filestore.SymbolicLinkRequest) context.Context {
			if err := os.Symlink("old-missing", filepath.Join(directory, "link")); err != nil {
				t.Fatal(err)
			}
			return t.Context()
		}, want: core.ErrFilestoreConflict, native: fs.ErrExist},
		{name: "negative occupied directory is never replaced", setup: func(t *testing.T, directory string, _ *os.Root, _ *filestore.SymbolicLinkRequest) context.Context {
			if err := os.Mkdir(filepath.Join(directory, "link"), 0o700); err != nil {
				t.Fatal(err)
			}
			return t.Context()
		}, want: core.ErrFilestoreConflict, native: fs.ErrExist},
		{name: "negative nil root refuses before namespace", setup: func(t *testing.T, _ string, _ *os.Root, r *filestore.SymbolicLinkRequest) context.Context {
			r.Location.Root = nil
			return t.Context()
		}, want: core.ErrFilestoreContract},
		{name: "negative zero path refuses before namespace", setup: func(t *testing.T, _ string, _ *os.Root, r *filestore.SymbolicLinkRequest) context.Context {
			r.Location.Path = core.RelativePath{}
			return t.Context()
		}, want: core.ErrFilestoreContract},
		{name: "negative root entry is not a mutable destination", setup: func(t *testing.T, _ string, root *os.Root, r *filestore.SymbolicLinkRequest) context.Context {
			r.Location = symbolicLinkFixtureLocation(t, root, ".")
			return t.Context()
		}, want: core.ErrFilestoreContract},
		{name: "negative zero target refuses before namespace", setup: func(t *testing.T, _ string, _ *os.Root, r *filestore.SymbolicLinkRequest) context.Context {
			r.Target = filestore.SymbolicLinkTarget{}
			return t.Context()
		}, want: core.ErrFilestoreContract},
		{name: "negative closed root preserves native closed cause", setup: func(t *testing.T, _ string, root *os.Root, _ *filestore.SymbolicLinkRequest) context.Context {
			if err := root.Close(); err != nil {
				t.Fatal(err)
			}
			return t.Context()
		}, want: core.ErrFilestoreActivation, native: fs.ErrClosed},
		{name: "neutral canceled context cannot create a link", setup: func(t *testing.T, _ string, _ *os.Root, _ *filestore.SymbolicLinkRequest) context.Context {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			return ctx
		}, want: context.Canceled},
		{name: "neutral elapsed deadline cannot create a link", setup: func(t *testing.T, _ string, _ *os.Root, _ *filestore.SymbolicLinkRequest) context.Context {
			ctx, cancel := context.WithDeadline(t.Context(), time.Unix(1, 0))
			t.Cleanup(cancel)
			return ctx
		}, want: context.DeadlineExceeded},
		{name: "negative nil context cannot create a link", setup: func(_ *testing.T, _ string, _ *os.Root, _ *filestore.SymbolicLinkRequest) context.Context { return nil }, want: core.ErrNilContext},
		{name: "negative missing parent preserves native absence", setup: func(t *testing.T, _ string, root *os.Root, r *filestore.SymbolicLinkRequest) context.Context {
			r.Location = symbolicLinkFixtureLocation(t, root, "missing/link")
			return t.Context()
		}, want: core.ErrFilestoreActivation, native: fs.ErrNotExist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = root.Close() })
			target, err := filestore.ParseSymbolicLinkTarget("new-missing")
			if err != nil {
				t.Fatal(err)
			}
			r := filestore.SymbolicLinkRequest{Location: symbolicLinkFixtureLocation(t, root, "link"), Target: target}
			ctx := tc.setup(t, directory, root, &r)
			before, beforeErr := os.Lstat(filepath.Join(directory, "link"))
			got := filestore.CreateSymbolicLink(ctx, r)
			if !errors.Is(got, tc.want) || (tc.native != nil && !errors.Is(got, tc.native)) {
				t.Fatalf("CreateSymbolicLink = %v, want %v with native %v", got, tc.want, tc.native)
			}
			after, afterErr := os.Lstat(filepath.Join(directory, "link"))
			if errors.Is(beforeErr, fs.ErrNotExist) {
				if !errors.Is(afterErr, fs.ErrNotExist) {
					t.Fatalf("refused link stat = %v, want absent", afterErr)
				}
			} else if beforeErr != nil || afterErr != nil || !os.SameFile(before, after) {
				t.Fatalf("occupied entry identity = (%v,%v), want original inode retained", beforeErr, afterErr)
			} else if before.Mode().IsRegular() {
				body, err := os.ReadFile(filepath.Join(directory, "link"))
				if err != nil || string(body) != "kept" {
					t.Fatalf("occupied bytes = (%q,%v), want kept", body, err)
				}
			} else if before.Mode()&fs.ModeSymlink != 0 {
				text, err := os.Readlink(filepath.Join(directory, "link"))
				if err != nil || text != "old-missing" {
					t.Fatalf("occupied target = (%q,%v), want old-missing", text, err)
				}
			}
		})
	}
}

func TestCreateSymbolicLinkConfinesDestinationAncestors(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	inside, outside := filepath.Join(directory, "root"), filepath.Join(directory, "outside")
	for _, name := range []string{inside, outside} {
		if err := os.Mkdir(name, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(inside, "escape")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(inside)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	target, err := filestore.ParseSymbolicLinkTarget("opaque")
	if err != nil {
		t.Fatal(err)
	}
	got := filestore.CreateSymbolicLink(t.Context(), filestore.SymbolicLinkRequest{Location: symbolicLinkFixtureLocation(t, root, "escape/link"), Target: target})
	var native *os.LinkError
	if !errors.Is(got, core.ErrFilestoreActivation) || !errors.As(got, &native) {
		t.Fatalf("escaped create = %v, want activation and native LinkError", got)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside entries = (%v,%v), want none", entries, err)
	}
}

func TestCreateSymbolicLinkParentSyncFailureRetainsCreatedEntry(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("native directory-read refusal requires non-root Unix permissions")
	}
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	target, err := filestore.ParseSymbolicLinkTarget("../missing")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0o300); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chmod(directory, 0o700); err != nil {
			t.Errorf("restored parent mode = %v, want nil", err)
		}
	}()
	got := filestore.CreateSymbolicLink(t.Context(), filestore.SymbolicLinkRequest{Location: symbolicLinkFixtureLocation(t, root, "link"), Target: target})
	if !errors.Is(got, core.ErrFilestoreActivation) || !errors.Is(got, fs.ErrPermission) {
		t.Fatalf("post-create sync = %v, want activation and native permission refusal", got)
	}
	text, err := os.Readlink(filepath.Join(directory, "link"))
	if err != nil || text != target.String() {
		t.Fatalf("post-failure link = (%q,%v), want retained %q", text, err, target.String())
	}
}

func symbolicLinkFixtureLocation(t testing.TB, root *os.Root, raw string) filestore.Location {
	t.Helper()
	path, err := core.ParseRelativePath(raw)
	if err != nil {
		t.Fatal(err)
	}
	return filestore.Location{Root: root, Path: path}
}
