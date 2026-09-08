package filestore_test

import (
	"bytes"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func custodyAbsolute(t *testing.T, elements ...string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(filepath.Join(elements...))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

// The types own admission. Native custody, lock effects, and namespace failures
// have separate tables; this table isolates the validator's complete fields.
func TestCustodyRequestAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	type door uint8
	const (
		touch door = iota
		durable
		lock
	)
	for _, tc := range []struct {
		name      string
		operation door
		path      string
		instant   temporal.Instant
		mode      fs.FileMode
		rootless  bool
		wantErr   error
	}{
		{name: "epoch is selected rather than unset", path: "entry", instant: temporal.InstantFromNanoseconds(0)},
		{name: "minimum signed instant cannot be narrowed", path: "entry", instant: temporal.InstantFromNanoseconds(math.MinInt64)},
		{name: "maximum signed instant cannot be narrowed", path: "entry", instant: temporal.InstantFromNanoseconds(math.MaxInt64)},
		{name: "nested touch remains a relative agreement", path: "nested/entry", instant: temporal.InstantFromNanoseconds(-1)},
		{name: "touch without root refuses", path: "entry", instant: temporal.InstantFromNanoseconds(0), rootless: true, wantErr: core.ErrFilestoreContract},
		{name: "touch without path refuses", instant: temporal.InstantFromNanoseconds(0), wantErr: core.ErrFilestoreContract},
		{name: "root entry cannot acquire a parent durability promise", path: ".", instant: temporal.InstantFromNanoseconds(0), wantErr: core.ErrFilestoreContract},
		{name: "unset instant cannot turn into current time", path: "entry", wantErr: core.ErrFilestoreContract},
		{name: "zero touch cannot manufacture ownership", rootless: true, wantErr: core.ErrFilestoreContract},
		{name: "durability admits a named entry", operation: durable, path: "entry"},
		{name: "durability admits a nested entry", operation: durable, path: "nested/entry"},
		{name: "durability without root refuses", operation: durable, path: "entry", rootless: true, wantErr: core.ErrFilestoreContract},
		{name: "durability without path refuses", operation: durable, wantErr: core.ErrFilestoreContract},
		{name: "durability cannot promise an outside parent", operation: durable, path: ".", wantErr: core.ErrFilestoreContract},
		{name: "zero durability cannot manufacture ownership", operation: durable, rootless: true, wantErr: core.ErrFilestoreContract},
		{name: "lock admits the lowest permission bit", operation: lock, path: "entry", mode: 1},
		{name: "lock admits all ordinary permission bits", operation: lock, path: "entry", mode: fs.ModePerm},
		{name: "lock admits a nested dot entry", operation: lock, path: "nested/.lock", mode: 0o600},
		{name: "lock without root refuses", operation: lock, path: "entry", mode: 0o600, rootless: true, wantErr: core.ErrFilestoreContract},
		{name: "lock without path refuses", operation: lock, mode: 0o600, wantErr: core.ErrFilestoreContract},
		{name: "lock cannot name the root itself", operation: lock, path: ".", mode: 0o600, wantErr: core.ErrFilestoreContract},
		{name: "lock cannot select an unset permission mode", operation: lock, path: "entry", wantErr: core.ErrFilestoreContract},
		{name: "permission field cannot carry a high bit", operation: lock, path: "entry", mode: fs.ModePerm + 1, wantErr: core.ErrFilestoreContract},
		{name: "all bits set cannot disguise type or special flags", operation: lock, path: "entry", mode: ^fs.FileMode(0), wantErr: core.ErrFilestoreContract},
		{name: "zero lock cannot manufacture ownership", operation: lock, rootless: true, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			if tc.rootless {
				root = nil
			}
			var path core.RelativePath
			if tc.path != "" {
				path = mustRelativePath(t, tc.path)
			}
			location := filestore.Location{Root: root, Path: path}
			var got error
			switch tc.operation {
			case touch:
				got = (filestore.TouchRequest{Location: location, ModifiedAt: tc.instant}).Validate()
			case durable:
				got = (filestore.DurabilityRequest{Location: location}).Validate()
			case lock:
				got = (filestore.LockFileRequest{Location: location, Mode: tc.mode}).Validate()
			default:
				t.Fatalf("door = %v, want declared custody validator", tc.operation)
			}
			if !errors.Is(got, tc.wantErr) {
				t.Fatalf("Validate = %v, want %v", got, tc.wantErr)
			}
			for _, class := range []error{core.ErrFilestoreSource, core.ErrFilestoreDestination, core.ErrFilestoreActivation, core.ErrFilestoreCleanup, core.ErrFilestoreConflict, core.ErrFilestoreSize} {
				if errors.Is(got, class) {
					t.Fatalf("validation = %v, want no %v execution", got, class)
				}
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 0 {
				t.Fatalf("validator effects = (%v,%v), want no entries", entries, err)
			}
		})
	}
}

// Go's native timestamp observation is the precision oracle. Primitive must
// neither clamp time to seconds nor narrow signed nanoseconds to 32-bit time.
func TestTouchNativeTimestampBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		instant temporal.Instant
		wantErr error
	}{
		{name: "unset time refuses before stamping", wantErr: core.ErrFilestoreContract},
		{name: "minimum signed nanosecond retains native precision", instant: temporal.InstantFromNanoseconds(math.MinInt64)},
		{name: "one above minimum cannot round down", instant: temporal.InstantFromNanoseconds(math.MinInt64 + 1)},
		{name: "last nanosecond before epoch remains negative", instant: temporal.InstantFromNanoseconds(-1)},
		{name: "epoch remains explicitly selected", instant: temporal.InstantFromNanoseconds(0)},
		{name: "first positive nanosecond cannot become epoch", instant: temporal.InstantFromNanoseconds(1)},
		{name: "first second beyond signed 32 bit remains representable", instant: temporal.InstantFromNanoseconds((int64(math.MaxInt32) + 1) * int64(time.Second))},
		{name: "one below maximum cannot round up", instant: temporal.InstantFromNanoseconds(math.MaxInt64 - 1)},
		{name: "maximum signed nanosecond retains native precision", instant: temporal.InstantFromNanoseconds(math.MaxInt64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := []byte{0, 255, 3, 16}
			initial := temporal.InstantFromNanoseconds(int64(time.Second))
			initialTime, err := initial.Time()
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"entry", "oracle", "neighbor"} {
				if err := root.WriteFile(name, payload, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := root.Chtimes(name, initialTime, initialTime); err != nil {
					t.Fatal(err)
				}
			}
			before, err := root.Stat("entry")
			if err != nil {
				t.Fatal(err)
			}
			neighborBefore, err := root.Stat("neighbor")
			if err != nil {
				t.Fatal(err)
			}
			var nativeErr error
			if tc.wantErr == nil {
				stamp, err := tc.instant.Time()
				if err != nil {
					t.Fatal(err)
				}
				nativeErr = root.Chtimes("oracle", stamp, stamp)
			}
			wantInfo, err := root.Stat("oracle")
			if err != nil {
				t.Fatal(err)
			}
			gotErr := filestore.Touch(t.Context(), filestore.TouchRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "entry")}, ModifiedAt: tc.instant})
			wantErr := tc.wantErr
			if nativeErr != nil {
				wantErr = core.ErrFilestoreActivation
				var native *os.PathError
				if !errors.As(nativeErr, &native) || !errors.Is(gotErr, native.Err) {
					t.Fatalf("Touch = %v, want native %v", gotErr, nativeErr)
				}
			}
			if !errors.Is(gotErr, wantErr) {
				t.Fatalf("Touch = %v, want %v", gotErr, wantErr)
			}
			for _, class := range []error{core.ErrFilestoreSource, core.ErrFilestoreDestination, core.ErrFilestoreActivation, core.ErrFilestoreCleanup, core.ErrFilestoreSize} {
				if errors.Is(gotErr, class) != (class == wantErr) {
					t.Fatalf("Touch class = %v, want only %v", gotErr, wantErr)
				}
			}
			after, err := root.Stat("entry")
			if err != nil {
				t.Fatal(err)
			}
			gotBytes, err := root.ReadFile("entry")
			if err != nil || !bytes.Equal(gotBytes, payload) || !os.SameFile(before, after) || after.Mode() != before.Mode() || !after.ModTime().Equal(wantInfo.ModTime()) {
				t.Fatalf("entry = (%v,%v,%v), want same inode, bytes and native time %v", after, gotBytes, err, wantInfo.ModTime())
			}
			observation, err := filestore.Inspect(t.Context(), custodyAbsolute(t, directory, "entry"))
			if err != nil {
				t.Fatal(err)
			}
			gotInstant, err := observation.ModifiedAt()
			if err != nil {
				t.Fatal(err)
			}
			gotTime, err := gotInstant.Time()
			if err != nil || !gotTime.Equal(wantInfo.ModTime()) {
				t.Fatalf("observed timestamp = (%v,%v), want native %v", gotTime, err, wantInfo.ModTime())
			}
			neighborAfter, err := root.Stat("neighbor")
			if err != nil {
				t.Fatal(err)
			}
			gotNeighbor, err := root.ReadFile("neighbor")
			if err != nil || !bytes.Equal(gotNeighbor, payload) || !os.SameFile(neighborBefore, neighborAfter) || neighborAfter.Mode() != neighborBefore.Mode() || !neighborAfter.ModTime().Equal(neighborBefore.ModTime()) {
				t.Fatalf("neighbor = (%v,%v,%v), want unchanged", neighborAfter, gotNeighbor, err)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 3 {
				t.Fatalf("namespace = (%v,%v), want entry, oracle and neighbor", entries, err)
			}
		})
	}
}
