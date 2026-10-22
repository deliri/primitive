package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestConfirmTreeDurableNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		files int
		link  bool
		want  error
	}{
		{name: "empty private tree"},
		{name: "single file", files: 1},
		{name: "multiple native batches", files: 130},
		{name: "symbolic link refused", link: true, want: fs.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if err := os.MkdirAll(filepath.Join(directory, "private", "a", "b"), 0755); err != nil {
				t.Fatal(err)
			}
			for i := range tc.files {
				name := filepath.Join(directory, "private", "a", "b", strconv.Itoa(i))
				if err := os.WriteFile(name, []byte{byte(i), byte(i >> 8)}, 0644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.link {
				if err := os.Symlink("a/b", filepath.Join(directory, "private", "link")); err != nil {
					t.Fatal(err)
				}
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			path, err := core.ParseRelativePath("private")
			if err != nil {
				t.Fatal(err)
			}
			got := ConfirmTreeDurable(t.Context(), Location{Root: root, Path: path})
			if !errors.Is(got, tc.want) {
				t.Fatalf("ConfirmTreeDurable = %v, want %v", got, tc.want)
			}
			for i := range tc.files {
				data, err := os.ReadFile(filepath.Join(directory, "private", "a", "b", strconv.Itoa(i)))
				if err != nil || len(data) != 2 || data[0] != byte(i) || data[1] != byte(i>>8) {
					t.Fatalf("member %d = %v/%v, want original bytes", i, data, err)
				}
			}
		})
	}
}

func TestConfirmTreeDurableAdmissionAndNativeFailures(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"nil context", "cancelled", "missing tree", "regular file", "symbolic root", "closed root"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if err := os.Mkdir(filepath.Join(directory, "tree"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "file"), []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("tree", filepath.Join(directory, "link")); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			ctx := t.Context()
			name := "tree"
			var want error
			switch mode {
			case "nil context":
				ctx = nil
				want = core.ErrNilContext
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				want = context.Canceled
			case "missing tree":
				name = "absent"
				want = fs.ErrNotExist
			case "regular file":
				name = "file"
				want = syscall.ENOTDIR
			case "symbolic root":
				name = "link"
				want = fs.ErrInvalid
			case "closed root":
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
				want = os.ErrClosed
			}
			path, err := core.ParseRelativePath(name)
			if err != nil {
				t.Fatal(err)
			}
			got := ConfirmTreeDurable(ctx, Location{Root: root, Path: path})
			if !errors.Is(got, want) {
				t.Fatalf("ConfirmTreeDurable = %v, want %v", got, want)
			}
		})
	}
	if got := ConfirmTreeDurable(t.Context(), Location{}); !errors.Is(got, core.ErrFilestoreContract) {
		t.Fatalf("empty request = %v, want contract refusal", got)
	}
}

func TestTreeMemberSyncPreservesClosedHandleFailure(t *testing.T) {
	t.Parallel()
	file, err := os.CreateTemp(t.TempDir(), "closed")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	want := error(os.ErrClosed)
	if runtime.GOOS == "darwin" {
		connection, err := file.SyscallConn()
		if err != nil {
			t.Fatal(err)
		}
		want = connection.Control(func(uintptr) {})
		if want == nil {
			t.Fatal("closed native connection accepted")
		}
	}
	if got := syncTreeMember(file); !errors.Is(got, want) {
		t.Fatalf("syncTreeMember(closed) = %v, want native %v", got, want)
	}
}

// ConfirmTreeDurableNativeProofForTest binds the capability inventory to the
// native behavior tests without introducing a production testing seam.
func ConfirmTreeDurableNativeProofForTest(t *testing.T) {
	t.Run("tree", TestConfirmTreeDurableNativeLayerTriad)
	t.Run("refusals", TestConfirmTreeDurableAdmissionAndNativeFailures)
}
