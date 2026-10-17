package filestore

import (
	"go/ast"
	"strconv"
)

// This structural check follows a method on its actual receiver through
// package-local helpers to a private Go file/buffer field. A matching method
// name or an unused native field cannot grant custody. Runtime contracts prove
// conservation, refusal and cleanup separately; this scan does not claim them.
func delegatesOwnedGoFileMethod(files []productionFile, method *ast.FuncDecl, effect string, visited []*ast.FuncDecl) bool {
	for _, seen := range visited {
		if seen == method {
			return false
		}
	}
	visited = append(visited, method)
	if method.Recv == nil || len(method.Recv.List) != 1 || len(method.Recv.List[0].Names) != 1 {
		return false
	}
	typeName := receiverName(method.Recv.List[0].Type)
	receiver := method.Recv.List[0].Names[0].Name
	for node := range ast.Preorder(method.Body) {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			continue
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if field, ok := selector.X.(*ast.SelectorExpr); ok && selector.Sel.Name == effect {
			if target, ok := field.X.(*ast.Ident); ok && target.Name == receiver && isOwnedGoFileField(files, typeName, field.Sel.Name, effect) {
				return true
			}
		}
		if target, ok := selector.X.(*ast.Ident); ok && target.Name == receiver {
			for _, source := range files {
				for _, declaration := range source.file.Decls {
					helper, ok := declaration.(*ast.FuncDecl)
					if ok && helper.Recv != nil && receiverName(helper.Recv.List[0].Type) == typeName && helper.Name.Name == selector.Sel.Name && delegatesOwnedGoFileMethod(files, helper, effect, visited) {
						return true
					}
				}
			}
		}
	}
	return false
}

func isOwnedGoFileField(files []productionFile, receiver, fieldName, effect string) bool {
	for _, source := range files {
		for _, declaration := range source.file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, raw := range generic.Specs {
				spec, ok := raw.(*ast.TypeSpec)
				if !ok || spec.Name.Name != receiver {
					continue
				}
				structure, ok := spec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, field := range structure.Fields.List {
					if len(field.Names) != 1 || field.Names[0].Name != fieldName || ast.IsExported(fieldName) {
						continue
					}
					pointer, ok := field.Type.(*ast.StarExpr)
					if !ok {
						continue
					}
					qualified, ok := pointer.X.(*ast.SelectorExpr)
					if !ok {
						continue
					}
					owner, ok := qualified.X.(*ast.Ident)
					if !ok {
						continue
					}
					for _, dependency := range source.file.Imports {
						path, err := strconv.Unquote(dependency.Path.Value)
						if err != nil {
							continue
						}
						name := path
						if dependency.Name != nil {
							name = dependency.Name.Name
						}
						if name != owner.Name {
							continue
						}
						if path == "os" && qualified.Sel.Name == "File" {
							return true
						}
						if path == "bufio" && qualified.Sel.Name == "Writer" && effect == "Write" {
							return true
						}
					}
				}
			}
		}
	}
	return false
}
