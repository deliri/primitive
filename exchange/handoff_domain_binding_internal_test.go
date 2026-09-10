package exchange

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// The matrix's recognized error domain follows actual compiler-bound functions.
// A new core identity in a classifier must also appear in the matrix fixture.
func TestHandoffErrorDomainBindsClassifierInputs(t *testing.T) {
	t.Parallel()
	model := handoffFunctionCoreSelectors(t, TestHTTPProducerClassifierRefusalLatticeLayerTriad)
	functions := []struct {
		name     string
		function any
	}{
		{name: "aggregate refusal decision", function: classifyAggregateCause},
		{name: "stream refusal precedence", function: terminalStreamReplayCause},
		{name: "stream retry decision", function: replayStreamDecision},
	}
	for _, tc := range functions {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			identities := handoffFunctionCoreSelectors(t, tc.function)
			if len(identities) == 0 {
				t.Fatalf("classifier core identities=%q, want a nonempty domain", identities)
			}
			for _, identity := range identities {
				if !slices.Contains(model, identity) {
					t.Errorf("classifier core symbol %s is absent from exhaustive producer fixture", identity)
				}
			}
		})
	}
}

// Source setup follows the compiler's function location, reads through Filestore,
// and uses Go's AST. Import ownership, not the local name "core", identifies the
// selectors. This helper contains no behavioral assertions about a handoff.
func handoffFunctionCoreSelectors(t *testing.T, function any) []string {
	t.Helper()
	compiled := runtime.FuncForPC(reflect.ValueOf(function).Pointer())
	if compiled == nil {
		t.Fatalf("compiled function=%v, want a bound function", compiled)
	}
	file, _ := compiled.FileLine(compiled.Entry())
	name := compiled.Name()
	name = name[strings.LastIndexByte(name, '.')+1:]
	absolute, err := core.ParseAbsolutePath(filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	root, err := filestore.OpenRoot(t.Context(), absolute)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Errorf("source root close = %v, want nil", err)
		}
	}()
	path, err := core.ParseRelativePath(filepath.Base(file))
	if err != nil {
		t.Fatal(err)
	}
	var source bytes.Buffer
	if _, err := filestore.Read(t.Context(), filestore.ReadRequest{Location: filestore.Location{Root: root, Path: path}, Destination: &source}); err != nil {
		t.Fatal(err)
	}
	syntax, err := parser.ParseFile(token.NewFileSet(), file, source.Bytes(), parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	corePath := reflect.TypeOf(core.ErrExchangeRequest).PkgPath()
	var coreName string
	for _, spec := range syntax.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if path == corePath {
			coreName = filepath.Base(path)
			if spec.Name != nil {
				coreName = spec.Name.Name
			}
		}
	}
	if coreName == "" {
		t.Fatalf("core import binding=%q, want a resolved import", coreName)
	}
	var result []string
	found := false
	for _, decl := range syntax.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name {
			continue
		}
		found = true
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			owner, ok := selector.X.(*ast.Ident)
			if ok && owner.Name == coreName && !slices.Contains(result, selector.Sel.Name) {
				result = append(result, selector.Sel.Name)
			}
			return true
		})
	}
	if !found {
		t.Fatalf("compiler-bound function %s is absent from %s", name, file)
	}
	return result
}
