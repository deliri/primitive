package garble

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
	protocolFact[T any]      struct{}
	sealedProjection[T any]  struct{}
	wireProjection[T any]    struct{}
	operationRequest[T any]  struct{}
	internalFlow[T any]      struct{}
	capabilityWrapper[T any] struct{}
)

// garbleContractInventory classifies every production struct by its real
// data-flow role. It is a compiler-visible wiring ratchet, not behavior proof.
type garbleContractInventory struct {
	Seed               sealedProjection[Seed]
	seedJSONWire       wireProjection[seedJSONWire]
	Custody            capabilityWrapper[Custody]
	DerivationIdentity protocolFact[DerivationIdentity]
	DeriveRequest      operationRequest[DeriveRequest]
	derivationFrame    internalFlow[derivationFrame]
	BuildRequest       operationRequest[BuildRequest]
	BuildIntent        sealedProjection[BuildIntent]
	Argument           sealedProjection[Argument]
}

var (
	_ = garbleContractInventory{}.seedJSONWire
	_ = garbleContractInventory{}.derivationFrame
)

func TestGarbleProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanGarbleArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanGarbleArchitecture() error = %v, want nil", gotErr)
	}
	wantClassified, gotClassifiedErr := classifiedGarbleStructs()
	if gotClassifiedErr != nil {
		t.Fatalf("classifiedGarbleStructs() error = %v, want nil", gotClassifiedErr)
	}
	if !slices.Equal(gotScan.structs, wantClassified) {
		t.Fatalf("Garble production structs = %q, want classified %q", gotScan.structs, wantClassified)
	}
}

func TestGarbleExactPublicSurfaceAndNoTypeAliases(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanGarbleArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanGarbleArchitecture() error = %v, want nil", gotErr)
	}
	wantSurface := []string{
		"const ArgumentKindBuild",
		"const ArgumentKindLiterals",
		"const ArgumentKindSeed",
		"const ArgumentKindTiny",
		"const ArgumentKindUnknown",
		"const CustodyBytes",
		"const DerivationGenerationOne",
		"const DerivationGenerationUnknown",
		"const DerivationSalt",
		"const DiagnosticPolicyPreserve",
		"const DiagnosticPolicyStrip",
		"const DiagnosticPolicyUnknown",
		"const LiteralPolicyObfuscate",
		"const LiteralPolicyPreserve",
		"const LiteralPolicyUnknown",
		"const SeedBytes",
		"const SeedCanonicalJSONBytes",
		"const SeedEncodedBytes",
		"const SeedJSONMaximumBytes",
		"const SeedJSONWhitespaceAllowanceBytes",
		"const ToolIdentityPrimitive2026",
		"const ToolIdentityUnknown",
		"func CurrentDerivationGeneration",
		"func CurrentTool",
		"func Derive",
		"func NewCustody",
		"func NewDerivationIdentity",
		"func NewSeed",
		"func ParseSeed",
		"func PrepareBuild",
		"method Argument.Kind",
		"method Argument.Text",
		"method Argument.Validate",
		"method ArgumentKind.Validate",
		"method BuildIntent.Arguments",
		"method BuildIntent.Validate",
		"method BuildRequest.Validate",
		"method Custody.Format",
		"method Custody.Validate",
		"method DerivationGeneration.Validate",
		"method DerivationIdentity.Validate",
		"method DeriveRequest.Validate",
		"method DiagnosticPolicy.Validate",
		"method LiteralPolicy.Validate",
		"method Seed.Bytes",
		"method Seed.Encoded",
		"method Seed.Format",
		"method Seed.MarshalJSON",
		"method Seed.UnmarshalJSON",
		"method Seed.Validate",
		"method ToolIdentity.MinimumGoVersion",
		"method ToolIdentity.ModulePath",
		"method ToolIdentity.ModuleSum",
		"method ToolIdentity.Revision",
		"method ToolIdentity.UnsupportedGoVersion",
		"method ToolIdentity.Validate",
		"method ToolIdentity.Version",
		"type Argument",
		"type ArgumentKind",
		"type BuildIntent",
		"type BuildRequest",
		"type Custody",
		"type DerivationGeneration",
		"type DerivationIdentity",
		"type DeriveRequest",
		"type DiagnosticPolicy",
		"type LiteralPolicy",
		"type Seed",
		"type ToolIdentity",
	}
	slices.Sort(wantSurface)
	if !slices.Equal(gotScan.surface, wantSurface) {
		t.Fatalf("Garble public surface = %q, want %q", gotScan.surface, wantSurface)
	}
	if len(gotScan.aliases) != 0 {
		t.Fatalf("Garble production type aliases = %q, want none", gotScan.aliases)
	}
}

