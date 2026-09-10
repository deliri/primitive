package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type stageEntryChange uint8

const (
	stageEntryUnchanged stageEntryChange = iota
	stageEntryHardLink
	stageEntrySymlinkToOwner
	stageEntrySymlinkToStranger
	stageEntryDanglingLink
	stageEntryForeignFile
	stageEntryAbsent
)

type stageEntryEffect uint8

const (
	stageEntryCommitCreate stageEntryEffect = iota
	stageEntryCommitReplace
	stageEntryRecoverCreate
	stageEntryRecoverReplace
	stageEntryDiscard
	stageEntryRead
)

// A receipt describes the real inode. A hard link is that inode; a symbolic
// link is a different directory entry even when Stat would follow it back to
// the original. Every effect gets an independently produced receipt and exact
// namespace/content observations before and after the attempted handoff.
func TestStagedNamespaceEntryLayerTriad(t *testing.T) {
	t.Parallel()
	effects := []struct {
		name   string
		effect stageEntryEffect
	}{
		{"create activation", stageEntryCommitCreate},
		{"replace activation", stageEntryCommitReplace},
		{"create recovery", stageEntryRecoverCreate},
		{"replace recovery", stageEntryRecoverReplace},
		{"discard custody", stageEntryDiscard},
		{"open staged read", stageEntryRead},
	}
	cases := []struct {
		name       string
		change     stageEntryChange
		wantOwned  bool
		wantAbsent bool
	}{
		{name: "untouched inode supplies exact bytes", wantOwned: true},
		{name: "hard link to the original inode retains ownership", change: stageEntryHardLink, wantOwned: true},
		{name: "symlink cannot borrow the original inode identity", change: stageEntrySymlinkToOwner},
		{name: "symlink to a stranger stays a foreign entry", change: stageEntrySymlinkToStranger},
		{name: "dangling symlink is occupied namespace rather than absence", change: stageEntryDanglingLink},
		{name: "equal-size regular replacement retains its own identity", change: stageEntryForeignFile},
		{name: "missing stage cannot invent completed activation", change: stageEntryAbsent, wantAbsent: true},
	}
	for _, effect := range effects {
		for _, tc := range cases {
			t.Run(effect.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				root := requireTestRoot(t, t.TempDir())
				stagePath, target := mustRelativePath(t, ".stage"), mustRelativePath(t, "target")
				original, foreign := []byte{0, 0xff}, []byte{0xff, 0}
				staged, err := filestore.Stage(t.Context(), filestore.StageRequest{Temporary: filestore.Location{Root: root, Path: stagePath}, Source: bytes.NewReader(original), Mode: 0o600})
				if err != nil || staged.Validate() != nil || staged.BytesWritten().Uint64() != uint64(len(original)) {
					t.Fatalf("producer = (%v,%v), want exact valid receipt", staged, err)
				}
				if err := root.Link(stagePath.String(), "archive"); err != nil {
					t.Fatal(err)
				}
				stranger, err := root.OpenFile("stranger", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				_, writeErr := stranger.Write(foreign)
				if err := errors.Join(writeErr, stranger.Close()); err != nil {
					t.Fatal(err)
				}
				if tc.change != stageEntryUnchanged {
					if err := root.Remove(stagePath.String()); err != nil {
						t.Fatal(err)
					}
				}
				switch tc.change {
				case stageEntryUnchanged, stageEntryAbsent:
				case stageEntryHardLink:
					err = root.Link("archive", stagePath.String())
				case stageEntrySymlinkToOwner:
					err = root.Symlink("archive", stagePath.String())
				case stageEntrySymlinkToStranger:
					err = root.Symlink("stranger", stagePath.String())
				case stageEntryDanglingLink:
					err = root.Symlink("absent", stagePath.String())
				case stageEntryForeignFile:
					err = root.Link("stranger", stagePath.String())
				default:
					t.Fatalf("fixture change = %d, want declared change", tc.change)
				}
				if err != nil {
					t.Fatal(err)
				}
				before, beforeErr := root.Lstat(stagePath.String())
				if tc.wantAbsent {
					if !errors.Is(beforeErr, fs.ErrNotExist) {
						t.Fatalf("absent fixture = %v, want native absence", beforeErr)
					}
				} else if beforeErr != nil {
					t.Fatal(beforeErr)
				}
				request := filestore.CommitRequest{Staged: staged, Target: target, Install: filestore.InstallCreate}
				var file *os.File
				var gotErr error
				switch effect.effect {
				case stageEntryCommitCreate:
					gotErr = filestore.Commit(t.Context(), request)
				case stageEntryCommitReplace:
					request.Install = filestore.InstallReplace
					gotErr = filestore.Commit(t.Context(), request)
				case stageEntryRecoverCreate:
					gotErr = filestore.Recover(t.Context(), request)
				case stageEntryRecoverReplace:
					request.Install = filestore.InstallReplace
					gotErr = filestore.Recover(t.Context(), request)
				case stageEntryDiscard:
					gotErr = filestore.Discard(t.Context(), staged)
				case stageEntryRead:
					file, gotErr = filestore.OpenStagedRead(t.Context(), staged)
				default:
					t.Fatalf("fixture effect = %d, want declared effect", effect.effect)
				}
				if file != nil {
					t.Cleanup(func() {
						if err := file.Close(); err != nil {
							t.Error(err)
						}
					})
				}
				var wantErr error
				if !tc.wantOwned {
					wantErr = core.ErrFilestoreActivationIndeterminate
					if effect.effect == stageEntryDiscard {
						wantErr = core.ErrFilestoreCleanup
					}
					if tc.wantAbsent {
						wantErr = core.ErrFilestoreActivation
						if effect.effect == stageEntryDiscard {
							wantErr = nil
						}
					}
				}
				if !errors.Is(gotErr, wantErr) {
					t.Fatalf("receipt-to-effect refusal = %v, want %v", gotErr, wantErr)
				}
				if tc.wantAbsent && effect.effect != stageEntryDiscard && !errors.Is(gotErr, fs.ErrNotExist) {
					t.Fatalf("missing stage cause = %v, want native absence", gotErr)
				}
				if !tc.wantOwned && !tc.wantAbsent && effect.effect == stageEntryDiscard && !errors.Is(gotErr, core.ErrFilestoreConflict) {
					t.Fatalf("discard foreign cause = %v, want conflict", gotErr)
				}
				if effect.effect == stageEntryRead && tc.wantOwned {
					if file == nil {
						t.Fatalf("opened staged handle = %v, want a live file", file)
					}
					got, err := io.ReadAll(file)
					if err != nil || !bytes.Equal(got, original) {
						t.Fatalf("opened bytes = (%x,%v), want %x", got, err, original)
					}
				} else if file != nil {
					t.Fatalf("non-read or refused effect returned a file: %v", file)
				}
				for _, protected := range []struct {
					name string
					want []byte
				}{{"archive", original}, {"stranger", foreign}} {
					got, err := root.ReadFile(protected.name)
					if err != nil || !bytes.Equal(got, protected.want) {
						t.Fatalf("protected %s = (%x,%v), want exact %x", protected.name, got, err, protected.want)
					}
				}
				activation := effect.effect <= stageEntryRecoverReplace
				after, afterErr := root.Lstat(stagePath.String())
				wantStageAbsent := tc.wantAbsent || tc.wantOwned && effect.effect != stageEntryRead
				if wantStageAbsent {
					if !errors.Is(afterErr, fs.ErrNotExist) {
						t.Fatalf("settled stage = (%v,%v), want absent", after, afterErr)
					}
				} else if afterErr != nil || !os.SameFile(before, after) || before.Mode().Type() != after.Mode().Type() {
					t.Fatalf("unconsumed stage = (%v,%v), want exact original entry", after, afterErr)
				}
				targetInfo, targetErr := root.Lstat(target.String())
				if activation && tc.wantOwned {
					archive, archiveErr := root.Stat("archive")
					got, readErr := root.ReadFile(target.String())
					if targetErr != nil || archiveErr != nil || readErr != nil || !os.SameFile(archive, targetInfo) || !targetInfo.Mode().IsRegular() || !bytes.Equal(got, original) {
						t.Fatalf("activated target = (%v,%v,%v,%x), want exact regular inode and bytes", targetErr, archiveErr, readErr, got)
					}
				} else if !errors.Is(targetErr, fs.ErrNotExist) {
					t.Fatalf("non-activation target = (%v,%v), want absent", targetInfo, targetErr)
				}
			})
		}
	}
}
