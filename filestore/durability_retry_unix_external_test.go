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

// Native search/write permission admits unlink and creation; missing directory
// read permission refuses the real parent fsync acquisition. No fake syscall
// or process-global hook stands in for the failure being repaired.
func TestDurableRetriesCannotTurnUnsynchronizedNamespaceIntoSuccess(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the native parent read-permission refusal")
	}
	for _, tc := range []struct {
		name          string
		makeDirectory bool
		startAbsent   bool
		execute       func(context.Context, filestore.Location) error
		wantErr       core.ErrorIdentity
	}{
		{name: "leaf removal retry still needs parent fsync", execute: func(ctx context.Context, location filestore.Location) error {
			return filestore.Remove(ctx, filestore.RemovalRequest{Location: location, Durability: filestore.RemovalDurabilityDurable})
		}, wantErr: core.ErrFilestoreCleanup},
		{name: "tree removal retry still needs parent fsync", makeDirectory: true, execute: func(ctx context.Context, location filestore.Location) error {
			return filestore.RemoveTree(ctx, filestore.TreeRemovalRequest{Location: location})
		}, wantErr: core.ErrFilestoreCleanup},
		{name: "directory creation retry still needs parent fsync", startAbsent: true, execute: func(ctx context.Context, location filestore.Location) error {
			return filestore.EnsureDirectory(ctx, filestore.DirectoryRequest{Location: location, Mode: 0o700})
		}, wantErr: core.ErrFilestoreActivation},
		{name: "existing directory still needs namespace fsync", makeDirectory: true, execute: func(ctx context.Context, location filestore.Location) error {
			return filestore.EnsureDirectory(ctx, filestore.DirectoryRequest{Location: location, Mode: 0o700})
		}, wantErr: core.ErrFilestoreActivation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			absolute, err := core.ParseAbsolutePath(directory)
			if err != nil {
				t.Fatalf("ParseAbsolutePath() error = %v, want nil", err)
			}
			path, err := core.ParseRelativePath("subject")
			if err != nil {
				t.Fatalf("ParseRelativePath() error = %v, want nil", err)
			}
			if !tc.startAbsent {
				if tc.makeDirectory {
					err = os.Mkdir(filepath.Join(directory, path.String()), 0o700)
				} else {
					err = os.WriteFile(filepath.Join(directory, path.String()), []byte{0, 255, 31}, 0o600)
				}
				if err != nil {
					t.Fatalf("native fixture creation error = %v, want nil", err)
				}
			}
			result, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: absolute, Use: func(ctx context.Context, root *os.Root) error {
				if err := os.Chmod(directory, 0o300); err != nil {
					return err
				}
				defer func() {
					if err := os.Chmod(directory, 0o700); err != nil {
						t.Errorf("fixture permission restoration error = %v, want nil", err)
					}
				}()
				location := filestore.Location{Root: root, Path: path}
				first := tc.execute(ctx, location)
				retry := tc.execute(ctx, location)
				if !errors.Is(first, tc.wantErr) || !errors.Is(first, fs.ErrPermission) || !errors.Is(retry, tc.wantErr) || !errors.Is(retry, fs.ErrPermission) {
					t.Fatalf("durable attempts = (%v, %v), want %v and native permission refusal on both attempts", first, retry, tc.wantErr)
				}
				if err := os.Chmod(directory, 0o700); err != nil {
					return err
				}
				if recovered := tc.execute(ctx, location); recovered != nil {
					t.Fatalf("durable retry after native permission repair error = %v, want nil", recovered)
				}
				return nil
			}})
			if gotErr := errors.Join(err, result.Validate(), result.OperationError, result.CleanupError); gotErr != nil {
				t.Fatalf("owned native root scope error = %v, want nil", gotErr)
			}
		})
	}
}
