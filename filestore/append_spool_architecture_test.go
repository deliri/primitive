package filestore

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The one native append descriptor owns a mutex, reader join and closed-signal
// broadcast. The compiled struct is the shape authority; no queue or unrelated
// synchronization owner is admitted by this structural rule.
func ownsAppendSpoolFields(declaration ast.Decl, imports map[string]string) bool {
	general, ok := declaration.(*ast.GenDecl)
	if !ok || len(general.Specs) != 1 {
		return false
	}
	specification, ok := general.Specs[0].(*ast.TypeSpec)
	owner := reflect.TypeFor[AppendSpool]()
	if !ok || specification.Name.Name != owner.Name() {
		return false
	}
	fields, ok := specification.Type.(*ast.StructType)
	if !ok || len(fields.Fields.List) != owner.NumField() {
		return false
	}
	for index, field := range fields.Fields.List {
		want := owner.Field(index)
		if len(field.Names) != 1 || field.Names[0].Name != want.Name || !appendSpoolFieldType(field.Type, want.Type, imports) {
			return false
		}
	}
	return true
}

func appendSpoolFieldType(expression ast.Expr, want reflect.Type, imports map[string]string) bool {
	if want.PkgPath() != "" {
		if want.PkgPath() == reflect.TypeFor[AppendSpool]().PkgPath() {
			name, ok := expression.(*ast.Ident)
			return ok && name.Name == want.Name()
		}
		selector, ok := expression.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		name, ok := selector.X.(*ast.Ident)
		return ok && imports[name.Name] == want.PkgPath() && selector.Sel.Name == want.Name()
	}
	switch want.Kind() {
	case reflect.Pointer:
		pointer, ok := expression.(*ast.StarExpr)
		return ok && appendSpoolFieldType(pointer.X, want.Elem(), imports)
	case reflect.Chan:
		channel, ok := expression.(*ast.ChanType)
		return ok && channel.Dir == ast.SEND|ast.RECV && appendSpoolFieldType(channel.Value, want.Elem(), imports)
	case reflect.Struct:
		fields, ok := expression.(*ast.StructType)
		return ok && want.NumField() == 0 && fields.Fields.NumFields() == 0
	default:
		name, ok := expression.(*ast.Ident)
		return ok && name.Name == want.Name()
	}
}

func ownsAppendSpoolNotification(declaration ast.Decl, imports map[string]string) bool {
	if ownsAppendSpoolFields(declaration, imports) {
		return true
	}
	function, ok := declaration.(*ast.FuncDecl)
	if !ok {
		return false
	}
	symbol := declarationName(function)
	scope := FilestoreIngressSymbolForTest(reflect.ValueOf(WithAppendSpoolScope))
	notify := FilestoreIngressSymbolForTest(reflect.ValueOf((*AppendSpool).notify))
	reader := FilestoreIngressSymbolForTest(reflect.ValueOf((*AppendSpool).WithReader))
	if symbol != scope && symbol != notify && symbol != reader {
		return false
	}
	channels, signals, workers, queues, loops, joins := 0, 0, 0, 0, 0, 0
	var calls []string
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.GoStmt:
			workers++
		case *ast.MapType, *ast.SendStmt:
			queues++
		case *ast.ForStmt, *ast.RangeStmt:
			loops++
		case *ast.ChanType:
			channels++
		case *ast.UnaryExpr:
			if name, ok := value.X.(*ast.Ident); value.Op == token.ARROW && ok && name.Name == "joined" {
				joins++
			}
		case *ast.CallExpr:
			switch called := value.Fun.(type) {
			case *ast.Ident:
				calls = append(calls, called.Name)
				if called.Name == "make" && len(value.Args) == 1 {
					if channel, ok := value.Args[0].(*ast.ChanType); ok {
						if empty, ok := channel.Value.(*ast.StructType); ok && empty.Fields.NumFields() == 0 {
							signals++
						}
					}
				}
			case *ast.SelectorExpr:
				calls = append(calls, called.Sel.Name)
			}
		}
		return true
	})
	if channels != 1 || signals != 1 || workers != 0 || queues != 0 || loops != 0 {
		return false
	}
	count := func(want string) int {
		got := 0
		for _, call := range calls {
			if call == want {
				got++
			}
		}
		return got
	}
	switch symbol {
	case scope:
		return count(FilestoreIngressSymbolForTest(reflect.ValueOf(OpenAppend))) == 1 && count("Wait") == 1 && count("Close") == 1 && joins == 0
	case notify:
		return count("close") == 1 && joins == 0
	case reader:
		return count("AfterFunc") == 1 && count("Add") == 1 && count("Done") == 1 && count("close") == 1 && joins == 1
	default:
		return false
	}
}

