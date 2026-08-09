package id

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type (
	operationRequest[T any] struct{}
	wireValue[T any]        struct{}
)

// idContractInventory classifies every production struct by its actual
// data-flow role. Both identities are wire values: they render to one
// canonical text a peer or a persisted document carries.
type idContractInventory struct {
	Request operationRequest[Request]
	UUIDv7  wireValue[UUIDv7]
	ULID    wireValue[ULID]
}

var _ = idContractInventory{}

func TestIDProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanIDArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanIDArchitecture() error = %v, want nil", gotErr)
	}
	wantClassified, gotClassifiedErr := classifiedIDStructs()
	if gotClassifiedErr != nil {
		t.Fatalf("classifiedIDStructs() error = %v, want nil", gotClassifiedErr)
	}
	if !slices.Equal(gotScan.structs, wantClassified) {
		t.Fatalf("ID production structs = %q, want classified %q", gotScan.structs, wantClassified)
	}
}

func TestIDExactPublicSurfaceFieldsAndNoAliases(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanIDArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanIDArchitecture() error = %v, want nil", gotErr)
	}
	wantSurface := []string{
		"func NewULID",
		"func NewUUIDv7",
		"func ParseULID",
		"func ParseUUIDv7",
		"method Request.Validate",
		"method ULID.AppendText",
		"method ULID.IsValid",
		"method ULID.IsZero",
		"method ULID.MarshalJSON",
		"method ULID.String",
		"method ULID.UnmarshalJSON",
		"method ULID.Validate",
		"method UUIDv7.AppendText",
		"method UUIDv7.IsValid",
		"method UUIDv7.IsZero",
		"method UUIDv7.MarshalJSON",
		"method UUIDv7.String",
		"method UUIDv7.UnmarshalJSON",
		"method UUIDv7.Validate",
		"type Request",
		"type ULID",
		"type UUIDv7",
	}
	wantFields := []string{"Request.Entropy", "Request.Observation"}
	if !slices.Equal(gotScan.surface, wantSurface) {
		t.Fatalf("ID public surface = %q, want %q", gotScan.surface, wantSurface)
	}
	if !slices.Equal(gotScan.exportedFields, wantFields) {
		t.Fatalf("ID exported struct fields = %q, want %q", gotScan.exportedFields, wantFields)
	}
	if len(gotScan.aliases) != 0 {
		t.Fatalf("ID production type aliases = %q, want none", gotScan.aliases)
	}
}

// TestIDProductionImportsProvePureValueConstruction pins the whole point of
// the package: no clock, no entropy source, no effect substrate of any kind
// can be reached from production code, because the import set cannot express
// one.
func TestIDProductionImportsProvePureValueConstruction(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanIDArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanIDArchitecture() error = %v, want nil", gotErr)
	}
	wantImports := []string{
		"encoding",
		"encoding/binary",
		"encoding/hex",
		"errors",
		"github.com/deliri/primitive/v2026/core",
		"github.com/deliri/primitive/v2026/temporal",
		"math",
		"strings",
	}
	if !slices.Equal(gotScan.imports, wantImports) {
		t.Fatalf("ID production imports = %q, want %q", gotScan.imports, wantImports)
	}
	if len(gotScan.importAliases) != 0 {
		t.Fatalf("ID production import aliases = %q, want none", gotScan.importAliases)
	}
}

func TestIDProductionOwnsNoMapBasedProtocolOrState(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanIDArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanIDArchitecture() error = %v, want nil", gotErr)
	}
	if len(gotScan.maps) != 0 {
		t.Fatalf("ID production map types = %q, want none", gotScan.maps)
	}
}

func TestIDArchitectureMatcherClassifiesSyntheticBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code string
		want idArchitectureScan
	}{
		{
			name: "exported function enters public surface",
			code: "package synthetic\nfunc Mint() {}\n",
			want: idArchitectureScan{surface: []string{"func Mint"}},
		},
		{
			name: "private function stays outside public surface",
			code: "package synthetic\nfunc mint() {}\n",
		},
		{
			name: "exported value receiver method enters public surface",
			code: "package synthetic\ntype Request struct{}\nfunc (Request) Validate() {}\n",
			want: idArchitectureScan{
				structs: []string{"Request"},
				surface: []string{"method Request.Validate", "type Request"},
			},
		},
		{
			name: "exported pointer receiver method enters public surface",
			code: "package synthetic\ntype Value struct{}\nfunc (*Value) UnmarshalJSON() {}\n",
			want: idArchitectureScan{
				structs: []string{"Value"},
				surface: []string{"method Value.UnmarshalJSON", "type Value"},
			},
		},
		{
			name: "method on private receiver stays outside public surface",
			code: "package synthetic\ntype request struct{}\nfunc (request) Validate() {}\n",
			want: idArchitectureScan{structs: []string{"request"}},
		},
		{
			name: "exported alias is classified and enters public surface",
			code: "package synthetic\ntype Alias = uint8\n",
			want: idArchitectureScan{
				surface: []string{"type Alias"},
				aliases: []string{"Alias"},
			},
		},
		{
			name: "exported constant enters public surface",
			code: "package synthetic\nconst Maximum = 1\n",
			want: idArchitectureScan{surface: []string{"const Maximum"}},
		},
		{
			name: "exported struct field is inventoried",
			code: "package synthetic\ntype Request struct{ Size uint64 }\n",
			want: idArchitectureScan{
				structs:        []string{"Request"},
				surface:        []string{"type Request"},
				exportedFields: []string{"Request.Size"},
			},
		},
		{
			name: "named import alias enters alias inventory",
			code: "package synthetic\nimport clock \"time\"\n",
			want: idArchitectureScan{
				imports:       []string{"time"},
				importAliases: []string{"clock=time"},
			},
		},
		{
			name: "map field enters forbidden map inventory",
			code: "package synthetic\ntype request struct{ values map[string]string }\n",
			want: idArchitectureScan{
				structs: []string{"request"},
				maps:    []string{"synthetic.go"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			path := filepath.Join(root, "synthetic.go")
			if gotErr := os.WriteFile(path, []byte(tc.code), 0o600); gotErr != nil {
				t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, gotErr)
			}
			got, gotErr := scanIDArchitecture(root)
			if gotErr != nil {
				t.Fatalf("scanIDArchitecture(synthetic) error = %v, want nil", gotErr)
			}
			if !idArchitectureScansEqual(got, tc.want) {
				t.Fatalf("synthetic architecture scan = %+v, want %+v", got, tc.want)
			}
		})
	}
}

