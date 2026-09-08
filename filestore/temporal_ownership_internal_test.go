package filestore

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This is a compiler-bound registry of forbidden execution symbols, not a
// clock effect. Only this registry's declaration is exempt from the scan.
func temporalEffectSymbols() []string {
	functions := []reflect.Value{
		reflect.ValueOf(time.Now), reflect.ValueOf(time.Since), reflect.ValueOf(time.Until),
		reflect.ValueOf(time.Sleep), reflect.ValueOf(time.After), reflect.ValueOf(time.AfterFunc),
		reflect.ValueOf(time.NewTimer), reflect.ValueOf(time.NewTicker), reflect.ValueOf(time.Tick),
		reflect.ValueOf(context.WithTimeout), reflect.ValueOf(context.WithTimeoutCause),
		reflect.ValueOf(context.WithDeadline), reflect.ValueOf(context.WithDeadlineCause),
	}
	symbols := make([]string, 0, len(functions))
	for _, function := range functions {
		symbols = append(symbols, runtime.FuncForPC(function.Pointer()).Name())
	}
	return symbols
}

func temporalBypasses(file *ast.File) []string {
	var imports []struct{ alias, path string }
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		alias := path.Base(imported)
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		imports = append(imports, struct{ alias, path string }{alias, imported})
	}
	var got []string
	symbols := temporalEffectSymbols()
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == strings.TrimPrefix(runtime.FuncForPC(reflect.ValueOf(temporalEffectSymbols).Pointer()).Name(), reflect.TypeFor[Location]().PkgPath()+".") {
			continue
		}
		ast.Inspect(decl, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			for _, imported := range imports {
				symbol := imported.path + "." + selector.Sel.Name
				if qualifier.Name == imported.alias && slices.Contains(symbols, symbol) {
					got = append(got, symbol)
				}
			}
			return true
		})
	}
	slices.Sort(got)
	return got
}

func TestFilestoreTimeEffectsBelongToTemporal(t *testing.T) {
	t.Parallel()
	entries, err := filestoreGoSources.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()
			source, err := filestoreGoSources.ReadFile(entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if got := temporalBypasses(file); len(got) != 0 {
				t.Fatalf("time execution bypasses = %v, want Temporal ownership", got)
			}
		})
	}
}

func TestTemporalOwnershipMatcherRequiresActualImportIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		want         []string
	}{
		{name: "clock read is an execution effect", source: `import "time";func run(){_ = time.Now()}`, want: []string{"time.Now"}},
		{name: "renamed timer import cannot bypass ownership", source: `import clock "time";func run(){_ = clock.After(0)}`, want: []string{"time.After"}},
		{name: "captured timer function cannot hide later execution", source: `import "time";func run(){_ = time.NewTimer}`, want: []string{"time.NewTimer"}},
		{name: "context timeout still owns a clock effect", source: `import "context";func run(){_,_ = context.WithTimeout(nil,0)}`, want: []string{"context.WithTimeout"}},
		{name: "context deadline cause still owns a clock effect", source: `import ctx "context";func run(){_ = ctx.WithDeadlineCause}`, want: []string{"context.WithDeadlineCause"}},
		{name: "duration values add no clock execution", source: `import "time";func run(){_ = time.Second}`},
		{name: "explicit cancellation needs no timer", source: `import "context";func run(){_,_ = context.WithCancel(nil)}`},
		{name: "Temporal wrapper is the admitted owner", source: `import temporal "github.com/deliri/primitive/v2026/temporal";func run(){_ = temporal.WithTimeout}`},
		{name: "foreign Now spelling is not Go clock ownership", source: `import time "foreign";func run(){_ = time.Now}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture;"+tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if got := temporalBypasses(file); !slices.Equal(got, tc.want) {
				t.Fatalf("bypasses = %v, want %v", got, tc.want)
			}
		})
	}
}
