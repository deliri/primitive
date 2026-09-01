package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

const (
	walkReplacementOriginalContents = "owned"
	walkReplacementFuzzByteMaximum  = 4096
)

func TestWalkReplacementStandingLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive stable directory identity admits its exact child", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		stable := filepath.Join(rootDirectory, "stable")
		if err := os.Mkdir(stable, 0o700); err != nil {
			t.Fatalf("os.Mkdir(stable) error = %v, want nil", err)
		}
		if err := os.WriteFile(filepath.Join(stable, "child"), []byte("owned"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(stable child) error = %v, want nil", err)
		}
		root := requireTestRoot(t, rootDirectory)
		var gotPaths []core.RelativePath
		gotErr := filestore.Walk(t.Context(), filestore.WalkRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, ".")},
			Order:    filestore.WalkOrderNative,
			Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
				gotPaths = append(gotPaths, entry.Path)
				return filestore.WalkContinue, nil
			},
		})
		wantPaths := []core.RelativePath{
			mustRelativePath(t, "stable"),
			mustRelativePath(t, filepath.Join("stable", "child")),
		}
		if gotErr != nil || !slices.Equal(gotPaths, wantPaths) {
			t.Fatalf("filestore.Walk(stable) = (%v, %v), want (%v, nil)", gotPaths, gotErr, wantPaths)
		}
	})

	t.Run("negative directory replaced by symlink refuses before foreign descent", func(t *testing.T) {
		t.Parallel()

		fixture := newWalkReplacementFixture(t, []byte(walkReplacementOriginalContents))
		var gotPaths []core.RelativePath
		gotErr := filestore.Walk(t.Context(), filestore.WalkRequest{
			Location: filestore.Location{Root: fixture.root, Path: fixture.walk},
			Order:    filestore.WalkOrderNative,
			Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
				gotPaths = append(gotPaths, entry.Path)
				if entry.Path == fixture.branch {
					fixture.mutateBranch(t, walkReplacementSymlink)
				}
				return filestore.WalkContinue, nil
			},
		})
		wantPaths := []core.RelativePath{fixture.branch}
		if !errors.Is(gotErr, core.ErrFilestoreSource) || !errors.Is(gotErr, fs.ErrInvalid) {
			t.Fatalf("filestore.Walk(replaced directory) error = %v, want %v and %v", gotErr, core.ErrFilestoreSource, fs.ErrInvalid)
		}
		if !slices.Equal(gotPaths, wantPaths) {
			t.Fatalf("filestore.Walk(replaced directory) paths = %v, want %v without foreign child", gotPaths, wantPaths)
		}
		fixture.proveOriginalAndForeignRemain(t)
	})

	t.Run("neutral skipped directory is never reopened after replacement", func(t *testing.T) {
		t.Parallel()

		fixture := newWalkReplacementFixture(t, []byte(walkReplacementOriginalContents))
		var gotPaths []core.RelativePath
		gotErr := filestore.Walk(t.Context(), filestore.WalkRequest{
			Location: filestore.Location{Root: fixture.root, Path: fixture.walk},
			Order:    filestore.WalkOrderNative,
			Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
				gotPaths = append(gotPaths, entry.Path)
				fixture.mutateBranch(t, walkReplacementSymlink)
				return filestore.WalkSkipDirectory, nil
			},
		})
		wantPaths := []core.RelativePath{fixture.branch}
		if gotErr != nil || !slices.Equal(gotPaths, wantPaths) {
			t.Fatalf("filestore.Walk(skipped replacement) = (%v, %v), want (%v, nil)", gotPaths, gotErr, wantPaths)
		}
		fixture.proveOriginalAndForeignRemain(t)
	})
}

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

func newWalkReplacementFixture(t *testing.T, original []byte) walkReplacementFixture {
	t.Helper()

	rootDirectory := t.TempDir()
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

func (f walkReplacementFixture) proveOriginalAndForeignRemain(t *testing.T) {
	t.Helper()

	originalPath := filepath.Join(f.rootDirectory, f.held.String(), "original")
	gotOriginal, err := os.ReadFile(originalPath)
	if err != nil || !bytes.Equal(gotOriginal, f.original) {
		t.Fatalf("os.ReadFile(%q) = (%q, %v), want (%q, nil)", originalPath, gotOriginal, err, f.original)
	}
	foreignPath := filepath.Join(f.rootDirectory, f.foreign.String(), "foreign-child")
	if _, err := os.Stat(foreignPath); err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", foreignPath, err)
	}
}

