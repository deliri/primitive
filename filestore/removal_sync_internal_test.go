package filestore

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type removalSyncOwners struct {
	Remove     func(context.Context, RemovalRequest) error
	RemoveTree func(context.Context, TreeRemovalRequest) error
	syncParent func(*os.Root, core.RelativePath) error
}

var _ = removalSyncOwners{Remove: Remove, RemoveTree: RemoveTree, syncParent: syncParent}

// Native namespace tests cannot observe a power-loss acknowledgment. This
// supplementary source ratchet retains the explicit synchronization calls in
// the compiled program; it is not a hardware durability or control-flow proof.
func TestRemovalRetainsParentSynchronization(t *testing.T) {
	t.Parallel()
	source, err := filestoreGoSources.ReadFile("append_remove.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "append_remove.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	owners := reflect.TypeFor[removalSyncOwners]()
	for _, tc := range []struct{ name, owner string }{
		{name: "leaf deletion retains its parent synchronization", owner: owners.Field(0).Name},
		{name: "recursive deletion retains its parent synchronization", owner: owners.Field(1).Name},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotDeclarations, gotCalls := ownedEffectCalls(file, tc.owner, owners.Field(2).Name)
			if gotDeclarations != 1 || gotCalls != 1 {
				t.Fatalf("declarations/sync calls = (%d,%d), want (1,1)", gotDeclarations, gotCalls)
			}
		})
	}
}

func ownedEffectCalls(file *ast.File, owner, callee string) (int, int) {
	declarations, calls := 0, 0
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != owner {
			continue
		}
		declarations++
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := call.Fun.(*ast.Ident)
			if ok && name.Name == callee {
				calls++
			}
			return true
		})
	}
	return declarations, calls
}

func TestRemovalSynchronizationMatcherRejectsMissingAndBorrowedCalls(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source                string
		wantDeclarations, wantCalls int
	}{
		{name: "actual call in owner is counted", source: "package p;func owner(){syncParent()}", wantDeclarations: 1, wantCalls: 1},
		{name: "callee value does not prove invocation", source: "package p;func owner(){_ = syncParent}", wantDeclarations: 1},
		{name: "another function cannot lend its call", source: "package p;func other(){syncParent()};func owner(){}", wantDeclarations: 1},
		{name: "missing owner cannot disappear from inventory", source: "package p;func other(){syncParent()}"},
		{name: "duplicate calls cannot hide repeated synchronization", source: "package p;func owner(){syncParent();syncParent()}", wantDeclarations: 1, wantCalls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "synthetic.go", tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			gotDeclarations, gotCalls := ownedEffectCalls(file, "owner", "syncParent")
			if gotDeclarations != tc.wantDeclarations || gotCalls != tc.wantCalls {
				t.Fatalf("declarations/calls = (%d,%d), want (%d,%d)", gotDeclarations, gotCalls, tc.wantDeclarations, tc.wantCalls)
			}
		})
	}
}
