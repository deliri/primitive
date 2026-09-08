package filestore_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

type nativeInspectionFixture uint8

const (
	nativeInspectionRegular nativeInspectionFixture = iota
	nativeInspectionDirectory
	nativeInspectionRoot
	nativeInspectionFinalLink
	nativeInspectionDanglingLink
	nativeInspectionSelfLink
	nativeInspectionParentLink
	nativeInspectionAbsent
	nativeInspectionMissingParent
	nativeInspectionFileParent
	nativeInspectionFIFO
)

func TestInspectNativeObservationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		fixture  nativeInspectionFixture
		mode     fs.FileMode
		extent   int64
		wantKind filestore.PathKind
	}{
		{name: "filesystem root needs no final component", fixture: nativeInspectionRoot, wantKind: filestore.PathKindDirectory},
		{name: "empty directory cannot fabricate regular storage facts", fixture: nativeInspectionDirectory, mode: 0o700, wantKind: filestore.PathKindDirectory},
		{name: "empty regular file is an observed zero extent", mode: 0o600, wantKind: filestore.PathKindRegularFile},
		{name: "unreadable regular file still exposes exact native metadata", mode: 0, extent: 3, wantKind: filestore.PathKindRegularFile},
		{name: "final link does not borrow target kind or metadata", fixture: nativeInspectionFinalLink, wantKind: filestore.PathKindSymbolicLink},
		{name: "dangling link is present without a target", fixture: nativeInspectionDanglingLink, wantKind: filestore.PathKindSymbolicLink},
		{name: "self link is observed without traversal", fixture: nativeInspectionSelfLink, wantKind: filestore.PathKindSymbolicLink},
		{name: "intermediate link follows Go path semantics", fixture: nativeInspectionParentLink, extent: 3, wantKind: filestore.PathKindRegularFile},
		{name: "reachable missing leaf produces only absence", fixture: nativeInspectionAbsent, wantKind: filestore.PathKindAbsent},
		{name: "missing parent cannot become reachable absence", fixture: nativeInspectionMissingParent, wantKind: filestore.PathKindUnreachable},
		{name: "regular parent cannot become reachable absence", fixture: nativeInspectionFileParent, wantKind: filestore.PathKindUnreachable},
		{name: "named pipe is observed without opening a stream", fixture: nativeInspectionFIFO, wantKind: filestore.PathKindOther},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			name := filepath.Join(directory, "observed")
			switch tc.fixture {
			case nativeInspectionRegular:
				file, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := file.Close(); err != nil {
						t.Error(err)
					}
				})
				if err := file.Truncate(tc.extent); err != nil {
					t.Fatal(err)
				}
				if err := file.Chmod(tc.mode); err != nil {
					t.Fatal(err)
				}
			case nativeInspectionDirectory:
				if err := os.Mkdir(name, tc.mode); err != nil {
					t.Fatal(err)
				}
			case nativeInspectionRoot:
				name = filepath.VolumeName(directory) + string(os.PathSeparator)
			case nativeInspectionFinalLink:
				if err := os.WriteFile(filepath.Join(directory, "target"), []byte{0, 255, 1}, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("target", name); err != nil {
					t.Fatal(err)
				}
			case nativeInspectionDanglingLink:
				if err := os.Symlink("missing", name); err != nil {
					t.Fatal(err)
				}
			case nativeInspectionSelfLink:
				if err := os.Symlink(filepath.Base(name), name); err != nil {
					t.Fatal(err)
				}
			case nativeInspectionParentLink:
				if err := os.Mkdir(filepath.Join(directory, "target"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "target", "child"), []byte{0, 255, 1}, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("target", name); err != nil {
					t.Fatal(err)
				}
				name = filepath.Join(name, "child")
			case nativeInspectionAbsent:
			case nativeInspectionMissingParent:
				name = filepath.Join(name, "child")
			case nativeInspectionFileParent:
				if err := os.WriteFile(name, []byte{0, 255}, 0o600); err != nil {
					t.Fatal(err)
				}
				name = filepath.Join(name, "child")
			case nativeInspectionFIFO:
				if err := syscallMkfifo(name); errors.Is(err, errors.ErrUnsupported) {
					t.Skip("native FIFO creation unavailable on this platform")
				} else if err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("fixture = %v, want admitted setup", tc.fixture)
			}
			before, beforeErr := os.Lstat(name)
			path, err := core.ParseAbsolutePath(name)
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := filestore.Inspect(t.Context(), path)
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf("Inspect = (%v,%v), want valid native observation", got, gotErr)
			}
			kind, err := got.Kind()
			if err != nil || kind != tc.wantKind {
				t.Fatalf("kind = (%v,%v), want %v", kind, err, tc.wantKind)
			}
			size, sizeErr := got.SizeBytes()
			modified, modifiedErr := got.ModifiedAt()
			permissions, permissionsErr := got.Permissions()
			allocation, allocationErr := got.Allocation()
			absent := tc.wantKind == filestore.PathKindAbsent || tc.wantKind == filestore.PathKindUnreachable
			if absent {
				if beforeErr == nil {
					t.Fatalf("absence fixture = %v, want no native entry", before)
				}
				for _, err := range []error{sizeErr, modifiedErr, permissionsErr, allocationErr} {
					if !errors.Is(err, core.ErrFilestoreContract) {
						t.Fatalf("absent fact = %v, want contract refusal", err)
					}
				}
				if size != (core.ByteLength{}) || modified != (temporal.Instant{}) || permissions != (filestore.Permissions{}) || allocation != (filestore.Allocation{}) {
					t.Fatalf("absent metadata = (%v,%v,%v,%v), want zero facts", size, modified, permissions, allocation)
				}
				return
			}
			if beforeErr != nil {
				t.Fatal(beforeErr)
			}
			stamp, stampErr := modified.Nanoseconds()
			mode, modeErr := permissions.FileMode()
			if modifiedErr != nil || permissionsErr != nil || stampErr != nil || modeErr != nil || mode != before.Mode().Perm() {
				t.Fatalf("native metadata = (%d,%v), errors (%v,%v,%v,%v), want exact native metadata", stamp, mode, modifiedErr, permissionsErr, stampErr, modeErr)
			}
			// The filesystem root is ambient; its timestamp can change outside
			// this fixture. Every owned entry has an exact immutable timestamp.
			if tc.fixture != nativeInspectionRoot && stamp != before.ModTime().UnixNano() {
				t.Fatalf("timestamp = %d, want %d", stamp, before.ModTime().UnixNano())
			}
			if tc.wantKind == filestore.PathKindRegularFile {
				if sizeErr != nil || allocationErr != nil || size.Uint64() != uint64(tc.extent) {
					t.Fatalf("storage = (%v,%v,%v), want exact %d byte extent", size, sizeErr, allocationErr, tc.extent)
				}
			} else if !errors.Is(sizeErr, core.ErrFilestoreContract) || !errors.Is(allocationErr, core.ErrFilestoreContract) || size != (core.ByteLength{}) || allocation != (filestore.Allocation{}) {
				t.Fatalf("non-regular storage = (%v,%v,%v,%v), want zero facts and typed refusals", size, allocation, sizeErr, allocationErr)
			}
			after, err := os.Lstat(name)
			if err != nil || !os.SameFile(before, after) || after.Mode() != before.Mode() {
				t.Fatalf("after inode/mode = (%v,%v), want unchanged native entry", after, err)
			}
			if tc.fixture != nativeInspectionRoot && (after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime())) {
				t.Fatalf("observed extent/time = (%d,%v), want (%d,%v)", after.Size(), after.ModTime(), before.Size(), before.ModTime())
			}
		})
	}
}
