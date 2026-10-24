//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestNamedPipeRequestPermissionDomain(t *testing.T) {
	t.Parallel()
	path, err := core.ParseAbsolutePath("/typed-native-fifo")
	if err != nil {
		t.Fatal(err)
	}
	for mode := range 1 << 9 {
		request := filestore.NamedPipeRequest{Path: path, Mode: fs.FileMode(mode)}
		if err := request.Validate(); err != nil {
			t.Fatalf("permission mode=%#o, want admitted native permissions: %v", mode, err)
		}
	}
	for bit := 9; bit < 32; bit++ {
		request := filestore.NamedPipeRequest{Path: path, Mode: 0o600 | fs.FileMode(1)<<bit}
		if err := request.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("non-permission bit=%d error=%v, want contract refusal", bit, err)
		}
	}
}

func TestCreateNamedPipeNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, mode := range []fs.FileMode{0, 0o600, 0o644} {
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path := filepath.Join(directory, "native-fifo")
			absolute, err := core.ParseAbsolutePath(path)
			if err != nil {
				t.Fatal(err)
			}
			request := filestore.NamedPipeRequest{Path: absolute, Mode: mode}
			if err := syscall.Mkfifo(path+".oracle", uint32(mode)); err != nil {
				t.Fatal(err)
			}
			oracle, err := os.Lstat(path + ".oracle")
			if err != nil {
				t.Fatal(err)
			}
			if err := filestore.CreateNamedPipe(t.Context(), request); err != nil {
				t.Fatalf("CreateNamedPipe(%v) = %v, want native FIFO", mode, err)
			}
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode()&fs.ModeNamedPipe == 0 || info.Mode().Perm() != oracle.Mode().Perm() {
				t.Fatalf("native mode=%v, want FIFO and independent native permissions %v for requested %v", info.Mode(), oracle.Mode(), mode)
			}
			before := info
			err = filestore.CreateNamedPipe(t.Context(), request)
			info, statErr := os.Lstat(path)
			if !errors.Is(err, core.ErrFilestoreConflict) || !errors.Is(err, fs.ErrExist) || statErr != nil || !os.SameFile(before, info) {
				t.Fatalf("duplicate FIFO=%v native=%v, want conflict preserving same inode", err, statErr)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 2 {
				t.Fatalf("native entries=%v/%v, want requested FIFO and independent oracle", entries, err)
			}
		})
	}
}

func TestCreateNamedPipeRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"absent absolute path", "mode carries directory bit", "mode carries setuid bit", "cancelled context preserves cause", "nil context", "occupied regular file", "missing parent", "parent denies creation"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path := filepath.Join(directory, "fifo")
			absolute, err := core.ParseAbsolutePath(path)
			if err != nil {
				t.Fatal(err)
			}
			request := filestore.NamedPipeRequest{Path: absolute, Mode: 0o600}
			ctx := t.Context()
			want := error(core.ErrFilestoreContract)
			var native error
			switch name {
			case "absent absolute path":
				request.Path = core.AbsolutePath{}
			case "mode carries directory bit":
				request.Mode |= fs.ModeDir
			case "mode carries setuid bit":
				request.Mode |= fs.ModeSetuid
			case "cancelled context preserves cause":
				cancelled, cancel := context.WithCancelCause(ctx)
				cancel(fs.ErrPermission)
				ctx = cancelled
				want, native = context.Canceled, fs.ErrPermission
			case "nil context":
				ctx = nil
				want = core.ErrNilContext
			case "occupied regular file":
				if err := os.WriteFile(path, []byte("kept"), 0o600); err != nil {
					t.Fatal(err)
				}
				want, native = core.ErrFilestoreConflict, fs.ErrExist
			case "missing parent":
				request.Path, err = core.ParseAbsolutePath(filepath.Join(directory, "missing", "fifo"))
				if err != nil {
					t.Fatal(err)
				}
				want, native = core.ErrFilestoreActivation, fs.ErrNotExist
			case "parent denies creation":
				if err := os.Chmod(directory, 0o500); err != nil {
					t.Fatal(err)
				}
				defer os.Chmod(directory, 0o700)
				want, native = core.ErrFilestoreActivation, fs.ErrPermission
			default:
				t.Fatalf("fixture name=%q, want declared refusal", name)
			}
			err = filestore.CreateNamedPipe(ctx, request)
			if !errors.Is(err, want) || native != nil && !errors.Is(err, native) {
				t.Fatalf("CreateNamedPipe = %v, want %v and native %v", err, want, native)
			}
			if name == "occupied regular file" {
				body, readErr := os.ReadFile(path)
				if readErr != nil || string(body) != "kept" {
					t.Fatalf("occupied bytes=%q/%v, want kept", body, readErr)
				}
			} else if _, statErr := os.Lstat(path); !errors.Is(statErr, fs.ErrNotExist) {
				t.Fatalf("refused creation entry=%v, want absent", statErr)
			}
		})
	}
}

func TestOpenNamedPipeReadWriteNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"native FIFO roundtrip", "confined link to FIFO"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if err := syscall.Mkfifo(filepath.Join(directory, "fifo"), 0o600); err != nil {
				t.Fatal(err)
			}
			path := "fifo"
			if name == "confined link to FIFO" {
				if err := os.Symlink("fifo", filepath.Join(directory, "link")); err != nil {
					t.Fatal(err)
				}
				path = "link"
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			relative, err := core.ParseRelativePath(path)
			if err != nil {
				t.Fatal(err)
			}
			file, err := filestore.OpenNamedPipeReadWrite(t.Context(), filestore.Location{Root: root, Path: relative})
			if err != nil {
				t.Fatalf("OpenNamedPipeReadWrite = %v, want native handle", err)
			}
			defer file.Close()
			info, err := file.Stat()
			if err != nil || info.Mode()&fs.ModeNamedPipe == 0 {
				t.Fatalf("held mode=%v/%v, want FIFO", info, err)
			}
			want := []byte{0, 255, 10, 47, 31}
			if count, err := file.Write(want); count != len(want) || err != nil {
				t.Fatalf("native write=%d/%v, want %d", count, err, len(want))
			}
			got := make([]byte, len(want))
			if count, err := io.ReadFull(file, got); count != len(want) || err != nil || !bytes.Equal(got, want) {
				t.Fatalf("native FIFO bytes=%v/%v, want %v", got, err, want)
			}
		})
	}
}

func TestOpenNamedPipeReadWriteRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"nil root", "unset relative path", "closed root", "regular file preserved", "directory", "missing entry", "escaping link", "FIFO denies open", "cancelled context preserves cause", "nil context"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			relative, err := core.ParseRelativePath("entry")
			if err != nil {
				t.Fatal(err)
			}
			location := filestore.Location{Root: root, Path: relative}
			path := filepath.Join(directory, "entry")
			ctx := t.Context()
			want := error(core.ErrFilestoreSource)
			var native error
			switch name {
			case "nil root":
				location.Root = nil
				want = core.ErrFilestoreContract
			case "unset relative path":
				location.Path = core.RelativePath{}
				want = core.ErrFilestoreContract
			case "closed root":
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
				native = fs.ErrClosed
			case "regular file preserved":
				if err := os.WriteFile(path, []byte("kept"), 0o600); err != nil {
					t.Fatal(err)
				}
				native = fs.ErrInvalid
			case "directory":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
				native = syscall.EISDIR
			case "missing entry":
				native = fs.ErrNotExist
			case "escaping link":
				outside := t.TempDir()
				if err := syscall.Mkfifo(filepath.Join(outside, "outside"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(outside, "outside"), path); err != nil {
					t.Fatal(err)
				}
			case "FIFO denies open":
				if err := syscall.Mkfifo(path, 0); err != nil {
					t.Fatal(err)
				}
				native = fs.ErrPermission
			case "cancelled context preserves cause":
				cancelled, cancel := context.WithCancelCause(ctx)
				cancel(fs.ErrPermission)
				ctx = cancelled
				want, native = context.Canceled, fs.ErrPermission
			case "nil context":
				ctx = nil
				want = core.ErrNilContext
			default:
				t.Fatalf("fixture name=%q, want declared refusal", name)
			}
			var nativeEscape *os.PathError
			if name == "escaping link" {
				escaped, nativeErr := root.OpenFile(location.Path.String(), os.O_RDWR, 0)
				if escaped != nil {
					_ = escaped.Close()
					t.Fatalf("native escaping handle=%v, want rooted refusal", escaped)
				}
				if !errors.As(nativeErr, &nativeEscape) || nativeEscape.Err == nil {
					t.Fatalf("native escaping error=%v, want exact PathError cause", nativeErr)
				}
			}
			file, err := filestore.OpenNamedPipeReadWrite(ctx, location)
			if file != nil {
				_ = file.Close()
				t.Fatalf("refused handle=%v, want nil", file)
			}
			if !errors.Is(err, want) || native != nil && !errors.Is(err, native) {
				t.Fatalf("OpenNamedPipeReadWrite = %v, want %v and native %v", err, want, native)
			}
			if name == "escaping link" {
				var pathErr *os.PathError
				if !errors.As(err, &pathErr) || pathErr.Path != nativeEscape.Path || pathErr.Op != nativeEscape.Op || !errors.Is(err, nativeEscape.Err) {
					t.Fatalf("escaping native path error=%v, want retained native %v", err, nativeEscape)
				}
			}
			if name == "regular file preserved" {
				got, readErr := os.ReadFile(path)
				if readErr != nil || string(got) != "kept" {
					t.Fatalf("refused regular bytes=%q/%v, want kept", got, readErr)
				}
			}
		})
	}
}
