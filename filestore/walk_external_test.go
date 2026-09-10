package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

const (
	walkReplacementOriginalContents       = "owned"
	walkReplacementFuzzByteMaximum        = 4096
	walkReplacementCancelBit        uint8 = 1 << 7
)

type walkReplacementFixture struct {
	root          *os.Root
	rootDirectory string
	walk          core.RelativePath
	branch        core.RelativePath
	held          core.RelativePath
	foreign       core.RelativePath
	original      []byte
}

type walkReplacementMutation uint8

const (
	walkReplacementStable walkReplacementMutation = iota
	walkReplacementSymlink
	walkReplacementDirectory
	walkReplacementRegular
	walkReplacementMissing
	walkReplacementMutationLimit
)

func newWalkReplacementFixture(t *testing.T, rootDirectory string, original []byte) walkReplacementFixture {
	t.Helper()

	walk := mustRelativePath(t, "walk")
	branch := mustRelativePath(t, filepath.Join(walk.String(), "branch"))
	held := mustRelativePath(t, filepath.Join(walk.String(), "held"))
	foreign := mustRelativePath(t, "foreign")
	for _, directory := range []core.RelativePath{walk, branch, foreign} {
		if err := os.Mkdir(filepath.Join(rootDirectory, directory.String()), 0o700); err != nil {
			t.Fatalf("os.Mkdir(%v) error = %v, want nil", directory, err)
		}
	}
	if err := os.WriteFile(filepath.Join(rootDirectory, branch.String(), "original"), original, 0o600); err != nil {
		t.Fatalf("os.WriteFile(original child) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(rootDirectory, foreign.String(), "foreign-child"), []byte("foreign"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(foreign child) error = %v, want nil", err)
	}
	return walkReplacementFixture{
		rootDirectory: rootDirectory,
		root:          requireTestRoot(t, rootDirectory),
		original:      slices.Clone(original),
		walk:          walk,
		branch:        branch,
		held:          held,
		foreign:       foreign,
	}
}

