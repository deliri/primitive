package gotoolchain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Region filtering must precede the native ancestry walk. Result equality
// cannot distinguish the repeated work that doubled the consumer benchmark.
func TestSSABlocksNarrowRegionBeforeUntakenTraversal(t *testing.T) {
	t.Parallel()
	source, err := goToolchainSource.ReadFile("ssa_blocks.go")
	if err != nil {
		t.Fatalf("block query source error=%v, want nil", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "ssa_blocks.go", source, 0)
	if err != nil {
		t.Fatalf("parse block query error=%v, want nil", err)
	}
	var narrow, untaken token.Pos
	narrowCalls, untakenCalls := 0, 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			if fn.Sel.Name == "Dominates" {
				narrow, narrowCalls = call.Pos(), narrowCalls+1
			}
		case *ast.Ident:
			if fn.Name == "argumentCallUntakenBlock" {
				untaken, untakenCalls = call.Pos(), untakenCalls+1
			}
		}
		return true
	})
	if narrowCalls != 1 || untakenCalls != 1 || narrow >= untaken {
		t.Fatalf("region/ancestry calls=%d/%d at %d/%d, want one each with region first", narrowCalls, untakenCalls, narrow, untaken)
	}
}
