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
	swapErr   error
	directory string
	name      string
	preserve  string
	remaining []byte
	foreign   []byte
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

// The receipt handoff is distinct from the native Rename namespace matrix:
// activation must publish the synchronized inode, preserve displaced handles,
// and refuse reuse of a consumed receipt. Empty content is still a real file.
func TestReplaceActivationLayerTriad(t *testing.T) {
	t.Parallel()
	type targetKind uint8
	const (
		targetAbsent targetKind = iota
		targetRegular
		targetEmptyDirectory
		targetNonemptyDirectory
	)
	for _, tc := range []struct {
		name    string
		target  targetKind
		payload []byte
		wantErr error
	}{
		{name: "absent target receives the exact synchronized inode", payload: []byte{0, 255, 7}},
		{name: "occupied target is displaced without truncating its open handle", target: targetRegular, payload: []byte{0, 255, 7}},
		{name: "empty stage replaces occupied target without becoming absence", target: targetRegular},
		{name: "empty directory refuses without consuming the stage", target: targetEmptyDirectory, payload: []byte{0, 255, 7}, wantErr: core.ErrFilestoreActivation},
		{name: "nonempty directory retains child and stage custody", target: targetNonemptyDirectory, payload: []byte{0, 255, 7}, wantErr: core.ErrFilestoreActivation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			original := []byte{255, 0, 19, 3}
			var displaced *os.File
			switch tc.target {
			case targetAbsent:
			case targetRegular:
				if err := root.WriteFile("target", original, 0o600); err != nil {
					t.Fatal(err)
				}
				var err error
				displaced, err = root.Open("target")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := displaced.Close(); err != nil {
						t.Error(err)
					}
				})
			case targetEmptyDirectory, targetNonemptyDirectory:
				if err := root.Mkdir("target", 0o700); err != nil {
					t.Fatal(err)
				}
				if tc.target == targetNonemptyDirectory {
					if err := root.WriteFile("target/child", original, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			default:
				t.Fatalf("fixture = %d, want a declared target shape", tc.target)
			}
			targetBefore, targetBeforeErr := root.Lstat("target")
			staged, err := filestore.Stage(t.Context(), filestore.StageRequest{Source: bytes.NewReader(tc.payload), Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600, MaximumBytes: mustByteCount(t, uint64(max(len(tc.payload), 1)))})
			if err != nil {
				t.Fatal(err)
			}
			stageBefore, err := root.Lstat("stage")
			if err != nil {
				t.Fatal(err)
			}
			before, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			request := filestore.CommitRequest{Staged: staged, Target: mustRelativePath(t, "target"), Install: filestore.InstallReplace}
			gotErr := filestore.Commit(t.Context(), request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Commit = %v, want %v", gotErr, tc.wantErr)
			}
			for _, class := range []error{core.ErrFilestoreSource, core.ErrFilestoreDestination, core.ErrFilestoreConflict, core.ErrFilestoreCleanup, core.ErrFilestoreSize, core.ErrFilestoreActivationIndeterminate} {
				if errors.Is(gotErr, class) {
					t.Fatalf("Commit = %v, want no %v", gotErr, class)
				}
			}
			if tc.wantErr != nil {
				if _, ok := errors.AsType[*os.LinkError](gotErr); !ok {
					t.Fatalf("refusal = %v, want Go LinkError", gotErr)
				}
				// Go owns platform errno spelling; the same native operation on these
				// refused entries supplies the exact cause and is itself non-mutating.
				nativeErr := root.Rename("stage", "target")
				var oracle *os.LinkError
				if !errors.As(nativeErr, &oracle) || !errors.Is(gotErr, oracle.Err) {
					t.Fatalf("refusal = %v, want native %v", gotErr, nativeErr)
				}
				after, err := removalFixtureSnapshot(directory)
				if err != nil || len(after) != len(before) {
					t.Fatalf("namespace = (%v,%v), want %v", after, err, before)
				}
				for i, entry := range before {
					if after[i].name != entry.name || after[i].mode != entry.mode || after[i].target != entry.target || !bytes.Equal(after[i].data, entry.data) {
						t.Fatalf("entry %d = %+v, want %+v", i, after[i], entry)
					}
				}
				stageAfter, err := root.Lstat("stage")
				if err != nil {
					t.Fatal(err)
				}
				targetAfter, err := root.Lstat("target")
				if err != nil {
					t.Fatal(err)
				}
				if targetBeforeErr != nil || !os.SameFile(stageBefore, stageAfter) || !os.SameFile(targetBefore, targetAfter) || !stageBefore.ModTime().Equal(stageAfter.ModTime()) || !targetBefore.ModTime().Equal(targetAfter.ModTime()) {
					t.Fatalf("refused stage/target = (%v,%v), want original (%v,%v)", stageAfter, targetAfter, stageBefore, targetBefore)
				}
				return
			}
			targetAfter, err := root.Lstat("target")
			if err != nil {
				t.Fatal(err)
			}
			got, err := root.ReadFile("target")
			if err != nil || !bytes.Equal(got, tc.payload) || !os.SameFile(stageBefore, targetAfter) || targetAfter.Mode() != stageBefore.Mode() || !targetAfter.ModTime().Equal(stageBefore.ModTime()) {
				t.Fatalf("target = (%v,%v,%v), want staged inode, metadata and %v", targetAfter, got, err, tc.payload)
			}
			if displaced != nil {
				gotOld, err := io.ReadAll(displaced)
				heldInfo, statErr := displaced.Stat()
				if err != nil || statErr != nil || !bytes.Equal(gotOld, original) || !os.SameFile(heldInfo, targetBefore) || os.SameFile(heldInfo, targetAfter) {
					t.Fatalf("displaced handle = (%v,%v,%v), want original bytes and separate inode", gotOld, err, statErr)
				}
			}
			repeated := filestore.Commit(t.Context(), request)
			if !errors.Is(repeated, core.ErrFilestoreActivation) || !errors.Is(repeated, fs.ErrNotExist) || errors.Is(repeated, core.ErrFilestoreActivationIndeterminate) {
				t.Fatalf("consumed receipt = %v, want definite missing-stage refusal", repeated)
			}
			finalInfo, err := root.Lstat("target")
			if err != nil {
				t.Fatal(err)
			}
			finalBytes, err := root.ReadFile("target")
			if err != nil || !os.SameFile(targetAfter, finalInfo) || !targetAfter.ModTime().Equal(finalInfo.ModTime()) || !bytes.Equal(finalBytes, tc.payload) {
				t.Fatalf("repeated activation changed target: (%v,%v,%v)", finalInfo, finalBytes, err)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 || entries[0].Name() != "target" {
				t.Fatalf("namespace = (%v,%v), want only target", entries, err)
			}
		})
	}
}
