package filestore

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type renameSynchronizationOwners struct {
	Rename     func(context.Context, RenameRequest) error
	syncParent func(*os.Root, core.RelativePath) error
}

var _ = renameSynchronizationOwners{Rename: Rename, syncParent: syncParent}

// Native permission failures prove the first post-effect refusal. This
// supplementary source guard retains both parent arguments, including the old
// parent whose physical power-loss persistence cannot be observed in a test.
func TestRenameRetainsBothChangedParentSynchronizationArguments(t *testing.T) {
	t.Parallel()
	source, err := filestoreGoSources.ReadFile("rename.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "rename.go", source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	owners := reflect.TypeFor[renameSynchronizationOwners]()
	request := reflect.TypeFor[RenameRequest]()
	location := reflect.TypeFor[Location]()
	declarations, arguments := ownedCallSelectorArguments(file, owners.Field(0).Name, owners.Field(1).Name, 1, 1)
	want := [][]string{{request.Field(1).Name}, {request.Field(0).Name, location.Field(1).Name}}
	if declarations != 1 || !slices.EqualFunc(arguments, want, func(a, b []string) bool { return slices.Equal(a, b) }) {
		t.Fatalf("owner declarations/parent arguments = (%d,%v), want (1,%v)", declarations, arguments, want)
	}
}

func ownedCallSelectorArguments(file *ast.File, owner, callee string, parameterIndex, argumentIndex int) (int, [][]string) {
	declarations := 0
	var arguments [][]string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != owner {
			continue
		}
		declarations++
		if function.Type.Params == nil || len(function.Type.Params.List) <= parameterIndex || len(function.Type.Params.List[parameterIndex].Names) != 1 {
			continue
		}
		parameter := function.Type.Params.List[parameterIndex].Names[0].Name
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := call.Fun.(*ast.Ident)
			if !ok || name.Name != callee {
				return true
			}
			var selectors []string
			if len(call.Args) > argumentIndex {
				expression := call.Args[argumentIndex]
				for {
					selector, ok := expression.(*ast.SelectorExpr)
					if !ok {
						break
					}
					selectors = append(selectors, selector.Sel.Name)
					expression = selector.X
				}
				receiver, ok := expression.(*ast.Ident)
				if !ok || receiver.Name != parameter {
					selectors = nil
				}
				slices.Reverse(selectors)
			}
			arguments = append(arguments, selectors)
			return true
		})
	}
	return declarations, arguments
}

func TestOwnedParentArgumentMatcherRejectsMissingAndForeignFacts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source     string
		wantDeclarations int
		want             [][]string
	}{
		{name: "actual target then source arguments stay ordered", source: "package p;func owner(ctx C,r R){sync(root,r.Target);sync(root,r.Location.Path)}", wantDeclarations: 1, want: [][]string{{"Target"}, {"Location", "Path"}}},
		{name: "parameter rename cannot erase ownership", source: "package p;func owner(ctx C,renamed R){sync(root,renamed.Target)}", wantDeclarations: 1, want: [][]string{{"Target"}}},
		{name: "foreign request cannot lend a target", source: "package p;func owner(ctx C,r R){sync(root,foreign.Target)}", wantDeclarations: 1, want: [][]string{nil}},
		{name: "literal cannot masquerade as compiler-owned path", source: "package p;func owner(ctx C,r R){sync(root,\"Target\")}", wantDeclarations: 1, want: [][]string{nil}},
		{name: "missing path argument stays visible", source: "package p;func owner(ctx C,r R){sync(root)}", wantDeclarations: 1, want: [][]string{nil}},
		{name: "method value is not a parent effect", source: "package p;func owner(ctx C,r R){_ = sync}", wantDeclarations: 1},
		{name: "different owner cannot lend a parent", source: "package p;func owner(ctx C,r R){};func other(ctx C,r R){sync(root,r.Target)}", wantDeclarations: 1},
		{name: "missing owner stays visible", source: "package p;func other(ctx C,r R){sync(root,r.Target)}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "synthetic.go", tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			declarations, arguments := ownedCallSelectorArguments(file, "owner", "sync", 1, 1)
			if declarations != tc.wantDeclarations || !slices.EqualFunc(arguments, tc.want, func(a, b []string) bool { return slices.Equal(a, b) }) {
				t.Fatalf("declarations/arguments = (%d,%v), want (%d,%v)", declarations, arguments, tc.wantDeclarations, tc.want)
			}
		})
	}
}
