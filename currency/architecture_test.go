package currency

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
	currencyProtocolFact[T any]   struct{}
	currencySealedValue[T any]    struct{}
	currencyWireProjection[T any] struct{}
	currencyDefinitionFact[T any] struct{}
	currencyInternalScalar[T any] struct{}
)

// currencyContractInventory classifies every production struct by its actual
// data-flow role. Currency owns no persistence carrier or mutable capability.
type currencyContractInventory struct {
	CodeDefinition currencyDefinitionFact[currencyDefinition]
	Amount         currencySealedValue[Amount]
	MinorUnitsJSON currencyInternalScalar[minorUnitsJSON]
	AmountWire     currencyWireProjection[amountWire]
}

var (
	_                                               = currencyProtocolFact[Code]{}
	_                                               = currencyContractInventory{}
	_ func(Amount, Amount) (core.Comparison, error) = Amount.Compare
)

func TestCurrencyProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanCurrencyArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanCurrencyArchitecture() error = %v, want nil", gotErr)
	}
	want := []string{"Amount", "amountWire", "currencyDefinition", "minorUnitsJSON"}
	if !slices.Equal(gotScan.structs, want) {
		t.Fatalf("Currency production structs = %q, want classified %q", gotScan.structs, want)
	}
}

func TestCurrencyExactPublicSurfaceAndNoAliases(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanCurrencyArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanCurrencyArchitecture() error = %v, want nil", gotErr)
	}
	wantSurface := []string{
		"const AmountCanonicalJSONMaximumBytes",
		"const AmountJSONDocumentSlackBytes",
		"const AmountJSONMaximumBytes",
		"const CodeAUD",
		"const CodeBHD",
		"const CodeCAD",
		"const CodeCHF",
		"const CodeCLF",
		"const CodeEUR",
		"const CodeGBP",
		"const CodeHKD",
		"const CodeJPY",
		"const CodeJSONMaximumBytes",
		"const CodeNZD",
		"const CodeSGD",
		"const CodeTokenAUD",
		"const CodeTokenBHD",
		"const CodeTokenCAD",
		"const CodeTokenCHF",
		"const CodeTokenCLF",
		"const CodeTokenEUR",
		"const CodeTokenGBP",
		"const CodeTokenHKD",
		"const CodeTokenJPY",
		"const CodeTokenNZD",
		"const CodeTokenSGD",
		"const CodeTokenUSD",
		"const CodeUnknown",
		"const CodeUSD",
		"const DecimalMaximumBytes",
		"const JSONFieldCurrency",
		"const JSONFieldMinorUnits",
		"const MinorUnitDigitsFour",
		"const MinorUnitDigitsThree",
		"const MinorUnitDigitsTwo",
		"const MinorUnitDigitsZero",
		"func New",
		"func Parse",
		"func ParseCode",
		"method Amount.Add",
		"method Amount.Code",
		"method Amount.Compare",
		"method Amount.Decimal",
		"method Amount.MarshalJSON",
		"method Amount.MinorUnits",
		"method Amount.Subtract",
		"method Amount.UnmarshalJSON",
		"method Amount.Validate",
		"method Code.FractionDigits",
		"method Code.IsValid",
		"method Code.MarshalJSON",
		"method Code.String",
		"method Code.UnmarshalJSON",
		"method Code.Validate",
		"type Amount",
		"type Code",
	}
	slices.Sort(wantSurface)
	if !slices.Equal(gotScan.surface, wantSurface) {
		t.Fatalf("Currency public surface = %q, want %q", gotScan.surface, wantSurface)
	}
	if len(gotScan.aliases) != 0 {
		t.Fatalf("Currency production type aliases = %q, want none", gotScan.aliases)
	}
}

func TestCurrencyProductionUsesOnlyStandardLibraryAndCoreWithoutMaps(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanCurrencyArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanCurrencyArchitecture() error = %v, want nil", gotErr)
	}
	wantImports := []string{
		"bytes",
		"errors",
		"github.com/deliri/primitive/v2026/core",
		"math",
		"strconv",
		"strings",
	}
	if !slices.Equal(gotScan.imports, wantImports) {
		t.Fatalf("Currency production imports = %q, want %q", gotScan.imports, wantImports)
	}
	if len(gotScan.importAliases) != 0 {
		t.Fatalf("Currency production import aliases = %q, want none", gotScan.importAliases)
	}
	if len(gotScan.mapFiles) != 0 {
		t.Fatalf("Currency production map declarations = %q, want none", gotScan.mapFiles)
	}
}