func FuzzWalkReplacementStandingSemanticClosure(f *testing.F) {
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
		mutation := walkReplacementMutation(selector>>1) % walkReplacementMutationLimit
		skipEntry := selector&1 == 1
		fixture := newWalkReplacementFixture(t, contents)
		var gotPaths []core.RelativePath
		gotErr := filestore.Walk(t.Context(), filestore.WalkRequest{
			Location: filestore.Location{Root: fixture.root, Path: fixture.walk},
			Order:    filestore.WalkOrderNative,
			Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
				gotPaths = append(gotPaths, entry.Path)
				if entry.Path != fixture.branch {
					return filestore.WalkContinue, nil
				}
				fixture.mutateBranch(t, mutation)
				if skipEntry {
					return filestore.WalkSkipDirectory, nil
				}
				return filestore.WalkContinue, nil
			},
		})
		proveWalkReplacementOracle(t, walkReplacementOracleInput{
			fixture:   fixture,
			mutation:  mutation,
			skipEntry: skipEntry,
			gotPaths:  gotPaths,
			gotErr:    gotErr,
		})
	})
}

func encodeWalkReplacementSelector(mutation walkReplacementMutation, skipEntry bool) uint8 {
	selector := uint8(mutation << 1)
	if skipEntry {
		selector++
	}
	return selector
}

type walkReplacementOracleInput struct {
	gotErr    error
	fixture   walkReplacementFixture
	gotPaths  []core.RelativePath
	mutation  walkReplacementMutation
	skipEntry bool
}

func proveWalkReplacementOracle(t *testing.T, input walkReplacementOracleInput) {
	t.Helper()

	wantPaths := []core.RelativePath{input.fixture.branch}
	if input.mutation == walkReplacementStable && !input.skipEntry {
		wantPaths = append(wantPaths, mustRelativePath(t, filepath.Join(input.fixture.branch.String(), "original")))
	}
	if !slices.Equal(input.gotPaths, wantPaths) {
		t.Fatalf("filestore.Walk(mutation %d, skip %t) paths = %v, want %v", input.mutation, input.skipEntry, input.gotPaths, wantPaths)
	}
	if input.skipEntry || input.mutation == walkReplacementStable {
		if input.gotErr != nil {
			t.Fatalf("filestore.Walk(mutation %d, skip %t) error = %v, want nil", input.mutation, input.skipEntry, input.gotErr)
		}
		if input.mutation != walkReplacementStable {
			input.fixture.proveOriginalAndForeignRemain(t)
			return
		}
		proveWalkOriginalContents(t, input)
		return
	}
	if !errors.Is(input.gotErr, core.ErrFilestoreSource) {
		t.Fatalf("filestore.Walk(mutation %d) error = %v, want %v", input.mutation, input.gotErr, core.ErrFilestoreSource)
	}
	if input.mutation == walkReplacementMissing {
		if !errors.Is(input.gotErr, fs.ErrNotExist) {
			t.Fatalf("filestore.Walk(missing) error = %v, want %v", input.gotErr, fs.ErrNotExist)
		}
	}
	input.fixture.proveOriginalAndForeignRemain(t)
}

func proveWalkOriginalContents(t *testing.T, input walkReplacementOracleInput) {
	t.Helper()

	path := filepath.Join(input.fixture.rootDirectory, input.fixture.branch.String(), "original")
	if input.mutation != walkReplacementStable {
		path = filepath.Join(input.fixture.rootDirectory, input.fixture.held.String(), "original")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, input.fixture.original) {
		t.Fatalf("os.ReadFile(original) = (%q, %v), want (%q, nil)", got, err, input.fixture.original)
	}
}

func TestWalkStreamsFixedBatchesAndHonorsTypedDirectorySkip(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for index := range 129 {
		path := filepath.Join(directory, fmt.Sprintf("entry-%03d", index))
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
		}
	}
	skipped := filepath.Join(directory, "skipped")
	if err := os.Mkdir(skipped, 0o700); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", skipped, err)
	}
	if err := os.WriteFile(filepath.Join(skipped, "hidden"), []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(skipped child) error = %v, want nil", err)
	}

	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	rootPath, err := core.ParseRelativePath(".")
	if err != nil {
		t.Fatalf("core.ParseRelativePath(root) error = %v, want nil", err)
	}
	files := 0
	directories := 0
	err = filestore.Walk(t.Context(), filestore.WalkRequest{
		Location: filestore.Location{Root: root, Path: rootPath},
		Order:    filestore.WalkOrderNative,
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if entry.Entry.IsDir() {
				directories++
				return filestore.WalkSkipDirectory, nil
			}
			files++
			return filestore.WalkContinue, nil
		},
	})
	if err != nil {
		t.Fatalf("filestore.Walk() error = %v, want nil", err)
	}
	if files != 129 || directories != 1 {
		t.Fatalf("filestore.Walk() observations = files:%d directories:%d, want 129/1", files, directories)
	}
}

