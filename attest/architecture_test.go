package attest

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
	proofCarrier[T any]      struct{}
	internalFlow[T any]      struct{}
	capabilityWrapper[T any] struct{}
	productionStructName     string
)

const attestProductionStructMaximum = 17

type productionStructInventory struct {
	names [attestProductionStructMaximum]productionStructName
	count uint8
}

// attestContractInventory classifies every production struct by its real role.
// It is a compiler-visible wiring ratchet, not behavioral proof.
type attestContractInventory struct {
	canonicalFacts        internalFlow[canonicalFacts[internalTestDomain]]
	canonicalDigestWriter capabilityWrapper[canonicalDigestWriter]
	CanonicalObject       capabilityWrapper[CanonicalObject]
	canonicalNameSpan     internalFlow[canonicalNameSpan]
	TrustedKeysRequest    protocolFact[TrustedKeysRequest]
	SignRequest           operationRequest[SignRequest[internalTestDomain]]
	VerifyRequest         operationRequest[VerifyRequest[internalTestDomain]]
	domainToken           internalFlow[domainToken]
	Envelope              sealedProjection[Envelope[internalTestDomain]]
	envelopeWire          wireProjection[envelopeWire]
	attestationFrame      internalFlow[attestationFrame]
	guardedResult         internalFlow[guardedResult[struct{}]]
	Signature             sealedProjection[Signature]
	signingCapability     capabilityWrapper[signingCapability]
	TrustedKeys           capabilityWrapper[TrustedKeys]
	Verified              proofCarrier[Verified[internalTestDomain]]
}

var (
	_ = attestContractInventory{}.canonicalFacts
	_ = attestContractInventory{}.canonicalDigestWriter
	_ = attestContractInventory{}.canonicalNameSpan
	_ = attestContractInventory{}.domainToken
	_ = attestContractInventory{}.envelopeWire
	_ = attestContractInventory{}.attestationFrame
	_ = attestContractInventory{}.guardedResult
	_ = attestContractInventory{}.signingCapability
)

func TestAttestProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	gotProduction, gotErr := productionStructNames(".")
	if gotErr != nil {
		t.Fatalf("productionStructNames() error = %v, want nil", gotErr)
	}
	wantClassified, gotClassifiedErr := classifiedStructNames()
	if gotClassifiedErr != nil {
		t.Fatalf("classifiedStructNames() error = %v, want nil", gotClassifiedErr)
	}
	for _, gotName := range gotProduction.Values() {
		if !wantClassified.Contains(gotName) {
			t.Errorf("production struct %q has no compiler-visible data-flow role", gotName)
		}
	}
	for _, wantName := range wantClassified.Values() {
		if !gotProduction.Contains(wantName) {
			t.Errorf("classified struct %q does not exist in production", wantName)
		}
	}
}

func TestAttestExactPublicSurfaceAndNoTypeAliases(t *testing.T) {
	t.Parallel()

	gotSurface, gotAliases, gotErr := productionPublicSurface(".")
	if gotErr != nil {
		t.Fatalf("productionPublicSurface() error = %v, want nil", gotErr)
	}
	wantSurface := []string{
		"const CanonicalBodyMaximumBytes",
		"const CanonicalFieldNameMaximumBytes",
		"const CanonicalFieldNameSeparator",
		"const CanonicalObjectMaximumFields",
		"const EnvelopeCanonicalJSONMaximumBytes",
		"const EnvelopeJSONMaximumBytes",
		"const SigningDomainMaximumBytes",
		"const TrustedKeyMaximumCount",
		"func BeginCanonicalObject",
		"func NewTrustedKeys",
		"func Sign",
		"func Verify",
		"method CanonicalObject.Bool",
		"method CanonicalObject.End",
		"method CanonicalObject.Int64",
		"method CanonicalObject.String",
		"method CanonicalObject.Uint64",
		"method CanonicalObject.Value",
		"method Envelope.MarshalJSON",
		"method Envelope.UnmarshalJSON",
		"method Envelope.Validate",
		"method SignRequest.Validate",
		"method Signature.Bytes",
		"method Signature.Hex",
		"method Signature.MarshalJSON",
		"method Signature.UnmarshalJSON",
		"method Signature.Validate",
		"method TrustedKeys.Validate",
		"method TrustedKeysRequest.Validate",
		"method Verified.Envelope",
		"method Verified.Validate",
		"method VerifyRequest.Validate",
		"type CanonicalBody",
		"type CanonicalObject",
		"type Envelope",
		"type SignRequest",
		"type Signature",
		"type SigningDomain",
		"type TrustedKeys",
		"type TrustedKeysRequest",
		"type Verified",
		"type VerifyRequest",
	}
	slices.Sort(wantSurface)
	if !slices.Equal(gotSurface, wantSurface) {
		t.Fatalf("Attest public surface = %q, want %q", gotSurface, wantSurface)
	}
	if len(gotAliases) != 0 {
		t.Fatalf("Attest production type aliases = %q, want none", gotAliases)
	}
}

