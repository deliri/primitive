package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type renameNativeLayout uint8

const (
	renameNativeSameParent renameNativeLayout = iota
	renameNativeCrossParent
	renameNativeConfinedParent
	renameNativeEscapingSource
	renameNativeEscapingTarget
	renameNativeMissingTargetParent
	renameNativeDescendant
	renameNativeAliasedSameName
	renameNativeLayoutLimit
)

func FuzzRenameNativeNamespaceCustody(f *testing.F) {
	root, err := os.OpenRoot(f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Cleanup(func() {
		if err := root.Close(); err != nil {
			f.Error(err)
		}
	})
	source, err := core.ParseRelativePath("source")
	if err != nil {
		f.Fatal(err)
	}
	target, err := core.ParseRelativePath("target")
	if err != nil {
		f.Fatal(err)
	}
	seed := filestore.RenameRequest{Location: filestore.Location{Root: root, Path: source}, Target: target}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	if err := root.WriteFile(source.String(), emitted, 0o600); err != nil {
		f.Fatal(err)
	}
	if err := filestore.Rename(f.Context(), seed); err != nil {
		f.Fatal(err)
	}
	renamed, err := root.ReadFile(target.String())
	if err != nil || !bytes.Equal(renamed, emitted) {
		f.Fatalf("renamed seed = (%v,%v), want exact emitted bytes", renamed, err)
	}
	f.Add(uint8(renameEntryRegular), uint8(renameEntryMissing), uint8(renameNativeSameParent), renamed, []byte{}, false, false, false)
	for shape := range renameEntryHardLink {
		f.Add(uint8(shape), uint8(renameEntryMissing), uint8(renameNativeSameParent), []byte{0, 255}, []byte{1, 254}, false, false, false)
	}
	for shape := renameEntryRegular; shape < renameEntryShapeLimit; shape++ {
		f.Add(uint8(renameEntryRegular), uint8(shape), uint8(renameNativeSameParent), []byte{0, 255}, []byte{1, 254}, false, false, false)
	}
	for layout := renameNativeCrossParent; layout < renameNativeLayoutLimit; layout++ {
		f.Add(uint8(renameEntryRegular), uint8(renameEntryMissing), uint8(layout), []byte{0, 255}, []byte{}, false, false, false)
	}
	f.Add(uint8(renameEntryRegular), uint8(renameEntryRegular), uint8(renameNativeSameParent), []byte{}, []byte{1, 254}, false, false, false)
	f.Add(uint8(renameEntryRegular), uint8(renameEntryRegular), uint8(renameNativeSameParent), []byte{0, 255}, []byte{1, 254}, true, false, false)
	f.Add(uint8(renameEntryRegular), uint8(renameEntryRegular), uint8(renameNativeSameParent), []byte{0, 255}, []byte{1, 254}, false, true, false)
	f.Add(uint8(renameEntryRegular), uint8(renameEntryRegular), uint8(renameNativeSameParent), []byte{0, 255}, []byte{1, 254}, false, false, true)
	f.Fuzz(func(t *testing.T, rawSource, rawTarget, rawLayout uint8, payload, targetPayload []byte, canceled, closed, samePath bool) {
		payload = payload[:min(len(payload), 1024)]
		targetPayload = targetPayload[:min(len(targetPayload), 1024)]
		fixture := renameNativeFixture{source: source.String(), target: target.String(), sourceShape: renameEntryShape(rawSource % uint8(renameEntryHardLink)), targetShape: renameEntryShape(rawTarget % uint8(renameEntryShapeLimit)), payload: payload, targetPayload: targetPayload}
		// The hard-link fixture always constructs a real regular inode; it never
		// silently falls back to two unrelated files when a directory was selected.
		if fixture.targetShape == renameEntryHardLink {
			fixture.sourceShape = renameEntryRegular
		}
		layout := renameNativeLayout(rawLayout % uint8(renameNativeLayoutLimit))
		switch layout {
		case renameNativeSameParent:
		case renameNativeCrossParent:
			fixture.source = filepath.Join("from", fixture.source)
			fixture.target = filepath.Join("to", fixture.target)
		case renameNativeConfinedParent:
			fixture.source = filepath.Join("alias", fixture.source)
			fixture.target = filepath.Join("real", fixture.target)
		case renameNativeEscapingSource:
			fixture.source = filepath.Join("escape", "retained")
			fixture.sourceShape = renameEntryMissing
			if fixture.targetShape == renameEntryHardLink {
				fixture.targetShape = renameEntryMissing
			}
		case renameNativeEscapingTarget:
			fixture.target = filepath.Join("escape", fixture.target)
			fixture.targetShape = renameEntryMissing
		case renameNativeMissingTargetParent:
			fixture.target = filepath.Join("missing", fixture.target)
			fixture.targetShape = renameEntryMissing
		case renameNativeAliasedSameName:
			fixture.source = filepath.Join("alias", fixture.source)
			fixture.target = filepath.Join("real", filepath.Base(fixture.source))
			fixture.targetShape = renameEntryMissing
		case renameNativeDescendant:
			fixture.sourceShape = renameEntryPopulatedDirectory
			fixture.target = filepath.Join(fixture.source, fixture.target)
			fixture.targetShape = renameEntryMissing
		}
		outside := t.TempDir()
		outsideName := filepath.Join(outside, "retained")
		if err := os.WriteFile(outsideName, payload, 0o600); err != nil {
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
		directories := [2]string{t.TempDir(), t.TempDir()}
		var roots [2]*os.Root
		for i, directory := range directories {
			if err := createRenameNativeFixture(directory, outside, fixture); err != nil {
				t.Fatal(err)
			}
			roots[i], err = os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			if !closed {
				t.Cleanup(func() {
					if err := roots[i].Close(); err != nil {
						t.Error(err)
					}
				})
			}
		}
		sourceBefore, sourceErr := roots[0].Lstat(fixture.source)
		targetBefore, targetErr := roots[0].Lstat(fixture.target)
		if fixture.targetShape == renameEntryHardLink {
			if sourceErr != nil || targetErr != nil || !os.SameFile(sourceBefore, targetBefore) {
				t.Fatalf("hard-link native facts = (%v,%v,%v,%v), want one shared inode", sourceBefore, targetBefore, sourceErr, targetErr)
			}
		}
		request := filestore.RenameRequest{Location: filestore.Location{Root: roots[0], Path: mustRelativePath(t, fixture.source)}, Target: mustRelativePath(t, fixture.target)}
		if samePath {
			request.Target = request.Location.Path
		}
		ctx := t.Context()
		if canceled {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
		}
		if closed {
			for _, root := range roots {
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			}
		}
		var wantBoundary, nativeErr error
		switch {
		case canceled:
			wantBoundary = context.Canceled
		case samePath:
			wantBoundary = core.ErrFilestoreContract
		default:
			nativeErr = roots[1].Rename(fixture.source, fixture.target)
			if nativeErr != nil {
				wantBoundary = core.ErrFilestoreActivation
			}
		}
		gotErr := filestore.Rename(ctx, request)
		if !errors.Is(gotErr, wantBoundary) {
			t.Fatalf("rename = %v, want %v with native %v", gotErr, wantBoundary, nativeErr)
		}
		if nativeErr != nil {
			var native, gotNative *os.LinkError
			if !errors.As(nativeErr, &native) || !errors.As(gotErr, &gotNative) || !errors.Is(gotErr, native.Err) {
				t.Fatalf("native cause = %v, want Go %v", gotErr, nativeErr)
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
			t.Fatalf("namespace = %+v, want Go %+v", got, want)
		}
		for i := range want {
			if got[i].name != want[i].name || got[i].mode != want[i].mode || got[i].target != want[i].target || !bytes.Equal(got[i].data, want[i].data) {
				t.Fatalf("entry %d = %+v, want Go %+v", i, got[i], want[i])
			}
		}
		if gotErr == nil {
			after, err := os.Lstat(filepath.Join(directories[0], fixture.target))
			if sourceErr != nil || err != nil || !os.SameFile(sourceBefore, after) || sourceBefore.Mode() != after.Mode() || sourceBefore.ModTime().UnixNano() != after.ModTime().UnixNano() {
				t.Fatalf("target inode/metadata = (%v,%v), want source %v", after, err, sourceBefore)
			}
			retained, err := os.Lstat(filepath.Join(directories[0], fixture.source))
			if targetErr == nil && os.SameFile(sourceBefore, targetBefore) {
				if err != nil || !os.SameFile(sourceBefore, retained) {
					t.Fatalf("hard-link no-op source = (%v,%v), want original", retained, err)
				}
			} else if !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("old source = (%v,%v), want absent", retained, err)
			}
		} else {
			for _, entry := range []struct {
				name string
				info fs.FileInfo
				err  error
			}{{fixture.source, sourceBefore, sourceErr}, {fixture.target, targetBefore, targetErr}} {
				if entry.err != nil {
					continue
				}
				after, err := os.Lstat(filepath.Join(directories[0], entry.name))
				if err != nil || !os.SameFile(entry.info, after) || entry.info.Mode() != after.Mode() || entry.info.ModTime().UnixNano() != after.ModTime().UnixNano() {
					t.Fatalf("refused name %s = (%v,%v), want unchanged %v", entry.name, after, err, entry.info)
				}
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
		if err != nil || !bytes.Equal(data, payload) || len(outsideEntries) != 1 || outsideEntries[0].Name() != "retained" || !os.SameFile(outsideBefore, outsideAfter) || outsideBefore.Mode() != outsideAfter.Mode() || outsideBefore.ModTime().UnixNano() != outsideAfter.ModTime().UnixNano() || !os.SameFile(outsideFileBefore, outsideFileAfter) || outsideFileBefore.Mode() != outsideFileAfter.Mode() || outsideFileBefore.ModTime().UnixNano() != outsideFileAfter.ModTime().UnixNano() {
			t.Fatalf("outside inode/namespace/metadata/bytes changed: %v", err)
		}
	})
}
