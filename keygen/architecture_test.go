package keygen

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
	operationRequest[T any]  struct{}
	capabilityWrapper[T any] struct{}
)

// keygenContractInventory classifies every production struct by its actual
// data-flow role. Keygen deliberately owns no wire or persistence carrier.
type keygenContractInventory struct {
	SecretRequest      operationRequest[SecretRequest]
	RandomTokenRequest operationRequest[RandomTokenRequest]
	SigningKey         capabilityWrapper[SigningKey]
}

var _ = keygenContractInventory{}

func TestKeygenProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanKeygenArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanKeygenArchitecture() error = %v, want nil", gotErr)
	}
	wantClassified, gotClassifiedErr := classifiedKeygenStructs()
	if gotClassifiedErr != nil {
		t.Fatalf("classifiedKeygenStructs() error = %v, want nil", gotClassifiedErr)
	}
	if !slices.Equal(gotScan.structs, wantClassified) {
		t.Fatalf("Keygen production structs = %q, want classified %q", gotScan.structs, wantClassified)
	}
}

func TestKeygenExactPublicSurfaceFieldsAndNoAliases(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanKeygenArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanKeygenArchitecture() error = %v, want nil", gotErr)
	}
	wantSurface := []string{
		"const RandomTokenMaximumBytes",
		"func AdoptSigningKey",
		"func GenerateSecret",
		"func GenerateSigningKey",
		"func RandomToken",
		"func RandomUint64",
		"method RandomTokenRequest.Validate",
		"method SecretRequest.Validate",
		"method SigningKey.Destroy",
		"method SigningKey.Format",
		"method SigningKey.PrivateKey",
		"method SigningKey.PublicKey",
		"method SigningKey.Validate",
		"type RandomTokenRequest",
		"type SecretRequest",
		"type SigningKey",
	}
	wantFields := []string{"RandomTokenRequest.Size", "SecretRequest.Size"}
	if !slices.Equal(gotScan.surface, wantSurface) {
		t.Fatalf("Keygen public surface = %q, want %q", gotScan.surface, wantSurface)
	}
	if !slices.Equal(gotScan.exportedFields, wantFields) {
		t.Fatalf("Keygen exported struct fields = %q, want %q", gotScan.exportedFields, wantFields)
	}
	if len(gotScan.aliases) != 0 {
		t.Fatalf("Keygen production type aliases = %q, want none", gotScan.aliases)
	}
}

func TestKeygenProductionImportsStayOnExactStandardLibraryAndCoreSubstrate(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanKeygenArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanKeygenArchitecture() error = %v, want nil", gotErr)
	}
	wantImports := []string{
		"crypto/ed25519",
		"crypto/rand",
		"crypto/subtle",
		"encoding/binary",
		"errors",
		"fmt",
		"github.com/deliri/primitive/v2026/core",
		"io",
	}
	if !slices.Equal(gotScan.imports, wantImports) {
		t.Fatalf("Keygen production imports = %q, want %q", gotScan.imports, wantImports)
	}
	if len(gotScan.importAliases) != 0 {
		t.Fatalf("Keygen production import aliases = %q, want none", gotScan.importAliases)
	}
}

func TestKeygenUsesOnlyGo126ProtectedProductionEntropyEffects(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanKeygenArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanKeygenArchitecture() error = %v, want nil", gotErr)
	}
	wantSelectors := []string{
		"random.go:rand.Read",
		"random.go:rand.Read",
		"secret.go:rand.Read",
		"signing.go:ed25519.GenerateKey",
		"signing.go:ed25519.NewKeyFromSeed",
		"signing.go:ed25519.NewKeyFromSeed",
		"signing.go:ed25519.NewKeyFromSeed",
		"signing.go:subtle.ConstantTimeCompare",
	}
	if !slices.Equal(gotScan.cryptographicSelectors, wantSelectors) {
		t.Fatalf("Keygen cryptographic selectors = %q, want %q", gotScan.cryptographicSelectors, wantSelectors)
	}
	if len(gotScan.globalRandomSelectors) != 0 {
		t.Fatalf("Keygen mutable global random selectors = %q, want none", gotScan.globalRandomSelectors)
	}
}

func TestKeygenProductionOwnsNoMapBasedProtocolOrState(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanKeygenArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanKeygenArchitecture() error = %v, want nil", gotErr)
	}
	if len(gotScan.maps) != 0 {
		t.Fatalf("Keygen production map types = %q, want none", gotScan.maps)
	}
}

