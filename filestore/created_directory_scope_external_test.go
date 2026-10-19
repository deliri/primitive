package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestCreatedDirectoryScopeNativeLifetimeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		want       error
		files      int
		cancel     bool
		closeRoot  bool
		removeName bool
	}{
		{name: "successful empty directory is retained"},
		{name: "successful nested native files are retained", files: 3},
		{name: "consumer refusal removes empty owned directory", want: io.ErrClosedPipe},
		{name: "consumer refusal removes nested native files", files: 3, want: io.ErrClosedPipe},
		{name: "cancellation after acquisition removes owned directory", cancel: true, want: context.Canceled},
		{name: "cancellation after native writes preserves cause and removes tree", files: 3, cancel: true, want: context.Canceled},
		{name: "consumer closing borrowed root cannot acknowledge retention", closeRoot: true, want: fs.ErrClosed},
		{name: "removed output name cannot acknowledge retention", removeName: true, want: fs.ErrNotExist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			parent := requireTestRoot(t, directory)
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			var borrowed *os.Root
			request := filestore.CreatedDirectoryScopeRequest{
				Directory: filestore.DirectoryRequest{Location: filestore.Location{Root: parent, Path: mustRelativePath(t, "owned")}, Mode: 0750},
				Use: func(_ context.Context, root *os.Root) error {
					borrowed = root
					if tc.files > 0 {
						if err := root.Mkdir("nested", 0700); err != nil {
							return err
						}
						for _, name := range []string{"nested/first", "second", "third"}[:tc.files] {
							file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
							if err != nil {
								return err
							}
							_, writeErr := file.Write([]byte("native content"))
							if err := errors.Join(writeErr, file.Close()); err != nil {
								return err
							}
						}
					}
					if tc.cancel {
						cancel(io.ErrNoProgress)
					}
					if tc.closeRoot {
						return root.Close()
					}
					if tc.removeName {
						return parent.Remove("owned")
					}
					if errors.Is(tc.want, io.ErrClosedPipe) {
						return io.ErrClosedPipe
					}
					return nil
				},
			}
			gotErr := filestore.WithCreatedDirectoryScope(ctx, request)
			if !errors.Is(gotErr, tc.want) || tc.cancel && !errors.Is(gotErr, io.ErrNoProgress) {
				t.Fatalf("scope=%v, want %v and original cancellation cause when present", gotErr, tc.want)
			}
			if borrowed == nil {
				t.Fatal("native root was never lent")
			}
			if _, err := borrowed.Stat("."); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("borrowed root escaped completed scope: %v", err)
			}
			path := filepath.Join(directory, "owned")
			info, err := os.Stat(path)
			if tc.want != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("refused native tree=%v/%v, want absence", info, err)
				}
				return
			}
			if err != nil || !info.IsDir() || info.Mode().Perm() != 0750 {
				t.Fatalf("retained native directory=%v/%v, want actual directory mode 0750", info, err)
			}
			if tc.files > 0 {
				body, err := os.ReadFile(filepath.Join(path, "nested", "first"))
				if err != nil || !bytes.Equal(body, []byte("native content")) {
					t.Fatalf("retained native bytes=%q/%v", body, err)
				}
			}
		})
	}
}

func TestCreatedDirectoryScopePanicClosesAndRemovesNativeTree(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	parent := requireTestRoot(t, directory)
	var borrowed *os.Root
	func() {
		defer func() {
			if got := recover(); got != io.ErrClosedPipe {
				t.Fatalf("consumer panic=%v, want original native identity", got)
			}
		}()
		err := filestore.WithCreatedDirectoryScope(t.Context(), filestore.CreatedDirectoryScopeRequest{
			Directory: filestore.DirectoryRequest{Location: filestore.Location{Root: parent, Path: mustRelativePath(t, "owned")}, Mode: 0700},
			Use: func(_ context.Context, root *os.Root) error {
				borrowed = root
				if err := root.Mkdir("child", 0700); err != nil {
					t.Fatal(err)
				}
				panic(io.ErrClosedPipe)
			},
		})
		t.Fatalf("scope returned %v instead of propagating consumer panic", err)
	}()
	if borrowed == nil {
		t.Fatal("native root was never lent")
	}
	if _, err := borrowed.Stat("."); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("panic retained borrowed root: %v", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("panic left native tree: %v/%v", entries, err)
	}
}

