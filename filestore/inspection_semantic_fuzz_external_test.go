package filestore_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzInspectionNativeFactsSemanticClosure(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}

	kinds := [...]filestore.PathKind{filestore.PathKindAbsent, filestore.PathKindDirectory, filestore.PathKindRegularFile, filestore.PathKindSymbolicLink, filestore.PathKindOther, filestore.PathKindUnreachable}
	f.Add(emitted, uint8(slices.Index(kinds[:], filestore.PathKindRegularFile)), uint16(0o600))
	for index := range kinds {
		f.Add([]byte{0, 255}, uint8(index), uint16(0o600))
	}
	for _, seed := range []struct {
		kind    filestore.PathKind
		payload []byte
		mode    uint16
	}{
		{kind: filestore.PathKindRegularFile, payload: []byte{}},
		{kind: filestore.PathKindUnreachable, payload: []byte{0}, mode: 1},
	} {
		index := slices.Index(kinds[:], seed.kind)
		if index < 0 {
			f.Fatalf("seed kind %v is absent from fixture domain", seed.kind)
		}
		f.Add(seed.payload, uint8(index), seed.mode)
	}
	f.Fuzz(func(t *testing.T, payload []byte, selector uint8, rawMode uint16) {
		payload = payload[:min(len(payload), 4096)]
		kind := kinds[int(selector)%len(kinds)]
		directory := t.TempDir()
		name := filepath.Join(directory, "observed")
		mode := fs.FileMode(rawMode).Perm()
		var opened *os.File
		switch kind {
		case filestore.PathKindRegularFile:
			var err error
			opened, err = os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := opened.Close(); err != nil {
					t.Error(err)
				}
			})
			if n, err := opened.Write(payload); err != nil || n != len(payload) {
				t.Fatalf("fixture write = (%d,%v), want %d bytes", n, err, len(payload))
			}
			if err := opened.Chmod(mode); err != nil {
				t.Fatal(err)
			}
		case filestore.PathKindDirectory:
			if err := os.Mkdir(name, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(name, 0o700); err != nil {
					t.Error(err)
				}
			})
			if err := os.Chmod(name, mode); err != nil {
				t.Fatal(err)
			}
		case filestore.PathKindSymbolicLink:
			if err := os.Symlink("target-"+hex.EncodeToString(payload[:min(len(payload), 16)]), name); err != nil {
				t.Fatal(err)
			}
		case filestore.PathKindOther:
			if err := syscallMkfifo(name); errors.Is(err, errors.ErrUnsupported) {
				t.Skip("this host has no POSIX FIFO fixture")
			} else if err != nil {
				t.Fatal(err)
			}
		case filestore.PathKindUnreachable:
			if rawMode&1 != 0 {
				if err := os.WriteFile(name, payload, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			name = filepath.Join(name, "child")
		case filestore.PathKindAbsent:
		default:
			t.Fatalf("fixture kind = %v, want known OS entry kind", kind)
		}
		before, beforeErr := os.Lstat(name)
		present := kind != filestore.PathKindAbsent && kind != filestore.PathKindUnreachable
		if present && beforeErr != nil {
			t.Fatal(beforeErr)
		}
		if !present && !nativeInspectionAbsence(beforeErr) {
			t.Fatalf("missing fixture = %v, want native absence or non-directory parent", beforeErr)
		}
		path, err := core.ParseAbsolutePath(name)
		if err != nil {
			t.Fatal(err)
		}
		observation, err := filestore.Inspect(t.Context(), path)
		if err != nil || observation.Validate() != nil {
			t.Fatalf("Inspect = (%v,%v), want valid observation", observation, err)
		}
		gotKind, err := observation.Kind()
		if err != nil || gotKind != kind {
			t.Fatalf("kind = (%v,%v), want %v", gotKind, err, kind)
		}
		size, sizeErr := observation.SizeBytes()
		modified, modifiedErr := observation.ModifiedAt()
		permissions, permissionsErr := observation.Permissions()
		owner, ownerErr := observation.Ownership()
		allocation, allocationErr := observation.Allocation()
		if !present {
			for _, gotErr := range []error{sizeErr, modifiedErr, permissionsErr, ownerErr, allocationErr} {
				if !errors.Is(gotErr, core.ErrFilestoreContract) {
					t.Fatalf("absent metadata error = %v, want contract refusal", gotErr)
				}
			}
			if size != (core.ByteLength{}) || modified != (temporal.Instant{}) || permissions != (filestore.Permissions{}) || owner != (filestore.Ownership{}) || allocation != (filestore.Allocation{}) {
				t.Fatalf("absent metadata = (%v,%v,%v,%v,%v), want zero facts", size, modified, permissions, owner, allocation)
			}
		} else {
			stamp, stampErr := modified.Nanoseconds()
			bits, bitsErr := permissions.FileMode()
			uid, uidErr := owner.UID()
			gid, gidErr := owner.GID()
			native, nativeErr := nativeInspectionStorage(before)
			if nativeErr != nil {
				t.Fatal(nativeErr)
			}
			if modifiedErr != nil || permissionsErr != nil || stampErr != nil || bitsErr != nil || stamp != before.ModTime().UnixNano() || bits != before.Mode().Perm() {
				t.Fatalf("entry facts = (%d,%v,%d,%d), errors (%v,%v,%v,%v,%v,%v,%v), want exact native time/mode/owner", stamp, bits, uid, gid, modifiedErr, permissionsErr, ownerErr, stampErr, bitsErr, uidErr, gidErr)
			}
			if native.reported {
				if ownerErr != nil || uidErr != nil || gidErr != nil || uid != native.uid || gid != native.gid || !owner.IsSet() {
					t.Fatalf("owner = (%v,%v,%v,%v), want native uid=%d gid=%d", owner, ownerErr, uidErr, gidErr, native.uid, native.gid)
				}
			} else if owner != (filestore.Ownership{}) || uid != 0 || gid != 0 || !errors.Is(ownerErr, core.ErrFilestoreContract) || !errors.Is(uidErr, core.ErrFilestoreContract) || !errors.Is(gidErr, core.ErrFilestoreContract) {
				t.Fatalf("unavailable owner = (%v,%v,%v,%v), want zero and typed refusals", owner, ownerErr, uidErr, gidErr)
			}
			if kind == filestore.PathKindRegularFile {
				allocated, allocatedErr := allocation.Bytes()
				if sizeErr != nil || allocationErr != nil || size.Uint64() != uint64(len(payload)) || allocation.Reported() != native.reported || allocated.Uint64() != native.bytes {
					t.Fatalf("regular storage = (%v,%v), errors (%v,%v,%v), want exact payload/native allocation", size, allocated, sizeErr, allocationErr, allocatedErr)
				}
				if native.reported && allocatedErr != nil || !native.reported && (!errors.Is(allocatedErr, core.ErrFilestoreContract) || allocated != (core.ByteLength{})) {
					t.Fatalf("allocation bytes = (%v,%v), want reported=%t", allocated, allocatedErr, native.reported)
				}
				gotBytes := make([]byte, len(payload))
				if n, err := opened.ReadAt(gotBytes, 0); err != nil || n != len(payload) || !bytes.Equal(gotBytes, payload) {
					t.Fatalf("retained bytes = (%d,%v,%v), want unchanged %v", n, gotBytes, err, payload)
				}
			} else if !errors.Is(sizeErr, core.ErrFilestoreContract) || !errors.Is(allocationErr, core.ErrFilestoreContract) || size != (core.ByteLength{}) || allocation != (filestore.Allocation{}) {
				t.Fatalf("non-regular storage = (%v,%v,%v,%v), want zero facts and refusals", size, allocation, sizeErr, allocationErr)
			}
		}
		after, afterErr := os.Lstat(name)
		if present {
			if afterErr != nil || !os.SameFile(before, after) || after.Mode() != before.Mode() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
				t.Fatalf("after stat = (%v,%v), want identical inode/mode/extent/time", after, afterErr)
			}
		} else if !nativeInspectionAbsence(afterErr) {
			t.Fatalf("absent after = %v, want unchanged native absence", afterErr)
		}
		entries, err := os.ReadDir(directory)
		wantEntries := 0
		if present || kind == filestore.PathKindUnreachable && rawMode&1 != 0 {
			wantEntries = 1
		}
		if err != nil || len(entries) != wantEntries {
			t.Fatalf("namespace = (%v,%v), want %d original entries", entries, err, wantEntries)
		}
	})
}

// These native coordinates differ between Go's POSIX and Windows FileInfo.Sys.
// The fixture leaves return facts; the semantic checks stay in the fuzz body.
type inspectionNativeStorage struct {
	uid, gid uint32
	bytes    uint64
	reported bool
}
