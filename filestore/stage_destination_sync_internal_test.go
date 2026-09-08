package filestore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type stageDestinationEffectOwners struct {
	finishStageDestination func(*StageDestination) (StagedFile, error)
	syncParent             func(*os.Root, core.RelativePath) error
	Sync                   func(*os.File) error
}

var _ = stageDestinationEffectOwners{finishStageDestination: finishStageDestination, syncParent: syncParent, Sync: (*os.File).Sync}

// Native byte/identity observations cannot prove persistence after power loss.
// This supplementary compiled-source guard retains the two explicit sync calls;
// it does not claim to prove arbitrary control flow or physical durability.
func TestStageDestinationRetainsFileAndParentSynchronization(t *testing.T) {
	t.Parallel()
	source, err := filestoreGoSources.ReadFile("stage_destination.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "stage_destination.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	owners := reflect.TypeFor[stageDestinationEffectOwners]()
	ownedField, ok := reflect.TypeFor[StageDestination]().FieldByName("file")
	if !ok || ownedField.Type != reflect.TypeFor[*os.File]() {
		t.Fatal("owned file field = missing or changed type, want Go file inventory updated")
	}
	for _, tc := range []struct {
		name      string
		method    bool
		wantCalls int
	}{
		{name: "stage bytes require synchronization of the owned Go file", method: true, wantCalls: 1},
		{name: "custody name requires synchronization of its parent", wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			declarations, calls := ownedEffectCalls(file, owners.Field(0).Name, owners.Field(1).Name)
			if tc.method {
				declarations, calls = ownedParameterMethodCalls(file, owners.Field(0).Name, ownedField.Name, owners.Field(2).Name)
			}
			if declarations != 1 || calls != tc.wantCalls {
				t.Fatalf("owner declarations/sync calls = (%d,%d), want (1,%d)", declarations, calls, tc.wantCalls)
			}
		})
	}
}

func ownedParameterMethodCalls(file *ast.File, owner, field, method string) (int, int) {
	declarations, calls := 0, 0
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != owner {
			continue
		}
		declarations++
		if function.Type.Params == nil || len(function.Type.Params.List) == 0 || len(function.Type.Params.List[0].Names) != 1 {
			continue
		}
		parameter := function.Type.Params.List[0].Names[0].Name
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != method {
				return true
			}
			var receiverExpression ast.Expr = selector.X
			if field != "" {
				owned, ok := selector.X.(*ast.SelectorExpr)
				if !ok || owned.Sel.Name != field {
					return true
				}
				receiverExpression = owned.X
			}
			receiver, ok := receiverExpression.(*ast.Ident)
			if ok && receiver.Name == parameter {
				calls++
			}
			return true
		})
	}
	return declarations, calls
}

func TestOwnedParameterSyncMatcherRejectsBorrowedAndUncalledMethods(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source                string
		wantDeclarations, wantCalls int
	}{
		{name: "owned parameter call is counted", source: "package p;func owner(d *D){d.file.Sync()}", wantDeclarations: 1, wantCalls: 1},
		{name: "parameter spelling is not a hidden convention", source: "package p;func owner(renamed *D){renamed.file.Sync()}", wantDeclarations: 1, wantCalls: 1},
		{name: "method value does not prove synchronization", source: "package p;func owner(d *D){_ = d.file.Sync}", wantDeclarations: 1},
		{name: "foreign receiver cannot lend synchronization", source: "package p;func owner(d *D){other.file.Sync()}", wantDeclarations: 1},
		{name: "foreign field cannot lend synchronization", source: "package p;func owner(d *D){d.other.Sync()}", wantDeclarations: 1},
		{name: "another function cannot lend synchronization", source: "package p;func other(d *D){d.file.Sync()};func owner(d *D){}", wantDeclarations: 1},
		{name: "missing owner stays visible", source: "package p;func other(d *D){d.file.Sync()}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "synthetic.go", tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			declarations, calls := ownedParameterMethodCalls(file, "owner", "file", "Sync")
			if declarations != tc.wantDeclarations || calls != tc.wantCalls {
				t.Fatalf("declarations/calls = (%d,%d), want (%d,%d)", declarations, calls, tc.wantDeclarations, tc.wantCalls)
			}
		})
	}
}