func TestAttestProductionImportsStayOnApprovedStandardLibraryAndCoreSubstrate(t *testing.T) {
	t.Parallel()

	gotImports, gotErr := productionImports(".")
	if gotErr != nil {
		t.Fatalf("productionImports() error = %v, want nil", gotErr)
	}
	wantImports := []string{
		"bytes",
		"crypto",
		"crypto/ed25519",
		"crypto/rand",
		"crypto/sha256",
		"crypto/subtle",
		"encoding",
		"encoding/binary",
		"encoding/hex",
		"encoding/json",
		"errors",
		"github.com/deliri/primitive/v2026/core",
		"hash",
		"io",
		"math",
		"slices",
		"strconv",
		"unicode/utf8",
	}
	if !slices.Equal(gotImports, wantImports) {
		t.Fatalf("Attest production imports = %q, want %q", gotImports, wantImports)
	}
	gotAliases, gotAliasErr := productionImportAliases(".")
	if gotAliasErr != nil {
		t.Fatalf("productionImportAliases() error = %v, want nil", gotAliasErr)
	}
	if len(gotAliases) != 0 {
		t.Fatalf("Attest production import aliases = %q, want none", gotAliases)
	}
}

func TestAttestRawCryptographicEffectsHaveOneCompilerVisibleOwner(t *testing.T) {
	t.Parallel()

	gotCalls, gotErr := productionSelectorCalls(".")
	if gotErr != nil {
		t.Fatalf("productionSelectorCalls() error = %v, want nil", gotErr)
	}
	wantCalls := []string{
		"canonical.go:sha256.New",
		"operations.go:ed25519.Verify",
		"operations.go:ed25519.Verify",
		"operations.go:signer.Sign",
	}
	slices.Sort(wantCalls)
	if !slices.Equal(gotCalls, wantCalls) {
		t.Fatalf("raw cryptographic calls = %q, want %q", gotCalls, wantCalls)
	}
}