func TestWalkRejectsInvalidVisitorDirectiveAtTheRealDirectoryBoundary(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "entry"), []byte("x"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(entry) error = %v, want nil", err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	rootPath, err := core.ParseRelativePath(".")
	if err != nil {
		t.Fatalf("core.ParseRelativePath(root) error = %v, want nil", err)
	}
	err = filestore.Walk(t.Context(), filestore.WalkRequest{
		Location: filestore.Location{Root: root, Path: rootPath},
		Order:    filestore.WalkOrderNative,
		Visit: func(filestore.WalkEntry) (filestore.WalkDirective, error) {
			return 0, nil
		},
	})
	if !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("filestore.Walk(invalid directive) error = %v, want %v", err, core.ErrFilestoreContract)
	}
}

func TestWalkLexicalOrderRequiresAndEnforcesExactDirectoryCeiling(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for _, name := range []string{"third", "first", "second"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v, want nil", name, err)
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	rootPath, err := core.ParseRelativePath(".")
	if err != nil {
		t.Fatalf("core.ParseRelativePath(root) error = %v, want nil", err)
	}
	exactMaximum, err := filestore.NewDirectoryEntryMaximum(3)
	if err != nil {
		t.Fatalf("filestore.NewDirectoryEntryMaximum(3) error = %v, want nil", err)
	}
	names := make([]string, 0, 3)
	err = filestore.Walk(t.Context(), filestore.WalkRequest{
		Location:              filestore.Location{Root: root, Path: rootPath},
		Order:                 filestore.WalkOrderLexical,
		DirectoryEntryMaximum: exactMaximum,
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			names = append(names, entry.Entry.Name())
			return filestore.WalkContinue, nil
		},
	})
	wantNames := []string{"first", "second", "third"}
	if err != nil || !slices.Equal(names, wantNames) {
		t.Fatalf("filestore.Walk(lexical) = (%q, %v), want (%q, nil)", names, err, wantNames)
	}
	belowMaximum, err := filestore.NewDirectoryEntryMaximum(2)
	if err != nil {
		t.Fatalf("filestore.NewDirectoryEntryMaximum(2) error = %v, want nil", err)
	}
	err = filestore.Walk(t.Context(), filestore.WalkRequest{
		Location:              filestore.Location{Root: root, Path: rootPath},
		Order:                 filestore.WalkOrderLexical,
		DirectoryEntryMaximum: belowMaximum,
		Visit: func(filestore.WalkEntry) (filestore.WalkDirective, error) {
			return filestore.WalkContinue, nil
		},
	})
	if !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("filestore.Walk(over ceiling) error = %v, want %v", err, core.ErrFilestoreContract)
	}
}

func TestDirectoryEntryMaximumRejectsAllocationAndConversionOverflow(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   uint32
	}{
		{name: "zero rejects", value: 0, wantErr: core.ErrFilestoreContract},
		{name: "one admits", value: 1},
		{name: "one below allocation ceiling admits", value: filestore.DirectoryEntryMaximumLimit - 1},
		{name: "exact allocation ceiling admits", value: filestore.DirectoryEntryMaximumLimit},
		{name: "one above allocation ceiling rejects", value: filestore.DirectoryEntryMaximumLimit + 1, wantErr: core.ErrFilestoreContract},
		{name: "maximum uint32 rejects before int conversion", value: math.MaxUint32, wantErr: core.ErrFilestoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := filestore.NewDirectoryEntryMaximum(tc.value)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("filestore.NewDirectoryEntryMaximum(%d) error = %v, want %v", tc.value, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != (filestore.DirectoryEntryMaximum{}) {
				t.Fatalf("filestore.NewDirectoryEntryMaximum(%d) = %v, want zero", tc.value, got)
			}
		})
	}
}

func TestWalkRejectsCancelledContextBeforeFilesystemObservation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := filestore.Walk(ctx, filestore.WalkRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("filestore.Walk(cancelled) error = %v, want %v", err, context.Canceled)
	}
}