func TestCurrencyArchitectureMatcherClassifiesSyntheticBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		source            string
		wantStructs       []string
		wantSurface       []string
		wantAliases       []string
		wantImports       []string
		wantImportAliases []string
		wantMapFiles      []string
	}{
		{
			name:        "named struct is inventoried",
			source:      "package synthetic\ntype flow struct{}\n",
			wantStructs: []string{"flow"},
		},
		{
			name:        "exported named struct enters surface and inventory",
			source:      "package synthetic\ntype Flow struct{}\n",
			wantStructs: []string{"Flow"},
			wantSurface: []string{"type Flow"},
		},
		{
			name:   "anonymous struct is not a named carrier",
			source: "package synthetic\nvar flow = struct{ Value string }{}\n",
		},
		{
			name:        "exported constant enters surface",
			source:      "package synthetic\nconst Protocol = 1\n",
			wantSurface: []string{"const Protocol"},
		},
		{
			name:   "private constant stays outside surface",
			source: "package synthetic\nconst protocol = 1\n",
		},
		{
			name:        "exported function enters surface",
			source:      "package synthetic\nfunc Project() {}\n",
			wantSurface: []string{"func Project"},
		},
		{
			name:   "private function stays outside surface",
			source: "package synthetic\nfunc project() {}\n",
		},
		{
			name:        "exported value receiver method enters surface",
			source:      "package synthetic\ntype Flow struct{}\nfunc (Flow) Validate() error { return nil }\n",
			wantStructs: []string{"Flow"},
			wantSurface: []string{"method Flow.Validate", "type Flow"},
		},
		{
			name:        "exported pointer receiver method is normalized",
			source:      "package synthetic\ntype Flow struct{}\nfunc (*Flow) Decode() {}\n",
			wantStructs: []string{"Flow"},
			wantSurface: []string{"method Flow.Decode", "type Flow"},
		},
		{
			name:        "exported method on private receiver stays private",
			source:      "package synthetic\ntype flow struct{}\nfunc (flow) Validate() error { return nil }\n",
			wantStructs: []string{"flow"},
		},
		{
			name:        "type alias is detected without new carrier",
			source:      "package synthetic\ntype Value uint8\ntype Alias = Value\n",
			wantSurface: []string{"type Alias", "type Value"},
			wantAliases: []string{"Alias"},
		},
		{
			name:        "ordinary import has no alias",
			source:      "package synthetic\nimport \"strings\"\nvar _ = strings.Compare\n",
			wantImports: []string{"strings"},
		},
		{
			name:              "named import alias is detected",
			source:            "package synthetic\nimport text \"strings\"\nvar _ = text.Compare\n",
			wantImports:       []string{"strings"},
			wantImportAliases: []string{"text=strings"},
		},
		{
			name:              "dot import alias is detected",
			source:            "package synthetic\nimport . \"strings\"\nvar _ = Compare\n",
			wantImports:       []string{"strings"},
			wantImportAliases: []string{".=strings"},
		},
		{
			name:              "blank import alias is detected",
			source:            "package synthetic\nimport _ \"strings\"\n",
			wantImports:       []string{"strings"},
			wantImportAliases: []string{"_=strings"},
		},
		{
			name:         "map variable is detected",
			source:       "package synthetic\nvar forbidden map[string]string\n",
			wantMapFiles: []string{"synthetic.go"},
		},
		{
			name:         "map struct field is detected",
			source:       "package synthetic\ntype Flow struct{ Values map[string]string }\n",
			wantStructs:  []string{"Flow"},
			wantSurface:  []string{"type Flow"},
			wantMapFiles: []string{"synthetic.go"},
		},
		{
			name:         "map function parameter is detected",
			source:       "package synthetic\nfunc Project(values map[string]string) {}\n",
			wantSurface:  []string{"func Project"},
			wantMapFiles: []string{"synthetic.go"},
		},
		{
			name:         "map function result is detected",
			source:       "package synthetic\nfunc Project() map[string]string { return nil }\n",
			wantSurface:  []string{"func Project"},
			wantMapFiles: []string{"synthetic.go"},
		},
		{
			name:   "slice is not misclassified as map",
			source: "package synthetic\nvar values []string\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			path := filepath.Join(root, "synthetic.go")
			if gotErr := os.WriteFile(path, []byte(tc.source), 0o600); gotErr != nil {
				t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, gotErr)
			}
			gotScan, gotErr := scanCurrencyArchitecture(root)
			if gotErr != nil {
				t.Fatalf("scanCurrencyArchitecture(synthetic) error = %v, want nil", gotErr)
			}
			if !slices.Equal(gotScan.structs, tc.wantStructs) {
				t.Fatalf("synthetic structs = %q, want %q", gotScan.structs, tc.wantStructs)
			}
			if !slices.Equal(gotScan.surface, tc.wantSurface) {
				t.Fatalf("synthetic surface = %q, want %q", gotScan.surface, tc.wantSurface)
			}
			if !slices.Equal(gotScan.aliases, tc.wantAliases) {
				t.Fatalf("synthetic aliases = %q, want %q", gotScan.aliases, tc.wantAliases)
			}
			if !slices.Equal(gotScan.imports, tc.wantImports) {
				t.Fatalf("synthetic imports = %q, want %q", gotScan.imports, tc.wantImports)
			}
			if !slices.Equal(gotScan.importAliases, tc.wantImportAliases) {
				t.Fatalf(
					"synthetic import aliases = %q, want %q",
					gotScan.importAliases,
					tc.wantImportAliases,
				)
			}
			if !slices.Equal(gotScan.mapFiles, tc.wantMapFiles) {
				t.Fatalf("synthetic map files = %q, want %q", gotScan.mapFiles, tc.wantMapFiles)
			}
		})
	}
}