func TestAppendSpoolNotificationAdmissionRefusesQueuesAndDetachedWork(t *testing.T) {
	t.Parallel()
	owner := FilestoreIngressSymbolForTest(reflect.ValueOf((*AppendSpool).notify))
	method := strings.TrimPrefix(owner, reflect.TypeFor[AppendSpool]().Name()+".")
	for _, tc := range []struct {
		name, receiver, capacity, item, extra string
		want                                  bool
	}{
		{name: "native close broadcast has no data queue", receiver: reflect.TypeFor[AppendSpool]().Name(), item: "struct{}", want: true},
		{name: "unrelated receiver has no notification authority", receiver: "foreign", item: "struct{}"},
		{name: "buffered notification is refused", receiver: reflect.TypeFor[AppendSpool]().Name(), item: "struct{}", capacity: ",1"},
		{name: "data payload is refused", receiver: reflect.TypeFor[AppendSpool]().Name(), item: "byte"},
		{name: "detached goroutine is refused", receiver: reflect.TypeFor[AppendSpool]().Name(), item: "struct{}", extra: "go func(){}()"},
		{name: "additional channel is refused", receiver: reflect.TypeFor[AppendSpool]().Name(), item: "struct{}", extra: "_ = make(chan struct{})"},
		{name: "data send is refused", receiver: reflect.TypeFor[AppendSpool]().Name(), item: "struct{}", extra: "s.changed <- struct{}{}"},
		{name: "worker loop is refused", receiver: reflect.TypeFor[AppendSpool]().Name(), item: "struct{}", extra: "for {}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := fmt.Sprintf("package filestore; func(s *%s) %s(){ close(s.changed);s.changed=make(chan %s%s);%s }", tc.receiver, method, tc.item, tc.capacity, tc.extra)
			file, err := parser.ParseFile(token.NewFileSet(), "notification.go", source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if got := ownsAppendSpoolNotification(file.Decls[0], nil); got != tc.want {
				t.Fatalf("native append notification admission = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestAppendSpoolSynchronizationRequiresExactNativeDescriptor(t *testing.T) {
	t.Parallel()
	var template *ast.GenDecl
	imports := make(map[string]string)
	for _, production := range parseProductionFiles(t) {
		if production.name != "append_spool.go" {
			continue
		}
		for _, specification := range production.file.Imports {
			path, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			name := path[strings.LastIndex(path, "/")+1:]
			if specification.Name != nil {
				name = specification.Name.Name
			}
			imports[name] = path
		}
		for _, declaration := range production.file.Decls {
			if ownsAppendSpoolFields(declaration, imports) {
				template = declaration.(*ast.GenDecl)
			}
		}
	}
	if template == nil {
		t.Fatalf("native descriptor template = %v, want compiled spool fields", template)
	}
	for _, change := range []string{"unchanged", "foreign_owner", "foreign_sync_import", "wrong_mutex_type", "additional_queue"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			declaration := *template
			specification := *template.Specs[0].(*ast.TypeSpec)
			body := *specification.Type.(*ast.StructType)
			fields := *body.Fields
			fields.List = slices.Clone(fields.List)
			body.Fields = &fields
			specification.Type = &body
			declaration.Specs = []ast.Spec{&specification}
			names := maps.Clone(imports)
			switch change {
			case "foreign_owner":
				specification.Name = ast.NewIdent("foreign")
			case "foreign_sync_import":
				names["sync"] = "foreign/sync"
			case "wrong_mutex_type":
				for index, field := range fields.List {
					if field.Names[0].Name == "mutex" {
						replacement := *field
						replacement.Type = ast.NewIdent("string")
						fields.List[index] = &replacement
					}
				}
			case "additional_queue":
				fields.List = append(fields.List, &ast.Field{Names: []*ast.Ident{ast.NewIdent("queue")}, Type: &ast.ChanType{Dir: ast.SEND | ast.RECV, Value: ast.NewIdent("byte")}})
			}
			want := change == "unchanged"
			if got := ownsAppendSpoolFields(&declaration, names); got != want {
				t.Fatalf("native descriptor admission = %t, want %t", got, want)
			}
		})
	}
}
