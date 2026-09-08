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

type nativeHandleDoor uint8

const (
	nativeHandleRead nativeHandleDoor = iota
	nativeHandleUpdate
	nativeHandleAppend
	nativeHandleLock
	nativeHandleDoorLimit
)

type nativeHandleEntry uint8

const (
	nativeHandleRegular nativeHandleEntry = iota
	nativeHandleMissing
	nativeHandleDirectory
	nativeHandleConfinedLink
	nativeHandleDanglingLink
	nativeHandleOutsideLink
	nativeHandleEntryLimit
)

func FuzzReadUpdateAppendLockNativeHandleCustody(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(uint8(nativeHandleRead), uint8(nativeHandleRegular), uint8(0), emitted, []byte{31}, uint16(1), false, false)
	for _, seed := range []struct {
		door              nativeHandleDoor
		entry             nativeHandleEntry
		append            filestore.AppendMode
		payload, change   []byte
		canceled, replace bool
	}{
		{door: nativeHandleRead, payload: []byte{0, 255}, change: []byte{31}, replace: true},
		{door: nativeHandleRead, entry: nativeHandleConfinedLink, payload: []byte{0, 255}},
		{door: nativeHandleRead, entry: nativeHandleDirectory},
		{door: nativeHandleUpdate, payload: []byte{0, 255}, change: []byte{31}, replace: true},
		{door: nativeHandleUpdate, entry: nativeHandleMissing},
		{door: nativeHandleAppend, append: filestore.AppendCreate, entry: nativeHandleMissing, change: []byte{0, 255}},
		{door: nativeHandleAppend, append: filestore.AppendCreate, payload: []byte{0, 255}},
		{door: nativeHandleAppend, append: filestore.AppendExisting, payload: []byte{0, 255}, change: []byte{31}, replace: true},
		{door: nativeHandleAppend, append: filestore.AppendCreateOrOpen, entry: nativeHandleDanglingLink},
		{door: nativeHandleLock, entry: nativeHandleMissing, change: []byte{0, 255}},
		{door: nativeHandleLock, entry: nativeHandleDanglingLink, change: []byte{0, 255}},
		{door: nativeHandleLock, payload: []byte{0, 255}, replace: true},
		{door: nativeHandleLock, entry: nativeHandleOutsideLink, payload: []byte{0, 255}},
		{door: nativeHandleLock, entry: nativeHandleMissing, canceled: true},
	} {
		appendIndex := uint8(0)
		switch seed.append {
		case filestore.AppendExisting:
			appendIndex = 1
		case filestore.AppendCreateOrOpen:
			appendIndex = 2
		}
		f.Add(uint8(seed.door), uint8(seed.entry), appendIndex, seed.payload, seed.change, uint16(1), seed.canceled, seed.replace)
	}
	f.Fuzz(func(t *testing.T, rawDoor, rawEntry, rawAppend uint8, payload, change []byte, rawOffset uint16, canceled, replaced bool) {
		door := nativeHandleDoor(rawDoor % uint8(nativeHandleDoorLimit))
		entry := nativeHandleEntry(rawEntry % uint8(nativeHandleEntryLimit))
		appendModes := [...]filestore.AppendMode{filestore.AppendCreate, filestore.AppendExisting, filestore.AppendCreateOrOpen}
		appendMode := appendModes[int(rawAppend)%len(appendModes)]
		payload = payload[:min(len(payload), 1024)]
		change = change[:min(len(change), 1024)]
		offset := int64(rawOffset % uint16(len(payload)+2))
		directories := [2]string{t.TempDir(), t.TempDir()}
		var roots [2]*os.Root
		var files [2]*os.File
		var openErrors [2]error
		var original [2]fs.FileInfo
		outside := t.TempDir()
		outsidePath := filepath.Join(outside, "outside")
		outsideBytes := []byte{41, 0, 255}
		if err := os.WriteFile(outsidePath, outsideBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		outsideInfo, err := os.Lstat(outsidePath)
		if err != nil {
			t.Fatal(err)
		}
		physical := "entry"
		if entry == nativeHandleConfinedLink || entry == nativeHandleDanglingLink {
			physical = "subject"
		}
		for i, directory := range directories {
			roots[i] = requireTestRoot(t, directory)
			if err := os.WriteFile(filepath.Join(directory, "neighbor"), outsideBytes, 0o600); err != nil {
				t.Fatal(err)
			}
			switch entry {
			case nativeHandleRegular:
				if err := os.WriteFile(filepath.Join(directory, "entry"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
			case nativeHandleMissing:
			case nativeHandleDirectory:
				if err := os.Mkdir(filepath.Join(directory, "entry"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "entry", "child"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
			case nativeHandleConfinedLink, nativeHandleDanglingLink:
				if entry == nativeHandleConfinedLink {
					if err := os.WriteFile(filepath.Join(directory, "subject"), payload, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if err := roots[i].Symlink("subject", "entry"); err != nil {
					t.Fatal(err)
				}
			case nativeHandleOutsideLink:
				if err := roots[i].Symlink(outsidePath, "entry"); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("entry = %d, want closed fixture domain", entry)
			}
			original[i], err = os.Stat(filepath.Join(directory, physical))
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Fatal(err)
			}
		}
		location := filestore.Location{Root: roots[0], Path: mustRelativePath(t, "entry")}
		ctx := t.Context()
		if canceled {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
		}
		switch door {
		case nativeHandleRead:
			files[0], openErrors[0] = filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: location})
		case nativeHandleUpdate:
			files[0], openErrors[0] = filestore.OpenUpdate(ctx, filestore.UpdateHandleRequest{Location: location})
		case nativeHandleAppend:
			files[0], openErrors[0] = filestore.OpenAppend(ctx, filestore.AppendRequest{Location: location, Mode: 0o640, Append: appendMode})
		case nativeHandleLock:
			files[0], openErrors[0] = filestore.OpenLockFile(ctx, filestore.LockFileRequest{Location: location, Mode: 0o640})
		default:
			t.Fatalf("door = %d, want closed public boundary", door)
		}
		// Independent Go flags spell the admitted native capability. Create-or-open
		// may open an existing entry but cannot create through a dangling link.
		if canceled {
			openErrors[1] = context.Canceled
		} else {
			flags := os.O_RDONLY
			switch door {
			case nativeHandleRead:
			case nativeHandleUpdate:
				flags = os.O_RDWR
			case nativeHandleLock:
				flags = os.O_CREATE | os.O_RDWR
			case nativeHandleAppend:
				flags = os.O_APPEND | os.O_WRONLY
				if appendMode != filestore.AppendExisting {
					flags |= os.O_CREATE | os.O_EXCL
				}
			}
			files[1], openErrors[1] = roots[1].OpenFile("entry", flags, 0o640)
			if door == nativeHandleAppend && appendMode == filestore.AppendCreateOrOpen && errors.Is(openErrors[1], fs.ErrExist) {
				files[1], openErrors[1] = roots[1].OpenFile("entry", os.O_APPEND|os.O_WRONLY, 0)
			}
			if openErrors[1] == nil {
				info, err := files[1].Stat()
				if err != nil {
					t.Fatal(err)
				}
				if !info.Mode().IsRegular() {
					if err := files[1].Close(); err != nil {
						t.Fatal(err)
					}
					files[1] = nil
					openErrors[1] = fs.ErrInvalid
				}
			}
			if openErrors[1] == nil && (door == nativeHandleLock || door == nativeHandleAppend && original[1] == nil) {
				if err := files[1].Chmod(0o640); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, file := range files {
			if file != nil {
				t.Cleanup(func() { _ = file.Close() })
			}
		}
		if openErrors[1] != nil {
			wantNative := openErrors[1]
			if pathErr, ok := errors.AsType[*fs.PathError](wantNative); ok {
				wantNative = pathErr.Err
			}
			var wantBoundary error = core.ErrFilestoreActivation
			switch door {
			case nativeHandleRead:
				wantBoundary = core.ErrFilestoreSource
			case nativeHandleLock:
				wantBoundary = core.ErrFilestoreDestination
			case nativeHandleAppend:
				if appendMode == filestore.AppendCreate && errors.Is(wantNative, fs.ErrExist) {
					wantBoundary = core.ErrFilestoreConflict
				}
			}
			if canceled {
				wantBoundary = context.Canceled
			}
			if files[0] != nil || !errors.Is(openErrors[0], wantBoundary) || !errors.Is(openErrors[0], wantNative) {
				t.Fatalf("acquisition = (%v,%v), want no capability and %v/native %v", files[0], openErrors[0], wantBoundary, wantNative)
			}
		} else {
			if files[0] == nil || openErrors[0] != nil {
				t.Fatalf("acquisition = (%v,%v), want live native file", files[0], openErrors[0])
			}
			for i, file := range files {
				info, err := file.Stat()
				if err != nil {
					t.Fatal(err)
				}
				if original[i] != nil && !os.SameFile(info, original[i]) {
					t.Fatalf("opened inode = %v, want original %v", info, original[i])
				}
				if replaced {
					if err := roots[i].Rename(physical, "archive"); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(directories[i], physical), outsideBytes, 0o600); err != nil {
						t.Fatal(err)
					}
					stranger, err := roots[i].Lstat(physical)
					if err != nil {
						t.Fatal(err)
					}
					if os.SameFile(info, stranger) {
						t.Fatalf("substitution inode = %v, want distinct from %v", stranger, info)
					}
				}
			}
			// The native read-only and append-only restrictions are exercised even
			// for empty source/change data, so those capabilities cannot become wider.
			if door == nativeHandleRead {
				var counts [2]int
				var causes [2]error
				for i, file := range files {
					counts[i], causes[i] = file.Write([]byte{255})
				}
				var native *fs.PathError
				if !errors.As(causes[1], &native) || counts[0] != 0 || !errors.Is(causes[0], native.Err) {
					t.Fatalf("read-only write = (%d,%v), want native refusal %v", counts[0], causes[0], causes[1])
				}
			} else {
				for i, file := range files {
					var n int
					var err error
					if door == nativeHandleAppend {
						if _, err := file.Seek(0, io.SeekStart); err != nil {
							t.Fatal(err)
						}
						n, err = file.Write(change)
					} else {
						n, err = file.WriteAt(change, offset)
					}
					if err != nil || n != len(change) {
						t.Fatalf("native change %d = (%d,%v), want %d", i, n, err, len(change))
					}
				}
			}
			if door == nativeHandleAppend {
				var one [1]byte
				_, nativeErr := files[1].ReadAt(one[:], 0)
				_, gotErr := files[0].ReadAt(one[:], 0)
				var native *fs.PathError
				if !errors.As(nativeErr, &native) || !errors.Is(gotErr, native.Err) {
					t.Fatalf("append-only read = %v, want native refusal %v", gotErr, nativeErr)
				}
			} else {
				var readBytes [2][]byte
				for i, file := range files {
					if _, err := file.Seek(0, io.SeekStart); err != nil {
						t.Fatal(err)
					}
					var err error
					readBytes[i], err = io.ReadAll(io.LimitReader(file, 3073))
					if err != nil {
						t.Fatal(err)
					}
				}
				if !bytes.Equal(readBytes[0], readBytes[1]) {
					t.Fatalf("held native bytes = %v, want %v", readBytes[0], readBytes[1])
				}
			}
			for _, file := range files {
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
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
		for i, entry := range got {
			if entry.name != want[i].name || entry.mode != want[i].mode || entry.target != want[i].target || !bytes.Equal(entry.data, want[i].data) {
				t.Fatalf("entry %d = %+v, want native %+v", i, entry, want[i])
			}
		}
		retained, err := os.Lstat(outsidePath)
		if err != nil {
			t.Fatal(err)
		}
		retainedBytes, err := os.ReadFile(outsidePath)
		if err != nil || !bytes.Equal(retainedBytes, outsideBytes) || !os.SameFile(retained, outsideInfo) || retained.Mode() != outsideInfo.Mode() || !retained.ModTime().Equal(outsideInfo.ModTime()) {
			t.Fatalf("outside = (%v,%v,%v), want unchanged bytes and metadata", retained, retainedBytes, err)
		}
	})
}
