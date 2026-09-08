package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type renameEntryShape uint8

const (
	renameEntryMissing renameEntryShape = iota
	renameEntryRegular
	renameEntryEmptyDirectory
	renameEntryPopulatedDirectory
	renameEntryConfinedLink
	renameEntryDanglingLink
	renameEntryOutsideLink
	renameEntryLoopLink
	renameEntryHardLink
	renameEntryShapeLimit
)

type renameNativeFixture struct {
	source, target           string
	sourceShape, targetShape renameEntryShape
	payload, targetPayload   []byte
	retainSourceAlias        bool
}

// Fixture construction only. All comparisons live in the table/fuzz body.
func createRenameNativeFixture(directory, outside string, fixture renameNativeFixture) error {
	for _, name := range []string{"from", "to", "real"} {
		if err := os.Mkdir(filepath.Join(directory, name), 0o700); err != nil {
			return err
		}
	}
	for _, name := range []string{"neighbor", "obstacle", filepath.Join("real", "retained")} {
		if err := os.WriteFile(filepath.Join(directory, name), fixture.payload, 0o600); err != nil {
			return err
		}
	}
	for _, link := range []struct{ name, target string }{{"alias", "real"}, {"escape", outside}} {
		if err := os.Symlink(link.target, filepath.Join(directory, link.name)); err != nil {
			return err
		}
	}
	for _, entry := range []struct {
		name    string
		shape   renameEntryShape
		payload []byte
	}{{fixture.source, fixture.sourceShape, fixture.payload}, {fixture.target, fixture.targetShape, fixture.targetPayload}} {
		path := filepath.Join(directory, entry.name)
		switch entry.shape {
		case renameEntryMissing:
		case renameEntryRegular:
			if err := os.WriteFile(path, entry.payload, 0o600); err != nil {
				return err
			}
		case renameEntryEmptyDirectory, renameEntryPopulatedDirectory:
			if err := os.Mkdir(path, 0o700); err != nil {
				return err
			}
			if entry.shape == renameEntryPopulatedDirectory {
				if err := os.WriteFile(filepath.Join(path, "child"), entry.payload, 0o600); err != nil {
					return err
				}
				if err := os.Symlink(outside, filepath.Join(path, "outside-child")); err != nil {
					return err
				}
			}
		case renameEntryConfinedLink:
			relative, err := filepath.Rel(filepath.Dir(path), filepath.Join(directory, "real", "retained"))
			if err != nil {
				return err
			}
			if err := os.Symlink(relative, path); err != nil {
				return err
			}
		case renameEntryDanglingLink:
			if err := os.Symlink("missing-referent", path); err != nil {
				return err
			}
		case renameEntryOutsideLink:
			if err := os.Symlink(outside, path); err != nil {
				return err
			}
		case renameEntryLoopLink:
			if err := os.Symlink(filepath.Base(path), path); err != nil {
				return err
			}
		case renameEntryHardLink:
			if err := os.Link(filepath.Join(directory, fixture.source), path); err != nil {
				return err
			}
		default:
			return fs.ErrInvalid
		}
	}
	if fixture.retainSourceAlias {
		return os.Link(filepath.Join(directory, fixture.source), filepath.Join(directory, "source-alias"))
	}
	return nil
}

type renameRequestMutation uint8

const (
	renameRequestUnchanged renameRequestMutation = iota
	renameRequestNilContext
	renameRequestCanceled
	renameRequestNilRoot
	renameRequestClosedRoot
	renameRequestZeroSource
	renameRequestZeroTarget
	renameRequestRootSource
	renameRequestRootTarget
	renameRequestSamePath
)