func (f walkReplacementFixture) mutateBranch(t *testing.T, mutation walkReplacementMutation) {
	t.Helper()

	if mutation == walkReplacementStable {
		return
	}
	branch := filepath.Join(f.rootDirectory, f.branch.String())
	held := filepath.Join(f.rootDirectory, f.held.String())
	if err := os.Rename(branch, held); err != nil {
		t.Fatalf("os.Rename(branch, held) error = %v, want nil", err)
	}
	switch mutation {
	case walkReplacementSymlink:
		if err := os.Symlink(filepath.Join("..", f.foreign.String()), branch); err != nil {
			t.Fatalf("os.Symlink(foreign, branch) error = %v, want nil", err)
		}
	case walkReplacementDirectory:
		if err := os.Mkdir(branch, 0o700); err != nil {
			t.Fatalf("os.Mkdir(replacement branch) error = %v, want nil", err)
		}
		if err := os.WriteFile(filepath.Join(branch, "impostor"), []byte("impostor"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(impostor) error = %v, want nil", err)
		}
	case walkReplacementRegular:
		if err := os.WriteFile(branch, []byte("regular"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(replacement branch) error = %v, want nil", err)
		}
	case walkReplacementMissing:
	default:
		t.Fatalf("walk replacement mutation = %d, want known mutation", mutation)
	}
}

// Reads bounded native fixture facts; callers retain every comparison.
func (f walkReplacementFixture) retainedBytes(mutation walkReplacementMutation) (original, foreign []byte, err error) {
	path := f.branch
	if mutation != walkReplacementStable {
		path = f.held
	}
	original, err = os.ReadFile(filepath.Join(f.rootDirectory, path.String(), "original"))
	if err != nil {
		return nil, nil, err
	}
	foreign, err = os.ReadFile(filepath.Join(f.rootDirectory, f.foreign.String(), "foreign-child"))
	return original, foreign, err
}

func FuzzWalkReplacementStandingSemanticClosure(f *testing.F) {
	emitted, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(emitted, encodeWalkReplacementSelector(walkReplacementStable, false))
	f.Add(emitted, encodeWalkReplacementSelector(walkReplacementMissing, false)|walkReplacementCancelBit)
	f.Add(emitted, encodeWalkReplacementSelector(walkReplacementStable, false)|walkReplacementCancelBit)
	for _, seed := range []struct {
		contents  []byte
		mutation  walkReplacementMutation
		skipEntry bool
	}{
		{contents: []byte{}, mutation: walkReplacementStable},
		{contents: []byte(walkReplacementOriginalContents), mutation: walkReplacementSymlink},
		{contents: []byte{0x00, 0xff}, mutation: walkReplacementDirectory},
		{contents: []byte("boundary"), mutation: walkReplacementRegular},
		{contents: []byte("missing"), mutation: walkReplacementMissing},
		{contents: []byte("skip"), mutation: walkReplacementSymlink, skipEntry: true},
	} {
		f.Add(seed.contents, encodeWalkReplacementSelector(seed.mutation, seed.skipEntry))
	}

	f.Fuzz(func(t *testing.T, contents []byte, selector uint8) {
		if len(contents) > walkReplacementFuzzByteMaximum {
			contents = contents[:walkReplacementFuzzByteMaximum]
		}
		mutation := walkReplacementMutation((selector&^walkReplacementCancelBit)>>1) % walkReplacementMutationLimit
		skipEntry := selector&1 == 1
		canceled := selector&walkReplacementCancelBit != 0 && !skipEntry
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		fixture := newWalkReplacementFixture(t, t.TempDir(), contents)
		var gotPaths []core.RelativePath
		gotErr := filestore.Walk(ctx, filestore.WalkRequest{
			Location: filestore.Location{Root: fixture.root, Path: fixture.walk},
			Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
				gotPaths = append(gotPaths, entry.Path)
				if entry.Path != fixture.branch {
					return filestore.WalkContinue, nil
				}
				fixture.mutateBranch(t, mutation)
				if canceled {
					cancel()
				}
				if skipEntry {
					return filestore.WalkSkipDirectory, nil
				}
				return filestore.WalkContinue, nil
			},
		})
		wantPaths := []core.RelativePath{fixture.branch}
		var wantErr, wantNative error
		if mutation == walkReplacementStable && !skipEntry && !canceled {
			wantPaths = append(wantPaths, mustRelativePath(t, filepath.Join(fixture.branch.String(), "original")))
		}
		if mutation != walkReplacementStable && !skipEntry {
			wantErr = core.ErrFilestoreSource
			wantNative = fs.ErrInvalid
			if mutation == walkReplacementMissing {
				wantNative = fs.ErrNotExist
			}
		}
		if canceled {
			wantErr = context.Canceled
			wantNative = nil
		}
		if mutation == walkReplacementRegular && wantNative != nil {
			nativeErr := nativeWalkDirectoryReopen(fixture.root, fixture.branch.String())
			if native, ok := errors.AsType[*os.PathError](nativeErr); ok {
				wantNative = native.Err
			} else {
				wantNative = nativeErr
			}
			if wantNative == nil {
				t.Fatalf("native replacement refusal = %v, want directory acquisition failure", wantNative)
			}
		}
		if !errors.Is(gotErr, wantErr) || wantNative != nil && !errors.Is(gotErr, wantNative) || !slices.Equal(gotPaths, wantPaths) {
			t.Fatalf("Walk = (%v,%v), want (%v,%v/native %v)", gotPaths, gotErr, wantPaths, wantErr, wantNative)
		}
		for _, class := range []error{core.ErrFilestoreDestination, core.ErrFilestoreActivation, core.ErrFilestoreCleanup, core.ErrFilestoreConflict, core.ErrFilestoreSize} {
			if errors.Is(gotErr, class) {
				t.Fatalf("walk refusal = %v, want no %v", gotErr, class)
			}
		}
		original, foreign, err := fixture.retainedBytes(mutation)
		if err != nil || !bytes.Equal(original, contents) || !bytes.Equal(foreign, []byte("foreign")) {
			t.Fatalf("retained native bytes = (%v,%v,%v), want (%v,foreign,nil)", original, foreign, err, contents)
		}
	})
}

func encodeWalkReplacementSelector(mutation walkReplacementMutation, skipEntry bool) uint8 {
	selector := uint8(mutation << 1)
	if skipEntry {
		selector++
	}
	return selector
}
