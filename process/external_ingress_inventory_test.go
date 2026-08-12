package process_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/process"
)

// processExternalDoorInventory compiles every raw argv and environment door.
// The three Parse functions exercise the three element constructors inside the
// semantic fuzz callbacks in external_ingress_fuzz_test.go.
type processExternalDoorInventory struct {
	NewArgument               func(string) (process.Argument, error)
	NewEnvironmentName        func(string) (process.EnvironmentName, error)
	NewEnvironmentValue       func(string) (process.EnvironmentValue, error)
	ParseArguments            func([]string) ([]process.Argument, error)
	ParseEffectiveEnvironment func([]string) (process.Environment, error)
	ParseExactEnvironment     func([]string) (process.Environment, error)
}

var processExternalDoors = processExternalDoorInventory{
	NewArgument:               process.NewArgument,
	NewEnvironmentName:        process.NewEnvironmentName,
	NewEnvironmentValue:       process.NewEnvironmentValue,
	ParseArguments:            process.ParseArguments,
	ParseEffectiveEnvironment: process.ParseEffectiveEnvironment,
	ParseExactEnvironment:     process.ParseExactEnvironment,
}

func TestProcessExternalIngressInventoryMatchesProduction(t *testing.T) {
	t.Parallel()

	got, gotErr := scanProcessExternalDoors(".")
	if gotErr != nil {
		t.Fatalf("scanProcessExternalDoors() error = %v, want nil", gotErr)
	}
	want := processExternalDoorFieldNames(processExternalDoors)
	if !slices.Equal(got, want) {
		t.Fatalf("Process external argv/environment doors = %q, want compiler inventory %q", got, want)
	}
}

func processExternalDoorFieldNames(inventory any) []string {
	typeOf := reflect.TypeOf(inventory)
	fields := make([]string, 0, typeOf.NumField())
	for index := range typeOf.NumField() {
		fields = append(fields, typeOf.Field(index).Name)
	}
	slices.Sort(fields)
	return fields
}

func scanProcessExternalDoors(root string) ([]string, error) {
	set := token.NewFileSet()
	entries, gotErr := os.ReadDir(root)
	if gotErr != nil {
		return nil, gotErr
	}
	var doors []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			set,
			filepath.Join(root, entry.Name()),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return nil, parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			declaration, ok := node.(*ast.FuncDecl)
			if !ok || declaration.Recv != nil {
				return true
			}
			name := declaration.Name.Name
			if strings.HasPrefix(name, "Parse") || name == "NewArgument" ||
				name == "NewEnvironmentName" || name == "NewEnvironmentValue" {
				doors = append(doors, name)
			}
			return false
		})
	}
	slices.Sort(doors)
	return doors, nil
}
