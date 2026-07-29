package testserial

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

func TestTestserialExactPublicSurfaceAndStandardTestingBoundary(t *testing.T) {
	t.Parallel()

	got, gotErr := scanTestserialArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanTestserialArchitecture() error = %v, want nil", gotErr)
	}
	want := testserialArchitectureScan{
		surface: []string{"func " + core.TestIsolationDeclarationFunctionName},
		imports: []string{
			core.TestIsolationCorePackagePath,
			"testing",
		},
	}
	if !testserialArchitectureScansEqual(got, want) {
		t.Fatalf("Testserial production architecture = %+v, want %+v", got, want)
	}
}

func TestTestserialArchitectureMatcherClassifiesSyntheticBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code string
		want testserialArchitectureScan
	}{
		{
			name: "exported function enters public surface",
			code: "package synthetic\nfunc Declare() {}\n",
			want: testserialArchitectureScan{surface: []string{"func Declare"}},
		},
		{
			name: "private function stays outside public surface",
			code: "package synthetic\nfunc declare() {}\n",
		},
		{
			name: "exported value method enters public surface",
			code: "package synthetic\ntype Declaration struct{}\nfunc (Declaration) Validate() {}\n",
			want: testserialArchitectureScan{
				structs: []string{"Declaration"},
				surface: []string{"method Declaration.Validate", "type Declaration"},
			},
		},
		{
			name: "exported pointer method enters public surface",
			code: "package synthetic\ntype Declaration struct{}\nfunc (*Declaration) Validate() {}\n",
			want: testserialArchitectureScan{
				structs: []string{"Declaration"},
				surface: []string{"method Declaration.Validate", "type Declaration"},
			},
		},
		{
			name: "private receiver method stays outside public surface",
			code: "package synthetic\ntype declaration struct{}\nfunc (declaration) Validate() {}\n",
			want: testserialArchitectureScan{structs: []string{"declaration"}},
		},
		{
			name: "exported nominal type enters public surface",
			code: "package synthetic\ntype Scope uint8\n",
			want: testserialArchitectureScan{surface: []string{"type Scope"}},
		},
		{
			name: "exported alias enters alias and public inventories",
			code: "package synthetic\ntype Alias = uint8\n",
			want: testserialArchitectureScan{
				surface: []string{"type Alias"},
				aliases: []string{"Alias"},
			},
		},
		{
			name: "private alias still enters alias inventory",
			code: "package synthetic\ntype alias = uint8\n",
			want: testserialArchitectureScan{aliases: []string{"alias"}},
		},
		{
			name: "exported constant enters public surface",
			code: "package synthetic\nconst FunctionName = \"Declare\"\n",
			want: testserialArchitectureScan{surface: []string{"const FunctionName"}},
		},
		{
			name: "exported variable enters public surface",
			code: "package synthetic\nvar Scheduler = 1\n",
			want: testserialArchitectureScan{surface: []string{"var Scheduler"}},
		},
		{
			name: "ordinary import enters import inventory",
			code: "package synthetic\nimport \"testing\"\n",
			want: testserialArchitectureScan{imports: []string{"testing"}},
		},
		{
			name: "named import enters alias inventory",
			code: "package synthetic\nimport standard \"testing\"\n",
			want: testserialArchitectureScan{
				imports:       []string{"testing"},
				importAliases: []string{"standard=testing"},
			},
		},
		{
			name: "dot import enters alias inventory",
			code: "package synthetic\nimport . \"testing\"\n",
			want: testserialArchitectureScan{
				imports:       []string{"testing"},
				importAliases: []string{".=testing"},
			},
		},
		{
			name: "struct enters data carrier inventory",
			code: "package synthetic\ntype declaration struct{ scope uint8 }\n",
			want: testserialArchitectureScan{structs: []string{"declaration"}},
		},
		{
			name: "map enters forbidden map inventory",
			code: "package synthetic\nvar declarations map[string]uint8\n",
			want: testserialArchitectureScan{maps: []string{"synthetic.go"}},
		},
		{
			name: "two maps preserve nonvacuous occurrence count",
			code: "package synthetic\nvar first map[string]uint8\nvar second map[uint8]string\n",
			want: testserialArchitectureScan{
				maps: []string{"synthetic.go", "synthetic.go"},
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
			got, gotErr := scanTestserialArchitecture(root)
			if gotErr != nil {
				t.Fatalf("scanTestserialArchitecture(synthetic) error = %v, want nil", gotErr)
			}
			if !testserialArchitectureScansEqual(got, tc.want) {
				t.Fatalf("synthetic Testserial architecture = %+v, want %+v", got, tc.want)
			}
		})
	}
}

