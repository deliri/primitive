package filestore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestWindowsOpenRootStructurallyPinsInspectedDirectoryIdentity(t *testing.T) {
	t.Parallel()

	parsed, err := parser.ParseFile(token.NewFileSet(), "open_root_windows.go", nil, 0)
	if err != nil {
		t.Fatalf("parser.ParseFile(open_root_windows.go) error = %v, want nil", err)
	}
	wantCallNames := []string{"os.Lstat", "windowsReparsePoint", "os.OpenRoot", "root.Stat", "validateWindowsSameDirectory"}
	foundCalls := make(map[string]bool, len(wantCallNames))
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch function := call.Fun.(type) {
		case *ast.Ident:
			for _, name := range wantCallNames {
				if function.Name == name {
					foundCalls[name] = true
				}
			}
		case *ast.SelectorExpr:
			owner, ok := function.X.(*ast.Ident)
			if !ok {
				return true
			}
			name := owner.Name + "." + function.Sel.Name
			for _, want := range wantCallNames {
				if name == want {
					foundCalls[want] = true
				}
			}
		}
		return true
	})
	for _, name := range wantCallNames {
		if !foundCalls[name] {
			t.Fatalf("openRootDirectory structural call %s present = false, want true", name)
		}
	}
}
