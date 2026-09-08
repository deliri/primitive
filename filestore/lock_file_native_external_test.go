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
	"github.com/deliri/primitive/v2026/filelock"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestLockFileNativeCapabilityLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		entry          nativeHandleEntry
		mutation       readUpdateMutation
		mode           fs.FileMode
		empty, noWrite bool
		wantErr        error
	}{
		{name: "missing carrier creates a real readable writable file", entry: nativeHandleMissing, mode: 0o640},
		{name: "existing binary carrier is reopened without truncation", mode: 0o640},
		{name: "empty existing carrier retains its inode", empty: true, mode: 0o640},
		{name: "same-mode reopen without writes preserves all metadata", mode: 0o600, noWrite: true},
		{name: "smallest nonzero permissions do not revoke held Go capability", mode: 1},
		{name: "all ordinary permission bits are applied exactly", mode: 0o777},
		{name: "confined link reopens only its regular referent", entry: nativeHandleConfinedLink, mode: 0o640},
		{name: "dangling confined link permits Go create on referent", entry: nativeHandleDanglingLink, mode: 0o640},
		{name: "directory cannot become a lock carrier", entry: nativeHandleDirectory, mode: 0o640, wantErr: core.ErrFilestoreDestination},
		{name: "outside link cannot become a lock carrier", entry: nativeHandleOutsideLink, mode: 0o640, wantErr: core.ErrFilestoreDestination},
		{name: "closed root cannot create a carrier", mutation: readUpdateClosedRoot, mode: 0o640, wantErr: core.ErrFilestoreDestination},
		{name: "nil context cannot acquire a carrier", mutation: readUpdateNilContext, mode: 0o640, wantErr: core.ErrNilContext},
		{name: "canceled context cannot acquire a carrier", mutation: readUpdateCanceled, mode: 0o640, wantErr: context.Canceled},
		{name: "nil root cannot select process cwd", mutation: readUpdateNilRoot, mode: 0o640, wantErr: core.ErrFilestoreContract},
		{name: "zero path cannot select a default carrier", mutation: readUpdateZeroPath, mode: 0o640, wantErr: core.ErrFilestoreContract},
		{name: "root path cannot become a mutable carrier", mutation: readUpdateRootPath, mode: 0o640, wantErr: core.ErrFilestoreContract},
		{name: "zero mode cannot silently select permissions", wantErr: core.ErrFilestoreContract},
		{name: "file type bits cannot be permission intent", mode: fs.ModeDir | 0o640, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := []byte{0, 255, 8, 0}
			if tc.empty {
				payload = nil
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
			wantNamespace, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			before, beforeErr := os.Lstat(filepath.Join(directory, physical))
			if beforeErr != nil && !errors.Is(beforeErr, fs.ErrNotExist) {
				t.Fatal(beforeErr)
			}
			request := filestore.LockFileRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "entry")}, Mode: tc.mode}
			ctx := t.Context()
			switch tc.mutation {
			case readUpdateUnchanged:
			case readUpdateClosedRoot:
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			case readUpdateNilContext:
				ctx = nil
			case readUpdateCanceled:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case readUpdateNilRoot:
				request.Location.Root = nil
			case readUpdateZeroPath:
				request.Location.Path = core.RelativePath{}
			case readUpdateRootPath:
				request.Location.Path = mustRelativePath(t, ".")
			default:
				t.Fatalf("mutation = %d, want declared request", tc.mutation)
			}
			var wantNative error
			if errors.Is(tc.wantErr, core.ErrFilestoreDestination) {
				native, nativeErr := root.OpenFile("entry", os.O_CREATE|os.O_RDWR, 0o640)
				if native != nil {
					_ = native.Close()
				}
				if nativeErr == nil {
					t.Fatalf("native fixture error = %v, want a native refusal", nativeErr)
				}
				wantNative = nativeErr
				if nativePath, ok := errors.AsType[*fs.PathError](nativeErr); ok {
					wantNative = nativePath.Err
				}
			}
			got, gotErr := filestore.OpenLockFile(ctx, request)
			if got != nil {
				t.Cleanup(func() { _ = got.Close() })
			}
			if !errors.Is(gotErr, tc.wantErr) || wantNative != nil && !errors.Is(gotErr, wantNative) {
				t.Fatalf("OpenLockFile = (%v,%v), want %v/native %v", got, gotErr, tc.wantErr, wantNative)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatalf("refused carrier = %v, want nil", got)
				}
				retained, err := removalFixtureSnapshot(directory)
				if err != nil {
					t.Fatal(err)
				}
				if len(retained) != len(wantNamespace) {
					t.Fatalf("namespace = %+v, want unchanged %+v", retained, wantNamespace)
				}
				for i, got := range retained {
					want := wantNamespace[i]
					if got.name != want.name || got.mode != want.mode || got.target != want.target || !bytes.Equal(got.data, want.data) {
						t.Fatalf("entry %d = %+v, want %+v", i, got, want)
					}
				}
			} else {
				if got == nil {
					t.Fatal("carrier = nil, want real Go file")
				}
				wantBytes := bytes.Clone(payload)
				if tc.entry == nativeHandleMissing || tc.entry == nativeHandleDanglingLink {
					wantBytes = nil
				}
				initial, err := got.Stat()
				if err != nil {
					t.Fatal(err)
				}
				if !initial.Mode().IsRegular() || initial.Mode().Perm() != tc.mode || initial.Size() != int64(len(wantBytes)) || before != nil && (!os.SameFile(before, initial) || !before.ModTime().Equal(initial.ModTime())) {
					t.Fatalf("carrier metadata = %v, want original inode/bytes and mode %#o", initial, tc.mode)
				}
				readBytes, err := io.ReadAll(io.LimitReader(got, int64(len(wantBytes)+1)))
				if err != nil || !bytes.Equal(readBytes, wantBytes) {
					t.Fatalf("carrier bytes = (%v,%v), want %v", readBytes, err, wantBytes)
				}
				if !tc.noWrite {
					change := []byte{255, 0}
					if n, err := got.WriteAt(change, 0); err != nil || n != len(change) {
						t.Fatalf("carrier rewrite = (%d,%v), want %d", n, err, len(change))
					}
					if len(wantBytes) < len(change) {
						wantBytes = append(wantBytes, make([]byte, len(change)-len(wantBytes))...)
					}
					copy(wantBytes, change)
				}
				if _, err := got.Seek(0, io.SeekStart); err != nil {
					t.Fatal(err)
				}
				retainedBytes, err := io.ReadAll(io.LimitReader(got, int64(len(wantBytes)+1)))
				if err != nil || !bytes.Equal(retainedBytes, wantBytes) {
					t.Fatalf("rewritten bytes = (%v,%v), want exact %v", retainedBytes, err, wantBytes)
				}
				info, err := got.Stat()
				if err != nil {
					t.Fatal(err)
				}
				named, err := root.Stat("entry")
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(info, initial) || !os.SameFile(named, info) || info.Mode().Perm() != tc.mode || info.Size() != int64(len(wantBytes)) || tc.noWrite && !info.ModTime().Equal(initial.ModTime()) {
					t.Fatalf("settled carrier = %v, want exact held inode and bytes", info)
				}
				if err := got.Close(); err != nil {
					t.Fatal(err)
				}
				if _, err := got.Stat(); !errors.Is(err, fs.ErrClosed) {
					t.Fatalf("closed carrier = %v, want native closed", err)
				}
				entries, err := os.ReadDir(directory)
				if err != nil {
					t.Fatal(err)
				}
				wantCount := 2
				if tc.entry == nativeHandleConfinedLink || tc.entry == nativeHandleDanglingLink {
					wantCount = 3
					target, err := root.Readlink("entry")
					if err != nil || target != "subject" {
						t.Fatalf("retained link = (%q,%v), want subject", target, err)
					}
				}
				if len(entries) != wantCount {
					t.Fatalf("namespace = %v, want %d entries", entries, wantCount)
				}
			}
			if tc.wantErr != nil && before != nil {
				retained, err := os.Lstat(filepath.Join(directory, physical))
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(before, retained) || before.Mode() != retained.Mode() || !before.ModTime().Equal(retained.ModTime()) {
					t.Fatalf("refused inode = %v, want unchanged %v", retained, before)
				}
			}
			gotNeighbor, err := os.ReadFile(filepath.Join(directory, "neighbor"))
			if err != nil || !bytes.Equal(gotNeighbor, neighbor) {
				t.Fatalf("neighbor = (%v,%v), want %v", gotNeighbor, err, neighbor)
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

type lockIdentityFixture uint8

const (
	lockSameName lockIdentityFixture = iota
	lockHardLink
	lockReplacedName
	lockDistinctName
)

func TestLockFileProducesExactNativeLockIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                  string
		fixture               lockIdentityFixture
		empty, wantSecondHeld bool
	}{
		{name: "two opens of one name contend on the same inode"},
		{name: "empty carrier still supplies real lock identity", empty: true},
		{name: "hard-linked names cannot acquire independent exclusivity", fixture: lockHardLink},
		{name: "replaced name identifies independent lock despite identical bytes", fixture: lockReplacedName, wantSecondHeld: true},
		{name: "separate same-byte files hold independent native locks", fixture: lockDistinctName, wantSecondHeld: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := []byte{0, 255, 7}
			if tc.empty {
				payload = nil
			}
			first, err := filestore.OpenLockFile(t.Context(), filestore.LockFileRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "entry")}, Mode: 0o600})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = first.Close() })
			if n, err := first.WriteAt(payload, 0); err != nil || n != len(payload) {
				t.Fatalf("carrier write = (%d,%v), want %d", n, err, len(payload))
			}
			original, err := first.Stat()
			if err != nil {
				t.Fatal(err)
			}
			secondName := "entry"
			wantNames := 1
			switch tc.fixture {
			case lockSameName:
			case lockHardLink:
				secondName = "alias"
				wantNames++
				if err := root.Link("entry", secondName); err != nil {
					t.Fatal(err)
				}
			case lockReplacedName:
				wantNames++
				if err := root.Rename("entry", "archive"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "entry"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
			case lockDistinctName:
				secondName = "other"
				wantNames++
				if err := os.WriteFile(filepath.Join(directory, secondName), payload, 0o600); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("fixture = %d, want declared native identity", tc.fixture)
			}
			second, err := filestore.OpenLockFile(t.Context(), filestore.LockFileRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, secondName)}, Mode: 0o600})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = second.Close() })
			secondInfo, err := second.Stat()
			if err != nil {
				t.Fatal(err)
			}
			if same := os.SameFile(original, secondInfo); same == tc.wantSecondHeld {
				t.Fatalf("native identity same = %t, want second independent %t", same, tc.wantSecondHeld)
			}
			t.Cleanup(func() {
				_ = filelock.Release(context.Background(), first)
				_ = filelock.Release(context.Background(), second)
			})
			firstAcquisition, err := filelock.Acquire(t.Context(), filelock.Request{File: first, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate})
			if err != nil {
				t.Fatal(err)
			}
			firstHeld, err := firstAcquisition.Held()
			if err != nil || !firstHeld {
				t.Fatalf("first lock = (%t,%v), want held", firstHeld, err)
			}
			secondAcquisition, err := filelock.Acquire(t.Context(), filelock.Request{File: second, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate})
			if err != nil {
				t.Fatal(err)
			}
			secondHeld, err := secondAcquisition.Held()
			if err != nil || secondHeld != tc.wantSecondHeld {
				t.Fatalf("second lock = (%t,%v), want held %t", secondHeld, err, tc.wantSecondHeld)
			}
			if secondHeld {
				if err := filelock.Release(t.Context(), second); err != nil {
					t.Fatal(err)
				}
			}
			if err := filelock.Release(t.Context(), first); err != nil {
				t.Fatal(err)
			}
			releasedAcquisition, err := filelock.Acquire(t.Context(), filelock.Request{File: second, Exclusivity: filelock.Exclusive, Patience: filelock.Immediate})
			if err != nil {
				t.Fatal(err)
			}
			releasedHeld, err := releasedAcquisition.Held()
			if err != nil || !releasedHeld {
				t.Fatalf("second lock after release = (%t,%v), want held", releasedHeld, err)
			}
			if err := filelock.Release(t.Context(), second); err != nil {
				t.Fatal(err)
			}
			for i, file := range []*os.File{first, second} {
				info, err := file.Stat()
				if err != nil {
					t.Fatal(err)
				}
				wantInfo := original
				if i == 1 {
					wantInfo = secondInfo
				}
				if !os.SameFile(info, wantInfo) || info.Mode() != wantInfo.Mode() || info.Size() != int64(len(payload)) || !info.ModTime().Equal(wantInfo.ModTime()) {
					t.Fatalf("carrier %d = %v, want retained metadata %v", i, info, wantInfo)
				}
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					t.Fatal(err)
				}
				got, err := io.ReadAll(io.LimitReader(file, int64(len(payload)+1)))
				if err != nil || !bytes.Equal(got, payload) {
					t.Fatalf("carrier %d bytes = (%v,%v), want %v", i, got, err, payload)
				}
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != wantNames {
				t.Fatalf("namespace = (%v,%v), want %d carriers", entries, err, wantNames)
			}
		})
	}
}
