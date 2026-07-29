package filestore_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type stageNamespaceState uint8

const (
	stageNamespaceOwned stageNamespaceState = iota + 1
	stageNamespaceMissing
	stageNamespaceForeign
)

type targetNamespaceState uint8

const (
	targetNamespaceAbsent targetNamespaceState = iota + 1
	targetNamespaceSameFile
	targetNamespaceForeignFile
	targetNamespaceDirectory
)

func TestCommitExhaustsOwnedAndHostileNamespaceStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                string
		stage               stageNamespaceState
		target              targetNamespaceState
		install             filestore.InstallMode
		wantErr             error
		wantTarget          string
		wantStage           string
		wantTargetDirectory bool
	}{
		{
			name:  "create with owned stage and absent target publishes candidate",
			stage: stageNamespaceOwned, target: targetNamespaceAbsent,
			install: filestore.InstallCreate, wantTarget: "candidate",
		},
		{
			name:  "create with owned stage and foreign target preserves both owners",
			stage: stageNamespaceOwned, target: targetNamespaceForeignFile,
			install: filestore.InstallCreate, wantErr: core.ErrFilestoreConflict,
			wantTarget: "foreign-target", wantStage: "candidate",
		},
		{
			name:  "create with owned stage and directory target preserves directory and stage",
			stage: stageNamespaceOwned, target: targetNamespaceDirectory,
			install: filestore.InstallCreate, wantErr: core.ErrFilestoreConflict,
			wantStage: "candidate", wantTargetDirectory: true,
		},
		{
			name:  "replace with owned stage and absent target publishes candidate",
			stage: stageNamespaceOwned, target: targetNamespaceAbsent,
			install: filestore.InstallReplace, wantTarget: "candidate",
		},
		{
			name:  "replace with owned stage and foreign target atomically replaces target",
			stage: stageNamespaceOwned, target: targetNamespaceForeignFile,
			install: filestore.InstallReplace, wantTarget: "candidate",
		},
		{
			name:  "replace with owned stage and directory target fails without consuming either entry",
			stage: stageNamespaceOwned, target: targetNamespaceDirectory,
			install: filestore.InstallReplace, wantErr: core.ErrFilestoreActivation,
			wantStage: "candidate", wantTargetDirectory: true,
		},
		{
			name:  "create with missing stage preserves absent target",
			stage: stageNamespaceMissing, target: targetNamespaceAbsent,
			install: filestore.InstallCreate, wantErr: core.ErrFilestoreActivation,
		},
		{
			name:  "replace with missing stage preserves foreign target",
			stage: stageNamespaceMissing, target: targetNamespaceForeignFile,
			install: filestore.InstallReplace, wantErr: core.ErrFilestoreActivation,
			wantTarget: "foreign-target",
		},
		{
			name:  "create with foreign replacement stage rejects ambiguous ownership",
			stage: stageNamespaceForeign, target: targetNamespaceAbsent,
			install: filestore.InstallCreate, wantErr: core.ErrFilestoreActivationIndeterminate,
			wantStage: "foreign-stage",
		},
		{
			name:  "replace with foreign replacement stage preserves both foreign names",
			stage: stageNamespaceForeign, target: targetNamespaceForeignFile,
			install: filestore.InstallReplace, wantErr: core.ErrFilestoreActivationIndeterminate,
			wantStage: "foreign-stage", wantTarget: "foreign-target",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			root := requireTestRoot(t, rootDirectory)
			staged := mustStage(t, root, ".stage", "candidate")
			prepareActivationNamespace(t, root, rootDirectory, staged, tc.stage, tc.target)
			gotErr := filestore.Commit(t.Context(), filestore.CommitRequest{
				Staged: staged, Target: mustRelativePath(t, "target"), Install: tc.install,
			})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("Commit() error = %v, want %v", gotErr, tc.wantErr)
				}
			} else if gotErr != nil {
				t.Fatalf("Commit() error = %v, want nil", gotErr)
			}
			requireActivationNamespace(
				t,
				rootDirectory,
				tc.wantStage,
				tc.wantTarget,
				tc.wantTargetDirectory,
			)
		})
	}
}