func TestKeygenProductionDeclaresNoSecretContentPredicate(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanKeygenArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanKeygenArchitecture() error = %v, want nil", gotErr)
	}
	if len(gotScan.bytePredicates) != 0 {
		t.Fatalf(
			"Keygen production byte-slice predicates = %q, want none; Core owns secret-content rejection",
			gotScan.bytePredicates,
		)
	}
}

func TestKeygenArchitectureMatcherClassifiesSyntheticBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		code string
		want keygenArchitectureScan
	}{
		{
			name: "exported function enters public surface",
			code: "package synthetic\nfunc Generate() {}\n",
			want: keygenArchitectureScan{surface: []string{"func Generate"}},
		},
		{
			name: "private function stays outside public surface",
			code: "package synthetic\nfunc generate() {}\n",
		},
		{
			name: "exported value receiver method enters public surface",
			code: "package synthetic\ntype Request struct{}\nfunc (Request) Validate() {}\n",
			want: keygenArchitectureScan{
				structs: []string{"Request"},
				surface: []string{"method Request.Validate", "type Request"},
			},
		},
		{
			name: "exported pointer receiver method enters public surface",
			code: "package synthetic\ntype Request struct{}\nfunc (*Request) Destroy() {}\n",
			want: keygenArchitectureScan{
				structs: []string{"Request"},
				surface: []string{"method Request.Destroy", "type Request"},
			},
		},
		{
			name: "method on private receiver stays outside public surface",
			code: "package synthetic\ntype request struct{}\nfunc (request) Validate() {}\n",
			want: keygenArchitectureScan{structs: []string{"request"}},
		},
		{
			name: "exported nominal type enters public surface",
			code: "package synthetic\ntype Kind uint8\n",
			want: keygenArchitectureScan{surface: []string{"type Kind"}},
		},
		{
			name: "exported alias is classified and enters public surface",
			code: "package synthetic\ntype Alias = uint8\n",
			want: keygenArchitectureScan{
				surface: []string{"type Alias"},
				aliases: []string{"Alias"},
			},
		},
		{
			name: "private alias is still classified as forbidden structure",
			code: "package synthetic\ntype alias = uint8\n",
			want: keygenArchitectureScan{aliases: []string{"alias"}},
		},
		{
			name: "exported constant enters public surface",
			code: "package synthetic\nconst Maximum = 1\n",
			want: keygenArchitectureScan{surface: []string{"const Maximum"}},
		},
		{
			name: "exported variable enters public surface",
			code: "package synthetic\nvar Provider = 1\n",
			want: keygenArchitectureScan{surface: []string{"var Provider"}},
		},
		{
			name: "exported struct field is inventoried",
			code: "package synthetic\ntype Request struct{ Size uint64 }\n",
			want: keygenArchitectureScan{
				structs:        []string{"Request"},
				surface:        []string{"type Request"},
				exportedFields: []string{"Request.Size"},
			},
		},
		{
			name: "private struct field stays outside exported field inventory",
			code: "package synthetic\ntype request struct{ size uint64 }\n",
			want: keygenArchitectureScan{structs: []string{"request"}},
		},
		{
			name: "ordinary import enters exact import inventory",
			code: "package synthetic\nimport \"crypto/rand\"\n",
			want: keygenArchitectureScan{imports: []string{"crypto/rand"}},
		},
		{
			name: "named import alias enters alias inventory",
			code: "package synthetic\nimport entropy \"crypto/rand\"\n",
			want: keygenArchitectureScan{
				imports:       []string{"crypto/rand"},
				importAliases: []string{"entropy=crypto/rand"},
			},
		},
		{
			name: "dot import enters alias inventory",
			code: "package synthetic\nimport . \"crypto/rand\"\n",
			want: keygenArchitectureScan{
				imports:       []string{"crypto/rand"},
				importAliases: []string{".=crypto/rand"},
			},
		},
		{
			name: "standard random read call enters effect inventory",
			code: "package synthetic\nimport \"crypto/rand\"\nfunc generate(b []byte) { _, _ = rand.Read(b) }\n",
			want: keygenArchitectureScan{
				imports:                []string{"crypto/rand"},
				cryptographicSelectors: []string{"synthetic.go:rand.Read"},
			},
		},
		{
			name: "aliased random read call enters effect inventory",
			code: "package synthetic\nimport entropy \"crypto/rand\"\nfunc generate(b []byte) { _, _ = entropy.Read(b) }\n",
			want: keygenArchitectureScan{
				imports:                []string{"crypto/rand"},
				importAliases:          []string{"entropy=crypto/rand"},
				cryptographicSelectors: []string{"synthetic.go:entropy.Read"},
			},
		},
		{
			name: "mutable random Reader reference enters global inventory but not call inventory",
			code: "package synthetic\nimport entropy \"crypto/rand\"\nfunc generate() { _ = entropy.Reader }\n",
			want: keygenArchitectureScan{
				imports:               []string{"crypto/rand"},
				importAliases:         []string{"entropy=crypto/rand"},
				globalRandomSelectors: []string{"synthetic.go:entropy.Reader"},
			},
		},
		{
			name: "Ed25519 generation call enters effect inventory",
			code: "package synthetic\nimport \"crypto/ed25519\"\nfunc generate() { _, _, _ = ed25519.GenerateKey(nil) }\n",
			want: keygenArchitectureScan{
				imports:                []string{"crypto/ed25519"},
				cryptographicSelectors: []string{"synthetic.go:ed25519.GenerateKey"},
			},
		},
		{
			name: "cryptographic selector reference is not misclassified as a call",
			code: "package synthetic\nimport \"crypto/ed25519\"\nfunc generate() { _ = ed25519.GenerateKey }\n",
			want: keygenArchitectureScan{imports: []string{"crypto/ed25519"}},
		},
		{
			name: "map field enters forbidden map inventory",
			code: "package synthetic\ntype request struct{ values map[string]string }\n",
			want: keygenArchitectureScan{
				structs: []string{"request"},
				maps:    []string{"synthetic.go"},
			},
		},
		{
			name: "two map types preserve nonvacuous occurrence count",
			code: "package synthetic\nvar first map[string]string\nvar second map[uint8]uint8\n",
			want: keygenArchitectureScan{
				maps: []string{"synthetic.go", "synthetic.go"},
			},
		},
		{
			name: "renamed byte-slice predicate enters secret-content inventory",
			code: "package synthetic\nfunc locallyClassify(value []byte) bool { return len(value) == 0 }\n",
			want: keygenArchitectureScan{
				bytePredicates: []string{"synthetic.go:locallyClassify"},
			},
		},
		{
			name: "uint8 spelling of byte-slice predicate enters secret-content inventory",
			code: "package synthetic\nfunc locallyClassify(value []uint8) bool { return len(value) == 0 }\n",
			want: keygenArchitectureScan{
				bytePredicates: []string{"synthetic.go:locallyClassify"},
			},
		},
		{
			name: "byte-slice transformer is not a content predicate",
			code: "package synthetic\nfunc copyValue(value []byte) []byte { return append([]byte(nil), value...) }\n",
		},
		{
			name: "string predicate is not a secret-byte predicate",
			code: "package synthetic\nfunc empty(value string) bool { return value == \"\" }\n",
		},
		{
			name: "two-byte-slice relation is not a unary content rule",
			code: "package synthetic\nfunc equal(left, right []byte) bool { return len(left) == len(right) }\n",
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
			got, gotErr := scanKeygenArchitecture(root)
			if gotErr != nil {
				t.Fatalf("scanKeygenArchitecture(synthetic) error = %v, want nil", gotErr)
			}
			if !keygenArchitectureScansEqual(got, tc.want) {
				t.Fatalf("synthetic architecture scan = %+v, want %+v", got, tc.want)
			}
		})
	}
}