func TestCreatedDirectoryScopeRefusesForeignNamespaceOnRetentionAndCleanup(t *testing.T) {
	t.Parallel()
	for _, refusal := range []error{nil, io.ErrClosedPipe} {
		t.Run(refusalName(refusal), func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			parent := requireTestRoot(t, directory)
			var borrowed *os.Root
			err := filestore.WithCreatedDirectoryScope(t.Context(), filestore.CreatedDirectoryScopeRequest{
				Directory: filestore.DirectoryRequest{Location: filestore.Location{Root: parent, Path: mustRelativePath(t, "owned")}, Mode: 0700},
				Use: func(_ context.Context, root *os.Root) error {
					borrowed = root
					if err := parent.Rename("owned", "original"); err != nil {
						return err
					}
					if err := parent.Mkdir("owned", 0700); err != nil {
						return err
					}
					if err := os.WriteFile(filepath.Join(directory, "owned", "foreign"), []byte("preserved"), 0600); err != nil {
						return err
					}
					return refusal
				},
			})
			if !errors.Is(err, core.ErrFilestoreConflict) || !errors.Is(err, core.ErrFilestoreCleanup) || refusal != nil && !errors.Is(err, refusal) {
				t.Fatalf("foreign namespace=%v, want original refusal plus native cleanup conflict", err)
			}
			if _, err := borrowed.Stat("."); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("foreign namespace retained original root: %v", err)
			}
			body, err := os.ReadFile(filepath.Join(directory, "owned", "foreign"))
			if err != nil || !bytes.Equal(body, []byte("preserved")) {
				t.Fatalf("foreign tree changed=%q/%v", body, err)
			}
			if info, err := os.Stat(filepath.Join(directory, "original")); err != nil || !info.IsDir() {
				t.Fatalf("moved native original=%v/%v", info, err)
			}
		})
	}
}

func refusalName(refusal error) string {
	if refusal == nil {
		return "successful consumer cannot retain a foreign directory"
	}
	return "failed consumer cannot remove a foreign directory"
}

func TestCreatedDirectoryScopeConcurrentOwnersCannotRemoveWinner(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	parent := requireTestRoot(t, directory)
	ctx, cancel := newFilesystemBackstop(t.Context(), t, 10*time.Second)
	defer cancel()
	const contenders = 8
	start := make(chan struct{})
	results := make(chan error, contenders)
	for range contenders {
		go func() {
			select {
			case <-start:
			case <-ctx.Done():
				results <- ctx.Err()
				return
			}
			results <- filestore.WithCreatedDirectoryScope(ctx, filestore.CreatedDirectoryScopeRequest{
				Directory: filestore.DirectoryRequest{Location: filestore.Location{Root: parent, Path: mustRelativePath(t, "owned")}, Mode: 0700},
				Use: func(_ context.Context, root *os.Root) error {
					file, err := root.OpenFile("winner", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
					if err != nil {
						return err
					}
					_, writeErr := file.Write([]byte("original winner"))
					return errors.Join(writeErr, file.Close())
				},
			})
		}()
	}
	close(start)
	winners, refused := 0, 0
	for range contenders {
		select {
		case err := <-results:
			if err == nil {
				winners++
			} else if errors.Is(err, core.ErrFilestoreConflict) && errors.Is(err, fs.ErrExist) {
				refused++
			} else {
				t.Errorf("native contender returned %v, want success or existence conflict", err)
			}
		case <-ctx.Done():
			t.Fatal("native directory contenders did not settle before the deadlock backstop")
		}
	}
	if winners != 1 || refused != contenders-1 {
		t.Fatalf("native winners/refusals=%d/%d, want 1/%d", winners, refused, contenders-1)
	}
	body, err := os.ReadFile(filepath.Join(directory, "owned", "winner"))
	if err != nil || !bytes.Equal(body, []byte("original winner")) {
		t.Fatalf("losing owners removed or changed winner=%q/%v", body, err)
	}
}
