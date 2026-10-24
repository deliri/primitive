//go:build linux && (amd64 || arm64)

package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestFilesystemDurabilityAdmission(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		want error
	}{
		{name: "nil context", want: core.ErrNilContext},
		{name: "nil use", want: core.ErrFilestoreContract},
		{name: "unset location", want: core.ErrFilestoreContract},
		{name: "unset path", want: core.ErrFilestoreContract},
		{name: "closed root", want: os.ErrClosed},
		{name: "missing directory", want: fs.ErrNotExist},
		{name: "regular file", want: fs.ErrInvalid},
		{name: "directory symlink", want: fs.ErrInvalid},
		{name: "dangling symlink", want: fs.ErrInvalid},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if err := os.Mkdir(filepath.Join(directory, "directory"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "file"), []byte("untouched"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("directory", filepath.Join(directory, "link")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("missing", filepath.Join(directory, "dangling")); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			path, err := core.ParseRelativePath("directory")
			if err != nil {
				t.Fatal(err)
			}
			uses := 0
			request := FilesystemDurabilityRequest{Location: Location{Root: root, Path: path}, Use: func(context.Context) error { uses++; return nil }}
			ctx := t.Context()
			switch row.name {
			case "nil context":
				ctx = nil
			case "nil use":
				request.Use = nil
			case "unset location":
				request.Location = Location{}
			case "unset path":
				request.Location.Path = core.RelativePath{}
			case "closed root":
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			case "missing directory", "regular file", "directory symlink", "dangling symlink":
				name := "missing"
				switch row.name {
				case "regular file":
					name = "file"
				case "directory symlink":
					name = "link"
				case "dangling symlink":
					name = "dangling"
				}
				request.Location.Path, err = core.ParseRelativePath(name)
				if err != nil {
					t.Fatal(err)
				}
			}
			got := WithFilesystemDurability(ctx, request)
			if !errors.Is(got, row.want) || uses != 0 {
				t.Fatalf("refusal=%v callback uses=%d, want %v and no execution", got, uses, row.want)
			}
			data, err := os.ReadFile(filepath.Join(directory, "file"))
			if err != nil || string(data) != "untouched" {
				t.Fatalf("refused bytes=%q/%v, want original native bytes", data, err)
			}
		})
	}
}
