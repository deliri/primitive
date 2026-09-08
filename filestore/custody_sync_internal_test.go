package filestore

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Fields and values bind the source-level call rule to the actual signatures.
type custodySyncOwners struct {
	Touch                func(context.Context, TouchRequest) error
	ConfirmDurable       func(context.Context, DurabilityRequest) error
	syncCloseCustodyFile func(*os.File) error
}

var _ = custodySyncOwners{Touch: Touch, ConfirmDurable: ConfirmDurable, syncCloseCustodyFile: syncCloseCustodyFile}

// The compiler embeds the exact source from this build, including Go overlays.
// Reading the working tree here would audit a different program under mutation.
//
//go:embed custody.go
var custodySynchronizationSource string

func TestCustodyEffectsUseOwnedFileSynchronization(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "custody.go", custodySynchronizationSource, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	owners := reflect.TypeFor[custodySyncOwners]()
	for _, tc := range []struct {
		name      string
		owner     string
		wantCalls int
	}{
		{name: "Touch cannot stamp without settling its file", owner: owners.Field(0).Name, wantCalls: 1},
		{name: "ConfirmDurable cannot substitute directory sync for file sync", owner: owners.Field(1).Name, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotDeclarations, gotCalls := 0, 0
			for _, declaration := range file.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Name.Name != tc.owner {
					continue
				}
				gotDeclarations++
				ast.Inspect(function.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					name, ok := call.Fun.(*ast.Ident)
					if ok && name.Name == owners.Field(2).Name {
						gotCalls++
					}
					return true
				})
			}
			if gotDeclarations != 1 || gotCalls != tc.wantCalls {
				t.Fatalf("owner declarations/sync calls = (%d,%d), want (1,%d)", gotDeclarations, gotCalls, tc.wantCalls)
			}
		})
	}
}

type custodySyncFixture uint8

const (
	custodySyncEmpty custodySyncFixture = iota
	custodySyncWritten
	custodySyncReadOnly
	custodySyncDirectory
	custodySyncPipeRead
	custodySyncPipeWrite
	custodySyncClosed
	custodySyncNil
)

func TestOwnedFileSyncAndCloseLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		fixture         custodySyncFixture
		wantErr         error
		wantSyncRefusal bool
	}{
		{name: "empty regular file is synchronized and closed", fixture: custodySyncEmpty},
		{name: "dirty binary file is synchronized without changing its bytes", fixture: custodySyncWritten},
		{name: "read only handle can synchronize the file", fixture: custodySyncReadOnly},
		{name: "directory handle uses Go synchronization semantics", fixture: custodySyncDirectory},
		{name: "pipe read end sync failure still releases owned handle", fixture: custodySyncPipeRead, wantSyncRefusal: true},
		{name: "pipe write end sync failure still releases owned handle", fixture: custodySyncPipeWrite, wantSyncRefusal: true},
		{name: "already closed handle preserves native closed identity", fixture: custodySyncClosed, wantErr: os.ErrClosed},
		{name: "nil handle cannot produce synchronization success", fixture: custodySyncNil, wantErr: os.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			var file *os.File
			var peer *os.File
			var err error
			switch tc.fixture {
			case custodySyncEmpty, custodySyncWritten, custodySyncClosed:
				file, err = os.OpenFile(directory+"/file", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
			case custodySyncReadOnly:
				if err := os.WriteFile(directory+"/file", nil, 0o600); err != nil {
					t.Fatal(err)
				}
				file, err = os.Open(directory + "/file")
			case custodySyncDirectory:
				file, err = os.Open(directory)
			case custodySyncPipeRead:
				file, peer, err = os.Pipe()
			case custodySyncPipeWrite:
				peer, file, err = os.Pipe()
			case custodySyncNil:
			default:
				t.Fatalf("fixture = %d, want declared native handle", tc.fixture)
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
			payload := []byte{0, 255, 7}
			if tc.fixture == custodySyncWritten {
				if n, err := file.Write(payload); err != nil || n != len(payload) {
					t.Fatalf("fixture write = (%d,%v), want %d", n, err, len(payload))
				}
			}
			if tc.fixture == custodySyncClosed {
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			wantErr := tc.wantErr
			if tc.wantSyncRefusal {
				var native *fs.PathError
				if err := file.Sync(); !errors.As(err, &native) || native.Err == nil {
					t.Fatalf("Go pipe Sync = %v, want a typed native refusal", err)
				}
				wantErr = native.Err
			}
			gotErr := syncCloseCustodyFile(file)
			if (gotErr == nil) != (wantErr == nil) || wantErr != nil && (!errors.Is(gotErr, core.ErrFilestoreActivation) || !errors.Is(gotErr, wantErr)) {
				t.Fatalf("sync/close = %v, want activation/%v", gotErr, wantErr)
			}
			if file != nil {
				if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("settled handle Stat = %v, want closed", err)
				}
			}
			if tc.fixture == custodySyncWritten {
				got, err := os.ReadFile(directory + "/file")
				if err != nil || !bytes.Equal(got, payload) {
					t.Fatalf("synchronized bytes = (%v,%v), want %v", got, err, payload)
				}
			}
		})
	}
}