func TestGarbleProductionImportsStayOnStandardLibraryAndCoreSubstrate(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanGarbleArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanGarbleArchitecture() error = %v, want nil", gotErr)
	}
	wantImports := []string{
		"crypto/hkdf",
		"crypto/sha256",
		"encoding/base64",
		"encoding/json",
		"errors",
		"fmt",
		"github.com/deliri/primitive/v2026/core",
		"io",
		"iter",
	}
	if !slices.Equal(gotScan.imports, wantImports) {
		t.Fatalf("Garble production imports = %q, want %q", gotScan.imports, wantImports)
	}
	if len(gotScan.importAliases) != 0 {
		t.Fatalf("Garble production import aliases = %q, want none", gotScan.importAliases)
	}
}

func TestGarbleBuildIntentNeverLowersToStringSliceInProduction(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanGarbleArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanGarbleArchitecture() error = %v, want nil", gotErr)
	}
	if len(gotScan.stringSlices) != 0 {
		t.Fatalf("Garble production []string declarations = %q, want none", gotScan.stringSlices)
	}
}

func TestGarbleProductionDeclaresNoMaps(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanGarbleArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanGarbleArchitecture() error = %v, want nil", gotErr)
	}
	if len(gotScan.mapFiles) != 0 {
		t.Fatalf("Garble production map declarations = %q, want none", gotScan.mapFiles)
	}
}

func TestGarbleRawDerivationEffectHasOneCompilerVisibleOwner(t *testing.T) {
	t.Parallel()

	gotScan, gotErr := scanGarbleArchitecture(".")
	if gotErr != nil {
		t.Fatalf("scanGarbleArchitecture() error = %v, want nil", gotErr)
	}
	wantCalls := []string{
		"derive.go:hkdf.Key",
	}
	if !slices.Equal(gotScan.cryptographicCalls, wantCalls) {
		t.Fatalf("Garble raw derivation calls = %q, want %q", gotScan.cryptographicCalls, wantCalls)
	}
	wantConstructors := []string{"derive.go:sha256.New"}
	if !slices.Equal(gotScan.hashConstructors, wantConstructors) {
		t.Fatalf(
			"Garble HKDF hash constructors = %q, want %q",
			gotScan.hashConstructors,
			wantConstructors,
		)
	}
}