type idArchitectureScan struct {
	structs        []string
	surface        []string
	exportedFields []string
	aliases        []string
	imports        []string
	importAliases  []string
	maps           []string
}

func scanIDArchitecture(root string) (idArchitectureScan, error) {
	files, err := idProductionGoFiles(root)
	if err != nil {
		return idArchitectureScan{}, err
	}
	var scan idArchitectureScan
	fileSet := token.NewFileSet()
	for _, name := range files {
		file, parseErr := parser.ParseFile(
			fileSet,
			filepath.Join(root, name),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return idArchitectureScan{}, parseErr
		}
		scanIDFile(name, file, &scan)
	}
	sortIDArchitectureScan(&scan)
	return scan, nil
}

func scanIDFile(name string, file *ast.File, scan *idArchitectureScan) {
	scanIDImports(file, scan)
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			scanIDFunction(typed, scan)
		case *ast.GenDecl:
			scanIDDeclaration(typed, scan)
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if _, ok := node.(*ast.MapType); ok {
			scan.maps = append(scan.maps, name)
		}
		return true
	})
}

func scanIDImports(file *ast.File, scan *idArchitectureScan) {
	for _, declaration := range file.Imports {
		path, err := strconv.Unquote(declaration.Path.Value)
		if err != nil {
			continue
		}
		if !slices.Contains(scan.imports, path) {
			scan.imports = append(scan.imports, path)
		}
		if declaration.Name != nil {
			scan.importAliases = append(scan.importAliases, declaration.Name.Name+"="+path)
		}
	}
}

func scanIDFunction(function *ast.FuncDecl, scan *idArchitectureScan) {
	if !function.Name.IsExported() {
		return
	}
	if function.Recv == nil {
		scan.surface = append(scan.surface, "func "+function.Name.Name)
		return
	}
	receiver := idReceiverName(function.Recv.List[0].Type)
	if ast.IsExported(receiver) {
		scan.surface = append(scan.surface, "method "+receiver+"."+function.Name.Name)
	}
}

func scanIDDeclaration(declaration *ast.GenDecl, scan *idArchitectureScan) {
	for _, raw := range declaration.Specs {
		switch spec := raw.(type) {
		case *ast.TypeSpec:
			if spec.Assign.IsValid() {
				scan.aliases = append(scan.aliases, spec.Name.Name)
			}
			structure, isStruct := spec.Type.(*ast.StructType)
			if isStruct {
				scan.structs = append(scan.structs, spec.Name.Name)
				scanIDExportedFields(spec.Name.Name, structure, scan)
			}
			if spec.Name.IsExported() {
				scan.surface = append(scan.surface, "type "+spec.Name.Name)
			}
		case *ast.ValueSpec:
			for _, name := range spec.Names {
				if name.IsExported() {
					scan.surface = append(
						scan.surface,
						strings.ToLower(declaration.Tok.String())+" "+name.Name,
					)
				}
			}
		}
	}
}

func scanIDExportedFields(
	typeName string,
	structure *ast.StructType,
	scan *idArchitectureScan,
) {
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			if name.IsExported() {
				scan.exportedFields = append(scan.exportedFields, typeName+"."+name.Name)
			}
		}
	}
}

func idReceiverName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return idReceiverName(typed.X)
	default:
		return ""
	}
}

func sortIDArchitectureScan(scan *idArchitectureScan) {
	slices.Sort(scan.structs)
	slices.Sort(scan.surface)
	slices.Sort(scan.exportedFields)
	slices.Sort(scan.aliases)
	slices.Sort(scan.imports)
	slices.Sort(scan.importAliases)
	slices.Sort(scan.maps)
}

func idArchitectureScansEqual(got idArchitectureScan, want idArchitectureScan) bool {
	return slices.Equal(got.structs, want.structs) &&
		slices.Equal(got.surface, want.surface) &&
		slices.Equal(got.exportedFields, want.exportedFields) &&
		slices.Equal(got.aliases, want.aliases) &&
		slices.Equal(got.imports, want.imports) &&
		slices.Equal(got.importAliases, want.importAliases) &&
		slices.Equal(got.maps, want.maps)
}

func idProductionGoFiles(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		files = append(files, entry.Name())
	}
	slices.Sort(files)
	return files, nil
}

func classifiedIDStructs() ([]string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(
		fileSet,
		"architecture_test.go",
		nil,
		parser.SkipObjectResolution,
	)
	if err != nil {
		return nil, err
	}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			spec := raw.(*ast.TypeSpec)
			if spec.Name.Name != "idContractInventory" {
				continue
			}
			structure, ok := spec.Type.(*ast.StructType)
			if !ok {
				return nil, core.ErrIDContract
			}
			var names []string
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					names = append(names, name.Name)
				}
			}
			slices.Sort(names)
			return names, nil
		}
	}
	return nil, core.ErrIDContract
}
