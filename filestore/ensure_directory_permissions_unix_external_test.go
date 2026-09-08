//go:build darwin || linux

package filestore_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Exhaust the owner permission domain at three distinct effect boundaries:
// changing an already acquired directory, acquiring a new one, and extending it.
// The named partial effect is part of the contract; failure is not rollback.
func TestEnsureDirectoryOwnerPermissionEffectsLayerTriad(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root bypasses native owner-permission refusals")
	}
	for _, bits := range []struct {
		mode                    fs.FileMode
		wantAcquire, wantExtend bool
	}{
		{mode: 0o000}, {mode: 0o100}, {mode: 0o200}, {mode: 0o300},
		{mode: 0o400, wantAcquire: true}, {mode: 0o500, wantAcquire: true},
		{mode: 0o600, wantAcquire: true}, {mode: 0o700, wantAcquire: true, wantExtend: true},
	} {
		for _, effect := range []struct {
			name            string
			existing, chain bool
		}{
			{name: "existing final descriptor retains custody after chmod", existing: true},
			{name: "new final acquisition refuses without permission widening"},
			{name: "new chain retains exact partial prefix on native refusal", chain: true},
		} {
			t.Run(fmt.Sprintf("owner %03o/%s", bits.mode, effect.name), func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				root := requireTestRoot(t, directory)
				parent := filepath.Join(directory, "parent")
				payload := []byte{0, 255}
				if err := os.WriteFile(filepath.Join(directory, "neighbor"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
				neighborBefore, err := os.Stat(filepath.Join(directory, "neighbor"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Chmod(parent, 0o700); err != nil && !errors.Is(err, fs.ErrNotExist) {
						t.Error(err)
					}
				})
				var before fs.FileInfo
				if effect.existing {
					if err := os.Mkdir(parent, 0o700); err != nil {
						t.Fatal(err)
					}
					before, err = os.Stat(parent)
					if err != nil {
						t.Fatal(err)
					}
				}
				pathText := "parent"
				if effect.chain {
					pathText = filepath.Join(pathText, "child")
				}
				request := filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, pathText)}, Mode: bits.mode}
				var wantErr error
				if bits.mode == 0 {
					wantErr = core.ErrFilestoreContract
				} else if !effect.existing && (!bits.wantAcquire || (effect.chain && !bits.wantExtend)) {
					wantErr = core.ErrFilestoreActivation
				}
				// An independent native fixture pins capability refusal without relying on
				// Filestore's wrapping or its directory-chain implementation.
				if bits.mode != 0 && !effect.existing {
					oracle := t.TempDir()
					oracleParent := filepath.Join(oracle, "parent")
					if err := os.Mkdir(oracleParent, 0o700); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := os.Chmod(oracleParent, 0o700); err != nil {
							t.Error(err)
						}
					})
					if err := os.Chmod(oracleParent, bits.mode); err != nil {
						t.Fatal(err)
					}
					file, nativeErr := os.OpenFile(oracleParent, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NONBLOCK, 0)
					if bits.wantAcquire {
						if nativeErr != nil {
							t.Fatal(nativeErr)
						}
						if err := file.Close(); err != nil {
							t.Fatal(err)
						}
					} else if !errors.Is(nativeErr, fs.ErrPermission) {
						if file != nil {
							_ = file.Close()
						}
						t.Fatalf("native acquire = %v, want permission", nativeErr)
					}
					if effect.chain && bits.wantAcquire {
						nativeErr := os.Mkdir(filepath.Join(oracleParent, "child"), bits.mode)
						if bits.wantExtend {
							if nativeErr != nil {
								t.Fatal(nativeErr)
							}
						} else if !errors.Is(nativeErr, fs.ErrPermission) {
							t.Fatalf("native extend = %v, want permission", nativeErr)
						}
					}
				}
				gotErr := filestore.EnsureDirectory(t.Context(), request)
				if !errors.Is(gotErr, wantErr) {
					t.Fatalf("ensure = %v, want %v", gotErr, wantErr)
				}
				if errors.Is(wantErr, core.ErrFilestoreActivation) {
					var native *fs.PathError
					if !errors.Is(gotErr, fs.ErrPermission) || !errors.As(gotErr, &native) {
						t.Fatalf("refusal = %v, want activation/native permission", gotErr)
					}
				}
				entries, err := os.ReadDir(directory)
				if err != nil {
					t.Fatal(err)
				}
				wantParent := effect.existing || bits.mode != 0
				wantEntries := 1
				if wantParent {
					wantEntries++
				}
				if len(entries) != wantEntries {
					t.Fatalf("retained root = %v, want %d exact entries", entries, wantEntries)
				}
				if wantParent {
					after, err := os.Stat(parent)
					if err != nil {
						t.Fatal(err)
					}
					wantMode := bits.mode
					if bits.mode == 0 {
						wantMode = 0o700
					}
					if !after.IsDir() || after.Mode().Perm() != wantMode || (before != nil && (!os.SameFile(before, after) || before.ModTime().UnixNano() != after.ModTime().UnixNano())) {
						t.Fatalf("retained parent = %v, want exact mode %#o and original identity when existing", after, wantMode)
					}
					if err := os.Chmod(parent, 0o700); err != nil {
						t.Fatal(err)
					}
					children, err := os.ReadDir(parent)
					if err != nil {
						t.Fatal(err)
					}
					wantChildren := 0
					if effect.chain && gotErr == nil {
						wantChildren = 1
					}
					if len(children) != wantChildren {
						t.Fatalf("partial prefix children = %v, want %d", children, wantChildren)
					}
					if wantChildren == 1 {
						child, err := os.Stat(filepath.Join(parent, "child"))
						if err != nil || children[0].Name() != "child" || !child.IsDir() || child.Mode().Perm() != bits.mode {
							t.Fatalf("created child = (%v,%v), want exact mode %#o", child, err, bits.mode)
						}
					}
				}
				neighborAfter, err := os.Stat(filepath.Join(directory, "neighbor"))
				if err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(filepath.Join(directory, "neighbor"))
				if err != nil || !bytes.Equal(got, payload) || !os.SameFile(neighborBefore, neighborAfter) || neighborBefore.Mode() != neighborAfter.Mode() || neighborBefore.ModTime().UnixNano() != neighborAfter.ModTime().UnixNano() {
					t.Fatalf("neighbor metadata or bytes changed: %v", err)
				}
			})
		}
	}
}