func TestGarbleArchitectureMatcherClassifiesSyntheticBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                   string
		source                 string
		wantStructs            []string
		wantSurface            []string
		wantAliases            []string
		wantImports            []string
		wantImportAliases      []string
		wantStringSlices       []string
		wantMapFiles           []string
		wantCryptographicCalls []string
		wantHashConstructors   []string
	}{
		{name: "private named struct is inventoried", source: "package synthetic\ntype flow struct{}\n", wantStructs: []string{"flow"}},
		{name: "exported named struct enters surface", source: "package synthetic\ntype Flow struct{}\n", wantStructs: []string{"Flow"}, wantSurface: []string{"type Flow"}},
		{name: "anonymous struct is not a named carrier", source: "package synthetic\nvar flow = struct{ Value string }{}\n"},
		{name: "exported constant enters surface", source: "package synthetic\nconst Protocol = 1\n", wantSurface: []string{"const Protocol"}},
		{name: "private constant stays outside surface", source: "package synthetic\nconst protocol = 1\n"},
		{name: "exported function enters surface", source: "package synthetic\nfunc Project() {}\n", wantSurface: []string{"func Project"}},
		{name: "private function stays outside surface", source: "package synthetic\nfunc project() {}\n"},
		{
			name:        "exported value receiver method enters surface",
			source:      "package synthetic\ntype Flow struct{}\nfunc (Flow) Validate() error { return nil }\n",
			wantStructs: []string{"Flow"},
			wantSurface: []string{"method Flow.Validate", "type Flow"},
		},
		{
			name:        "exported pointer receiver method is normalized",
			source:      "package synthetic\ntype Flow struct{}\nfunc (*Flow) Project() {}\n",
			wantStructs: []string{"Flow"},
			wantSurface: []string{"method Flow.Project", "type Flow"},
		},
		{name: "exported method on private receiver stays private", source: "package synthetic\ntype flow struct{}\nfunc (flow) Validate() error { return nil }\n", wantStructs: []string{"flow"}},
		{name: "type alias is detected without a new carrier", source: "package synthetic\ntype Value uint8\ntype Alias = Value\n", wantSurface: []string{"type Alias", "type Value"}, wantAliases: []string{"Alias"}},
		{name: "ordinary import has no alias", source: "package synthetic\nimport \"strings\"\nvar _ = strings.Compare\n", wantImports: []string{"strings"}},
		{name: "named import alias is detected", source: "package synthetic\nimport text \"strings\"\nvar _ = text.Compare\n", wantImports: []string{"strings"}, wantImportAliases: []string{"text=strings"}},
		{name: "dot import alias is detected", source: "package synthetic\nimport . \"strings\"\nvar _ = Compare\n", wantImports: []string{"strings"}, wantImportAliases: []string{".=strings"}},
		{name: "blank import alias is detected", source: "package synthetic\nimport _ \"strings\"\n", wantImports: []string{"strings"}, wantImportAliases: []string{"_=strings"}},
		{name: "string slice variable is detected", source: "package synthetic\nvar values []string\n", wantStringSlices: []string{"synthetic.go"}},
		{name: "string slice field is detected", source: "package synthetic\ntype Flow struct{ Values []string }\n", wantStructs: []string{"Flow"}, wantSurface: []string{"type Flow"}, wantStringSlices: []string{"synthetic.go"}},
		{name: "fixed string array is not a string slice", source: "package synthetic\nvar values [1]string\n"},
		{name: "byte slice is not a string slice", source: "package synthetic\nvar values []byte\n"},
		{name: "map variable is detected", source: "package synthetic\nvar values map[string]string\n", wantMapFiles: []string{"synthetic.go"}},
		{name: "map field is detected", source: "package synthetic\ntype Flow struct{ Values map[string]string }\n", wantStructs: []string{"Flow"}, wantSurface: []string{"type Flow"}, wantMapFiles: []string{"synthetic.go"}},
		{
			name:        "HKDF selector reference is not a call",
			source:      "package synthetic\nimport \"crypto/hkdf\"\nvar _ = hkdf.Key\n",
			wantImports: []string{"crypto/hkdf"},
		},
		{
			name:                   "HKDF invocation is detected as cryptographic call",
			source:                 "package synthetic\nimport \"crypto/hkdf\"\nfunc derive() { _, _ = hkdf.Key(nil, nil, nil, \"\", 0) }\n",
			wantImports:            []string{"crypto/hkdf"},
			wantCryptographicCalls: []string{"synthetic.go:hkdf.Key"},
		},
		{
			name:                 "SHA256 constructor binding is detected",
			source:               "package synthetic\nimport \"crypto/sha256\"\nvar _ = sha256.New\n",
			wantImports:          []string{"crypto/sha256"},
			wantHashConstructors: []string{"synthetic.go:sha256.New"},
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
			gotScan, gotErr := scanGarbleArchitecture(root)
			if gotErr != nil {
				t.Fatalf("scanGarbleArchitecture(synthetic) error = %v, want nil", gotErr)
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
				t.Fatalf("synthetic import aliases = %q, want %q", gotScan.importAliases, tc.wantImportAliases)
			}
			if !slices.Equal(gotScan.stringSlices, tc.wantStringSlices) {
				t.Fatalf("synthetic string slices = %q, want %q", gotScan.stringSlices, tc.wantStringSlices)
			}
			if !slices.Equal(gotScan.mapFiles, tc.wantMapFiles) {
				t.Fatalf("synthetic map files = %q, want %q", gotScan.mapFiles, tc.wantMapFiles)
			}
			if !slices.Equal(gotScan.cryptographicCalls, tc.wantCryptographicCalls) {
				t.Fatalf("synthetic cryptographic calls = %q, want %q", gotScan.cryptographicCalls, tc.wantCryptographicCalls)
			}
			if !slices.Equal(gotScan.hashConstructors, tc.wantHashConstructors) {
				t.Fatalf("synthetic hash constructors = %q, want %q", gotScan.hashConstructors, tc.wantHashConstructors)
			}
		})
	}
}