func TestRenameNativeNamespaceLayerTriad(t *testing.T) {
	t.Parallel()
	binary := []byte{0, 255, 1, 0}
	for _, tc := range []struct {
		name     string
		fixture  renameNativeFixture
		mutation renameRequestMutation
		wantErr  error
	}{
		{name: "binary inode moves within one parent", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, payload: binary}},
		{name: "empty source produces a real empty target", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular}},
		{name: "both changed parents retain exact unrelated entries", fixture: renameNativeFixture{source: "from/source", target: "to/target", sourceShape: renameEntryRegular, payload: binary}},
		{name: "directory moves with binary child and outside link unchanged", fixture: renameNativeFixture{source: "source", target: "to/target", sourceShape: renameEntryPopulatedDirectory, payload: binary}},
		{name: "Go refuses replacement of even an empty target directory", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryEmptyDirectory, targetShape: renameEntryEmptyDirectory}, wantErr: core.ErrFilestoreActivation},
		{name: "empty directory moves without inventing children", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryEmptyDirectory}},
		{name: "identical bytes cannot substitute for source inode identity", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, targetShape: renameEntryRegular, payload: binary, targetPayload: binary}},
		{name: "nonempty source replaces an empty regular target", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, targetShape: renameEntryRegular, payload: binary}},
		{name: "empty source erases old target bytes without phantom content", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, targetShape: renameEntryRegular, targetPayload: binary}},
		{name: "confined source link moves as its own inode", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryConfinedLink, payload: binary}},
		{name: "dangling source link is movable without a referent", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryDanglingLink, payload: binary}},
		{name: "outside source link moves without moving its target", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryOutsideLink, payload: binary}},
		{name: "looping source link is not resolved during rename", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryLoopLink, payload: binary}},
		{name: "outside target link is replaced without modifying outside", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, targetShape: renameEntryOutsideLink, payload: binary}},
		{name: "dangling target link cannot redirect the replacement", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, targetShape: renameEntryDanglingLink, payload: binary}},
		{name: "confined target link replacement preserves its referent", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, targetShape: renameEntryConfinedLink, payload: binary}},
		{name: "same inode hard-link rename is a native no-op retaining both names", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, targetShape: renameEntryHardLink, payload: binary}},
		{name: "moving one hard-link name preserves the independent alias", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, payload: binary, retainSourceAlias: true}},
		{name: "different parent spellings can resolve to one native directory", fixture: renameNativeFixture{source: "alias/source", target: "real/target", sourceShape: renameEntryRegular, payload: binary}},
		{name: "same regular entry through two parent aliases is a native no-op", fixture: renameNativeFixture{source: "alias/source", target: "real/source", sourceShape: renameEntryRegular, payload: binary}},
		{name: "Go refuses same directory basename through aliased parents", fixture: renameNativeFixture{source: "alias/source", target: "real/source", sourceShape: renameEntryEmptyDirectory}, wantErr: core.ErrFilestoreActivation},
		{name: "directory cannot move into its own descendant", fixture: renameNativeFixture{source: "source", target: "source/target", sourceShape: renameEntryPopulatedDirectory, payload: binary}, wantErr: core.ErrFilestoreActivation},
		{name: "absent source is not an acknowledged no-op", fixture: renameNativeFixture{source: "source", target: "target", targetShape: renameEntryRegular, targetPayload: binary}, wantErr: core.ErrFilestoreActivation},
		{name: "missing target parent preserves the exact source", fixture: renameNativeFixture{source: "source", target: "missing/target", sourceShape: renameEntryRegular, payload: binary}, wantErr: core.ErrFilestoreActivation},
		{name: "regular source ancestor preserves native not-directory cause", fixture: renameNativeFixture{source: "obstacle/source", target: "target", payload: binary}, wantErr: core.ErrFilestoreActivation},
		{name: "regular source cannot replace an empty directory", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, targetShape: renameEntryEmptyDirectory, payload: binary}, wantErr: core.ErrFilestoreActivation},
		{name: "directory source cannot replace regular target bytes", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryEmptyDirectory, targetShape: renameEntryRegular, targetPayload: binary}, wantErr: core.ErrFilestoreActivation},
		{name: "populated directory target cannot be recursively overwritten", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryPopulatedDirectory, targetShape: renameEntryPopulatedDirectory, payload: binary, targetPayload: []byte{1, 254}}, wantErr: core.ErrFilestoreActivation},
		{name: "escaping source ancestor cannot seize an outside name", fixture: renameNativeFixture{source: "escape/retained", target: "target", payload: binary}, wantErr: core.ErrFilestoreActivation},
		{name: "escaping target ancestor cannot receive the source", fixture: renameNativeFixture{source: "source", target: "escape/target", sourceShape: renameEntryRegular, payload: binary}, wantErr: core.ErrFilestoreActivation},
		{name: "nil context preserves both distinct occupied names", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, targetShape: renameEntryRegular, payload: binary, targetPayload: []byte{1, 254}}, mutation: renameRequestNilContext, wantErr: core.ErrNilContext},
		{name: "cancellation precedes replacement of occupied target", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, targetShape: renameEntryRegular, payload: binary, targetPayload: []byte{1, 254}}, mutation: renameRequestCanceled, wantErr: context.Canceled},
		{name: "nil root cannot imply ambient path authority", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, payload: binary}, mutation: renameRequestNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "closed root retains native closure before effects", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, payload: binary}, mutation: renameRequestClosedRoot, wantErr: core.ErrFilestoreActivation},
		{name: "zero source cannot select the root", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, payload: binary}, mutation: renameRequestZeroSource, wantErr: core.ErrFilestoreContract},
		{name: "zero target cannot select the root", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, payload: binary}, mutation: renameRequestZeroTarget, wantErr: core.ErrFilestoreContract},
		{name: "explicit root source cannot relocate the capability", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, payload: binary}, mutation: renameRequestRootSource, wantErr: core.ErrFilestoreContract},
		{name: "explicit root target cannot replace the capability", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, payload: binary}, mutation: renameRequestRootTarget, wantErr: core.ErrFilestoreContract},
		{name: "identical typed paths are rejected before native no-op", fixture: renameNativeFixture{source: "source", target: "target", sourceShape: renameEntryRegular, payload: binary}, mutation: renameRequestSamePath, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directories := [2]string{t.TempDir(), t.TempDir()}
			outside := t.TempDir()
			outsideName := filepath.Join(outside, "retained")
			if err := os.WriteFile(outsideName, tc.fixture.payload, 0o600); err != nil {
				t.Fatal(err)
			}
			outsideBefore, err := os.Stat(outside)
			if err != nil {
				t.Fatal(err)
			}
			outsideFileBefore, err := os.Stat(outsideName)
			if err != nil {
				t.Fatal(err)
			}
			var roots [2]*os.Root
			for i, directory := range directories {
				if err := createRenameNativeFixture(directory, outside, tc.fixture); err != nil {
					t.Fatal(err)
				}
				roots[i], err = os.OpenRoot(directory)
				if err != nil {
					t.Fatal(err)
				}
				if tc.mutation != renameRequestClosedRoot {
					t.Cleanup(func() {
						if err := roots[i].Close(); err != nil {
							t.Error(err)
						}
					})
				}
			}
			sourceBefore, sourceBeforeErr := roots[0].Lstat(tc.fixture.source)
			targetBefore, targetBeforeErr := roots[0].Lstat(tc.fixture.target)
			if tc.fixture.targetShape == renameEntryHardLink {
				if sourceBeforeErr != nil || targetBeforeErr != nil || !os.SameFile(sourceBefore, targetBefore) {
					t.Fatalf("hard-link native facts = (%v,%v,%v,%v), want one shared inode", sourceBefore, targetBefore, sourceBeforeErr, targetBeforeErr)
				}
			}
			if tc.fixture.sourceShape == renameEntryRegular && tc.fixture.targetShape == renameEntryRegular {
				if sourceBeforeErr != nil || targetBeforeErr != nil || os.SameFile(sourceBefore, targetBefore) {
					t.Fatalf("replacement native facts = (%v,%v,%v,%v), want distinct inodes", sourceBefore, targetBefore, sourceBeforeErr, targetBeforeErr)
				}
			}
			var heldTarget *os.File
			if tc.fixture.targetShape == renameEntryRegular {
				heldTarget, err = roots[0].Open(tc.fixture.target)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := heldTarget.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			request := filestore.RenameRequest{Location: filestore.Location{Root: roots[0], Path: mustRelativePath(t, tc.fixture.source)}, Target: mustRelativePath(t, tc.fixture.target)}
			ctx := t.Context()
			switch tc.mutation {
			case renameRequestNilContext:
				ctx = nil
			case renameRequestCanceled:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case renameRequestNilRoot:
				request.Location.Root = nil
			case renameRequestClosedRoot:
				for _, root := range roots {
					if err := root.Close(); err != nil {
						t.Fatal(err)
					}
				}
			case renameRequestZeroSource:
				request.Location.Path = core.RelativePath{}
			case renameRequestZeroTarget:
				request.Target = core.RelativePath{}
			case renameRequestRootSource:
				request.Location.Path = mustRelativePath(t, ".")
			case renameRequestRootTarget:
				request.Target = mustRelativePath(t, ".")
			case renameRequestSamePath:
				request.Target = request.Location.Path
			}
			var nativeErr error
			if tc.mutation == renameRequestUnchanged || tc.mutation == renameRequestClosedRoot {
				nativeErr = roots[1].Rename(tc.fixture.source, tc.fixture.target)
			}
			if (errors.Is(tc.wantErr, core.ErrFilestoreActivation)) != (nativeErr != nil) {
				t.Fatalf("native rename = %v, want boundary %v", nativeErr, tc.wantErr)
			}
			gotErr := filestore.Rename(ctx, request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("rename = %v, want %v", gotErr, tc.wantErr)
			}
			if nativeErr != nil {
				var native *os.LinkError
				var gotNative *os.LinkError
				if !errors.As(nativeErr, &native) || !errors.As(gotErr, &gotNative) || !errors.Is(gotErr, native.Err) {
					t.Fatalf("rename cause = %v, want Go %v with LinkError", gotErr, nativeErr)
				}
			}
			got, err := removalFixtureSnapshot(directories[0])
			if err != nil {
				t.Fatal(err)
			}
			want, err := removalFixtureSnapshot(directories[1])
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Fatalf("namespace = %+v, want native %+v", got, want)
			}
			for i := range want {
				if got[i].name != want[i].name || got[i].mode != want[i].mode || got[i].target != want[i].target || !bytes.Equal(got[i].data, want[i].data) {
					t.Fatalf("entry %d = %+v, want native %+v", i, got[i], want[i])
				}
			}
			if gotErr == nil {
				after, err := os.Lstat(filepath.Join(directories[0], tc.fixture.target))
				if sourceBeforeErr != nil || err != nil || !os.SameFile(sourceBefore, after) || sourceBefore.Mode() != after.Mode() || sourceBefore.ModTime().UnixNano() != after.ModTime().UnixNano() {
					t.Fatalf("moved target = (%v,%v), want original source %v", after, err, sourceBefore)
				}
				sourceAfter, err := os.Lstat(filepath.Join(directories[0], tc.fixture.source))
				if targetBeforeErr == nil && os.SameFile(sourceBefore, targetBefore) {
					if err != nil || !os.SameFile(sourceBefore, sourceAfter) {
						t.Fatalf("native no-op source = (%v,%v), want retained inode", sourceAfter, err)
					}
				} else if !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("old source = (%v,%v), want absent", sourceAfter, err)
				}
			} else {
				for _, entry := range []struct {
					name      string
					before    fs.FileInfo
					beforeErr error
				}{{tc.fixture.source, sourceBefore, sourceBeforeErr}, {tc.fixture.target, targetBefore, targetBeforeErr}} {
					if entry.beforeErr != nil {
						continue
					}
					after, err := os.Lstat(filepath.Join(directories[0], entry.name))
					if err != nil || !os.SameFile(entry.before, after) || entry.before.Mode() != after.Mode() || entry.before.ModTime().UnixNano() != after.ModTime().UnixNano() {
						t.Fatalf("refused entry %s = (%v,%v), want unchanged %v", entry.name, after, err, entry.before)
					}
				}
			}
			if heldTarget != nil {
				after, err := heldTarget.Stat()
				if err != nil || !os.SameFile(targetBefore, after) || targetBefore.Mode() != after.Mode() || targetBefore.ModTime().UnixNano() != after.ModTime().UnixNano() {
					t.Fatalf("caller-held old target = (%v,%v), want unchanged %v", after, err, targetBefore)
				}
				data := make([]byte, len(tc.fixture.targetPayload)+1)
				n, err := heldTarget.ReadAt(data, 0)
				if !errors.Is(err, io.EOF) || n != len(tc.fixture.targetPayload) || !bytes.Equal(data[:n], tc.fixture.targetPayload) {
					t.Fatalf("held target bytes = (%v,%v), want %v", data[:n], err, tc.fixture.targetPayload)
				}
			}
			outsideAfter, err := os.Stat(outside)
			if err != nil {
				t.Fatal(err)
			}
			outsideFileAfter, err := os.Stat(outsideName)
			if err != nil {
				t.Fatal(err)
			}
			outsideEntries, err := os.ReadDir(outside)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(outsideName)
			if err != nil || !bytes.Equal(data, tc.fixture.payload) || len(outsideEntries) != 1 || outsideEntries[0].Name() != "retained" || !os.SameFile(outsideBefore, outsideAfter) || outsideBefore.Mode() != outsideAfter.Mode() || outsideBefore.ModTime().UnixNano() != outsideAfter.ModTime().UnixNano() || !os.SameFile(outsideFileBefore, outsideFileAfter) || outsideFileBefore.Mode() != outsideFileAfter.Mode() || outsideFileBefore.ModTime().UnixNano() != outsideFileAfter.ModTime().UnixNano() {
				t.Fatalf("outside inode/namespace/metadata/bytes changed: %v", err)
			}
		})
	}
}
