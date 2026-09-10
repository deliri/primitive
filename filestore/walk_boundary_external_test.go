package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Each substitution starts from a successful walk of the same tree. Skipping
// is tested once: after the directive, the replacement is deliberately unread.
// This ratchets directory identity, not a producer/classifier evidence matrix.
func TestWalkReplacementStandingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		mutation  walkReplacementMutation
		directive filestore.WalkDirective
		wantErr   error
	}{
		{name: "stable identity retains child", mutation: walkReplacementStable, directive: filestore.WalkContinue},
		{name: "symlink substitution cannot redirect descent", mutation: walkReplacementSymlink, directive: filestore.WalkContinue, wantErr: core.ErrFilestoreSource},
		{name: "different directory cannot impersonate observed identity", mutation: walkReplacementDirectory, directive: filestore.WalkContinue, wantErr: core.ErrFilestoreSource},
		{name: "regular file cannot impersonate directory", mutation: walkReplacementRegular, directive: filestore.WalkContinue, wantErr: core.ErrFilestoreSource},
		{name: "removed directory retains missing source refusal", mutation: walkReplacementMissing, directive: filestore.WalkContinue, wantErr: fs.ErrNotExist},
		{name: "neutral skipped replacement produces no foreign entry", mutation: walkReplacementSymlink, directive: filestore.WalkSkipDirectory},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := newWalkReplacementFixture(t, t.TempDir(), []byte(walkReplacementOriginalContents))
			request := filestore.WalkRequest{Location: filestore.Location{Root: fixture.root, Path: fixture.walk}}
			var baseline []core.RelativePath
			request.Visit = func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
				baseline = append(baseline, entry.Path)
				return filestore.WalkContinue, nil
			}
			wantBaseline := []core.RelativePath{fixture.branch, mustRelativePath(t, filepath.Join(fixture.branch.String(), "original"))}
			if err := filestore.Walk(t.Context(), request); err != nil || !slices.Equal(baseline, wantBaseline) {
				t.Fatalf("baseline = %v/%v, want %v", baseline, err, wantBaseline)
			}
			var got []core.RelativePath
			mutations := 0
			request.Visit = func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
				got = append(got, entry.Path)
				if entry.Path == fixture.branch {
					fixture.mutateBranch(t, tc.mutation)
					mutations++
					return tc.directive, nil
				}
				return filestore.WalkContinue, nil
			}
			gotErr := filestore.Walk(t.Context(), request)
			if mutations != 1 || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("branch visits/error = %d/%v, want one visit/%v", mutations, gotErr, tc.wantErr)
			}
			wantPaths := []core.RelativePath{fixture.branch}
			if tc.mutation == walkReplacementStable && tc.directive == filestore.WalkContinue {
				wantPaths = wantBaseline
			}
			var wantNative error
			if tc.mutation != walkReplacementStable && tc.directive == filestore.WalkContinue {
				wantNative = fs.ErrInvalid
				if tc.mutation == walkReplacementMissing {
					wantNative = fs.ErrNotExist
				}
			}
			if tc.mutation == walkReplacementRegular && wantNative != nil {
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
			if !slices.Equal(got, wantPaths) || wantNative != nil && (!errors.Is(gotErr, core.ErrFilestoreSource) || !errors.Is(gotErr, wantNative)) {
				t.Fatalf("Walk = (%v,%v), want (%v,native %v)", got, gotErr, wantPaths, wantNative)
			}
			for _, class := range []error{core.ErrFilestoreDestination, core.ErrFilestoreActivation, core.ErrFilestoreCleanup, core.ErrFilestoreConflict, core.ErrFilestoreSize} {
				if errors.Is(gotErr, class) {
					t.Fatalf("walk refusal = %v, want no %v", gotErr, class)
				}
			}
			original, foreign, err := fixture.retainedBytes(tc.mutation)
			if err != nil || !bytes.Equal(original, fixture.original) || !bytes.Equal(foreign, []byte("foreign")) {
				t.Fatalf("retained native bytes = (%v,%v,%v), want (%v,foreign,nil)", original, foreign, err, fixture.original)
			}
		})
	}
}

type walkBoundaryFault uint8

const (
	walkBoundaryNoFault walkBoundaryFault = iota
	walkBoundaryNilContext
	walkBoundaryCancelled
	walkBoundaryNilRoot
	walkBoundaryUnsetPath
	walkBoundaryNilVisitor
	walkBoundaryMissingDirectory
	walkBoundaryClosedRoot
	walkBoundaryVisitorFailure
	walkBoundaryInvalidDirective
	walkBoundaryCancelDuringVisit
	walkBoundaryEmptyDirectory
	walkBoundaryCancelRemoveDirectory
)

func TestWalkRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		fault      walkBoundaryFault
		wantErr    error
		wantVisits int
	}{
		{name: "stable entries preserve native delivery", wantVisits: 2},
		{name: "empty directory emits no entries and stays empty", fault: walkBoundaryEmptyDirectory},
		{name: "nil context refuses before entry delivery", fault: walkBoundaryNilContext, wantErr: core.ErrNilContext},
		{name: "cancelled context refuses before entry delivery", fault: walkBoundaryCancelled, wantErr: context.Canceled},
		{name: "missing rooted capability refuses", fault: walkBoundaryNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "unset relative path refuses", fault: walkBoundaryUnsetPath, wantErr: core.ErrFilestoreContract},
		{name: "missing visitor refuses", fault: walkBoundaryNilVisitor, wantErr: core.ErrFilestoreContract},
		{name: "missing directory preserves source identity", fault: walkBoundaryMissingDirectory, wantErr: fs.ErrNotExist},
		{name: "closed root preserves source refusal", fault: walkBoundaryClosedRoot, wantErr: core.ErrFilestoreSource},
		{name: "visitor error prevents later delivery", fault: walkBoundaryVisitorFailure, wantErr: io.ErrClosedPipe, wantVisits: 1},
		{name: "invalid visitor directive prevents later delivery", fault: walkBoundaryInvalidDirective, wantErr: core.ErrFilestoreContract, wantVisits: 1},
		{name: "midwalk cancellation prevents later delivery", fault: walkBoundaryCancelDuringVisit, wantErr: context.Canceled, wantVisits: 1},
		{name: "cancellation before descent prevents reopening a removed directory", fault: walkBoundaryCancelRemoveDirectory, wantErr: context.Canceled, wantVisits: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			names := []string{"z", "a"}
			if tc.fault == walkBoundaryCancelRemoveDirectory {
				names = []string{"entry"}
			}
			if tc.fault == walkBoundaryEmptyDirectory {
				names = nil
			}
			for _, name := range names {
				if tc.fault == walkBoundaryCancelRemoveDirectory {
					if err := os.Mkdir(filepath.Join(directory, name), 0o700); err != nil {
						t.Fatal(err)
					}
					continue
				}
				if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			root := requireTestRoot(t, directory)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var got []core.RelativePath
			request := filestore.WalkRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, ".")}}
			request.Visit = func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
				got = append(got, entry.Path)
				switch tc.fault {
				case walkBoundaryVisitorFailure:
					return filestore.WalkContinue, io.ErrClosedPipe
				case walkBoundaryInvalidDirective:
					return filestore.WalkDirectiveUnknown, nil
				case walkBoundaryCancelDuringVisit:
					cancel()
				case walkBoundaryCancelRemoveDirectory:
					cancel()
					if err := root.Remove(entry.Path.String()); err != nil {
						return filestore.WalkDirectiveUnknown, err
					}
				}
				return filestore.WalkContinue, nil
			}
			switch tc.fault {
			case walkBoundaryNilContext:
				ctx = nil
			case walkBoundaryCancelled:
				cancel()
			case walkBoundaryNilRoot:
				request.Location.Root = nil
			case walkBoundaryUnsetPath:
				request.Location.Path = core.RelativePath{}
			case walkBoundaryNilVisitor:
				request.Visit = nil
			case walkBoundaryMissingDirectory:
				request.Location.Path = mustRelativePath(t, "missing")
			case walkBoundaryClosedRoot:
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			}
			var want []core.RelativePath
			native, err := os.Open(directory)
			if err != nil {
				t.Fatal(err)
			}
			entries, readErr := native.ReadDir(-1)
			closeErr := native.Close()
			if readErr != nil || closeErr != nil {
				t.Fatal(errors.Join(readErr, closeErr))
			}
			for _, entry := range entries {
				want = append(want, mustRelativePath(t, entry.Name()))
			}
			want = want[:tc.wantVisits]
			gotErr := filestore.Walk(ctx, request)
			if !errors.Is(gotErr, tc.wantErr) || !slices.Equal(got, want) {
				t.Fatalf("Walk = %v/%v, want %v/%v", got, gotErr, want, tc.wantErr)
			}
			wantSource := tc.fault == walkBoundaryMissingDirectory || tc.fault == walkBoundaryClosedRoot
			if errors.Is(gotErr, core.ErrFilestoreSource) != wantSource || tc.fault == walkBoundaryClosedRoot && !errors.Is(gotErr, fs.ErrClosed) {
				t.Fatalf("walk source identity = %v, want source=%t and native cause", gotErr, wantSource)
			}
			for _, class := range []error{core.ErrFilestoreDestination, core.ErrFilestoreActivation, core.ErrFilestoreCleanup, core.ErrFilestoreConflict, core.ErrFilestoreSize} {
				if errors.Is(gotErr, class) {
					t.Fatalf("walk refusal = %v, want no %v", gotErr, class)
				}
			}
			if tc.fault == walkBoundaryCancelRemoveDirectory {
				names = nil
			}
			remaining, err := os.ReadDir(directory)
			if err != nil || len(remaining) != len(names) {
				t.Fatalf("remaining files = %d/%v, want %d", len(remaining), err, len(names))
			}
			for _, name := range names {
				data, err := os.ReadFile(filepath.Join(directory, name))
				if err != nil || string(data) != name {
					t.Fatalf("source %s = %q/%v, want unchanged", name, data, err)
				}
			}
		})
	}
}