type keygenArchitectureScan struct {
	structs                []string
	surface                []string
	exportedFields         []string
	aliases                []string
	imports                []string
	importAliases          []string
	cryptographicSelectors []string
	globalRandomSelectors  []string
	maps                   []string
	bytePredicates         []string
}

func scanKeygenArchitecture(root string) (keygenArchitectureScan, error) {
	files, err := keygenProductionGoFiles(root)
	if err != nil {
		return keygenArchitectureScan{}, err
	}
	var scan keygenArchitectureScan
	fileSet := token.NewFileSet()
	for _, name := range files {
		file, parseErr := parser.ParseFile(
			fileSet,
			filepath.Join(root, name),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return keygenArchitectureScan{}, parseErr
		}
		scanKeygenFile(name, file, &scan)
	}
	sortKeygenArchitectureScan(&scan)
	return scan, nil
}

func scanKeygenFile(name string, file *ast.File, scan *keygenArchitectureScan) {
	scanKeygenImports(file, scan)
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			scanKeygenFunction(name, typed, scan)
		case *ast.GenDecl:
			scanKeygenDeclaration(typed, scan)
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		if _, ok := node.(*ast.MapType); ok {
			scan.maps = append(scan.maps, name)
		}
		if call, ok := node.(*ast.CallExpr); ok {
			scanKeygenCryptographicCall(name, call, scan)
		}
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		owner, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		if (owner.Name == "rand" || owner.Name == "entropy") &&
			selector.Sel.Name == "Reader" {
			scan.globalRandomSelectors = append(
				scan.globalRandomSelectors,
				name+":"+owner.Name+".Reader",
			)
		}
		return true
	})
}

