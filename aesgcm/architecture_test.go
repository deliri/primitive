package aesgcm

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type operationRequest[T any] struct{}
type ownedResult[T any] struct{}
type capabilityWrapper[T any] struct{}
type internalCapability[T any] struct{}

type aesgcmDataFlowInventory struct {
	Cipher      capabilityWrapper[Cipher]
	cipherState internalCapability[cipherState]
	SealRequest operationRequest[SealRequest]
	SealResult  ownedResult[SealResult]
	OpenRequest operationRequest[OpenRequest]
	OpenResult  ownedResult[OpenResult]
}

//go:embed *.go
var sources embed.FS

func TestAESGCMDataFlowInventory(t *testing.T) {
	t.Parallel()
	entries, err := sources.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		data, err := sources.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), data, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			gen, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				decl, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := decl.Type.(*ast.StructType); ok {
					got = append(got, decl.Name.Name)
				}
			}
		}
	}
	var want []string
	inventory := reflect.TypeFor[aesgcmDataFlowInventory]()
	for index := range inventory.NumField() {
		want = append(want, inventory.Field(index).Name)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("production structs=%v, want classified %v", got, want)
	}
}