func TestAttestArchitectureMatchersSyntheticRedGreenRatchet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name              string
		source            string
		wantStructs       []productionStructName
		wantSurface       []string
		wantAliases       []string
		wantImports       []string
		wantImportAliases []string
		wantCalls         []string
	}{
		{
			name: "export struct alias imports and raw effects are all visible",
			source: `package synthetic
import (
	"crypto/ed25519"
	"crypto/sha256"
	"github.com/deliri/primitive/v2026/core"
)
const ExportedConstant = 1
type Exported struct{}
type hidden struct{}
type Alias = Exported
func ExportedFunction() {
	_ = sha256.New()
	_ = ed25519.Sign(nil, nil)
	_ = ed25519.Verify(nil, nil, nil)
	_ = core.ErrAttestContract
}
func (Exported) ExportedMethod() {}
func (hidden) ExportedMethod() {}
`,
			wantStructs: []productionStructName{"Exported", "hidden"},
			wantSurface: []string{
				"const ExportedConstant",
				"func ExportedFunction",
				"method Exported.ExportedMethod",
				"type Alias",
				"type Exported",
			},
			wantAliases: []string{"Alias"},
			wantImports: []string{
				"crypto/ed25519",
				"crypto/sha256",
				"github.com/deliri/primitive/v2026/core",
			},
			wantCalls: []string{
				"synthetic.go:ed25519.Sign",
				"synthetic.go:ed25519.Verify",
				"synthetic.go:sha256.New",
			},
		},
		{
			name:        "unexported declarations and decoy selectors stay invisible",
			source:      "package synthetic\ntype hidden uint8\nfunc hiddenFunction() { other.Sign() }\n",
			wantSurface: []string{},
			wantAliases: []string{},
			wantImports: []string{},
			wantCalls:   []string{},
		},
		{
			name:              "one import alias is visible to the live import comparator",
			source:            "package synthetic\nimport ed \"crypto/ed25519\"\nvar _ = ed.Sign\n",
			wantSurface:       []string{},
			wantAliases:       []string{},
			wantImports:       []string{"crypto/ed25519"},
			wantImportAliases: []string{"ed=crypto/ed25519"},
			wantCalls:         []string{},
		},
		{
			name:        "one extra export is visible to the live surface comparator",
			source:      "package synthetic\nfunc ExtraExport() {}\n",
			wantSurface: []string{"func ExtraExport"},
			wantAliases: []string{},
			wantImports: []string{},
			wantCalls:   []string{},
		},
		{
			name:        "one extra raw signing effect is visible to the live effect comparator",
			source:      "package synthetic\nfunc hidden() { ed25519.Sign(nil, nil) }\n",
			wantSurface: []string{},
			wantAliases: []string{},
			wantImports: []string{},
			wantCalls:   []string{"synthetic.go:ed25519.Sign"},
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
			gotStructs, gotStructErr := productionStructNames(root)
			if gotStructErr != nil {
				t.Fatalf("productionStructNames() error = %v, want nil", gotStructErr)
			}
			if !slices.Equal(gotStructs.Values(), tc.wantStructs) {
				t.Fatalf("production structs = %q, want %q", gotStructs.Values(), tc.wantStructs)
			}
			gotSurface, gotAliases, gotSurfaceErr := productionPublicSurface(root)
			if gotSurfaceErr != nil {
				t.Fatalf("productionPublicSurface() error = %v, want nil", gotSurfaceErr)
			}
			if !slices.Equal(gotSurface, tc.wantSurface) {
				t.Fatalf("public surface = %q, want %q", gotSurface, tc.wantSurface)
			}
			if !slices.Equal(gotAliases, tc.wantAliases) {
				t.Fatalf("type aliases = %q, want %q", gotAliases, tc.wantAliases)
			}
			gotImports, gotImportErr := productionImports(root)
			if gotImportErr != nil {
				t.Fatalf("productionImports() error = %v, want nil", gotImportErr)
			}
			if !slices.Equal(gotImports, tc.wantImports) {
				t.Fatalf("production imports = %q, want %q", gotImports, tc.wantImports)
			}
			gotImportAliases, gotImportAliasErr := productionImportAliases(root)
			if gotImportAliasErr != nil {
				t.Fatalf("productionImportAliases() error = %v, want nil", gotImportAliasErr)
			}
			if !slices.Equal(gotImportAliases, tc.wantImportAliases) {
				t.Fatalf(
					"production import aliases = %q, want %q",
					gotImportAliases,
					tc.wantImportAliases,
				)
			}
			gotCalls, gotCallErr := productionSelectorCalls(root)
			if gotCallErr != nil {
				t.Fatalf("productionSelectorCalls() error = %v, want nil", gotCallErr)
			}
			if !slices.Equal(gotCalls, tc.wantCalls) {
				t.Fatalf("raw cryptographic calls = %q, want %q", gotCalls, tc.wantCalls)
			}
		})
	}
}

func productionStructNames(root string) (productionStructInventory, error) {
	files, err := productionGoFiles(root)
	if err != nil {
		return productionStructInventory{}, err
	}
	var names productionStructInventory
	fileSet := token.NewFileSet()
	for _, name := range files {
		file, parseErr := parser.ParseFile(
			fileSet,
			filepath.Join(root, name),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return productionStructInventory{}, parseErr
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				spec := raw.(*ast.TypeSpec)
				if _, ok := spec.Type.(*ast.StructType); ok {
					if addErr := names.Add(productionStructName(spec.Name.Name)); addErr != nil {
						return productionStructInventory{}, addErr
					}
				}
			}
		}
	}
	return names, nil
}

func classifiedStructNames() (productionStructInventory, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "architecture_test.go", nil, parser.SkipObjectResolution)
	if err != nil {
		return productionStructInventory{}, err
	}
	var names productionStructInventory
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			spec := raw.(*ast.TypeSpec)
			if spec.Name.Name != "attestContractInventory" {
				continue
			}
			structure := spec.Type.(*ast.StructType)
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					if addErr := names.Add(productionStructName(name.Name)); addErr != nil {
						return productionStructInventory{}, addErr
					}
				}
			}
			return names, nil
		}
	}
	return productionStructInventory{}, core.ErrAttestContract
}