func TestRecoverExhaustsReachableAndHostileNamespaceStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		stage      stageNamespaceState
		target     targetNamespaceState
		install    filestore.InstallMode
		wantErr    error
		wantTarget string
		wantStage  string
	}{
		{
			name:  "create resumes before namespace activation",
			stage: stageNamespaceOwned, target: targetNamespaceAbsent,
			install: filestore.InstallCreate, wantTarget: "candidate",
		},
		{
			name:  "replace resumes before namespace activation",
			stage: stageNamespaceOwned, target: targetNamespaceAbsent,
			install: filestore.InstallReplace, wantTarget: "candidate",
		},
		{
			name:  "create finishes after target link but before stage unlink",
			stage: stageNamespaceOwned, target: targetNamespaceSameFile,
			install: filestore.InstallCreate, wantTarget: "candidate",
		},
		{
			name:  "create refuses foreign target while retaining owned stage",
			stage: stageNamespaceOwned, target: targetNamespaceForeignFile,
			install: filestore.InstallCreate, wantErr: core.ErrFilestoreConflict,
			wantTarget: "foreign-target", wantStage: "candidate",
		},
		{
			name:  "replace may intentionally overwrite foreign target while stage remains",
			stage: stageNamespaceOwned, target: targetNamespaceForeignFile,
			install: filestore.InstallReplace, wantTarget: "candidate",
		},
		{
			name:  "create with both names absent cannot claim activation",
			stage: stageNamespaceMissing, target: targetNamespaceAbsent,
			install: filestore.InstallCreate, wantErr: core.ErrFilestoreActivation,
		},
		{
			name:  "replace with both names absent cannot claim activation",
			stage: stageNamespaceMissing, target: targetNamespaceAbsent,
			install: filestore.InstallReplace, wantErr: core.ErrFilestoreActivation,
		},
		{
			name:  "create recognizes its activated target after stage disappearance",
			stage: stageNamespaceMissing, target: targetNamespaceSameFile,
			install: filestore.InstallCreate, wantTarget: "candidate",
		},
		{
			name:  "replace recognizes its activated target after stage disappearance",
			stage: stageNamespaceMissing, target: targetNamespaceSameFile,
			install: filestore.InstallReplace, wantTarget: "candidate",
		},
		{
			name:  "create rejects foreign target after stage disappearance",
			stage: stageNamespaceMissing, target: targetNamespaceForeignFile,
			install: filestore.InstallCreate, wantErr: core.ErrFilestoreActivationIndeterminate,
			wantTarget: "foreign-target",
		},
		{
			name:  "replace rejects foreign target after stage disappearance",
			stage: stageNamespaceMissing, target: targetNamespaceForeignFile,
			install: filestore.InstallReplace, wantErr: core.ErrFilestoreActivationIndeterminate,
			wantTarget: "foreign-target",
		},
		{
			name:  "create rejects foreign stage with absent target",
			stage: stageNamespaceForeign, target: targetNamespaceAbsent,
			install: filestore.InstallCreate, wantErr: core.ErrFilestoreActivationIndeterminate,
			wantStage: "foreign-stage",
		},
		{
			name:  "replace rejects foreign stage with absent target",
			stage: stageNamespaceForeign, target: targetNamespaceAbsent,
			install: filestore.InstallReplace, wantErr: core.ErrFilestoreActivationIndeterminate,
			wantStage: "foreign-stage",
		},
		{
			name:  "create rejects foreign stage even when target is original file",
			stage: stageNamespaceForeign, target: targetNamespaceSameFile,
			install: filestore.InstallCreate, wantErr: core.ErrFilestoreActivationIndeterminate,
			wantStage: "foreign-stage", wantTarget: "candidate",
		},
		{
			name:  "replace rejects foreign stage and foreign target",
			stage: stageNamespaceForeign, target: targetNamespaceForeignFile,
			install: filestore.InstallReplace, wantErr: core.ErrFilestoreActivationIndeterminate,
			wantStage: "foreign-stage", wantTarget: "foreign-target",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			root := requireTestRoot(t, rootDirectory)
			staged := mustStage(t, root, ".stage", "candidate")
			prepareActivationNamespace(t, root, rootDirectory, staged, tc.stage, tc.target)
			gotErr := filestore.Recover(t.Context(), filestore.CommitRequest{
				Staged: staged, Target: mustRelativePath(t, "target"), Install: tc.install,
			})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("Recover() error = %v, want %v", gotErr, tc.wantErr)
				}
			} else if gotErr != nil {
				t.Fatalf("Recover() error = %v, want nil", gotErr)
			}
			requireActivationNamespace(t, rootDirectory, tc.wantStage, tc.wantTarget, false)
		})
	}
}

func prepareActivationNamespace(
	t *testing.T,
	root *os.Root,
	rootDirectory string,
	staged filestore.StagedFile,
	stageState stageNamespaceState,
	targetState targetNamespaceState,
) {
	t.Helper()

	if targetState == targetNamespaceSameFile {
		if err := root.Link(staged.Path().String(), "target"); err != nil {
			t.Fatal(err)
		}
	}
	switch stageState {
	case stageNamespaceOwned:
	case stageNamespaceMissing:
		if err := root.Remove(staged.Path().String()); err != nil {
			t.Fatal(err)
		}
	case stageNamespaceForeign:
		if err := root.Remove(staged.Path().String()); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rootDirectory, ".stage"), []byte("foreign-stage"), 0o600); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("stage namespace state = %d, want admitted state", stageState)
	}
	switch targetState {
	case targetNamespaceAbsent, targetNamespaceSameFile:
	case targetNamespaceForeignFile:
		if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("foreign-target"), 0o600); err != nil {
			t.Fatal(err)
		}
	case targetNamespaceDirectory:
		if err := os.Mkdir(filepath.Join(rootDirectory, "target"), 0o700); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("target namespace state = %d, want admitted state", targetState)
	}
}

func requireActivationNamespace(
	t *testing.T,
	rootDirectory string,
	wantStage string,
	wantTarget string,
	wantTargetDirectory bool,
) {
	t.Helper()

	requireOptionalFile(t, filepath.Join(rootDirectory, ".stage"), wantStage)
	if wantTargetDirectory {
		info, err := os.Stat(filepath.Join(rootDirectory, "target"))
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsDir() {
			t.Fatalf("target mode = %v, want directory", info.Mode())
		}
		return
	}
	requireOptionalFile(t, filepath.Join(rootDirectory, "target"), wantTarget)
}

func requireOptionalFile(t *testing.T, path, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if want == "" {
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("ReadFile(%s) error = %v, want %v", filepath.Base(path), err, os.ErrNotExist)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("ReadFile(%s) bytes = %q, want %q", filepath.Base(path), got, want)
	}
}