type currencyArchitectureScan struct {
	structs       []string
	surface       []string
	aliases       []string
	imports       []string
	importAliases []string
	mapFiles      []string
}

func scanCurrencyArchitecture(root string) (currencyArchitectureScan, error) {
	files, err := currencyProductionGoFiles(root)
	if err != nil {
		return currencyArchitectureScan{}, err
	}
	var scan currencyArchitectureScan
	fileSet := token.NewFileSet()
	for _, name := range files {
		file, parseErr := parser.ParseFile(
			fileSet,
			filepath.Join(root, name),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return currencyArchitectureScan{}, parseErr
		}
		scanCurrencyFile(name, file, &scan)
	}
	slices.Sort(scan.structs)
	slices.Sort(scan.surface)
	slices.Sort(scan.aliases)
	slices.Sort(scan.imports)
	slices.Sort(scan.importAliases)
	slices.Sort(scan.mapFiles)
	scan.imports = slices.Compact(scan.imports)
	scan.importAliases = slices.Compact(scan.importAliases)
	scan.mapFiles = slices.Compact(scan.mapFiles)
	return scan, nil
}

func currencyProductionGoFiles(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		files = append(files, entry.Name())
	}
	slices.Sort(files)
	return files, nil
}

func scanCurrencyFile(
	name string,
	file *ast.File,
	scan *currencyArchitectureScan,
) {
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			continue
		}
		scan.imports = append(scan.imports, path)
		if imported.Name != nil {
			scan.importAliases = append(
				scan.importAliases,
				imported.Name.Name+"="+path,
			)
		}
	}
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			scanCurrencyFunction(typed, scan)
		case *ast.GenDecl:
			scanCurrencyDeclaration(typed, scan)
		}
	}
	hasMap := false
	ast.Inspect(file, func(node ast.Node) bool {
		if _, ok := node.(*ast.MapType); ok {
			hasMap = true
		}
		return true
	})
	if hasMap {
		scan.mapFiles = append(scan.mapFiles, name)
	}
}

func scanCurrencyFunction(function *ast.FuncDecl, scan *currencyArchitectureScan) {
	if !function.Name.IsExported() {
		return
	}
	if function.Recv == nil {
		scan.surface = append(scan.surface, "func "+function.Name.Name)
		return
	}
	receiver := currencyReceiverName(function.Recv.List[0].Type)
	if !ast.IsExported(receiver) {
		return
	}
	scan.surface = append(
		scan.surface,
		"method "+receiver+"."+function.Name.Name,
	)
}

func currencyReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	identifier, ok := expression.(*ast.Ident)
	if !ok {
		return ""
	}
	return identifier.Name
}

func scanCurrencyDeclaration(
	declaration *ast.GenDecl,
	scan *currencyArchitectureScan,
) {
	for _, specification := range declaration.Specs {
		switch typed := specification.(type) {
		case *ast.TypeSpec:
			scanCurrencyType(declaration, typed, scan)
		case *ast.ValueSpec:
			if declaration.Tok == token.CONST {
				scanCurrencyConstants(typed, scan)
			}
		}
	}
}

func scanCurrencyType(
	declaration *ast.GenDecl,
	specification *ast.TypeSpec,
	scan *currencyArchitectureScan,
) {
	if specification.Assign.IsValid() {
		scan.aliases = append(scan.aliases, specification.Name.Name)
	}
	if _, ok := specification.Type.(*ast.StructType); ok {
		scan.structs = append(scan.structs, specification.Name.Name)
	}
	if declaration.Tok == token.TYPE && specification.Name.IsExported() {
		scan.surface = append(scan.surface, "type "+specification.Name.Name)
	}
}

func scanCurrencyConstants(
	specification *ast.ValueSpec,
	scan *currencyArchitectureScan,
) {
	for _, name := range specification.Names {
		if name.IsExported() {
			scan.surface = append(scan.surface, "const "+name.Name)
		}
	}
}
