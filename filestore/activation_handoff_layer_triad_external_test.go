package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// stageIdentitySwapSource is a real io.Reader that replaces the caller-named
// temporary with a different real file at the exact moment production reaches
// the end of the source stream. It is a test seam over the standard library
// interface, not a filesystem substitute: production still streams through its
// own bounded copy, sync, and stat path and observes a genuine identity change
// between staging and activation.
type stageIdentitySwapSource struct {
	remaining []byte
	foreign   []byte
	swapErr   error
	directory string
	name      string
	preserve  string
	swapped   bool
}

func (s *stageIdentitySwapSource) Read(buffer []byte) (int, error) {
	if len(s.remaining) > 0 {
		count := copy(buffer, s.remaining)
		s.remaining = s.remaining[count:]
		return count, nil
	}
	if !s.swapped {
		s.swapped = true
		path := filepath.Join(s.directory, s.name)
		if s.preserve != "" {
			s.swapErr = os.Link(path, filepath.Join(s.directory, s.preserve))
			if s.swapErr != nil {
				return 0, io.EOF
			}
		}
		if err := os.Remove(path); err != nil {
			s.swapErr = err
			return 0, io.EOF
		}
		s.swapErr = os.WriteFile(path, s.foreign, 0o600)
	}
	return 0, io.EOF
}

// TestReplaceActivationLayerTriad proves the direct replace-activation seam:
// a synchronized receipt is renamed onto its target, so the target becomes the
// staged file itself rather than a copy of it.
func TestReplaceActivationLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive replacement renames the staged file onto an occupied target without truncating the displaced file", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		targetPath := filepath.Join(rootDirectory, "target")
		if err := os.WriteFile(targetPath, []byte("original"), 0o600); err != nil {
			t.Fatal(err)
		}
		displaced, err := os.Open(targetPath)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if closeErr := displaced.Close(); closeErr != nil {
				t.Errorf("displaced handle Close() error = %v, want nil", closeErr)
			}
		})
		staged := mustStage(t, root, ".stage", "replacement")
		stagedInfo, err := os.Stat(filepath.Join(rootDirectory, ".stage"))
		if err != nil {
			t.Fatal(err)
		}
		gotErr := filestore.Commit(t.Context(), filestore.CommitRequest{
			Staged:  staged,
			Target:  mustRelativePath(t, "target"),
			Install: filestore.InstallReplace,
		})
		if gotErr != nil {
			t.Fatalf("Commit(replace) error = %v, want nil", gotErr)
		}
		activatedInfo, err := os.Stat(targetPath)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(stagedInfo, activatedInfo) {
			t.Fatalf("activated target identity = %v, want the staged file identity %v", activatedInfo.Name(), stagedInfo.Name())
		}
		got, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "replacement" {
			t.Fatalf("activated target bytes = %q, want %q", got, "replacement")
		}
		gotDisplaced, err := io.ReadAll(displaced)
		if err != nil {
			t.Fatal(err)
		}
		if string(gotDisplaced) != "original" {
			t.Fatalf("displaced open file bytes = %q, want %q", gotDisplaced, "original")
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{"target"})
	})
	t.Run("negative directory target refuses replacement and preserves both the directory tree and the stage", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.Mkdir(filepath.Join(rootDirectory, "target"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rootDirectory, "target", "child"), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		staged := mustStage(t, root, ".stage", "replacement")
		gotErr := filestore.Commit(t.Context(), filestore.CommitRequest{
			Staged:  staged,
			Target:  mustRelativePath(t, "target"),
			Install: filestore.InstallReplace,
		})
		var linkErr *os.LinkError
		if !errors.Is(gotErr, core.ErrFilestoreActivation) || !errors.As(gotErr, &linkErr) {
			t.Fatalf("Commit(replace directory) error = %v, want %v and *os.LinkError", gotErr, core.ErrFilestoreActivation)
		}
		gotChild, err := os.ReadFile(filepath.Join(rootDirectory, "target", "child"))
		if err != nil {
			t.Fatal(err)
		}
		gotStage, err := os.ReadFile(filepath.Join(rootDirectory, ".stage"))
		if err != nil {
			t.Fatal(err)
		}
		if string(gotChild) != "keep" || string(gotStage) != "replacement" {
			t.Fatalf("refused replacement bytes = child:%q stage:%q, want %q/%q", gotChild, gotStage, "keep", "replacement")
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{".stage", "target"})
	})
	t.Run("neutral absent target publishes once and a repeated commit of the consumed receipt fabricates no second effect", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		staged := mustStage(t, root, ".stage", "replacement")
		request := filestore.CommitRequest{
			Staged:  staged,
			Target:  mustRelativePath(t, "target"),
			Install: filestore.InstallReplace,
		}
		if gotErr := filestore.Commit(t.Context(), request); gotErr != nil {
			t.Fatalf("Commit(replace absent target) error = %v, want nil", gotErr)
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{"target"})
		for attempt := range 3 {
			gotErr := filestore.Commit(t.Context(), request)
			if !errors.Is(gotErr, core.ErrFilestoreActivation) ||
				!errors.Is(gotErr, fs.ErrNotExist) {
				t.Fatalf("repeated Commit() attempt %d error = %v, want %v and %v", attempt, gotErr, core.ErrFilestoreActivation, fs.ErrNotExist)
			}
		}
		got, err := os.ReadFile(filepath.Join(rootDirectory, "target"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "replacement" {
			t.Fatalf("target bytes after repeated commits = %q, want %q", got, "replacement")
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{"target"})
	})
}

