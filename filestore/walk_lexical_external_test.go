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
		for _, order := range []filestore.WalkOrder{filestore.WalkOrderNative, filestore.WalkOrderLexical} {
			t.Run(order.String()+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				fixture := newWalkReplacementFixture(t, t.TempDir(), []byte(walkReplacementOriginalContents))
				maximum, err := filestore.NewDirectoryEntryMaximum(2)
				if err != nil {
					t.Fatal(err)
				}
				request := filestore.WalkRequest{Location: filestore.Location{Root: fixture.root, Path: fixture.walk}, Order: order}
				if order == filestore.WalkOrderLexical {
					request.DirectoryEntryMaximum = maximum
				}
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
}

type lexicalFault uint8

const (
	lexicalNoFault lexicalFault = iota
	lexicalNilContext
	lexicalCancelled
	lexicalNilRoot
	lexicalUnsetPath
	lexicalUnsetOrder
	lexicalNilVisitor
	lexicalUnsetMaximum
	lexicalNativeMaximum
	lexicalMissingDirectory
	lexicalClosedRoot
	lexicalVisitorFailure
	lexicalInvalidDirective
	lexicalCancelDuringVisit
	lexicalEmptyDirectory
	lexicalCancelRemoveDirectory
)

func TestLexicalWalkRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		fault      lexicalFault
		wantErr    error
		wantVisits int
	}{
		{name: "stable entries remain ordered", wantVisits: 2},
		{name: "empty directory emits no entries and stays empty", fault: lexicalEmptyDirectory},
		{name: "nil context refuses before entry delivery", fault: lexicalNilContext, wantErr: core.ErrNilContext},
		{name: "cancelled context refuses before entry delivery", fault: lexicalCancelled, wantErr: context.Canceled},
		{name: "missing rooted capability refuses", fault: lexicalNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "unset relative path refuses", fault: lexicalUnsetPath, wantErr: core.ErrFilestoreContract},
		{name: "unset ordering refuses", fault: lexicalUnsetOrder, wantErr: core.ErrFilestoreContract},
		{name: "missing visitor refuses", fault: lexicalNilVisitor, wantErr: core.ErrFilestoreContract},
		{name: "missing lexical ceiling refuses", fault: lexicalUnsetMaximum, wantErr: core.ErrFilestoreContract},
		{name: "native order cannot carry lexical ceiling", fault: lexicalNativeMaximum, wantErr: core.ErrFilestoreContract},
		{name: "missing directory preserves source identity", fault: lexicalMissingDirectory, wantErr: fs.ErrNotExist},
		{name: "closed root preserves source refusal", fault: lexicalClosedRoot, wantErr: core.ErrFilestoreSource},
		{name: "visitor error prevents later delivery", fault: lexicalVisitorFailure, wantErr: io.ErrClosedPipe, wantVisits: 1},
		{name: "invalid visitor directive prevents later delivery", fault: lexicalInvalidDirective, wantErr: core.ErrFilestoreContract, wantVisits: 1},
		{name: "midwalk cancellation prevents later delivery", fault: lexicalCancelDuringVisit, wantErr: context.Canceled, wantVisits: 1},
		{name: "cancellation before descent prevents reopening a removed directory", fault: lexicalCancelRemoveDirectory, wantErr: context.Canceled, wantVisits: 1},
	} {
		for _, order := range []filestore.WalkOrder{filestore.WalkOrderNative, filestore.WalkOrderLexical} {
			if order == filestore.WalkOrderNative && (tc.fault == lexicalUnsetMaximum || tc.fault == lexicalNativeMaximum) {
				continue
			}
			t.Run(order.String()+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				names := []string{"z", "a"}
				if tc.fault == lexicalCancelRemoveDirectory {
					names = []string{"entry"}
				}
				if tc.fault == lexicalEmptyDirectory {
					names = nil
				}
				for _, name := range names {
					if tc.fault == lexicalCancelRemoveDirectory {
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
				maximum, err := filestore.NewDirectoryEntryMaximum(2)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				var got []core.RelativePath
				request := filestore.WalkRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, ".")}, Order: order}
				if order == filestore.WalkOrderLexical {
					request.DirectoryEntryMaximum = maximum
				}
				request.Visit = func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
					got = append(got, entry.Path)
					switch tc.fault {
					case lexicalVisitorFailure:
						return filestore.WalkContinue, io.ErrClosedPipe
					case lexicalInvalidDirective:
						return filestore.WalkDirectiveUnknown, nil
					case lexicalCancelDuringVisit:
						cancel()
					case lexicalCancelRemoveDirectory:
						cancel()
						if err := root.Remove(entry.Path.String()); err != nil {
							return filestore.WalkDirectiveUnknown, err
						}
					}
					return filestore.WalkContinue, nil
				}
				switch tc.fault {
				case lexicalNilContext:
					ctx = nil
				case lexicalCancelled:
					cancel()
				case lexicalNilRoot:
					request.Location.Root = nil
				case lexicalUnsetPath:
					request.Location.Path = core.RelativePath{}
				case lexicalUnsetOrder:
					request.Order = filestore.WalkOrderUnknown
				case lexicalNilVisitor:
					request.Visit = nil
				case lexicalUnsetMaximum:
					request.DirectoryEntryMaximum = filestore.DirectoryEntryMaximum{}
				case lexicalNativeMaximum:
					request.Order = filestore.WalkOrderNative
				case lexicalMissingDirectory:
					request.Location.Path = mustRelativePath(t, "missing")
				case lexicalClosedRoot:
					if err := root.Close(); err != nil {
						t.Fatal(err)
					}
				}
				var want []core.RelativePath
				if order == filestore.WalkOrderLexical {
					want = []core.RelativePath{mustRelativePath(t, "a"), mustRelativePath(t, "z")}
					if tc.fault == lexicalCancelRemoveDirectory {
						want = []core.RelativePath{mustRelativePath(t, "entry")}
					}
				} else {
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
				}
				want = want[:tc.wantVisits]
				gotErr := filestore.Walk(ctx, request)
				if !errors.Is(gotErr, tc.wantErr) || !slices.Equal(got, want) {
					t.Fatalf("Walk = %v/%v, want %v/%v", got, gotErr, want, tc.wantErr)
				}
				wantSource := tc.fault == lexicalMissingDirectory || tc.fault == lexicalClosedRoot
				if errors.Is(gotErr, core.ErrFilestoreSource) != wantSource || tc.fault == lexicalClosedRoot && !errors.Is(gotErr, fs.ErrClosed) {
					t.Fatalf("walk source identity = %v, want source=%t and native cause", gotErr, wantSource)
				}
				for _, class := range []error{core.ErrFilestoreDestination, core.ErrFilestoreActivation, core.ErrFilestoreCleanup, core.ErrFilestoreConflict, core.ErrFilestoreSize} {
					if errors.Is(gotErr, class) {
						t.Fatalf("walk refusal = %v, want no %v", gotErr, class)
					}
				}
				if tc.fault == lexicalCancelRemoveDirectory {
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
}

// Count and ceiling vary independently. The uint32 ceiling reaches the full
// constructor domain while callback filesystem work stays at most 255 entries.
func FuzzLexicalWalkCardinalitySemanticClosure(f *testing.F) {
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
	seedCeiling, err := filestore.NewDirectoryEntryMaximum(1)
	if err != nil {
		f.Fatal(err)
	}
	var emitted uint8
	seedRequest := filestore.WalkRequest{Location: filestore.Location{Root: seedRoot, Path: dot}, Order: filestore.WalkOrderLexical, DirectoryEntryMaximum: seedCeiling, Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
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
	f.Add(emitted, uint32(emitted))
	for _, seed := range []struct {
		count   uint8
		maximum uint32
	}{
		{0, 1}, {2, 1}, {63, 64}, {64, 64}, {65, 64}, {255, 255},
		{1, 0}, {1, filestore.DirectoryEntryMaximumLimit - 1}, {1, filestore.DirectoryEntryMaximumLimit}, {1, filestore.DirectoryEntryMaximumLimit + 1}, {1, ^uint32(0)},
	} {
		f.Add(seed.count, seed.maximum)
	}
	f.Fuzz(func(t *testing.T, count uint8, maximum uint32) {
		directory := t.TempDir()
		var want []core.RelativePath
		for index := range int(count) {
			name := fmt.Sprintf("entry-%03d", index)
			if err := os.WriteFile(filepath.Join(directory, name), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			want = append(want, mustRelativePath(t, name))
		}
		before, err := removalFixtureSnapshot(directory)
		if err != nil {
			t.Fatal(err)
		}
		ceiling, constructorErr := filestore.NewDirectoryEntryMaximum(maximum)
		var wantErr error
		invalidMaximum := maximum == 0 || maximum > filestore.DirectoryEntryMaximumLimit
		if invalidMaximum {
			wantErr = core.ErrFilestoreContract
			if !errors.Is(constructorErr, wantErr) || errors.Is(constructorErr, core.ErrFilestoreSource) || ceiling != (filestore.DirectoryEntryMaximum{}) {
				t.Fatalf("ceiling %d = (%v,%v), want zero pure Contract", maximum, ceiling, constructorErr)
			}
		} else if constructorErr != nil || ceiling.Validate() != nil {
			t.Fatalf("ceiling %d = (%v,%v), want admitted", maximum, ceiling, constructorErr)
		}
		var got []core.RelativePath
		request := filestore.WalkRequest{Location: filestore.Location{Root: requireTestRoot(t, directory), Path: mustRelativePath(t, ".")}, Order: filestore.WalkOrderLexical, DirectoryEntryMaximum: ceiling, Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if err := entry.Validate(); err != nil {
				return filestore.WalkDirectiveUnknown, err
			}
			if !entry.Entry.Type().IsRegular() {
				return filestore.WalkDirectiveUnknown, core.ErrFilestoreContract
			}
			got = append(got, entry.Path)
			return filestore.WalkContinue, nil
		}}
		if invalidMaximum || uint32(count) > maximum {
			wantErr = core.ErrFilestoreContract
			want = nil
		}
		gotErr := filestore.Walk(t.Context(), request)
		if !errors.Is(gotErr, wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) || !slices.Equal(got, want) {
			t.Fatalf("Walk(count=%d,max=%d) = (%v,%v), want (%v,%v) without partial prefix or Source", count, maximum, got, gotErr, want, wantErr)
		}
		after, err := removalFixtureSnapshot(directory)
		if err != nil || len(after) != len(before) {
			t.Fatalf("namespace = (%v,%v), want %v", after, err, before)
		}
		for i, entry := range before {
			if after[i].name != entry.name || after[i].mode != entry.mode || after[i].target != entry.target || !slices.Equal(after[i].data, entry.data) {
				t.Fatalf("namespace entry %d = %+v, want %+v", i, after[i], entry)
			}
		}
	})
}