// Cardinality and visitor refusal exercise real native delivery and exact cleanup.
func FuzzWalkCardinalitySemanticClosure(f *testing.F) {
	seedDirectory := f.TempDir()
	if err := os.WriteFile(filepath.Join(seedDirectory, "entry-000"), nil, 0o600); err != nil {
		f.Fatal(err)
	}
	absolute, err := core.ParseAbsolutePath(seedDirectory)
	if err != nil {
		f.Fatal(err)
	}
	seedRoot, err := filestore.OpenRoot(f.Context(), absolute)
	if err != nil {
		f.Fatal(err)
	}
	dot, err := core.ParseRelativePath(".")
	if err != nil {
		f.Fatal(err)
	}
	var emitted uint8
	seedRequest := filestore.WalkRequest{Location: filestore.Location{Root: seedRoot, Path: dot}, Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
		if err := entry.Validate(); err != nil {
			return filestore.WalkDirectiveUnknown, err
		}
		if entry.Path.String() != "entry-000" {
			return filestore.WalkDirectiveUnknown, core.ErrFilestoreContract
		}
		emitted++
		return filestore.WalkContinue, nil
	}}
	if err := seedRequest.Validate(); err != nil {
		f.Fatal(err)
	}
	walkErr := filestore.Walk(f.Context(), seedRequest)
	closeErr := seedRoot.Close()
	if walkErr != nil || closeErr != nil || emitted != 1 {
		f.Fatalf("seed cardinality = (%d,%v,%v), want one emitted entry", emitted, walkErr, closeErr)
	}
	f.Add(emitted, uint8(0))
	for _, count := range []uint8{0, 63, 64, 65, 127, 128, 129, 255} {
		f.Add(count, uint8(0))
		f.Add(count, uint8(1))
	}
	// uint8 bounds filesystem work to 255 entries per callback, a test budget.
	f.Fuzz(func(t *testing.T, count, stop uint8) {
		directory := t.TempDir()
		for index := range int(count) {
			name := fmt.Sprintf("entry-%03d", index)
			if err := os.WriteFile(filepath.Join(directory, name), []byte{byte(index)}, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		before, err := removalFixtureSnapshot(directory)
		if err != nil {
			t.Fatal(err)
		}
		native, err := os.Open(directory)
		if err != nil {
			t.Fatal(err)
		}
		entries, readErr := native.ReadDir(-1)
		if err := errors.Join(readErr, native.Close()); err != nil {
			t.Fatal(err)
		}
		var want []core.RelativePath
		for _, entry := range entries {
			want = append(want, mustRelativePath(t, entry.Name()))
		}
		var wantErr error
		if stop > 0 && int(stop) <= len(want) {
			want = want[:stop]
			wantErr = io.ErrClosedPipe
		}
		var got []core.RelativePath
		request := filestore.WalkRequest{
			Location: filestore.Location{Root: requireTestRoot(t, directory), Path: mustRelativePath(t, ".")},
			Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
				if err := entry.Validate(); err != nil {
					return filestore.WalkDirectiveUnknown, err
				}
				if !entry.Entry.Type().IsRegular() {
					return filestore.WalkDirectiveUnknown, core.ErrFilestoreContract
				}
				got = append(got, entry.Path)
				if stop > 0 && len(got) == int(stop) {
					return filestore.WalkContinue, io.ErrClosedPipe
				}
				return filestore.WalkContinue, nil
			},
		}
		gotErr := filestore.Walk(t.Context(), request)
		if !errors.Is(gotErr, wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) || !slices.Equal(got, want) {
			t.Fatalf("Walk(count=%d,stop=%d) = (%v,%v), want exact native prefix (%v,%v)", count, stop, got, gotErr, want, wantErr)
		}
		after, err := removalFixtureSnapshot(directory)
		if err != nil || len(after) != len(before) {
			t.Fatalf("namespace cardinality = (%d,%v), want %d", len(after), err, len(before))
		}
		for i, entry := range before {
			if after[i].name != entry.name || after[i].mode != entry.mode || after[i].target != entry.target || !slices.Equal(after[i].data, entry.data) {
				t.Fatalf("namespace entry %d = %+v, want %+v", i, after[i], entry)
			}
		}
	})
}