type garbleArchitectureScan struct {
	structs            []string
	surface            []string
	aliases            []string
	imports            []string
	importAliases      []string
	stringSlices       []string
	mapFiles           []string
	cryptographicCalls []string
	hashConstructors   []string
}

func scanGarbleArchitecture(root string) (garbleArchitectureScan, error) {
	files, err := garbleProductionGoFiles(root)
	if err != nil {
		return garbleArchitectureScan{}, err
	}
	var scan garbleArchitectureScan
	fileSet := token.NewFileSet()
	for _, name := range files {
		file, parseErr := parser.ParseFile(
			fileSet,
			filepath.Join(root, name),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return garbleArchitectureScan{}, parseErr
		}
		scanGarbleFile(name, file, &scan)
	}
	sortGarbleArchitectureScan(&scan)
	return scan, nil
}

func scanGarbleFile(name string, file *ast.File, scan *garbleArchitectureScan) {
	scanGarbleImports(file, scan)
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			scanGarbleFunction(typed, scan)
		case *ast.GenDecl:
			scanGarbleDeclaration(typed, scan)
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		scanGarbleNode(name, node, scan)
		return true
	})
}

func scanGarbleImports(file *ast.File, scan *garbleArchitectureScan) {
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

func scanGarbleFunction(function *ast.FuncDecl, scan *garbleArchitectureScan) {
	if !function.Name.IsExported() {
		return
	}
	if function.Recv == nil {
		scan.surface = append(scan.surface, "func "+function.Name.Name)
		return
	}
	receiver := garbleReceiverName(function.Recv.List[0].Type)
	if ast.IsExported(receiver) {
		scan.surface = append(scan.surface, "method "+receiver+"."+function.Name.Name)
	}
}

func scanGarbleDeclaration(declaration *ast.GenDecl, scan *garbleArchitectureScan) {
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

func scanGarbleNode(name string, node ast.Node, scan *garbleArchitectureScan) {
	if array, ok := node.(*ast.ArrayType); ok && array.Len == nil {
		if element, stringElement := array.Elt.(*ast.Ident); stringElement && element.Name == "string" {
			scan.stringSlices = append(scan.stringSlices, name)
		}
	}
	if _, ok := node.(*ast.MapType); ok &&
		!slices.Contains(scan.mapFiles, name) {
		scan.mapFiles = append(scan.mapFiles, name)
	}
	selector, selectorOK := node.(*ast.SelectorExpr)
	if selectorOK {
		owner, ownerOK := selector.X.(*ast.Ident)
		if ownerOK && owner.Name == "sha256" &&
			selector.Sel.Name == "New" {
			scan.hashConstructors = append(
				scan.hashConstructors,
				name+":sha256.New",
			)
		}
	}
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return
	}
	callSelector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	owner, ok := callSelector.X.(*ast.Ident)
	if !ok {
		return
	}
	qualified := owner.Name + "." + callSelector.Sel.Name
	if qualified == "hkdf.Key" || qualified == "sha256.New" {
		scan.cryptographicCalls = append(scan.cryptographicCalls, name+":"+qualified)
	}
}

func garbleReceiverName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return garbleReceiverName(typed.X)
	default:
		return ""
	}
}

func sortGarbleArchitectureScan(scan *garbleArchitectureScan) {
	slices.Sort(scan.structs)
	slices.Sort(scan.surface)
	slices.Sort(scan.aliases)
	slices.Sort(scan.imports)
	slices.Sort(scan.importAliases)
	slices.Sort(scan.stringSlices)
	slices.Sort(scan.mapFiles)
	slices.Sort(scan.cryptographicCalls)
	slices.Sort(scan.hashConstructors)
}

func garbleProductionGoFiles(root string) ([]string, error) {
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

func classifiedGarbleStructs() ([]string, error) {
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
			if spec.Name.Name != "garbleContractInventory" {
				continue
			}
			structure, ok := spec.Type.(*ast.StructType)
			if !ok {
				return nil, core.ErrGarbleContract
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
	return nil, core.ErrGarbleContract
}
