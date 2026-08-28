package filelock

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type filelockDataFlowRole uint8

const (
	filelockDataFlowRoleUnknown filelockDataFlowRole = iota
	filelockDataFlowRoleEffectIngress
	filelockDataFlowRoleSealedObservation
	filelockDataFlowRoleLimit
)

func (r filelockDataFlowRole) IsValid() bool {
	return r > filelockDataFlowRoleUnknown && r < filelockDataFlowRoleLimit
}

type filelockEffectIngressInventory struct {
	Request Request
}

type filelockSealedObservationInventory struct {
	Acquisition Acquisition
}

type filelockRoleInventory struct {
	fields reflect.Type
	role   filelockDataFlowRole
}

func TestFilelockProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	got, gotErr := filelockProductionStructNames()
	if gotErr != nil {
		t.Fatalf("filelockProductionStructNames() error = %v, want nil", gotErr)
	}
	want := filelockInventoryStructNames(t)
	if !slices.Equal(got, want) {
		t.Fatalf("production struct inventory = %v, want exact compiler-visible roles %v", got, want)
	}
}

func filelockInventoryStructNames(t testing.TB) []string {
	t.Helper()

	inventories := []filelockRoleInventory{
		{role: filelockDataFlowRoleEffectIngress, fields: reflect.TypeFor[filelockEffectIngressInventory]()},
		{role: filelockDataFlowRoleSealedObservation, fields: reflect.TypeFor[filelockSealedObservationInventory]()},
	}
	var names []string
	for _, inventory := range inventories {
		if !inventory.role.IsValid() {
			t.Fatalf("filelock data-flow role = %d, want admitted role", inventory.role)
		}
		for field := range inventory.fields.Fields() {
			if slices.Contains(names, field.Name) {
				t.Fatalf("filelock data-flow inventory duplicates %s, want one owner", field.Name)
			}
			names = append(names, field.Name)
		}
	}
	slices.Sort(names)
	return names
}

func filelockProductionStructNames() ([]string, error) {
	files, gotGlobErr := filepath.Glob("*.go")
	if gotGlobErr != nil {
		return nil, gotGlobErr
	}
	set := token.NewFileSet()
	var names []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, gotParseErr := parser.ParseFile(set, path, nil, parser.SkipObjectResolution)
		if gotParseErr != nil {
			return nil, gotParseErr
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				specification := raw.(*ast.TypeSpec)
				if _, ok := specification.Type.(*ast.StructType); ok {
					names = append(names, specification.Name.Name)
				}
			}
		}
	}
	slices.Sort(names)
	return names, nil
}
