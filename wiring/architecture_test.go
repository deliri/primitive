package wiring

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

type wiringDataFlowRole uint8

const (
	wiringDataFlowRoleUnknown wiringDataFlowRole = iota
	wiringDataFlowRoleGraphIngress
	wiringDataFlowRoleSealedProjection
	wiringDataFlowRoleTypedError
	wiringDataFlowRoleInternalFlow
	wiringDataFlowRoleLimit
)

func (r wiringDataFlowRole) IsValid() bool {
	return r > wiringDataFlowRoleUnknown && r < wiringDataFlowRoleLimit
}

type wiringGraphIngressInventory struct {
	Definition Definition[wiringTestIdentity]
	Request    Request[wiringTestIdentity]
}

type wiringSealedProjectionInventory struct {
	Manifest Manifest[wiringTestIdentity]
}

type wiringTypedErrorInventory struct {
	ContractError ContractError[wiringTestIdentity]
}

type wiringInternalFlowInventory struct {
	contractErrorRequest contractErrorRequest[wiringTestIdentity]
}

var _ = wiringInternalFlowInventory{}.contractErrorRequest

type wiringRoleInventory struct {
	fields reflect.Type
	role   wiringDataFlowRole
}

func TestProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	got, gotErr := wiringProductionStructNames()
	if gotErr != nil {
		t.Fatalf("wiringProductionStructNames() error = %v, want nil", gotErr)
	}
	want := wiringInventoryStructNames(t)
	if !slices.Equal(got, want) {
		t.Fatalf("production struct inventory = %v, want exact compiler-visible roles %v", got, want)
	}
}

func wiringInventoryStructNames(t testing.TB) []string {
	t.Helper()

	inventories := []wiringRoleInventory{
		{role: wiringDataFlowRoleGraphIngress, fields: reflect.TypeFor[wiringGraphIngressInventory]()},
		{role: wiringDataFlowRoleSealedProjection, fields: reflect.TypeFor[wiringSealedProjectionInventory]()},
		{role: wiringDataFlowRoleTypedError, fields: reflect.TypeFor[wiringTypedErrorInventory]()},
		{role: wiringDataFlowRoleInternalFlow, fields: reflect.TypeFor[wiringInternalFlowInventory]()},
	}
	var names []string
	for _, inventory := range inventories {
		if !inventory.role.IsValid() {
			t.Fatalf("wiring data-flow role = %d, want admitted role", inventory.role)
		}
		for field := range inventory.fields.Fields() {
			if slices.Contains(names, field.Name) {
				t.Fatalf("wiring data-flow inventory duplicates %s, want one owner", field.Name)
			}
			names = append(names, field.Name)
		}
	}
	slices.Sort(names)
	return names
}

func wiringProductionStructNames() ([]string, error) {
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
