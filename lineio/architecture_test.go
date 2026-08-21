package lineio_test

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

const lineioProductionDirectoryEntryMaximum uint32 = 64

type (
	lineioRequestContract[T any]    struct{}
	lineioCapabilityContract[T any] struct{}
)

// lineioContractInventory classifies every production carrier by its actual
// data-flow role. It is a compiler-visible wiring ratchet, not behavior proof.
type lineioContractInventory struct {
	BufferPolicy lineioRequestContract[lineio.BufferPolicy]
	Request      lineioRequestContract[lineio.Request]
	Scanner      lineioCapabilityContract[lineio.Scanner]
}

var (
	_ core.Validatable = lineio.BufferPolicy{}
	_ core.Validatable = lineio.Request{}
	_ core.Validatable = (*lineio.Scanner)(nil)
	_                  = core.DirectImportContract{Importer: core.PackageLineIO, Imported: core.PackageCore}
)

func TestLineIOProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	got, err := lineioProductionStructNames(t.Context())
	if err != nil {
		t.Fatalf("lineioProductionStructNames() error = %v, want nil", err)
	}
	want := lineioClassifiedStructNames()
	if !slices.Equal(got, want) {
		t.Fatalf("Lineio production structs = %q, want classified %q", got, want)
	}
}

func lineioProductionStructNames(ctx context.Context) (names []string, resultErr error) {
	_, sourceFilename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, core.ErrPrimitiveContract
	}
	sourcePath, err := core.ParseAbsolutePath(sourceFilename)
	if err != nil {
		return nil, err
	}
	directoryPath, err := sourcePath.Parent()
	if err != nil {
		return nil, err
	}
	root, err := filestore.OpenRoot(ctx, directoryPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, root.Close())
	}()
	locationPath, err := core.ParseRelativePath(".")
	if err != nil {
		return nil, err
	}
	entryMaximum, err := filestore.NewDirectoryEntryMaximum(lineioProductionDirectoryEntryMaximum)
	if err != nil {
		return nil, err
	}
	files := token.NewFileSet()
	walkErr := filestore.Walk(ctx, filestore.WalkRequest{
		Location:              filestore.Location{Root: root, Path: locationPath},
		Order:                 filestore.WalkOrderLexical,
		DirectoryEntryMaximum: entryMaximum,
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if entry.Entry.IsDir() {
				return filestore.WalkSkipDirectory, nil
			}
			name := entry.Entry.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return filestore.WalkContinue, nil
			}
			file, openErr := filestore.OpenRead(ctx, filestore.ReadHandleRequest{
				Location: filestore.Location{Root: root, Path: entry.Path},
			})
			if openErr != nil {
				return filestore.WalkContinue, openErr
			}
			parsed, parseErr := parser.ParseFile(files, entry.Path.String(), file, parser.SkipObjectResolution)
			closeErr := file.Close()
			if err := errors.Join(parseErr, closeErr); err != nil {
				return filestore.WalkContinue, err
			}
			names = append(names, productionStructNames(parsed)...)
			return filestore.WalkContinue, nil
		},
	})
	if walkErr != nil {
		return nil, walkErr
	}
	slices.Sort(names)
	return names, nil
}

func productionStructNames(file *ast.File) []string {
	var names []string
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			spec := raw.(*ast.TypeSpec)
			if _, ok := spec.Type.(*ast.StructType); ok {
				names = append(names, spec.Name.Name)
			}
		}
	}
	return names
}

func lineioClassifiedStructNames() []string {
	contract := reflect.TypeFor[lineioContractInventory]()
	names := make([]string, contract.NumField())
	for index := range contract.NumField() {
		names[index] = contract.Field(index).Name
	}
	slices.Sort(names)
	return names
}
