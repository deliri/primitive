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

func TestAppendIntentLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		append         filestore.AppendMode
		entry          nativeHandleEntry
		empty, noWrite bool
		wantErr        error
	}{
		{name: "exclusive create publishes a new append capability", append: filestore.AppendCreate, entry: nativeHandleMissing},
		{name: "exclusive create cannot truncate binary occupied file", append: filestore.AppendCreate, wantErr: core.ErrFilestoreConflict},
		{name: "exclusive create cannot mistake empty file for absence", append: filestore.AppendCreate, empty: true, wantErr: core.ErrFilestoreConflict},
		{name: "exclusive create cannot replace directory with child", append: filestore.AppendCreate, entry: nativeHandleDirectory, wantErr: core.ErrFilestoreConflict},
		{name: "exclusive create cannot follow confined occupied link", append: filestore.AppendCreate, entry: nativeHandleConfinedLink, wantErr: core.ErrFilestoreConflict},
		{name: "exclusive create cannot fill dangling link referent", append: filestore.AppendCreate, entry: nativeHandleDanglingLink, wantErr: core.ErrFilestoreConflict},
		{name: "exclusive create cannot follow outside occupied link", append: filestore.AppendCreate, entry: nativeHandleOutsideLink, wantErr: core.ErrFilestoreConflict},
		{name: "existing-only preserves prefix despite seek to beginning", append: filestore.AppendExisting},
		{name: "existing-only opens empty inode without substituting it", append: filestore.AppendExisting, empty: true},
		{name: "existing-only cannot create missing entry", append: filestore.AppendExisting, entry: nativeHandleMissing, wantErr: core.ErrFilestoreActivation},
		{name: "existing-only cannot expose a directory as append file", append: filestore.AppendExisting, entry: nativeHandleDirectory, wantErr: core.ErrFilestoreActivation},
		{name: "existing-only follows confined link to exact referent", append: filestore.AppendExisting, entry: nativeHandleConfinedLink},
		{name: "existing-only cannot create through dangling link", append: filestore.AppendExisting, entry: nativeHandleDanglingLink, wantErr: core.ErrFilestoreActivation},
		{name: "existing-only cannot escape through outside link", append: filestore.AppendExisting, entry: nativeHandleOutsideLink, wantErr: core.ErrFilestoreActivation},
		{name: "create-or-open creates only a truly absent entry", append: filestore.AppendCreateOrOpen, entry: nativeHandleMissing},
		{name: "create-or-open preserves occupied prefix despite seek", append: filestore.AppendCreateOrOpen},
		{name: "create-or-open cannot turn directory into append file", append: filestore.AppendCreateOrOpen, entry: nativeHandleDirectory, wantErr: core.ErrFilestoreActivation},
		{name: "create-or-open follows an existing confined referent", append: filestore.AppendCreateOrOpen, entry: nativeHandleConfinedLink},
		{name: "create-or-open cannot create through occupied dangling link", append: filestore.AppendCreateOrOpen, entry: nativeHandleDanglingLink, wantErr: core.ErrFilestoreActivation},
		{name: "create-or-open cannot escape through occupied outside link", append: filestore.AppendCreateOrOpen, entry: nativeHandleOutsideLink, wantErr: core.ErrFilestoreActivation},
		{name: "existing-only without writes preserves mode and timestamp", append: filestore.AppendExisting, noWrite: true},
		{name: "create-or-open without writes cannot apply create permissions", append: filestore.AppendCreateOrOpen, noWrite: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := []byte{0, 255, 31}
			if tc.empty {
				payload = nil
			}
			change := []byte{255, 0, 8}
			if tc.noWrite {
				change = nil
			}
			neighbor := []byte{41, 0, 255}
			if err := os.WriteFile(filepath.Join(directory, "neighbor"), neighbor, 0o600); err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			outsidePath := filepath.Join(outside, "outside")
			if err := os.WriteFile(outsidePath, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			outsideBefore, err := os.Lstat(outsidePath)
			if err != nil {
				t.Fatal(err)
			}
			physical := "entry"
			if tc.entry == nativeHandleConfinedLink || tc.entry == nativeHandleDanglingLink {
				physical = "subject"
			}
			switch tc.entry {
			case nativeHandleRegular:
				if err := os.WriteFile(filepath.Join(directory, "entry"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
			case nativeHandleMissing:
			case nativeHandleDirectory:
				if err := root.Mkdir("entry", 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "entry", "child"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
			case nativeHandleConfinedLink, nativeHandleDanglingLink:
				if tc.entry == nativeHandleConfinedLink {
					if err := os.WriteFile(filepath.Join(directory, "subject"), payload, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if err := root.Symlink("subject", "entry"); err != nil {
					t.Fatal(err)
				}
			case nativeHandleOutsideLink:
				if err := root.Symlink(outsidePath, "entry"); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("entry = %d, want declared fixture", tc.entry)
			}
			want, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			before, beforeErr := os.Lstat(filepath.Join(directory, physical))
			if beforeErr != nil && !errors.Is(beforeErr, fs.ErrNotExist) {
				t.Fatal(beforeErr)
			}
			var wantNative error
			if tc.wantErr != nil {
				flag := os.O_WRONLY | os.O_APPEND
				if tc.append == filestore.AppendCreate {
					flag |= os.O_CREATE | os.O_EXCL
				}
				native, nativeErr := root.OpenFile("entry", flag, 0o640)
				if native != nil {
					_ = native.Close()
				}
				if nativeErr == nil {
					t.Fatalf("native fixture error = %v, want a native refusal", nativeErr)
				}
				wantNative = nativeErr
				if pathErr, ok := errors.AsType[*fs.PathError](nativeErr); ok {
					wantNative = pathErr.Err
				}
			}
			got, gotErr := filestore.OpenAppend(t.Context(), filestore.AppendRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "entry")}, Mode: 0o640, Append: tc.append})
			if got != nil {
				t.Cleanup(func() { _ = got.Close() })
			}
			if !errors.Is(gotErr, tc.wantErr) || wantNative != nil && !errors.Is(gotErr, wantNative) {
				t.Fatalf("OpenAppend = (%v,%v), want %v with native %v", got, gotErr, tc.wantErr, wantNative)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatalf("refused handle = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Fatal("accepted append handle = nil, want Go file")
				}
				info, err := got.Stat()
				if err != nil {
					t.Fatal(err)
				}
				wantPrefix := payload
				wantMode := fs.FileMode(0o600)
				if tc.entry == nativeHandleMissing {
					wantPrefix = nil
					wantMode = 0o640
				}
				if !info.Mode().IsRegular() || info.Mode().Perm() != wantMode || info.Size() != int64(len(wantPrefix)) || before != nil && !os.SameFile(before, info) {
					t.Fatalf("acquired inode = %v, want exact original prefix and mode %#o", info, wantMode)
				}
				if _, err := got.Seek(0, io.SeekStart); err != nil {
					t.Fatal(err)
				}
				if n, err := got.Write(change); err != nil || n != len(change) {
					t.Fatalf("append write = (%d,%v), want %d", n, err, len(change))
				}
				var one [1]byte
				_, readErr := got.ReadAt(one[:], 0)
				native, err := root.OpenFile("neighbor", os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, nativeReadErr := native.ReadAt(one[:], 0)
				if err := native.Close(); err != nil {
					t.Fatal(err)
				}
				var nativePath *fs.PathError
				if !errors.As(nativeReadErr, &nativePath) || !errors.Is(readErr, nativePath.Err) {
					t.Fatalf("append-only read = %v, want native %v", readErr, nativeReadErr)
				}
				after, err := got.Stat()
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(info, after) || after.Mode() != info.Mode() || tc.noWrite && !info.ModTime().Equal(after.ModTime()) {
					t.Fatalf("held append inode = %v, want original metadata %v", after, info)
				}
				if err := got.Close(); err != nil {
					t.Fatal(err)
				}
				if _, err := got.Stat(); !errors.Is(err, fs.ErrClosed) {
					t.Fatalf("closed append = %v, want native closed", err)
				}
				wantBytes := append(bytes.Clone(wantPrefix), change...)
				if tc.entry == nativeHandleMissing {
					want = append([]removalFixtureEntry{{name: "entry", mode: wantMode, data: wantBytes}}, want...)
				} else {
					for i := range want {
						if want[i].name == physical {
							want[i].data = wantBytes
						}
					}
				}
			}
			gotNamespace, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			if len(gotNamespace) != len(want) {
				t.Fatalf("namespace = %+v, want %+v", gotNamespace, want)
			}
			for i, got := range gotNamespace {
				if got.name != want[i].name || got.mode != want[i].mode || got.target != want[i].target || !bytes.Equal(got.data, want[i].data) {
					t.Fatalf("entry %d = %+v, want %+v", i, got, want[i])
				}
			}
			if tc.wantErr != nil && before != nil {
				retained, err := os.Lstat(filepath.Join(directory, physical))
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(before, retained) || !before.ModTime().Equal(retained.ModTime()) {
					t.Fatalf("refused inode = %v, want unchanged %v", retained, before)
				}
			}
			outsideAfter, err := os.Lstat(outsidePath)
			if err != nil {
				t.Fatal(err)
			}
			outsideBytes, err := os.ReadFile(outsidePath)
			if err != nil || !bytes.Equal(outsideBytes, payload) || !os.SameFile(outsideBefore, outsideAfter) || outsideBefore.Mode() != outsideAfter.Mode() || !outsideBefore.ModTime().Equal(outsideAfter.ModTime()) {
				t.Fatalf("outside = (%v,%v,%v), want unchanged bytes and metadata", outsideAfter, outsideBytes, err)
			}
		})
	}
}

type rotationMutation uint8

const (
	rotationUnchanged rotationMutation = iota
	rotationNilContext
	rotationCanceled
	rotationNilOutgoing
	rotationClosedOutgoing
	rotationPipeOutgoing
	rotationDirectoryOutgoing
	rotationNilRoot
	rotationClosedRoot
	rotationZeroPath
	rotationRootPath
	rotationMissingParent
	rotationOccupiedIncoming
	rotationSameName
	rotationZeroMode
	rotationTypeMode
	rotationExistingIntent
	rotationCreateOrOpenIntent
	rotationUnknownIntent
	rotationFutureIntent
)

func TestRotateAppendValidationOwnershipLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                string
		mutation            rotationMutation
		empty, wantClosed   bool
		wantErr, wantNative error
	}{
		{name: "valid handoff closes exact outgoing and creates empty incoming", wantClosed: true},
		{name: "empty outgoing still transfers actual handle ownership", empty: true, wantClosed: true},
		{name: "nil context leaves outgoing caller-owned", mutation: rotationNilContext, wantErr: core.ErrNilContext},
		{name: "canceled context leaves outgoing caller-owned", mutation: rotationCanceled, wantErr: context.Canceled},
		{name: "nil outgoing does not consume independent live handle", mutation: rotationNilOutgoing, wantErr: core.ErrFilestoreContract},
		{name: "already closed outgoing retains native refusal without incoming", mutation: rotationClosedOutgoing, wantClosed: true, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrClosed},
		{name: "native pipe sync refusal still closes transferred handle", mutation: rotationPipeOutgoing, wantClosed: true, wantErr: core.ErrFilestoreActivation},
		{name: "native synchronizable directory is settled without inventing file policy", mutation: rotationDirectoryOutgoing, wantClosed: true},
		{name: "nil incoming root does not consume outgoing", mutation: rotationNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "closed incoming root fails after outgoing transfer", mutation: rotationClosedRoot, wantClosed: true, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrClosed},
		{name: "zero incoming path does not consume outgoing", mutation: rotationZeroPath, wantErr: core.ErrFilestoreContract},
		{name: "root incoming path does not consume outgoing", mutation: rotationRootPath, wantErr: core.ErrFilestoreContract},
		{name: "missing incoming parent fails after outgoing transfer", mutation: rotationMissingParent, wantClosed: true, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrNotExist},
		{name: "occupied incoming is preserved after outgoing transfer", mutation: rotationOccupiedIncoming, wantClosed: true, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "incoming same as outgoing name cannot overwrite sealed bytes", mutation: rotationSameName, wantClosed: true, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "zero incoming mode does not consume outgoing", mutation: rotationZeroMode, wantErr: core.ErrFilestoreContract},
		{name: "incoming type bits do not consume outgoing", mutation: rotationTypeMode, wantErr: core.ErrFilestoreContract},
		{name: "existing-only incoming intent does not consume outgoing", mutation: rotationExistingIntent, wantErr: core.ErrFilestoreContract},
		{name: "create-or-open incoming intent cannot borrow rotation authority", mutation: rotationCreateOrOpenIntent, wantErr: core.ErrFilestoreContract},
		{name: "unset incoming intent does not consume outgoing", mutation: rotationUnknownIntent, wantErr: core.ErrFilestoreContract},
		{name: "future incoming intent does not consume outgoing", mutation: rotationFutureIntent, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := []byte{0, 255, 31}
			if tc.empty {
				payload = nil
			}
			neighbor := []byte{41, 0, 255}
			if err := os.WriteFile(filepath.Join(directory, "neighbor"), neighbor, 0o600); err != nil {
				t.Fatal(err)
			}
			outgoing, err := filestore.OpenAppend(t.Context(), filestore.AppendRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "current")}, Mode: 0o600, Append: filestore.AppendCreate})
			if err != nil {
				t.Fatal(err)
			}
			if n, err := outgoing.Write(payload); err != nil || n != len(payload) {
				t.Fatalf("outgoing producer write = (%d,%v), want %d", n, err, len(payload))
			}
			t.Cleanup(func() { _ = outgoing.Close() })
			wantNative := tc.wantNative
			if tc.mutation == rotationPipeOutgoing || tc.mutation == rotationDirectoryOutgoing {
				if err := outgoing.Close(); err != nil {
					t.Fatal(err)
				}
				if tc.mutation == rotationPipeOutgoing {
					var peer *os.File
					outgoing, peer, err = os.Pipe()
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = peer.Close() })
					probe, probePeer, err := os.Pipe()
					if err != nil {
						t.Fatal(err)
					}
					syncErr := probe.Sync()
					closeErr, peerCloseErr := probe.Close(), probePeer.Close()
					var native *fs.PathError
					if closeErr != nil || peerCloseErr != nil || !errors.As(syncErr, &native) {
						t.Fatalf("Go pipe synchronization = (%v,%v,%v), want native sync refusal and closure", syncErr, closeErr, peerCloseErr)
					}
					wantNative = native.Err
				} else {
					if err := root.Mkdir("held-directory", 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(directory, "held-directory", "child"), payload, 0o600); err != nil {
						t.Fatal(err)
					}
					outgoing, err = os.Open(filepath.Join(directory, "held-directory"))
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			original, err := outgoing.Stat()
			if err != nil {
				t.Fatal(err)
			}
			request := filestore.RotationRequest{Outgoing: outgoing, Incoming: filestore.AppendRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "next")}, Mode: 0o640, Append: filestore.AppendCreate}}
			ctx := t.Context()
			switch tc.mutation {
			case rotationUnchanged, rotationPipeOutgoing, rotationDirectoryOutgoing:
			case rotationNilContext:
				ctx = nil
			case rotationCanceled:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case rotationNilOutgoing:
				request.Outgoing = nil
			case rotationClosedOutgoing:
				if err := outgoing.Close(); err != nil {
					t.Fatal(err)
				}
			case rotationNilRoot:
				request.Incoming.Location.Root = nil
			case rotationClosedRoot:
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			case rotationZeroPath:
				request.Incoming.Location.Path = core.RelativePath{}
			case rotationRootPath:
				request.Incoming.Location.Path = mustRelativePath(t, ".")
			case rotationMissingParent:
				request.Incoming.Location.Path = mustRelativePath(t, "missing/next")
			case rotationOccupiedIncoming:
				if err := os.WriteFile(filepath.Join(directory, "next"), neighbor, 0o600); err != nil {
					t.Fatal(err)
				}
			case rotationSameName:
				request.Incoming.Location.Path = mustRelativePath(t, "current")
			case rotationZeroMode:
				request.Incoming.Mode = 0
			case rotationTypeMode:
				request.Incoming.Mode = fs.ModeDir | 0o640
			case rotationExistingIntent:
				request.Incoming.Append = filestore.AppendExisting
			case rotationCreateOrOpenIntent:
				request.Incoming.Append = filestore.AppendCreateOrOpen
			case rotationUnknownIntent:
				request.Incoming.Append = filestore.AppendUnknown
			case rotationFutureIntent:
				request.Incoming.Append = filestore.AppendMode(255)
			default:
				t.Fatalf("mutation = %d, want declared boundary", tc.mutation)
			}
			want, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			var occupied fs.FileInfo
			if tc.mutation == rotationOccupiedIncoming {
				occupied, err = os.Lstat(filepath.Join(directory, "next"))
				if err != nil {
					t.Fatal(err)
				}
			}
			incoming, gotErr := filestore.RotateAppend(ctx, request)
			if incoming != nil {
				t.Cleanup(func() { _ = incoming.Close() })
			}
			if !errors.Is(gotErr, tc.wantErr) || wantNative != nil && !errors.Is(gotErr, wantNative) {
				t.Fatalf("rotation = (%v,%v), want %v with native %v", incoming, gotErr, tc.wantErr, wantNative)
			}
			gotOutgoing, gotOutgoingErr := outgoing.Stat()
			if tc.wantClosed {
				if !errors.Is(gotOutgoingErr, fs.ErrClosed) {
					t.Fatalf("transferred outgoing = %v, want native closed", gotOutgoingErr)
				}
			} else {
				if gotOutgoingErr != nil || !os.SameFile(original, gotOutgoing) || gotOutgoing.Size() != int64(len(payload)) || gotOutgoing.Mode() != original.Mode() || !gotOutgoing.ModTime().Equal(original.ModTime()) {
					t.Fatalf("caller-owned outgoing = (%v,%v), want unchanged live inode", gotOutgoing, gotOutgoingErr)
				}
				probe := []byte{8, 0, 255}
				if n, err := outgoing.Write(probe); err != nil || n != len(probe) {
					t.Fatalf("retained outgoing write = (%d,%v), want %d", n, err, len(probe))
				}
				for i := range want {
					if want[i].name == "current" {
						want[i].data = append(bytes.Clone(payload), probe...)
					}
				}
			}
			if tc.wantErr != nil {
				if incoming != nil {
					t.Fatalf("refused incoming = %v, want nil", incoming)
				}
			} else {
				if incoming == nil {
					t.Fatal("incoming = nil, want real append handle")
				}
				info, err := incoming.Stat()
				if err != nil {
					t.Fatal(err)
				}
				if !info.Mode().IsRegular() || info.Size() != 0 || info.Mode().Perm() != request.Incoming.Mode || os.SameFile(info, original) {
					t.Fatalf("incoming inode = %v, want new empty regular file mode %#o", info, request.Incoming.Mode)
				}
				suffix := []byte{255, 0, 7}
				if n, err := incoming.Write(suffix); err != nil || n != len(suffix) {
					t.Fatalf("incoming append = (%d,%v), want %d", n, err, len(suffix))
				}
				if _, err := incoming.Seek(0, io.SeekStart); err != nil {
					t.Fatal(err)
				}
				if n, err := incoming.Write(suffix); err != nil || n != len(suffix) {
					t.Fatalf("incoming append after seek = (%d,%v), want %d", n, err, len(suffix))
				}
				if err := incoming.Close(); err != nil {
					t.Fatal(err)
				}
				want = append(want, removalFixtureEntry{name: "next", mode: request.Incoming.Mode, data: append(bytes.Clone(suffix), suffix...)})
			}
			gotNamespace, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			if len(gotNamespace) != len(want) {
				t.Fatalf("namespace = %+v, want %+v", gotNamespace, want)
			}
			for i, got := range gotNamespace {
				if got.name != want[i].name || got.mode != want[i].mode || got.target != want[i].target || !bytes.Equal(got.data, want[i].data) {
					t.Fatalf("entry %d = %+v, want %+v", i, got, want[i])
				}
			}
			if occupied != nil {
				retained, err := os.Lstat(filepath.Join(directory, "next"))
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(occupied, retained) || !occupied.ModTime().Equal(retained.ModTime()) {
					t.Fatalf("occupied incoming = %v, want unchanged inode %v", retained, occupied)
				}
			}
		})
	}
}
