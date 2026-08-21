package shutdown

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

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
		"sync/atomic",
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

	for name := range shutdownProductionFiles(t) {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", name, err)
		}
		source := string(data)
		for _, forbidden := range []string{
			"os.Exit(",
			"runtime.Goexit(",
			"exec.Command",
			"syscall.Kill(",
			"runtime.NumGoroutine",
			"encoding/json/v2",
			"map[",
			"time.NewTimer(",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains %q, want no process policy or world model",
					name, forbidden)
			}
		}
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

	entries, err := os.ReadDir(".")
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
		file, err := parser.ParseFile(fileSet, name, nil, 0)
		if err != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v", name, err)
		}
		files[name] = file
	}
	return files
}
