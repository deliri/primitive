package filestore_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type recoveryLanding uint8

const (
	recoveryBeforeEffect recoveryLanding = iota
	recoveryTargetLinked
	recoveryStageConsumed
)

// Recovery cannot weaken the receipt checks just because a prior effect might
// have landed. The six real namespace/install combinations are crossed with
// exact, shortened, grown and permission-altered observations of the same inode.
func TestRecoveryRevalidatesObservedReceiptLayerTriad(t *testing.T) {
	t.Parallel()
	positions := []struct {
		name    string
		landing recoveryLanding
	}{
		{name: "before activation"},
		{name: "target linked but stage retained", landing: recoveryTargetLinked},
		{name: "stage consumed and target retained", landing: recoveryStageConsumed},
	}
	intents := []struct {
		name string
		mode filestore.InstallMode
	}{
		{"exclusive create", filestore.InstallCreate}, {"atomic replace", filestore.InstallReplace},
	}
	cases := []struct {
		name      string
		wantBytes []byte
		wantMode  fs.FileMode
		wantErr   error
	}{
		{name: "exact receipt completes without retaining a stage name", wantBytes: []byte{0, 0xff}, wantMode: 0o600},
		{name: "truncation to empty cannot become a completed two-byte receipt", wantMode: 0o600, wantErr: core.ErrFilestoreSize},
		{name: "one byte below receipt is a size refusal", wantBytes: []byte{0}, wantMode: 0o600, wantErr: core.ErrFilestoreSize},
		{name: "one byte above receipt is a size refusal", wantBytes: []byte{0, 0xff, 1}, wantMode: 0o600, wantErr: core.ErrFilestoreSize},
		{name: "lost owner-write permission contradicts the receipt", wantBytes: []byte{0, 0xff}, wantMode: 0o400, wantErr: core.ErrFilestoreActivation},
		{name: "added other-execute permission contradicts the receipt", wantBytes: []byte{0, 0xff}, wantMode: 0o601, wantErr: core.ErrFilestoreActivation},
	}
	for _, intent := range intents {
		for _, position := range positions {
			for _, tc := range cases {
				t.Run(intent.name+"/"+position.name+"/"+tc.name, func(t *testing.T) {
					t.Parallel()
					root := requireTestRoot(t, t.TempDir())
					stagePath, target := mustRelativePath(t, ".stage"), mustRelativePath(t, "target")
					original := []byte{0, 0xff}
					staged, err := filestore.Stage(t.Context(), filestore.StageRequest{Source: bytes.NewReader(original), Temporary: filestore.Location{Root: root, Path: stagePath}, Mode: 0o600, MaximumBytes: mustByteCount(t, uint64(len(original)))})
					if err != nil || staged.Validate() != nil {
						t.Fatalf("stage producer = (%v,%v), want valid exact receipt", staged, err)
					}
					if err := root.Link(stagePath.String(), "archive"); err != nil {
						t.Fatal(err)
					}
					if position.landing != recoveryBeforeEffect {
						if err := root.Link(stagePath.String(), target.String()); err != nil {
							t.Fatal(err)
						}
					}
					if position.landing == recoveryStageConsumed {
						if err := root.Remove(stagePath.String()); err != nil {
							t.Fatal(err)
						}
					}
					file, err := root.OpenFile("archive", os.O_RDWR, 0)
					if err != nil {
						t.Fatal(err)
					}
					count, writeErr := file.WriteAt(tc.wantBytes, 0)
					truncateErr := file.Truncate(int64(len(tc.wantBytes)))
					chmodErr := file.Chmod(tc.wantMode)
					if err := errors.Join(writeErr, truncateErr, chmodErr, file.Close()); err != nil || count != len(tc.wantBytes) {
						t.Fatalf("real mutation = (%d,%v), want %d bytes", count, err, len(tc.wantBytes))
					}
					request := filestore.CommitRequest{Staged: staged, Target: target, Install: intent.mode}
					if err := request.Validate(); err != nil {
						t.Fatalf("historical receipt admission = %v, want nil before re-observation", err)
					}
					gotErr := filestore.Recover(t.Context(), request)
					if !errors.Is(gotErr, tc.wantErr) {
						t.Fatalf("recovery = %v, want %v", gotErr, tc.wantErr)
					}
					archived, archiveErr := root.Stat("archive")
					got, readErr := root.ReadFile("archive")
					if archiveErr != nil || readErr != nil || !bytes.Equal(got, tc.wantBytes) || archived.Mode().Perm() != tc.wantMode {
						t.Fatalf("recovery rewrote observed inode = (%x,%v,%v), want %x mode %o", got, archiveErr, readErr, tc.wantBytes, tc.wantMode)
					}
					wantStageAbsent := position.landing == recoveryStageConsumed || tc.wantErr == nil
					stageInfo, stageErr := root.Lstat(stagePath.String())
					if wantStageAbsent {
						if !errors.Is(stageErr, fs.ErrNotExist) {
							t.Fatalf("settled stage = (%v,%v), want absent", stageInfo, stageErr)
						}
					} else if stageErr != nil || !os.SameFile(archived, stageInfo) {
						t.Fatalf("refused stage = (%v,%v), want observed inode retained", stageInfo, stageErr)
					}
					wantTarget := position.landing != recoveryBeforeEffect || tc.wantErr == nil
					targetInfo, targetErr := root.Lstat(target.String())
					if wantTarget {
						data, err := root.ReadFile(target.String())
						if targetErr != nil || err != nil || !os.SameFile(archived, targetInfo) || !bytes.Equal(data, tc.wantBytes) {
							t.Fatalf("target = (%v,%v,%x), want exact observed inode", targetErr, err, data)
						}
					} else if !errors.Is(targetErr, fs.ErrNotExist) {
						t.Fatalf("refused target = (%v,%v), want absent", targetInfo, targetErr)
					}
				})
			}
		}
	}
}

