package exchange

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"path"
	"reflect"
	"slices"
	"strconv"
	"testing"
)

// These are source-matcher counterexamples, not HTTP runtime fixtures. Every
// declaration exercises an ownership or AST-shape distinction that formatted
// substring matching cannot prove. The actual package still compiles normally.
func TestRawHTTPArchitectureBindingTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		source      string
		extraSource string
		wantRaw     bool
	}{
		{name: "canonical request pointer remains visible", source: `import "net/http"; func Door(*http.Request) {}`, wantRaw: true},
		{name: "canonical response writer remains visible", source: `import "net/http"; func Door(http.ResponseWriter) {}`, wantRaw: true},
		{name: "aliased import cannot hide request pointer", source: `import wire "net/http"; func Door(*wire.Request) {}`, wantRaw: true},
		{name: "aliased import cannot hide response writer", source: `import wire "net/http"; func Door(wire.ResponseWriter) {}`, wantRaw: true},
		{name: "dot import cannot hide raw ownership", source: `import . "net/http"; func Door(*Request) {}`, wantRaw: true},
		{name: "by value request is still raw HTTP", source: `import "net/http"; func Door(http.Request) {}`, wantRaw: true},
		{name: "pointer alias cannot hide raw ownership", source: `import "net/http"; type R = *http.Request; func Door(R) {}`, wantRaw: true},
		{name: "defined request carrier cannot hide raw ownership", source: `import "net/http"; type R http.Request; func Door(R) {}`, wantRaw: true},
		{name: "chained aliases preserve original package ownership", source: `import "net/http"; type R = *http.Request; type S = R; func Door(S) {}`, wantRaw: true},
		{name: "named embedded carrier cannot hide raw request", source: `import "net/http"; type R struct { *http.Request }; func Door(R) {}`, wantRaw: true},
		{name: "recursive carrier still exposes its raw writer sibling", source: `import "net/http"; type R struct { Next *R; Writer http.ResponseWriter }; func Door(R) {}`, wantRaw: true},
		{name: "callback parameter cannot hide raw request", source: `import "net/http"; func Door(func(*http.Request)) {}`, wantRaw: true},
		{name: "result slot cannot export raw writer", source: `import "net/http"; func Door() http.ResponseWriter { return nil }`, wantRaw: true},
		{name: "slice cannot hide raw request elements", source: `import "net/http"; func Door([]*http.Request) {}`, wantRaw: true},
		{name: "map cannot hide raw response writers", source: `import "net/http"; func Door(map[string]http.ResponseWriter) {}`, wantRaw: true},
		{name: "channel cannot hide raw request custody", source: `import "net/http"; func Door(<-chan *http.Request) {}`, wantRaw: true},
		{name: "generic argument cannot hide raw request", source: `import "net/http"; type Box[T any] struct{ V T }; func Door(Box[*http.Request]) {}`, wantRaw: true},
		{name: "neutral admitted standard client is a separate capability", source: `import "net/http"; func Door(*http.Client) {}`},
		{name: "neutral unrelated package named http is not HTTP ownership", source: `import http "net/url"; func Door(*http.URL) {}`},
		{name: "neutral local Request name is not Go HTTP ownership", source: `type Request struct { Value string }; func Door(*Request) {}`},
		{name: "neutral recursive carrier without raw HTTP terminates", source: `type R struct { Next *R; Value string }; func Door(R) {}`},
		{name: "cross-file alias resolves imports at its declaration", source: `import wire "net/url"; func Door(R, *wire.URL) {}`, extraSource: `import wire "net/http"; type R = *wire.Request`, wantRaw: true},
		{name: "neutral parameter name cannot impersonate dot-imported type", source: `import . "net/http"; func Door(Request string, _ *Client) {}`},
		{name: "neutral a tag containing type text is not a type", source: "func Door(struct { Value string `note:\"*http.Request\"` }) {}"},
		{name: "function type parameter shadows a package raw alias", source: `import "net/http"; type R = *http.Request; func Door[R any](R) {}`},
		{name: "function type parameter shadows a dot imported request", source: `import . "net/http"; func Door[Request any](Request, *Client) {}`},
		{name: "generic carrier parameter shadows a package raw alias", source: `import "net/http"; type R = *http.Request; type Box[R any] struct { Value R }; func Door(Box[int]) {}`},
		{name: "generic carrier parameter shadows a dot imported writer", source: `import . "net/http"; type Box[ResponseWriter any] struct { Value ResponseWriter }; func Door(Box[int], *Client) {}`},
		{name: "parameter scope cannot hide a distinct raw field", source: `import "net/http"; type R = *http.Request; type Box[R any] struct { Value R; Writer http.ResponseWriter }; func Door(Box[int]) {}`, wantRaw: true},
		{name: "a type constraint cannot hide raw writer admission", source: `import "net/http"; func Door[W http.ResponseWriter](W) {}`, wantRaw: true},
		{name: "an instantiated raw argument survives generic parameter shadowing", source: `import "net/http"; type R = *http.Request; type Box[R any] struct { Value R }; func Door(Box[*http.Request]) {}`, wantRaw: true},
		{name: "cross-file generic declaration owns its parameter scope", source: `func Door(Box[int]) {}`, extraSource: `import "net/http"; type R = *http.Request; type Box[R any] struct { Value R }`},
		{name: "caller type parameter cannot alter a package carrier binding", source: `import "net/http"; type R = *http.Request; type Box struct { Value R }; func Door[R any](Box, R) {}`, wantRaw: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fileSet := token.NewFileSet()
			file, err := parser.ParseFile(fileSet, "fixture.go", "package fixture\n"+tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("fixture parse = %v, want nil", err)
			}
			var signature *ast.FuncType
			for _, declaration := range file.Decls {
				if function, ok := declaration.(*ast.FuncDecl); ok {
					signature = function.Type
				}
			}
			if signature == nil {
				t.Fatalf("fixture signature=%v, want a public function", signature)
			}
			sources := []exchangeArchitectureSource{{name: "fixture.go", syntax: file}}
			if tc.extraSource != "" {
				extra, err := parser.ParseFile(fileSet, "extra.go", "package fixture\n"+tc.extraSource, parser.SkipObjectResolution)
				if err != nil {
					t.Fatalf("extra fixture parse = %v, want nil", err)
				}
				sources = append(sources, exchangeArchitectureSource{name: "extra.go", syntax: extra})
			}
			gotRaw := rawHTTPType(file, signature, rawHTTPBindings(sources), rawHTTPTypeScope{})
			if gotRaw != tc.wantRaw {
				t.Fatalf("raw HTTP ownership = %t, want %t", gotRaw, tc.wantRaw)
			}
		})
	}
}

