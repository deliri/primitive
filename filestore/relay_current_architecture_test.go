package filestore

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

// The only process-lifetime owner is private, accepts the two native Process
// scopes, and cannot return after starting work except through ExitCurrent.
// One bounded channel carries the copy observation and then its joined close.
// Native child tests prove those observations and the parent's final OS wait;
// this scan prevents turning that explicit boundary into a generic detached
// relay, callback executor, worker pool, or ordinary premature return.
func ownsCurrentProcessRelay(declaration ast.Decl) bool {
	function, ok := declaration.(*ast.FuncDecl)
	if !ok || function.Recv != nil || function.Name.Name != FilestoreIngressSymbolForTest(reflect.ValueOf(relayCurrentProcess)) || function.Type.Params.NumFields() != 2 || function.Type.Results.NumFields() != 1 {
		return false
	}
	for index, name := range []string{"Streams", "Lent"} {
		parameter := function.Type.Params.List[index]
		typeName, ok := parameter.Type.(*ast.SelectorExpr)
		if !ok || len(parameter.Names) != 1 || !currentRelayProcessSelector(typeName, name) {
			return false
		}
	}
	result, ok := function.Type.Results.List[0].Type.(*ast.Ident)
	if !ok || result.Name != "error" {
		return false
	}
	workers, channels, bounded, sends, receives, selects, defaults, loops, exits := 0, 0, 0, 0, 0, 0, 0, 0, 0
	started, invalid := false, false
	for node := range ast.Preorder(function.Body) {
		switch value := node.(type) {
		case *ast.GoStmt:
			workers++
			started = true
		case *ast.ChanType:
			channels++
		case *ast.SendStmt:
			sends++
		case *ast.SelectStmt:
			selects++
		case *ast.CommClause:
			if value.Comm == nil {
				defaults++
			}
		case *ast.ForStmt, *ast.RangeStmt:
			loops++
		case *ast.UnaryExpr:
			if value.Op == token.ARROW {
				receives++
			}
		case *ast.ReturnStmt:
			if started && !currentRelayTerminalReturn(value) {
				invalid = true
			}
		case *ast.AssignStmt:
			for index, expression := range value.Lhs {
				name, ok := expression.(*ast.Ident)
				if !ok || name.Name != "status" {
					continue
				}
				if index >= len(value.Rhs) {
					invalid = true
					continue
				}
				selector, ok := value.Rhs[index].(*ast.SelectorExpr)
				if !ok || !currentRelayProcessSelector(selector, "ExitSuccess") && !currentRelayProcessSelector(selector, "ExitFailure") {
					invalid = true
				}
			}
		case *ast.CallExpr:
			if selector, ok := value.Fun.(*ast.SelectorExpr); ok && currentRelayProcessSelector(selector, "ExitCurrent") {
				exits++
			}
			name, ok := value.Fun.(*ast.Ident)
			if !ok || name.Name != "make" || len(value.Args) != 2 {
				continue
			}
			channel, ok := value.Args[0].(*ast.ChanType)
			if !ok {
				continue
			}
			item, ok := channel.Value.(*ast.Ident)
			if !ok || item.Name != reflect.TypeFor[relayUpwardResult]().Name() {
				continue
			}
			capacity, ok := value.Args[1].(*ast.BasicLit)
			if ok && capacity.Kind == token.INT && capacity.Value == "1" {
				bounded++
			}
		}
	}
	if len(function.Body.List) == 0 {
		return false
	}
	last, ok := function.Body.List[len(function.Body.List)-1].(*ast.ReturnStmt)
	return ok && currentRelayTerminalReturn(last) && !invalid && workers == 1 && channels == 1 && bounded == 1 && sends == 2 && receives == 2 && selects == 1 && defaults == 1 && loops == 0 && exits == 2
}

func currentRelayProcessSelector(selector *ast.SelectorExpr, name string) bool {
	owner, ok := selector.X.(*ast.Ident)
	return ok && owner.Name == "process" && selector.Sel.Name == name
}

func currentRelayTerminalReturn(statement *ast.ReturnStmt) bool {
	if len(statement.Results) != 1 {
		return false
	}
	call, ok := statement.Results[0].(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !currentRelayProcessSelector(selector, "ExitCurrent") {
		return false
	}
	if status, ok := call.Args[0].(*ast.Ident); ok && status.Name == "status" {
		return true
	}
	status, ok := call.Args[0].(*ast.SelectorExpr)
	return ok && (currentRelayProcessSelector(status, "ExitSuccess") || currentRelayProcessSelector(status, "ExitFailure"))
}

func TestCurrentProcessRelayOwnershipAdmission(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, symbol, parameters, capacity, worker, observation, final string
		want                                                           bool
	}{
		{name: "native scopes with terminal exit", want: true},
		{name: "public borrowed owner is refused", symbol: "RelayBorrowed"},
		{name: "arbitrary stream parameters are refused", parameters: "own io.Reader, lent io.Writer"},
		{name: "unbounded completion is refused", capacity: ""},
		{name: "queue expansion is refused", capacity: ", 2"},
		{name: "ordinary post-start return is refused", final: "return nil"},
		{name: "invalid exit can return and is refused", final: "return process.ExitCurrent(process.ExitStatus(-1))"},
		{name: "callback termination is refused", final: "return callback(status)"},
		{name: "worker pool is refused", worker: "go func(){}(); go func(){}()"},
		{name: "lost known close join is refused", observation: "select { case <-upward: default: }"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			symbol, parameters, capacity := row.symbol, row.parameters, row.capacity
			if symbol == "" {
				symbol = FilestoreIngressSymbolForTest(reflect.ValueOf(relayCurrentProcess))
			}
			if parameters == "" {
				parameters = "own process.Streams, lent process.Lent"
			}
			if capacity == "" && row.name != "unbounded completion is refused" {
				capacity = ", 1"
			}
			worker, observation, final := row.worker, row.observation, row.final
			if worker == "" {
				worker = "go func(){ upward <- relayUpwardResult{}; upward <- relayUpwardResult{} }()"
			}
			if observation == "" {
				observation = "select { case <-upward: _ = <-upward; default: }"
			}
			if final == "" {
				final = "return process.ExitCurrent(status)"
			}
			source := fmt.Sprintf("package filestore\nfunc %s(%s) error { upward := make(chan relayUpwardResult%s); %s; %s; status := process.ExitSuccess; if nativeFailure { status = process.ExitFailure; if diagnosticFailure { return process.ExitCurrent(process.ExitFailure) } }; %s }", symbol, parameters, capacity, worker, observation, final)
			file, err := parser.ParseFile(token.NewFileSet(), "terminal.go", source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if got := ownsCurrentProcessRelay(file.Decls[0]); got != row.want {
				t.Fatalf("terminal relay ownership admission = %t, want %t", got, row.want)
			}
		})
	}
}