func productionPublicSurface(root string) ([]string, []string, error) {
	files, err := productionGoFiles(root)
	if err != nil {
		return nil, nil, err
	}
	var surface []string
	var aliases []string
	fileSet := token.NewFileSet()
	for _, name := range files {
		file, parseErr := parser.ParseFile(
			fileSet,
			filepath.Join(root, name),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return nil, nil, parseErr
		}
		for _, declaration := range file.Decls {
			switch typed := declaration.(type) {
			case *ast.FuncDecl:
				if typed.Name.IsExported() {
					if name, exported := publicFunctionName(typed); exported {
						surface = append(surface, name)
					}
				}
			case *ast.GenDecl:
				for _, raw := range typed.Specs {
					switch spec := raw.(type) {
					case *ast.TypeSpec:
						if spec.Assign.IsValid() {
							aliases = append(aliases, spec.Name.Name)
						}
						if spec.Name.IsExported() {
							surface = append(surface, "type "+spec.Name.Name)
						}
					case *ast.ValueSpec:
						for _, valueName := range spec.Names {
							if valueName.IsExported() {
								surface = append(surface, strings.ToLower(typed.Tok.String())+" "+valueName.Name)
							}
						}
					}
				}
			}
		}
	}
	slices.Sort(surface)
	slices.Sort(aliases)
	return surface, aliases, nil
}

func publicFunctionName(function *ast.FuncDecl) (string, bool) {
	if function.Recv == nil {
		return "func " + function.Name.Name, true
	}
	receiver := function.Recv.List[0].Type
	var name string
	switch typed := receiver.(type) {
	case *ast.Ident:
		name = typed.Name
	case *ast.IndexExpr:
		name = typed.X.(*ast.Ident).Name
	case *ast.StarExpr:
		name = receiverTypeName(typed.X)
	default:
		return "", false
	}
	if !ast.IsExported(name) {
		return "", false
	}
	return "method " + name + "." + function.Name.Name, true
}

func receiverTypeName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.IndexExpr:
		return typed.X.(*ast.Ident).Name
	default:
		return "unknown"
	}
}

func productionImports(root string) ([]string, error) {
	files, err := productionGoFiles(root)
	if err != nil {
		return nil, err
	}
	var imports []string
	fileSet := token.NewFileSet()
	for _, name := range files {
		file, parseErr := parser.ParseFile(
			fileSet,
			filepath.Join(root, name),
			nil,
			parser.ImportsOnly,
		)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, declaration := range file.Imports {
			path, unquoteErr := strconv.Unquote(declaration.Path.Value)
			if unquoteErr != nil {
				return nil, unquoteErr
			}
			if !slices.Contains(imports, path) {
				imports = append(imports, path)
			}
		}
	}
	slices.Sort(imports)
	return imports, nil
}

func productionImportAliases(root string) ([]string, error) {
	files, err := productionGoFiles(root)
	if err != nil {
		return nil, err
	}
	var aliases []string
	fileSet := token.NewFileSet()
	for _, name := range files {
		file, parseErr := parser.ParseFile(
			fileSet,
			filepath.Join(root, name),
			nil,
			parser.ImportsOnly,
		)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, declaration := range file.Imports {
			if declaration.Name == nil {
				continue
			}
			path, unquoteErr := strconv.Unquote(declaration.Path.Value)
			if unquoteErr != nil {
				return nil, unquoteErr
			}
			aliases = append(aliases, declaration.Name.Name+"="+path)
		}
	}
	slices.Sort(aliases)
	return aliases, nil
}

func productionSelectorCalls(root string) ([]string, error) {
	files, err := productionGoFiles(root)
	if err != nil {
		return nil, err
	}
	var calls []string
	fileSet := token.NewFileSet()
	for _, name := range files {
		file, parseErr := parser.ParseFile(
			fileSet,
			filepath.Join(root, name),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return nil, parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			owner, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			qualified := owner.Name + "." + selector.Sel.Name
			switch qualified {
			case "ed25519.Sign", "ed25519.Verify", "sha256.New", "signer.Sign":
				calls = append(calls, name+":"+qualified)
			}
			return true
		})
	}
	slices.Sort(calls)
	return calls, nil
}

func productionGoFiles(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var files []string
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

func (i *productionStructInventory) Add(name productionStructName) error {
	if name == "" || i.Contains(name) || int(i.count) >= len(i.names) {
		return core.ErrAttestContract
	}
	i.names[i.count] = name
	i.count++
	return nil
}

func (i productionStructInventory) Contains(name productionStructName) bool {
	return slices.Contains(i.Values(), name)
}

func (i productionStructInventory) Values() []productionStructName {
	return i.names[:i.count]
}
