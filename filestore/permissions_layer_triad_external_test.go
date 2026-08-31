package filestore_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestPermissionEffectLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive rooted file is sealed to the exact requested mode", func(t *testing.T) {
		t.Parallel()
		root := openPermissionRoot(t)
		path := mustRelativePath(t, "subject")
		writePermissionFixture(t, root, path)

		gotErr := filestore.SetPermissions(t.Context(), filestore.PermissionRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o400})
		if gotErr != nil {
			t.Fatalf("filestore.SetPermissions(0400) error = %v, want nil", gotErr)
		}
		file, openErr := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: filestore.Location{Root: root, Path: path}})
		if openErr != nil {
			t.Fatalf("filestore.OpenRead(sealed file) error = %v, want nil", openErr)
		}
		info, statErr := file.Stat()
		closeErr := file.Close()
		if statErr != nil || closeErr != nil || info.Mode().Perm() != 0o400 {
			t.Fatalf("sealed file mode/stat/close = (%#o, %v, %v), want (0400, nil, nil)", info.Mode().Perm(), statErr, closeErr)
		}
	})

	t.Run("negative missing target retains typed not-found identity", func(t *testing.T) {
		t.Parallel()
		root := openPermissionRoot(t)
		gotErr := filestore.SetPermissions(t.Context(), filestore.PermissionRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "missing")}, Mode: 0o400})
		if !errors.Is(gotErr, core.ErrFilestoreActivation) {
			t.Fatalf("filestore.SetPermissions(missing) error = %v, want errors.Is(..., %v)", gotErr, core.ErrFilestoreActivation)
		}
	})

	t.Run("neutral invalid zero request performs no rooted effect", func(t *testing.T) {
		t.Parallel()
		gotErr := filestore.SetPermissions(t.Context(), filestore.PermissionRequest{})
		if !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("filestore.SetPermissions(zero) error = %v, want errors.Is(..., %v)", gotErr, core.ErrFilestoreContract)
		}
	})
}

func openPermissionRoot(t testing.TB) *os.Root {
	t.Helper()
	root, err := filestore.OpenRoot(t.Context(), openRootAbsolute(t, t.TempDir()))
	if err != nil {
		t.Fatalf("filestore.OpenRoot() permission fixture error = %v, want nil", err)
	}
	t.Cleanup(func() { openRootClose(t, root) })
	return root
}

func writePermissionFixture(t testing.TB, root *os.Root, path core.RelativePath) {
	t.Helper()
	_, err := filestore.Write(t.Context(), filestore.WriteRequest{
		Source:    strings.NewReader("sealed"),
		Location:  filestore.Location{Root: root, Path: path},
		Temporary: permissionRelativePath(t, ".subject-stage"),
		Mode:      0o600, Install: filestore.InstallCreate, MaximumBytes: permissionByteCount(t, 6),
	})
	if err != nil {
		t.Fatalf("filestore.Write() permission fixture error = %v, want nil", err)
	}
}

func permissionRelativePath(t testing.TB, value string) core.RelativePath {
	t.Helper()
	got, err := core.ParseRelativePath(value)
	if err != nil {
		t.Fatalf("core.ParseRelativePath(%q) permission fixture error = %v, want nil", value, err)
	}
	return got
}

func permissionByteCount(t testing.TB, value uint64) core.ByteCount {
	t.Helper()
	got, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) permission fixture error = %v, want nil", value, err)
	}
	return got
}
