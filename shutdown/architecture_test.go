package shutdown

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

//go:embed *.go
var shutdownSources embed.FS

type (
	shutdownProtocolFact[T any] struct{}
	shutdownPolicy[T any]       struct{}
	shutdownObservation[T any]  struct{}
	shutdownCapability[T any]   struct{}
	shutdownInternalFlow[T any] struct{}
	shutdownTypedFailure[T any] struct{}
)

type shutdownContractInventory struct {
	StepPanicError     shutdownTypedFailure[StepPanicError]
	ContextPanicError  shutdownTypedFailure[ContextPanicError]
	StepID             shutdownProtocolFact[StepID]
	Step               shutdownProtocolFact[Step]
	PlanPolicy         shutdownPolicy[PlanPolicy]
	Plan               shutdownCapability[Plan]
	StepResult         shutdownObservation[StepResult]
	Report             shutdownObservation[Report]
	actionResult       shutdownInternalFlow[actionResult]
	stepClassification shutdownInternalFlow[stepClassification]
	SignalPolicy       shutdownPolicy[SignalPolicy]
	Escalation         shutdownObservation[Escalation]
	SignalCause        shutdownObservation[SignalCause]
	WatchRequest       shutdownProtocolFact[WatchRequest]
	signalSource       shutdownCapability[signalSource]
	Controller         shutdownCapability[Controller]
	escalationWait     shutdownInternalFlow[escalationWait]
}

var (
	_ = shutdownContractInventory{}.actionResult
	_ = shutdownContractInventory{}.stepClassification
	_ = shutdownContractInventory{}.signalSource
	_ = shutdownContractInventory{}.escalationWait
)

func TestProductionArchitectureHasOneOwnedGoroutineAndExactImports(t *testing.T) {
	t.Parallel()

	files := shutdownProductionFiles(t)
	imports := make(map[string]struct{})
	var goroutines []string
	for name, file := range files {
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("strconv.Unquote(%s) error = %v", imported.Path.Value, err)
			}
			imports[path] = struct{}{}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if _, ok := node.(*ast.GoStmt); ok {
				goroutines = append(goroutines, name)
			}
			return true
		})
	}
	gotImports := make([]string, 0, len(imports))
	for path := range imports {
		gotImports = append(gotImports, path)
	}
	sort.Strings(gotImports)
	wantImports := []string{
		"context",
		"errors",
		"fmt",
		"github.com/deliri/primitive/v2026/contextstate",
		"github.com/deliri/primitive/v2026/core",
		"github.com/deliri/primitive/v2026/temporal",
		"os",
		"os/signal",
		"strconv",
		"strings",
		"sync",
		"syscall",
		"unicode/utf8",
	}
	if !slices.Equal(gotImports, wantImports) {
		t.Fatalf("production imports = %v, want %v", gotImports, wantImports)
	}
	if len(goroutines) != 1 || goroutines[0] != "signal.go" {
		t.Fatalf("production goroutines = %v, want [signal.go]", goroutines)
	}
}

func TestProductionRejectsProcessExitAndWorldBuildingRatchet(t *testing.T) {
	t.Parallel()
	for name, file := range shutdownProductionFiles(t) {
		if got := shutdownForbiddenSyntax(file); len(got) != 0 {
			t.Fatalf("%s forbidden syntax = %v, want none", name, got)
		}
	}
}

func shutdownForbiddenSyntax(file *ast.File) []string {
	imports := make(map[string]string)
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = path
	}
	var forbidden []string
	ast.Inspect(file, func(node ast.Node) bool {
		if _, ok := node.(*ast.MapType); ok {
			forbidden = append(forbidden, "map type")
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		owner, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		qualified := imports[owner.Name] + "." + selector.Sel.Name
		if slices.Contains([]string{"os.Exit", "runtime.Goexit", "os/exec.Command", "os/exec.CommandContext", "syscall.Kill", "runtime.NumGoroutine", "time.NewTimer"}, qualified) {
			forbidden = append(forbidden, qualified)
		}
		return true
	})
	return forbidden
}

