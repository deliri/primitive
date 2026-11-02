//go:build linux || darwin

package gotoolchain

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/sys/unix"
)

func TestModuleClosureCacheFileLoanUsesExistingCustodyBoundary(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"regular", "symbolic_link", "directory", "fifo", "missing", "consumer_refusal"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path, err := core.ParseAbsolutePath(filepath.Join(root, "cache-entry"))
			if err != nil {
				t.Fatal(err)
			}
			body := []byte("native cache content\n")
			switch name {
			case "symbolic_link":
				target := filepath.Join(root, "target")
				if err := os.WriteFile(target, body, 0o600); err != nil {
					t.Fatal(err)
				}
				err = os.Symlink(target, path.String())
			case "directory":
				err = os.Mkdir(path.String(), 0o700)
			case "fifo":
				err = unix.Mkfifo(path.String(), 0o600)
			case "missing":
			default:
				err = os.WriteFile(path.String(), body, 0o600)
			}
			if err != nil {
				t.Fatal(err)
			}
			cause := errors.New("module cache consumer refused")
			calls := 0
			err = withModuleCacheFile(t.Context(), path, func(source io.Reader) error {
				calls++
				got, readErr := io.ReadAll(source)
				if readErr != nil {
					return readErr
				}
				if string(got) != string(body) {
					t.Fatalf("cache content=%q, want %q", got, body)
				}
				if name == "consumer_refusal" {
					return cause
				}
				return nil
			})
			switch name {
			case "regular":
				if err != nil || calls != 1 {
					t.Fatalf("regular loan=%d/%v, want one complete native read", calls, err)
				}
			case "consumer_refusal":
				if !errors.Is(err, cause) || calls != 1 {
					t.Fatalf("consumer=%d/%v, want exact refusal", calls, err)
				}
			case "missing":
				if !errors.Is(err, fs.ErrNotExist) || calls != 0 {
					t.Fatalf("missing=%d/%v, want native absence before loan", calls, err)
				}
			default:
				if err == nil || calls != 0 {
					t.Fatalf("nonregular %s=%d/%v, want refusal before loan", name, calls, err)
				}
			}
		})
	}
}