func scanKeygenCryptographicCall(
	name string,
	call *ast.CallExpr,
	scan *keygenArchitectureScan,
) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	owner, ok := selector.X.(*ast.Ident)
	if !ok {
		return
	}
	switch owner.Name {
	case "ed25519", "rand", "subtle", "entropy":
		scan.cryptographicSelectors = append(
			scan.cryptographicSelectors,
			name+":"+owner.Name+"."+selector.Sel.Name,
		)
	}
}

func scanKeygenImports(file *ast.File, scan *keygenArchitectureScan) {
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

func scanKeygenFunction(
	name string,
	function *ast.FuncDecl,
	scan *keygenArchitectureScan,
) {
	if keygenFunctionIsBytePredicate(function) {
		scan.bytePredicates = append(
			scan.bytePredicates,
			name+":"+function.Name.Name,
		)
	}
	if !function.Name.IsExported() {
		return
	}
	if function.Recv == nil {
		scan.surface = append(scan.surface, "func "+function.Name.Name)
		return
	}
	receiver := keygenReceiverName(function.Recv.List[0].Type)
	if ast.IsExported(receiver) {
		scan.surface = append(scan.surface, "method "+receiver+"."+function.Name.Name)
	}
}

func keygenFunctionIsBytePredicate(function *ast.FuncDecl) bool {
	if function.Type.Params == nil ||
		len(function.Type.Params.List) != 1 ||
		function.Type.Results == nil ||
		len(function.Type.Results.List) != 1 {
		return false
	}
	parameter := function.Type.Params.List[0]
	if len(parameter.Names) != 1 {
		return false
	}
	slice, ok := parameter.Type.(*ast.ArrayType)
	if !ok || slice.Len != nil {
		return false
	}
	element, ok := slice.Elt.(*ast.Ident)
	if !ok || (element.Name != "byte" && element.Name != "uint8") {
		return false
	}
	result, ok := function.Type.Results.List[0].Type.(*ast.Ident)
	return ok && result.Name == "bool"
}

func scanKeygenDeclaration(declaration *ast.GenDecl, scan *keygenArchitectureScan) {
	for _, raw := range declaration.Specs {
		switch spec := raw.(type) {
		case *ast.TypeSpec:
			if spec.Assign.IsValid() {
				scan.aliases = append(scan.aliases, spec.Name.Name)
			}
			structure, isStruct := spec.Type.(*ast.StructType)
			if isStruct {
				scan.structs = append(scan.structs, spec.Name.Name)
				scanKeygenExportedFields(spec.Name.Name, structure, scan)
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

func scanKeygenExportedFields(
	typeName string,
	structure *ast.StructType,
	scan *keygenArchitectureScan,
) {
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			if name.IsExported() {
				scan.exportedFields = append(scan.exportedFields, typeName+"."+name.Name)
			}
		}
	}
}

func keygenReceiverName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return keygenReceiverName(typed.X)
	default:
		return ""
	}
}

func sortKeygenArchitectureScan(scan *keygenArchitectureScan) {
	slices.Sort(scan.structs)
	slices.Sort(scan.surface)
	slices.Sort(scan.exportedFields)
	slices.Sort(scan.aliases)
	slices.Sort(scan.imports)
	slices.Sort(scan.importAliases)
	slices.Sort(scan.cryptographicSelectors)
	slices.Sort(scan.globalRandomSelectors)
	slices.Sort(scan.maps)
	slices.Sort(scan.bytePredicates)
}

func keygenArchitectureScansEqual(
	got keygenArchitectureScan,
	want keygenArchitectureScan,
) bool {
	return slices.Equal(got.structs, want.structs) &&
		slices.Equal(got.surface, want.surface) &&
		slices.Equal(got.exportedFields, want.exportedFields) &&
		slices.Equal(got.aliases, want.aliases) &&
		slices.Equal(got.imports, want.imports) &&
		slices.Equal(got.importAliases, want.importAliases) &&
		slices.Equal(
			got.cryptographicSelectors,
			want.cryptographicSelectors,
		) &&
		slices.Equal(
			got.globalRandomSelectors,
			want.globalRandomSelectors,
		) &&
		slices.Equal(got.maps, want.maps) &&
		slices.Equal(got.bytePredicates, want.bytePredicates)
}

func keygenProductionGoFiles(root string) ([]string, error) {
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

func classifiedKeygenStructs() ([]string, error) {
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
			if spec.Name.Name != "keygenContractInventory" {
				continue
			}
			structure, ok := spec.Type.(*ast.StructType)
			if !ok {
				return nil, core.ErrKeygenContract
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
	return nil, core.ErrKeygenContract
}
