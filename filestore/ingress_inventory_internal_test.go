package filestore

import (
	"fmt"
	"go/ast"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Test-only bridge: discovery reads the exact Go sources compiled into this
// binary, so an overlay cannot add an unlisted door behind a disk-only scan.
func FilestoreIngressDeclarationsForTest(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, source := range parseProductionFiles(t) {
		for _, decl := range source.file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || !function.Name.IsExported() {
				continue
			}
			name := function.Name.Name
			if function.Recv != nil {
				receiver := receiverName(function.Recv.List[0].Type)
				if !ast.IsExported(receiver) || filestoreOwningProjection(function) {
					continue
				}
				name = receiver + "." + name
			}
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func FilestoreIngressSymbolForTest(value reflect.Value) string {
	if !value.IsValid() || value.Kind() != reflect.Func || value.IsNil() {
		return ""
	}
	function := runtime.FuncForPC(value.Pointer())
	if function == nil {
		return ""
	}
	symbol, owned := strings.CutPrefix(function.Name(), reflect.TypeFor[Location]().PkgPath()+".")
	if !owned {
		return ""
	}
	return strings.NewReplacer("(*", "", ")", "").Replace(symbol)
}

// Only these exact no-input signatures project already typed state. A new
// decoder or a same-named method admitting input must remain in the inventory.
func filestoreOwningProjection(function *ast.FuncDecl) bool {
	contracts := []reflect.Type{
		reflect.TypeFor[core.Validatable](), reflect.TypeFor[fmt.Stringer](),
		reflect.TypeFor[interface{ IsValid() bool }](), reflect.TypeFor[interface{ OffWireEnum() }](),
	}
	for _, contract := range contracts {
		method := contract.Method(0)
		if function.Name.Name != method.Name || function.Type.TypeParams.NumFields() != 0 || function.Type.Params.NumFields() != 0 || function.Type.Results.NumFields() != method.Type.NumOut() {
			continue
		}
		if method.Type.NumOut() == 0 {
			return true
		}
		result, ok := function.Type.Results.List[0].Type.(*ast.Ident)
		if ok && result.Name == method.Type.Out(0).Name() {
			return true
		}
	}
	return false
}