func TestSameInodeActivationDoesNotLeaveAnUnconsumedReceiptLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		install   filestore.InstallMode
		recover   bool
		wantErr   error
		wantStage bool
	}{
		{name: "exclusive commit treats even the owned hard link as occupied", install: filestore.InstallCreate, wantErr: core.ErrFilestoreConflict, wantStage: true},
		{name: "replace commit settles Go rename of two names for one inode", install: filestore.InstallReplace},
		{name: "create recovery settles its already-landed hard link", install: filestore.InstallCreate, recover: true},
		{name: "replace recovery settles its already-landed hard link", install: filestore.InstallReplace, recover: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := requireTestRoot(t, t.TempDir())
			stagePath, target := mustRelativePath(t, ".stage"), mustRelativePath(t, "target")
			payload := []byte{0, 0xff}
			staged, err := filestore.Stage(t.Context(), filestore.StageRequest{Temporary: filestore.Location{Root: root, Path: stagePath}, Source: bytes.NewReader(payload), MaximumBytes: mustByteCount(t, uint64(len(payload))), Mode: 0o600})
			if err != nil {
				t.Fatal(err)
			}
			if err := root.Link(stagePath.String(), target.String()); err != nil {
				t.Fatal(err)
			}
			before, err := root.Lstat(target.String())
			if err != nil {
				t.Fatal(err)
			}
			request := filestore.CommitRequest{Staged: staged, Target: target, Install: tc.install}
			var gotErr error
			if tc.recover {
				gotErr = filestore.Recover(t.Context(), request)
			} else {
				gotErr = filestore.Commit(t.Context(), request)
			}
			if !errors.Is(gotErr, tc.wantErr) || tc.wantErr != nil && !errors.Is(gotErr, fs.ErrExist) {
				t.Fatalf("same-inode activation = %v, want %v with native conflict when occupied", gotErr, tc.wantErr)
			}
			after, statErr := root.Lstat(target.String())
			got, readErr := root.ReadFile(target.String())
			if statErr != nil || readErr != nil || !os.SameFile(before, after) || !bytes.Equal(got, payload) {
				t.Fatalf("target = (%v,%v,%x), want preserved inode and bytes", statErr, readErr, got)
			}
			stageInfo, stageErr := root.Lstat(stagePath.String())
			if tc.wantStage {
				if stageErr != nil || !os.SameFile(before, stageInfo) {
					t.Fatalf("refused stage = (%v,%v), want preserved", stageInfo, stageErr)
				}
			} else if !errors.Is(stageErr, fs.ErrNotExist) {
				t.Fatalf("completed stage = (%v,%v), want consumed", stageInfo, stageErr)
			}
		})
	}
}
