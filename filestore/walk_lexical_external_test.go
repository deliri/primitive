package filestore_test

import (
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
func TestLexicalWalkReplacementLayerTriad(t *testing.T) {
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
			maximum, err := filestore.NewDirectoryEntryMaximum(2)
			if err != nil {
				t.Fatal(err)
			}
			request := filestore.WalkRequest{Location: filestore.Location{Root: fixture.root, Path: fixture.walk}, Order: filestore.WalkOrderLexical, DirectoryEntryMaximum: maximum}
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
			proveWalkReplacementOracle(t, walkReplacementOracleInput{fixture: fixture, mutation: tc.mutation, skipEntry: tc.directive == filestore.WalkSkipDirectory, gotPaths: got, gotErr: gotErr})
		})
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			names := []string{"z", "a"}
			if tc.fault == lexicalEmptyDirectory {
				names = nil
			}
			for _, name := range names {
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
			request := filestore.WalkRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, ".")}, Order: filestore.WalkOrderLexical, DirectoryEntryMaximum: maximum}
			request.Visit = func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
				got = append(got, entry.Path)
				switch tc.fault {
				case lexicalVisitorFailure:
					return filestore.WalkContinue, io.ErrClosedPipe
				case lexicalInvalidDirective:
					return filestore.WalkDirectiveUnknown, nil
				case lexicalCancelDuringVisit:
					cancel()
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
			gotErr := filestore.Walk(ctx, request)
			want := []core.RelativePath{mustRelativePath(t, "a"), mustRelativePath(t, "z")}[:tc.wantVisits]
			if !errors.Is(gotErr, tc.wantErr) || !slices.Equal(got, want) {
				t.Fatalf("Walk = %v/%v, want %v/%v", got, gotErr, want, tc.wantErr)
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

// Count and ceiling vary independently. Successful delivery must be a complete
// sorted permutation; an oversized directory must deliver no partial prefix.
func FuzzLexicalWalkCardinalitySemanticClosure(f *testing.F) {
	for _, seed := range []struct{ count, maximum uint8 }{{0, 1}, {1, 1}, {2, 1}, {63, 64}, {64, 64}, {65, 64}, {127, 127}} {
		f.Add(seed.count, seed.maximum)
	}
	f.Fuzz(func(t *testing.T, count, maximum uint8) {
		directory := t.TempDir()
		for index := range int(count) {
			name := fmt.Sprintf("entry-%03d", int(count)-1-index)
			if err := os.WriteFile(filepath.Join(directory, name), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		ceiling, err := filestore.NewDirectoryEntryMaximum(uint32(maximum))
		if maximum == 0 {
			if !errors.Is(err, core.ErrFilestoreContract) || ceiling != (filestore.DirectoryEntryMaximum{}) {
				t.Fatalf("zero maximum = %+v/%v, want typed refusal and zero", ceiling, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		var got []core.RelativePath
		err = filestore.Walk(t.Context(), filestore.WalkRequest{Location: filestore.Location{Root: requireTestRoot(t, directory), Path: mustRelativePath(t, ".")}, Order: filestore.WalkOrderLexical, DirectoryEntryMaximum: ceiling, Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			got = append(got, entry.Path)
			return filestore.WalkContinue, nil
		}})
		if count > maximum {
			if !errors.Is(err, core.ErrFilestoreContract) || len(got) != 0 {
				t.Fatalf("oversized walk = %v/%v, want no prefix and typed refusal", got, err)
			}
			return
		}
		want := make([]core.RelativePath, 0, count)
		for index := range int(count) {
			want = append(want, mustRelativePath(t, fmt.Sprintf("entry-%03d", index)))
		}
		if err != nil || !slices.Equal(got, want) {
			t.Fatalf("walk = %v/%v, want exact permutation %v", got, err, want)
		}
	})
}
