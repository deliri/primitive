package filestore

import (
	"bytes"
	"context"
	"errors"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type directoryEffectOwners struct {
	EnsureDirectory                func(context.Context, DirectoryRequest) error
	synchronizeDirectoryMode       func(*os.Root, core.RelativePath, fs.FileMode) error
	synchronizeOpenedDirectoryMode func(*os.File, core.RelativePath, fs.FileMode) error
	ensureDirectoryEntry           func(directoryEntryEnsure) error
	syncParent                     func(*os.Root, core.RelativePath) error
	Sync                           func(*os.File) error
}

var _ = directoryEffectOwners{EnsureDirectory: EnsureDirectory, synchronizeDirectoryMode: synchronizeDirectoryMode, synchronizeOpenedDirectoryMode: synchronizeOpenedDirectoryMode, ensureDirectoryEntry: ensureDirectoryEntry, syncParent: syncParent, Sync: (*os.File).Sync}

// This compiled-source rule supplements native effects. It establishes explicit
// ownership calls, not arbitrary control-flow or physical power-loss durability.
func TestEnsureDirectoryRetainsOwnedSynchronization(t *testing.T) {
	t.Parallel()
	source, err := filestoreGoSources.ReadFile("directory_read.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "directory_read.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	owners := reflect.TypeFor[directoryEffectOwners]()
	for _, tc := range []struct {
		name          string
		owner, callee int
		method        bool
		wantCalls     int
	}{
		{name: "existing directory acquisition settles the acquired handle", owner: 0, callee: 2, wantCalls: 1},
		{name: "created directory acquisition settles the acquired handle", owner: 1, callee: 2, wantCalls: 1},
		{name: "every newly created name retains parent synchronization", owner: 3, callee: 4, wantCalls: 1},
		{name: "changed directory metadata retains native file synchronization", owner: 2, callee: 5, method: true, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			declarations, calls := ownedEffectCalls(file, owners.Field(tc.owner).Name, owners.Field(tc.callee).Name)
			if tc.method {
				declarations, calls = ownedParameterMethodCalls(file, owners.Field(tc.owner).Name, "", owners.Field(tc.callee).Name)
			}
			if declarations != 1 || calls != tc.wantCalls {
				t.Fatalf("declarations/calls = (%d,%d), want (1,%d)", declarations, calls, tc.wantCalls)
			}
		})
	}
}

func TestOwnedNativeFileMethodMatcherRejectsBorrowedCalls(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source                string
		wantDeclarations, wantCalls int
	}{
		{name: "owned direct parameter call counts", source: "package p;func owner(file *F){file.Sync()}", wantDeclarations: 1, wantCalls: 1},
		{name: "parameter spelling is not the contract", source: "package p;func owner(renamed *F){renamed.Sync()}", wantDeclarations: 1, wantCalls: 1},
		{name: "method value is not an effect", source: "package p;func owner(file *F){_ = file.Sync}", wantDeclarations: 1},
		{name: "foreign handle cannot lend synchronization", source: "package p;func owner(file *F){other.Sync()}", wantDeclarations: 1},
		{name: "nested field is not the owned file parameter", source: "package p;func owner(file *F){file.other.Sync()}", wantDeclarations: 1},
		{name: "different owner cannot lend its call", source: "package p;func owner(file *F){};func other(file *F){file.Sync()}", wantDeclarations: 1},
		{name: "missing owner remains visible", source: "package p;func other(file *F){file.Sync()}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "synthetic.go", tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			declarations, calls := ownedParameterMethodCalls(file, "owner", "", "Sync")
			if declarations != tc.wantDeclarations || calls != tc.wantCalls {
				t.Fatalf("declarations/calls = (%d,%d), want (%d,%d)", declarations, calls, tc.wantDeclarations, tc.wantCalls)
			}
		})
	}
}

func TestOpenedDirectoryModeNativeCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		fixture custodySyncFixture
		mode    fs.FileMode
		wantErr error
	}{
		{name: "empty directory mode changes on the held inode", fixture: custodySyncDirectory, mode: 0o750},
		{name: "nonempty directory keeps its child bytes", fixture: custodySyncWritten, mode: 0o750},
		{name: "held descriptor settles after removing path read access", fixture: custodySyncDirectory, mode: 0o100},
		{name: "same permission observation adds no child", fixture: custodySyncDirectory, mode: 0o700},
		{name: "regular file cannot be chmodded by directory settlement", fixture: custodySyncReadOnly, mode: 0o750, wantErr: fs.ErrExist},
		{name: "pipe refusal closes only the consumed endpoint", fixture: custodySyncPipeRead, mode: 0o750, wantErr: fs.ErrExist},
		{name: "closed descriptor preserves native closure", fixture: custodySyncClosed, mode: 0o750, wantErr: os.ErrClosed},
		{name: "nil descriptor cannot acknowledge a directory", fixture: custodySyncNil, mode: 0o750, wantErr: fs.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			target := filepath.Join(directory, "entry")
			payload := []byte{0, 255, 1}
			var file, peer *os.File
			var err error
			switch tc.fixture {
			case custodySyncDirectory, custodySyncWritten, custodySyncClosed:
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
				if tc.fixture == custodySyncWritten {
					if err := os.WriteFile(filepath.Join(target, "child"), payload, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				file, err = os.Open(target)
			case custodySyncReadOnly:
				if err := os.WriteFile(target, payload, 0o600); err != nil {
					t.Fatal(err)
				}
				file, err = os.Open(target)
			case custodySyncPipeRead:
				file, peer, err = os.Pipe()
			case custodySyncNil:
			default:
				t.Fatalf("unhandled native fixture %v", tc.fixture)
			}
			if err != nil {
				t.Fatal(err)
			}
			if file != nil {
				t.Cleanup(func() {
					if err := file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
						t.Error(err)
					}
				})
			}
			if peer != nil {
				t.Cleanup(func() {
					if err := peer.Close(); err != nil {
						t.Error(err)
					}
				})
			}
			var before fs.FileInfo
			if tc.fixture != custodySyncNil && tc.fixture != custodySyncPipeRead {
				before, err = os.Stat(target)
				if err != nil {
					t.Fatal(err)
				}
			}
			if tc.fixture == custodySyncClosed {
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			path, err := core.ParseRelativePath("entry")
			if err != nil {
				t.Fatal(err)
			}
			gotErr := synchronizeOpenedDirectoryMode(file, path, tc.mode)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("directory settlement = %v, want %v", gotErr, tc.wantErr)
			}
			if file != nil {
				if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("consumed handle = %v, want closed", err)
				}
			}
			if peer != nil {
				if _, err := peer.Stat(); err != nil {
					t.Fatalf("unowned peer = %v, want live", err)
				}
			}
			entries, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			wantEntries := 0
			if before != nil {
				wantEntries = 1
			}
			if len(entries) != wantEntries {
				t.Fatalf("namespace = %v, want %d original entries", entries, wantEntries)
			}
			if before == nil {
				return
			}
			after, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			wantMode := before.Mode()
			if tc.wantErr == nil {
				wantMode = fs.ModeDir | tc.mode
			}
			if !os.SameFile(before, after) || after.Mode() != wantMode || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
				t.Fatalf("settled inode/mode/time = %v, want original %v and mode %v", after, before, wantMode)
			}
			if before.IsDir() {
				if err := os.Chmod(target, 0o700); err != nil {
					t.Fatal(err)
				}
				children, err := os.ReadDir(target)
				if err != nil {
					t.Fatal(err)
				}
				wantChildren := 0
				if tc.fixture == custodySyncWritten {
					wantChildren = 1
				}
				if len(children) != wantChildren {
					t.Fatalf("children = %v, want %d", children, wantChildren)
				}
			}
			if tc.fixture == custodySyncWritten || tc.fixture == custodySyncReadOnly {
				name := target
				if before.IsDir() {
					name = filepath.Join(target, "child")
				}
				got, err := os.ReadFile(name)
				if err != nil || !bytes.Equal(got, payload) {
					t.Fatalf("retained bytes = (%v,%v), want %v", got, err, payload)
				}
			}
		})
	}
}