// TestRecoveryHandoffLayerTriad proves the ownership seam on the CommitRequest
// that Write returns. The handoff exists so an interrupted activation stays the
// caller's decision instead of Primitive guessing.
//
// The positive case obtains the nonzero handoff from Write itself, restores the
// exact staged inode through a real hard-link identity, and completes the
// operation through Recover. The negative case leaves the foreign inode in
// place and proves that neither recovery nor discard consumes it.
func TestRecoveryHandoffLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive handoff returned by write finishes after the caller restores the exact staged inode", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		payload := []byte("candidate-payload")
		source := &stageIdentitySwapSource{
			remaining: payload,
			foreign:   []byte("foreign-owner"),
			directory: rootDirectory,
			name:      ".stage",
			preserve:  ".preserved-stage",
		}
		gotRecovery, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{
			Source:       source,
			Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
			Temporary:    mustRelativePath(t, ".stage"),
			Mode:         0o600,
			Install:      filestore.InstallCreate,
			MaximumBytes: mustByteCount(t, uint64(len(payload))),
		})
		if !source.swapped || source.swapErr != nil {
			t.Fatalf("stage identity swap = swapped:%t error:%v, want true/nil", source.swapped, source.swapErr)
		}
		if !errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) {
			t.Fatalf("Write(swapped stage identity) error = %v, want %v", gotErr, core.ErrFilestoreActivationIndeterminate)
		}
		if err := gotRecovery.Validate(); err != nil {
			t.Fatalf("Write() handoff Validate() error = %v, want nil", err)
		}
		if err := os.Remove(filepath.Join(rootDirectory, ".stage")); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(
			filepath.Join(rootDirectory, ".preserved-stage"),
			filepath.Join(rootDirectory, ".stage"),
		); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(rootDirectory, ".preserved-stage")); err != nil {
			t.Fatal(err)
		}
		for attempt := range 3 {
			if recoverErr := filestore.Recover(t.Context(), gotRecovery); recoverErr != nil {
				t.Fatalf("Recover(Write handoff) attempt %d error = %v, want nil", attempt, recoverErr)
			}
			got, err := os.ReadFile(filepath.Join(rootDirectory, "target"))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, payload) {
				t.Fatalf("target bytes after attempt %d = %q, want %q", attempt, got, payload)
			}
			requireDirectoryEntryNames(t, rootDirectory, []string{"target"})
		}
	})
	t.Run("negative ambiguous identity hands back an exact request that refuses to consume the stranger", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		payload := []byte("candidate-payload")
		source := &stageIdentitySwapSource{
			remaining: payload,
			foreign:   []byte("foreign-owner"),
			directory: rootDirectory,
			name:      ".stage",
		}
		gotRecovery, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{
			Source:       source,
			Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
			Temporary:    mustRelativePath(t, ".stage"),
			Mode:         0o600,
			Install:      filestore.InstallCreate,
			MaximumBytes: mustByteCount(t, uint64(len(payload))),
		})
		if !source.swapped || source.swapErr != nil {
			t.Fatalf("stage identity swap = swapped:%t error:%v, want true/nil", source.swapped, source.swapErr)
		}
		if !errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) {
			t.Fatalf("Write(swapped stage identity) error = %v, want %v", gotErr, core.ErrFilestoreActivationIndeterminate)
		}
		if err := gotRecovery.Validate(); err != nil {
			t.Fatalf("recovery request Validate() error = %v, want nil after indeterminate activation", err)
		}
		if gotRecovery.Target != mustRelativePath(t, "target") ||
			gotRecovery.Install != filestore.InstallCreate ||
			gotRecovery.Staged.Path() != mustRelativePath(t, ".stage") ||
			gotRecovery.Staged.BytesWritten().Uint64() != uint64(len(payload)) {
			t.Fatalf(
				"recovery request = target:%q install:%d stage:%q bytes:%d, want %q/%d/%q/%d",
				gotRecovery.Target.String(), gotRecovery.Install,
				gotRecovery.Staged.Path().String(), gotRecovery.Staged.BytesWritten().Uint64(),
				"target", filestore.InstallCreate, ".stage", len(payload),
			)
		}
		if recoverErr := filestore.Recover(t.Context(), gotRecovery); !errors.Is(recoverErr, core.ErrFilestoreActivationIndeterminate) {
			t.Fatalf("Recover(handoff over foreign stage) error = %v, want %v", recoverErr, core.ErrFilestoreActivationIndeterminate)
		}
		discardErr := filestore.Discard(t.Context(), gotRecovery.Staged)
		if !errors.Is(discardErr, core.ErrFilestoreCleanup) ||
			!errors.Is(discardErr, core.ErrFilestoreConflict) {
			t.Fatalf("Discard(handoff over foreign stage) error = %v, want %v and %v", discardErr, core.ErrFilestoreCleanup, core.ErrFilestoreConflict)
		}
		got, err := os.ReadFile(filepath.Join(rootDirectory, ".stage"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "foreign-owner" {
			t.Fatalf("foreign stage bytes = %q, want %q", got, "foreign-owner")
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{".stage"})
	})
	t.Run("neutral resolved outcomes issue no handoff and the zero request cannot fake a recovery", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			wantErr    error
			wantNative error
			name       string
			initial    string
			source     string
			wantTarget string
			maximum    uint64
		}{
			{
				name:       "completed create leaves the caller nothing to finish",
				source:     "published",
				wantTarget: "published",
			},
			{
				name:       "definite create conflict is cleaned by Primitive before returning",
				initial:    "winner",
				source:     "candidate",
				wantErr:    core.ErrFilestoreConflict,
				wantNative: os.ErrExist,
				wantTarget: "winner",
			},
			{
				name:    "definite size rejection publishes nothing and retains no temporary",
				source:  "oversized-payload",
				maximum: 4,
				wantErr: core.ErrFilestoreSize,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				rootDirectory := t.TempDir()
				root := requireTestRoot(t, rootDirectory)
				if tc.initial != "" {
					if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte(tc.initial), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				maximum := tc.maximum
				if maximum == 0 {
					maximum = uint64(max(len(tc.source), 1))
				}
				gotRecovery, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{
					Source:       bytes.NewReader([]byte(tc.source)),
					Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
					Temporary:    mustRelativePath(t, ".stage"),
					Mode:         0o600,
					Install:      filestore.InstallCreate,
					MaximumBytes: mustByteCount(t, maximum),
				})
				if tc.wantErr != nil {
					if !errors.Is(gotErr, tc.wantErr) {
						t.Fatalf("Write() error = %v, want %v", gotErr, tc.wantErr)
					}
					if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
						t.Fatalf("Write() error = %v, want native %v", gotErr, tc.wantNative)
					}
				} else if gotErr != nil {
					t.Fatalf("Write() error = %v, want nil", gotErr)
				}
				if !errors.Is(gotRecovery.Validate(), core.ErrFilestoreContract) {
					t.Fatalf("recovery request Validate() = %v, want %v for a resolved outcome", gotRecovery.Validate(), core.ErrFilestoreContract)
				}
				if recoverErr := filestore.Recover(t.Context(), gotRecovery); !errors.Is(recoverErr, core.ErrFilestoreContract) {
					t.Fatalf("Recover(zero handoff) error = %v, want %v", recoverErr, core.ErrFilestoreContract)
				}
				requireOptionalFile(t, filepath.Join(rootDirectory, "target"), tc.wantTarget)
				if tc.wantTarget == "" {
					requireDirectoryEntryNames(t, rootDirectory, nil)
					return
				}
				requireDirectoryEntryNames(t, rootDirectory, []string{"target"})
			})
		}
	})
}
