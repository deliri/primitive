package filestore_test

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCrossDirectoryActivationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                      string
		install                   filestore.InstallMode
		landed, missing, occupied bool
		wantErr                   error
	}{
		{name: "create publishes staged inode into a different parent", install: filestore.InstallCreate},
		{name: "replace consumes occupied target in a different parent", install: filestore.InstallReplace, occupied: true},
		{name: "create cannot invent target parent", install: filestore.InstallCreate, missing: true, wantErr: core.ErrFilestoreActivation},
		{name: "replace cannot invent target parent", install: filestore.InstallReplace, missing: true, wantErr: core.ErrFilestoreActivation},
		{name: "recovery settles already linked cross-parent target", install: filestore.InstallCreate, landed: true},
		{name: "recovery recognizes already renamed cross-parent target", install: filestore.InstallReplace, landed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			if err := root.Mkdir("staging", 0o700); err != nil {
				t.Fatal(err)
			}
			if !tc.missing {
				if err := root.Mkdir("objects", 0o700); err != nil {
					t.Fatal(err)
				}
			}
			stageName := filepath.Join("staging", "stage")
			targetName := filepath.Join("objects", "target")
			if tc.occupied {
				if err := root.WriteFile(targetName, []byte{255, 0, 1}, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			staged := mustStage(t, root, stageName, "candidate")
			original, err := root.Lstat(stageName)
			if err != nil {
				t.Fatal(err)
			}
			var displaced *os.File
			if tc.occupied {
				displaced, err = root.Open(targetName)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := displaced.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			if tc.landed {
				if tc.install == filestore.InstallCreate {
					err = root.Link(stageName, targetName)
				} else {
					err = root.Rename(stageName, targetName)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			request := filestore.CommitRequest{Staged: staged, Target: mustRelativePath(t, targetName), Install: tc.install}
			var gotErr error
			if tc.landed {
				gotErr = filestore.Recover(t.Context(), request)
			} else {
				gotErr = filestore.Commit(t.Context(), request)
			}
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) || tc.missing && !errors.Is(gotErr, fs.ErrNotExist) {
				t.Fatalf("activation = %v, want %v and determinate native result", gotErr, tc.wantErr)
			}
			for attempt := range 2 {
				retained := targetName
				if tc.missing {
					retained = stageName
				}
				info, err := root.Lstat(retained)
				data, readErr := root.ReadFile(retained)
				if err != nil || readErr != nil || !os.SameFile(original, info) || info.Mode() != original.Mode() || info.Size() != original.Size() || !info.ModTime().Equal(original.ModTime()) || !bytes.Equal(data, []byte("candidate")) {
					t.Fatalf("retained entry = (%v,%v,%v,%v), want exact staged inode/bytes/metadata", info, data, err, readErr)
				}
				stageEntries, err := os.ReadDir(filepath.Join(directory, "staging"))
				wantStages := 0
				if tc.missing {
					wantStages = 1
				}
				if err != nil || len(stageEntries) != wantStages {
					t.Fatalf("stage namespace = (%v,%v), want %d", stageEntries, err, wantStages)
				}
				if tc.missing {
					if _, err := root.Lstat("objects"); !errors.Is(err, fs.ErrNotExist) {
						t.Fatalf("absent parent = %v, want native absence", err)
					}
				} else {
					entries, err := os.ReadDir(filepath.Join(directory, "objects"))
					if err != nil || len(entries) != 1 || entries[0].Name() != "target" {
						t.Fatalf("target namespace = (%v,%v), want target only", entries, err)
					}
				}
				if attempt == 0 {
					err := filestore.Recover(t.Context(), request)
					if !errors.Is(err, tc.wantErr) || tc.missing && !errors.Is(err, fs.ErrNotExist) {
						t.Fatalf("repeat recovery = %v, want %v", err, tc.wantErr)
					}
				}
			}
			if displaced != nil {
				var got [3]byte
				n, err := displaced.ReadAt(got[:], 0)
				if err != nil || n != len(got) || got != ([3]byte{255, 0, 1}) {
					t.Fatalf("displaced handle = (%v,%d,%v), want old bytes", got, n, err)
				}
			}
		})
	}
}