func TestShutdownForbiddenSyntaxMatcherIgnoresProseAndResolvesAliases(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		want         []string
	}{
		{name: "aliased process exit is still policy", source: `package p; import system "os";func f(){system.Exit(0)}`, want: []string{"os.Exit"}},
		{name: "map declaration is still an unbounded collection", source: `package p;type X map[string]int`, want: []string{"map type"}},
		{name: "direct timer bypasses Temporal", source: `package p;import clock "time";func f(){clock.NewTimer(1)}`, want: []string{"time.NewTimer"}},
		{name: "documentation does not execute effects", source: `package p;const note="os.Exit(0) and map[string]int"`},
		{name: "unrelated method spelling is not a standard library effect", source: `package p;type owner struct{};func f(x owner){x.Exit(0)}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), tc.name, tc.source, 0)
			if err != nil {
				t.Fatalf("fixture parse = %v, want nil", err)
			}
			if got := shutdownForbiddenSyntax(file); !slices.Equal(got, tc.want) {
				t.Fatalf("forbidden syntax = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShutdownProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	var production []string
	for _, file := range shutdownProductionFiles(t) {
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, specification := range general.Specs {
				named, ok := specification.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := named.Type.(*ast.StructType); ok {
					production = append(production, named.Name.Name)
				}
			}
		}
	}
	slices.Sort(production)

	inventoryType := reflect.TypeFor[shutdownContractInventory]()
	inventory := make([]string, 0, inventoryType.NumField())
	for field := range inventoryType.Fields() {
		inventory = append(inventory, field.Name)
	}
	slices.Sort(inventory)
	if !slices.Equal(production, inventory) {
		t.Fatalf("shutdown production structs = %v, compiler inventory = %v", production, inventory)
	}
}

func shutdownProductionFiles(t *testing.T) map[string]*ast.File {
	t.Helper()

	entries, err := shutdownSources.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(.) error = %v", err)
	}
	files := make(map[string]*ast.File)
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := shutdownSources.ReadFile(name)
		if err != nil {
			t.Fatalf("source %s = %v, want embedded bytes", name, err)
		}
		file, err := parser.ParseFile(fileSet, name, data, 0)
		if err != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v", name, err)
		}
		files[name] = file
	}
	return files
}

func TestShutdownExternalIngressHasSemanticFuzzInventory(t *testing.T) {
	t.Parallel()
	// Off-wire enums have no byte decoder. Their entire byte domains are
	// exhausted separately. These functions admit caller-owned nominal values,
	// callbacks or provider signals; each has a semantic fuzz oracle.
	inventory := []struct {
		name   string
		target func(*testing.F)
	}{
		{name: "NewStepID", target: FuzzStepIDNominalIngress},
		{name: "NewPlan", target: FuzzPlanRegistrationAndExecution},
		{name: "Watch", target: FuzzWatchPolicyAndSourceClosure},
	}
	var constructors []string
	for _, file := range shutdownProductionFiles(t) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			name := fn.Name.Name
			if !ast.IsExported(name) {
				continue
			}
			if strings.HasPrefix(name, "New") || name == "Watch" {
				constructors = append(constructors, name)
			}
			for _, prefix := range []string{"Parse", "Decode", "Read", "Load", "Replay", "Unmarshal"} {
				if strings.HasPrefix(name, prefix) {
					t.Fatalf("new external decoder %s has no semantic fuzz inventory, want explicit coverage", name)
				}
			}
		}
	}
	want := make([]string, 0, len(inventory))
	for _, entry := range inventory {
		if entry.target == nil {
			t.Fatalf("fuzz target for %s = nil, want compiled oracle", entry.name)
		}
		want = append(want, entry.name)
	}
	slices.Sort(constructors)
	slices.Sort(want)
	if !slices.Equal(constructors, want) {
		t.Fatalf("external constructors = %v, want fuzz inventory %v", constructors, want)
	}
}