// rawHTTPTypeBinding retains the declaration's file because import aliases are
// file-scoped even when the declared type is visible throughout the package.
type rawHTTPTypeBinding struct {
	file          *ast.File
	specification *ast.TypeSpec
}

// The matcher follows only source-level type carriers; compilation remains
// Go's responsibility. These lexical names keep a type parameter from being
// mistaken for an unrelated package declaration or dot import.
type rawHTTPTypeScope struct {
	visiting []*ast.TypeSpec
	shadowed []string
}

func (scope rawHTTPTypeScope) withTypeParameters(parameters *ast.FieldList) rawHTTPTypeScope {
	if parameters == nil {
		return scope
	}
	scope.shadowed = slices.Clone(scope.shadowed)
	for _, parameter := range parameters.List {
		for _, name := range parameter.Names {
			scope.shadowed = append(scope.shadowed, name.Name)
		}
	}
	return scope
}

func rawHTTPBindings(sources []exchangeArchitectureSource) []rawHTTPTypeBinding {
	var bindings []rawHTTPTypeBinding
	for _, source := range sources {
		for _, declaration := range source.syntax.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, specification := range group.Specs {
				if named, ok := specification.(*ast.TypeSpec); ok {
					bindings = append(bindings, rawHTTPTypeBinding{file: source.syntax, specification: named})
				}
			}
		}
	}
	return bindings
}

func rawHTTPType(file *ast.File, expression ast.Expr, bindings []rawHTTPTypeBinding, scope rawHTTPTypeScope) bool {
	found := false
	ast.Inspect(expression, func(node ast.Node) bool {
		if found {
			return false
		}
		switch typed := node.(type) {
		case *ast.FuncType:
			child := scope.withTypeParameters(typed.TypeParams)
			for _, list := range []*ast.FieldList{typed.TypeParams, typed.Params, typed.Results} {
				if list == nil {
					continue
				}
				for _, field := range list.List {
					if rawHTTPType(file, field.Type, bindings, child) {
						found = true
						return false
					}
				}
			}
			return false
		case *ast.Field:
			// Labels and tags cannot stand in for a field's actual type.
			found = rawHTTPType(file, typed.Type, bindings, scope)
			return false
		case *ast.ArrayType:
			found = rawHTTPType(file, typed.Elt, bindings, scope)
			return false
		case *ast.SelectorExpr:
			qualifier, ok := typed.X.(*ast.Ident)
			found = ok && rawHTTPImportedType(file, qualifier.Name, typed.Sel.Name)
			return false
		case *ast.Ident:
			found = !slices.Contains(scope.shadowed, typed.Name) && rawHTTPIdentifier(file, typed.Name, bindings, scope)
			return false
		}
		return true
	})
	return found
}

func rawHTTPIdentifier(file *ast.File, name string, bindings []rawHTTPTypeBinding, scope rawHTTPTypeScope) bool {
	for _, binding := range bindings {
		specification := binding.specification
		if specification.Name.Name != name {
			continue
		}
		// This is the admitted nominal socket, whose private Go handles are
		// deliberately sealed. Merely reusing its name in another package is
		// not this compiler-bound capability.
		sealed := reflect.TypeFor[SocketServerCall]()
		if name == sealed.Name() && binding.file.Name.Name == path.Base(sealed.PkgPath()) {
			return false
		}
		if slices.Contains(scope.visiting, specification) {
			return false
		}
		// Package declarations resolve in their own lexical scope, never in
		// the scope of a caller whose signature happens to reference them.
		declarationScope := rawHTTPTypeScope{visiting: append(scope.visiting, specification)}
		return rawHTTPType(binding.file, specification.Type, bindings, declarationScope.withTypeParameters(specification.TypeParams))
	}
	return rawHTTPImportedType(file, ".", name)
}

func rawHTTPImportedType(file *ast.File, qualifier, name string) bool {
	request := reflect.TypeFor[http.Request]()
	writer := reflect.TypeFor[http.ResponseWriter]()
	if name != request.Name() && name != writer.Name() {
		return false
	}
	for _, specification := range file.Imports {
		imported, err := strconv.Unquote(specification.Path.Value)
		if err != nil || imported != request.PkgPath() {
			continue
		}
		local := path.Base(imported)
		if specification.Name != nil {
			local = specification.Name.Name
		}
		if local == qualifier {
			return true
		}
	}
	return false
}
