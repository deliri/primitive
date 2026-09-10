package filelock

import (
	"context"
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

//go:embed *.go
var filelockSource embed.FS

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
	files, gotGlobErr := fs.Glob(filelockSource, "*.go")
	if gotGlobErr != nil {
		return nil, gotGlobErr
	}
	set := token.NewFileSet()
	var names []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, readErr := filelockSource.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		file, gotParseErr := parser.ParseFile(set, path, data, parser.SkipObjectResolution)
		if gotParseErr != nil {
			return nil, gotParseErr
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				specification, ok := raw.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := specification.Type.(*ast.StructType); ok {
					names = append(names, specification.Name.Name)
				}
			}
		}
	}
	slices.Sort(names)
	return names, nil
}

// The typed inventory binds the externally effectful API to fuzz coverage.
type filelockDoorInventory struct {
	Acquire func(context.Context, Request) (Acquisition, error)
	Release func(context.Context, *os.File) error
}

var filelockDoors = filelockDoorInventory{Acquire: Acquire, Release: Release}

func TestFilelockExternalDoorInventory(t *testing.T) {
	t.Parallel()
	entries, err := fs.Glob(filelockSource, "*.go")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := filelockSource.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		source, err := parser.ParseFile(token.NewFileSet(), path, data, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range source.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() {
				continue
			}
			got = append(got, fn.Name.Name)
		}
	}
	typ := reflect.TypeOf(filelockDoors)
	want := make([]string, 0, typ.NumField())
	for field := range typ.Fields() {
		want = append(want, field.Name)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("external doors=%v, want fuzz-covered %v", got, want)
	}
}
