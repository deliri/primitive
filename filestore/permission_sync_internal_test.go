package filestore

import (
	"context"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"
)

type permissionSyncOwners struct {
	SetPermissions       func(context.Context, PermissionRequest) error
	syncCloseCustodyFile func(*os.File) error
}

var _ = permissionSyncOwners{SetPermissions: SetPermissions, syncCloseCustodyFile: syncCloseCustodyFile}

// The native mode oracle does not prove persistence. This supplementary
// compiled-source rule requires the owned-file sync/close primitive, whose
// actual Go handle effects have their own native table.
func TestPermissionMetadataRetainsOwnedFileSynchronization(t *testing.T) {
	t.Parallel()
	source, err := filestoreGoSources.ReadFile("permissions.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "permissions.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	owners := reflect.TypeFor[permissionSyncOwners]()
	for _, tc := range []struct{ name, owner, callee string }{
		{name: "permission metadata cannot borrow a parent-only synchronization", owner: owners.Field(0).Name, callee: owners.Field(1).Name},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotDeclarations, gotCalls := ownedEffectCalls(file, tc.owner, tc.callee)
			if gotDeclarations != 1 || gotCalls != 1 {
				t.Fatalf("declarations/file-sync calls = (%d,%d), want (1,1)", gotDeclarations, gotCalls)
			}
		})
	}
}
