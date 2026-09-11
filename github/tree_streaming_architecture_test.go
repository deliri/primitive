package github

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

// The fixed tree grammar must not grow token, path, or collection storage.
// This structural guard complements generated-stream behavior and benchmarks.
func TestTreeStreamingParsersRejectGrowingAggregation(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"tree_decode_stream.go", "tree_entry_stream.go", "json_string_stream.go"} {
		source, err := githubSource.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		got := growingTreeStorage(t, source)
		if len(got) != 0 {
			t.Fatalf("%s growing storage=%v, want none", name, got)
		}
	}
	fixtures := []struct {
		name, source string
		want         []string
	}{
		{name: "growing append is visible", source: `package p; func f(){ _=append([]byte{},1) }`, want: []string{"append"}},
		{name: "dynamic make is visible", source: `package p; func f(n int){ _=make([]byte,n) }`, want: []string{"make"}},
		{name: "whole-stream read is visible", source: `package p; func f(){ io.ReadAll(r) }`, want: []string{"ReadAll"}},
		{name: "aggregate buffer is visible", source: `package p; var b bytes.Buffer`, want: []string{"Buffer"}},
		{name: "variable slice field is visible", source: `package p; type carrier struct { data []byte }`, want: []string{"slice field"}},
		{name: "map field is visible", source: `package p; type carrier struct { data map[string]int }`, want: []string{"map field"}},
		{name: "fixed scratch and borrowed read parameters are allowed", source: `package p; type carrier struct { scratch [4]byte };func f(p []byte){}`},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			got := growingTreeStorage(t, []byte(fixture.source))
			if !slices.Equal(got, fixture.want) {
				t.Fatalf("storage matches=%v, want %v", got, fixture.want)
			}
		})
	}
}
func growingTreeStorage(t *testing.T, source []byte) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "tree.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.CallExpr:
			if name, ok := value.Fun.(*ast.Ident); ok && (name.Name == "make" || name.Name == "append") {
				found = append(found, name.Name)
			}
		case *ast.SelectorExpr:
			switch value.Sel.Name {
			case "ReadAll", "ReadValue", "ReadToken", "NewDecoder", "NewReaderSize", "Buffer", "Builder":
				found = append(found, value.Sel.Name)
			}
		case *ast.StructType:
			for _, field := range value.Fields.List {
				switch kind := field.Type.(type) {
				case *ast.ArrayType:
					if kind.Len == nil {
						found = append(found, "slice field")
					}
				case *ast.MapType:
					found = append(found, "map field")
				}
			}
		}
		return true
	})
	return found
}