type testserialArchitectureScan struct {
	structs       []string
	surface       []string
	aliases       []string
	imports       []string
	importAliases []string
	maps          []string
}

func scanTestserialArchitecture(
	root string,
) (testserialArchitectureScan, error) {
	files, err := testserialProductionGoFiles(root)
	if err != nil {
		return testserialArchitectureScan{}, err
	}
	var scan testserialArchitectureScan
	fileSet := token.NewFileSet()
	for _, name := range files {
		file, parseErr := parser.ParseFile(
			fileSet,
			filepath.Join(root, name),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return testserialArchitectureScan{}, parseErr
		}
		scanTestserialFile(name, file, &scan)
	}
	sortTestserialArchitectureScan(&scan)
	return scan, nil
}

func scanTestserialFile(
	name string,
	file *ast.File,
	scan *testserialArchitectureScan,
) {
	for _, declaration := range file.Imports {
		path, err := strconv.Unquote(declaration.Path.Value)
		if err != nil {
			continue
		}
		if !slices.Contains(scan.imports, path) {
			scan.imports = append(scan.imports, path)
		}
		if declaration.Name != nil {
			scan.importAliases = append(
				scan.importAliases,
				declaration.Name.Name+"="+path,
			)
		}
	}
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			scanTestserialFunction(typed, scan)
		case *ast.GenDecl:
			scanTestserialDeclaration(typed, scan)
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if _, ok := node.(*ast.MapType); ok {
			scan.maps = append(scan.maps, name)
		}
		return true
	})
}

func scanTestserialFunction(
	function *ast.FuncDecl,
	scan *testserialArchitectureScan,
) {
	if !function.Name.IsExported() {
		return
	}
	if function.Recv == nil {
		scan.surface = append(scan.surface, "func "+function.Name.Name)
		return
	}
	receiver := testserialReceiverName(function.Recv.List[0].Type)
	if ast.IsExported(receiver) {
		scan.surface = append(
			scan.surface,
			"method "+receiver+"."+function.Name.Name,
		)
	}
}

func scanTestserialDeclaration(
	declaration *ast.GenDecl,
	scan *testserialArchitectureScan,
) {
	for _, raw := range declaration.Specs {
		switch spec := raw.(type) {
		case *ast.TypeSpec:
			if spec.Assign.IsValid() {
				scan.aliases = append(scan.aliases, spec.Name.Name)
			}
			if _, ok := spec.Type.(*ast.StructType); ok {
				scan.structs = append(scan.structs, spec.Name.Name)
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

func testserialReceiverName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return testserialReceiverName(typed.X)
	default:
		return ""
	}
}

func sortTestserialArchitectureScan(scan *testserialArchitectureScan) {
	slices.Sort(scan.structs)
	slices.Sort(scan.surface)
	slices.Sort(scan.aliases)
	slices.Sort(scan.imports)
	slices.Sort(scan.importAliases)
	slices.Sort(scan.maps)
}

func testserialArchitectureScansEqual(
	got testserialArchitectureScan,
	want testserialArchitectureScan,
) bool {
	return slices.Equal(got.structs, want.structs) &&
		slices.Equal(got.surface, want.surface) &&
		slices.Equal(got.aliases, want.aliases) &&
		slices.Equal(got.imports, want.imports) &&
		slices.Equal(got.importAliases, want.importAliases) &&
		slices.Equal(got.maps, want.maps)
}

func testserialProductionGoFiles(root string) ([]string, error) {
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
