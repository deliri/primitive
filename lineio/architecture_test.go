package lineio_test

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/lineio"
)

type (
	lineioRequestContract[T any]     struct{}
	lineioCapabilityContract[T any]  struct{}
	lineioObservationContract[T any] struct{}
	lineioReadCountGuardContract     struct{}
)

// lineioContractInventory classifies every production carrier by its actual
// data-flow role. It is a compiler-visible wiring ratchet, not behavior proof.
type lineioContractInventory struct {
	LastRecordRangeRequest   lineioRequestContract[lineio.LastRecordRangeRequest]
	RecordRangeObservation   lineioObservationContract[lineio.RecordRangeObservation]
	LiteralReplacement       lineioRequestContract[lineio.LiteralReplacement]
	ReplaceStreamRequest     lineioRequestContract[lineio.ReplaceStreamRequest]
	ReplaceStreamObservation lineioObservationContract[lineio.ReplaceStreamObservation]
	RecordRange              lineioRequestContract[lineio.RecordRange]
	RecordCharacter          lineioRequestContract[lineio.RecordCharacter]
	RecordReadRequest        lineioRequestContract[lineio.RecordReadRequest]
	RecordRangeReadRequest   lineioRequestContract[lineio.RecordRangeReadRequest]
	RecordRangeSource        lineioReadCountGuardContract `source:"recordRangeSource"`
	RecordReadObservation    lineioObservationContract[lineio.RecordReadObservation]
	RecordRangeRequest       lineioRequestContract[lineio.RecordRangeRequest]
	RecordFragmentRequest    lineioRequestContract[lineio.RecordFragmentRequest]
	RecordSuffixTrimRequest  lineioRequestContract[lineio.RecordSuffixTrimRequest]
	DelimitedFragment        lineioRequestContract[lineio.DelimitedFragment]
	DelimitedReader          lineioCapabilityContract[lineio.DelimitedReader]
	Character                lineioRequestContract[lineio.Character]
	CharacterPosition        lineioRequestContract[lineio.CharacterPosition]
	CharacterRequest         lineioRequestContract[lineio.CharacterRequest]
	CharacterSource          lineioReadCountGuardContract `source:"characterSource"`
	Fragment                 lineioRequestContract[lineio.Fragment]
	Request                  lineioRequestContract[lineio.Request]
	Reader                   lineioCapabilityContract[lineio.Reader]
	RangeReader              lineioCapabilityContract[lineio.RangeReader]
	RecordRangeReadGuard     lineioReadCountGuardContract `source:"recordRangeReadGuard"`
	RecordFragmentReader     lineioReadCountGuardContract `source:"recordFragmentReader"`
	CheckedReader            lineioReadCountGuardContract `source:"checkedReader"`
	RecordReadSource         lineioReadCountGuardContract `source:"recordReadSource"`
}

var (
	_ core.Validatable = lineio.Fragment{}
	_ core.Validatable = lineio.Request{}
	_ core.Validatable = (*lineio.Reader)(nil)
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
	// go test runs a package's tests in its own directory; -trimpath leaves
	// runtime.Caller only module-relative paths.
	directoryPath, err := hostfacts.WorkingDirectory()
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
	files := token.NewFileSet()
	walkErr := filestore.Walk(ctx, filestore.WalkRequest{
		Location: filestore.Location{Root: root, Path: locationPath},
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
		field := contract.Field(index)
		names[index] = field.Tag.Get("source")
		if names[index] == "" {
			names[index] = field.Name
		}
	}
	slices.Sort(names)
	return names
}

func TestProductionStructMatcherLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, source string
		want         []string
	}{
		{name: "grouped exported and private carriers are inventoried", source: "package fixture; type (Public struct{ Value int }; private struct{})", want: []string{"Public", "private"}},
		{name: "new hidden carrier cannot escape beside ordinary declarations", source: "package fixture; const Value=1; func F(){}; type hidden struct{}", want: []string{"hidden"}},
		{name: "scalar and interface declarations do not invent carriers", source: "package fixture; type Count int; type Reader interface{ Read([]byte)(int,error) }"},
		{name: "function local struct is not a package data carrier", source: "package fixture; func F(){ type local struct{}; _=local{} }"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parsed, err := parser.ParseFile(token.NewFileSet(), "fixture.go", tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("ParseFile() = %v, want nil", err)
			}
			got := productionStructNames(parsed)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("productionStructNames() = %q, want %q", got, tc.want)
			}
		})
	}
}
