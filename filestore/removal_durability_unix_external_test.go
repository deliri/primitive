//go:build unix

package filestore_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestRemovalDurabilityControlsNativeParentSynchronization(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the native directory read-permission refusal")
	}
	for _, tc := range []struct {
		name       string
		durability filestore.RemovalDurability
		want       error
		removed    bool
	}{
		{name: "ephemeral removal needs no parent read permission", durability: filestore.RemovalDurabilityEphemeral, removed: true},
		{name: "durable removal exposes refused parent synchronization", durability: filestore.RemovalDurabilityDurable, want: core.ErrFilestoreCleanup, removed: true},
		{name: "zero durability cannot remove native bytes", want: core.ErrFilestoreContract},
		{name: "future durability cannot remove native bytes", durability: filestore.RemovalDurability(255), want: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			absolute, err := core.ParseAbsolutePath(directory)
			if err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseRelativePath("subject")
			if err != nil {
				t.Fatal(err)
			}
			result, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: absolute, Use: func(ctx context.Context, root *os.Root) error {
				// The native provider itself is the subject. Directory write/search
				// permission permits unlink while read permission refuses syncParent.
				if err := os.WriteFile(filepath.Join(directory, path.String()), []byte{0, 255, 31}, 0o600); err != nil {
					return err
				}
				if err := os.Chmod(directory, 0o300); err != nil {
					return err
				}
				defer func() {
					if err := os.Chmod(directory, 0o700); err != nil {
						t.Errorf("restore fixture permissions=%v,want nil", err)
					}
				}()
				got := filestore.Remove(ctx, filestore.RemovalRequest{Location: filestore.Location{Root: root, Path: path}, Durability: tc.durability})
				if !errors.Is(got, tc.want) {
					t.Fatalf("native removal=%v,want %v", got, tc.want)
				}
				if tc.durability == filestore.RemovalDurabilityDurable && !errors.Is(got, fs.ErrPermission) {
					t.Fatalf("durable cleanup=%v,want native permission identity", got)
				}
				body, err := os.ReadFile(filepath.Join(directory, path.String()))
				if tc.removed {
					if !errors.Is(err, fs.ErrNotExist) {
						t.Fatalf("removed subject read=%v,want absent", err)
					}
					return nil
				}
				if err != nil || len(body) != 3 || body[0] != 0 || body[1] != 255 || body[2] != 31 {
					t.Fatalf("refused subject=(%v,%v),want unchanged native bytes", body, err)
				}
				return nil
			}})
			if err := errors.Join(err, result.Validate(), result.OperationError, result.CleanupError); err != nil {
				t.Fatalf("native root scope=%v,want owned cleanup", err)
			}
		})
	}
}
