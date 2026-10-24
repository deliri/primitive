package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestCreateHardLinkNativeNamespaceLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, source, target, occupied string
		wantErr, wantCause             error
		closed, nilRoot, stopped       bool
	}{
		{name: "positive regular file shares its native inode", source: "file", target: "copy"},
		{name: "positive empty file remains a hard link", source: "empty", target: "copy"},
		{name: "positive different parents share one inode", source: "file", target: "nested/copy"},
		{name: "positive dangling symbolic link is itself linked", source: "dangling", target: "copy"},
		{name: "positive outside symbolic target is never followed", source: "outside-link", target: "copy"},
		{name: "negative occupied file is preserved", source: "file", target: "copy", occupied: "file", wantErr: core.ErrFilestoreConflict, wantCause: fs.ErrExist},
		{name: "negative occupied directory is preserved", source: "file", target: "nested", wantErr: core.ErrFilestoreConflict, wantCause: fs.ErrExist},
		{name: "negative occupied link is preserved", source: "file", target: "dangling", wantErr: core.ErrFilestoreConflict, wantCause: fs.ErrExist},
		{name: "neutral existing source name is not replaced", source: "file", target: "file", wantErr: core.ErrFilestoreConflict, wantCause: fs.ErrExist},
		{name: "negative missing source preserves native absence", source: "missing", target: "copy", wantErr: core.ErrFilestoreActivation, wantCause: fs.ErrNotExist},
		{name: "negative missing target parent is not invented", source: "file", target: "missing/copy", wantErr: core.ErrFilestoreActivation, wantCause: fs.ErrNotExist},
		{name: "negative destination ancestor cannot escape", source: "file", target: "outside-directory/copy", wantErr: core.ErrFilestoreActivation},
		{name: "negative source ancestor cannot escape", source: "outside-directory/note", target: "copy", wantErr: core.ErrFilestoreActivation},
		{name: "negative directory source remains native refusal", source: "nested", target: "copy", wantErr: core.ErrFilestoreActivation},
		{name: "negative absent source nominal is refused", target: "copy", wantErr: core.ErrFilestoreContract},
		{name: "negative absent target nominal is refused", source: "file", wantErr: core.ErrFilestoreContract},
		{name: "negative root source is refused", source: ".", target: "copy", wantErr: core.ErrFilestoreContract},
		{name: "negative root target is refused", source: "file", target: ".", wantErr: core.ErrFilestoreContract},
		{name: "negative absent capability is refused", source: "file", target: "copy", nilRoot: true, wantErr: core.ErrFilestoreContract},
		{name: "negative closed capability preserves closed cause", source: "file", target: "copy", closed: true, wantErr: core.ErrFilestoreActivation, wantCause: os.ErrClosed},
		{name: "negative stopped context cannot create", source: "file", target: "copy", stopped: true, wantErr: context.Canceled},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			inside, outside := filepath.Join(directory, "root"), filepath.Join(directory, "outside")
			for _, path := range []string{inside, outside, filepath.Join(inside, "nested")} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			payload := []byte{0, 255, 1, 0, '\n'}
			for _, item := range []struct {
				name string
				data []byte
			}{{filepath.Join(inside, "file"), payload}, {filepath.Join(inside, "empty"), nil}, {filepath.Join(outside, "note"), []byte("outside unchanged")}} {
				if err := os.WriteFile(item.name, item.data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for _, item := range []struct{ name, target string }{{"dangling", "absent"}, {"outside-link", filepath.Join(outside, "note")}, {"outside-directory", outside}} {
				if err := os.Symlink(item.target, filepath.Join(inside, item.name)); err != nil {
					t.Fatal(err)
				}
			}
			if row.occupied != "" {
				if err := os.WriteFile(filepath.Join(inside, row.target), []byte("occupied unchanged"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			root, err := os.OpenRoot(inside)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = root.Close() })
			var source, target core.RelativePath
			if row.source != "" {
				source, err = core.ParseRelativePath(row.source)
				if err != nil {
					t.Fatal(err)
				}
			}
			if row.target != "" {
				target, err = core.ParseRelativePath(row.target)
				if err != nil {
					t.Fatal(err)
				}
			}
			request := filestore.HardLinkRequest{Location: filestore.Location{Root: root, Path: source}, Target: target}
			if row.nilRoot {
				request.Location.Root = nil
			}
			if row.closed {
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			}
			ctx := t.Context()
			if row.stopped {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			before, beforeErr := os.Lstat(filepath.Join(inside, row.target))
			gotErr := filestore.CreateHardLink(ctx, request)
			if !errors.Is(gotErr, row.wantErr) || row.wantCause != nil && !errors.Is(gotErr, row.wantCause) {
				t.Fatalf("CreateHardLink() = %v, want %v with %v", gotErr, row.wantErr, row.wantCause)
			}
			if errors.Is(row.wantErr, core.ErrFilestoreActivation) || errors.Is(row.wantErr, core.ErrFilestoreConflict) {
				var native *os.LinkError
				if !errors.As(gotErr, &native) {
					t.Fatalf("native cause = %v, want Go LinkError", gotErr)
				}
			}
			if gotErr == nil {
				original, err := os.Lstat(filepath.Join(inside, row.source))
				if err != nil {
					t.Fatal(err)
				}
				linked, err := os.Lstat(filepath.Join(inside, row.target))
				if err != nil || !os.SameFile(original, linked) || linked.Mode() != original.Mode() || linked.Size() != original.Size() {
					t.Fatalf("linked inode = (%v,%v), want same inode/mode/size as %v", linked, err, original)
				}
				if original.Mode()&fs.ModeSymlink != 0 {
					want, err := os.Readlink(filepath.Join(inside, row.source))
					got, readErr := os.Readlink(filepath.Join(inside, row.target))
					if err != nil || readErr != nil || got != want {
						t.Fatalf("link bytes = (%q,%v), want %q/%v", got, readErr, want, err)
					}
				}
			} else if row.target != "" {
				after, err := os.Lstat(filepath.Join(inside, row.target))
				if beforeErr == nil && (err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size()) || errors.Is(beforeErr, fs.ErrNotExist) && !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("refused target = (%v,%v), want unchanged (%v,%v)", after, err, before, beforeErr)
				}
			}
			outsideBytes, err := os.ReadFile(filepath.Join(outside, "note"))
			if err != nil || !bytes.Equal(outsideBytes, []byte("outside unchanged")) {
				t.Fatalf("outside = (%q,%v), want unchanged", outsideBytes, err)
			}
		})
	}
}

func TestCreateHardLinkParentSyncRefusalRetainsCreatedEntry(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "source"), []byte("existing inode"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Acquire the capability before removing read permission. Both native names
	// are direct children: traversing a nested unreadable parent would refuse
	// before linkat and would never reach the post-create acknowledgement.
	parent := directory
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	if err := os.Chmod(parent, 0o300); err != nil {
		t.Fatal(err)
	}
	source, _ := core.ParseRelativePath("source")
	target, _ := core.ParseRelativePath("linked")
	err = filestore.CreateHardLink(t.Context(), filestore.HardLinkRequest{Location: filestore.Location{Root: root, Path: source}, Target: target})
	if !errors.Is(err, core.ErrFilestoreActivation) || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("parent acknowledgement = %v, want activation with native permission refusal", err)
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	old, err := os.Lstat(filepath.Join(directory, "source"))
	if err != nil {
		t.Fatal(err)
	}
	created, err := os.Lstat(filepath.Join(parent, "linked"))
	if err != nil || !os.SameFile(old, created) {
		t.Fatalf("created entry after refused acknowledgement = (%v,%v), want existing source inode %v", created, err, old)
	}
}
