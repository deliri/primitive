package core

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

type cleanUpgradeDebtKind uint8

const (
	cleanUpgradeDebtUnknown cleanUpgradeDebtKind = iota
	cleanUpgradeDebtTypeAlias
	cleanUpgradeDebtDeprecatedDeclaration
	cleanUpgradeDebtCompatibilityName
	cleanUpgradeDebtCompatibilityFile
	cleanUpgradeDebtExportedForwarder
	cleanUpgradeDebtLimit
)

const (
	cleanUpgradeDebtMaximum        = 64
	cleanUpgradeDeprecatedPrefix   = "Deprecated:"
	cleanUpgradeLegacyToken        = "Legacy"
	cleanUpgradeDeprecatedToken    = "Deprecated"
	cleanUpgradeCompatibilityToken = "Compatibility"
	cleanUpgradeCompatToken        = "Compat"
	cleanUpgradeShimToken          = "Shim"
	cleanUpgradeBackwardToken      = "Backward"
	cleanUpgradeFallbackToken      = "Fallback"
)

type cleanUpgradeDebt struct {
	file  string
	name  string
	owner PackageIdentity
	kind  cleanUpgradeDebtKind
}

type cleanUpgradeDebtInventory struct {
	values [cleanUpgradeDebtMaximum]cleanUpgradeDebt
	count  uint8
}

func TestCleanUpgradeDebtIsAbsentFromLandedProduction(t *testing.T) {
	t.Parallel()

	got, err := scanLandedCleanUpgradeDebt("..")
	if err != nil {
		t.Fatalf("scanLandedCleanUpgradeDebt() error = %v, want nil", err)
	}
	if got.count != 0 {
		t.Fatalf("clean-upgrade debt = %+v, want none", got.Values())
	}
}

func TestCleanUpgradeDebtLayerTriad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		want   []cleanUpgradeDebt
		source realWorldSource
	}{
		{
			name: "positive typed provider binder owns a semantic choice",
			source: realWorldSource{owner: PackageObjectStore, name: "client.go", source: []byte(`package objectstore
type Provider uint8
const ProviderGCS Provider = 1
type Request struct{}
type Transfer struct{}
func upload(request Request, provider Provider) Transfer { return Transfer{} }
func UploadGCS(request Request) Transfer { return upload(request, ProviderGCS) }
`)},
		},
		{
			name: "negative aliases deprecated declarations names files and forwarders stay visible",
			source: realWorldSource{owner: PackageRelease, name: "release_compat.go", source: []byte(`package release
type Request struct{}
type Alias = Request
// Deprecated: use Build.
func OldBuild(request Request) error { return Build(request) }
func Build(request Request) error { return nil }
const LegacyMode = 1
func CompatibilityShim() {}
`)},
			want: []cleanUpgradeDebt{
				{owner: PackageRelease, kind: cleanUpgradeDebtTypeAlias, file: "release_compat.go", name: "Alias"},
				{owner: PackageRelease, kind: cleanUpgradeDebtDeprecatedDeclaration, file: "release_compat.go", name: "OldBuild"},
				{owner: PackageRelease, kind: cleanUpgradeDebtCompatibilityName, file: "release_compat.go", name: "CompatibilityShim"},
				{owner: PackageRelease, kind: cleanUpgradeDebtCompatibilityName, file: "release_compat.go", name: "LegacyMode"},
				{owner: PackageRelease, kind: cleanUpgradeDebtCompatibilityFile, file: "release_compat.go"},
				{owner: PackageRelease, kind: cleanUpgradeDebtExportedForwarder, file: "release_compat.go", name: "OldBuild"},
			},
		},
		{
			name: "neutral internal reconstruction helper is not a second public API",
			source: realWorldSource{owner: PackageRelease, name: "build.go", source: []byte(`package release
type Request struct{}
func Build(request Request) error { return build(request) }
func build(request Request) error { return nil }
`)},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := scanCleanUpgradeSources([]realWorldSource{testCase.source})
			if err != nil {
				t.Fatalf("scanCleanUpgradeSources() error = %v, want nil", err)
			}
			if !slices.Equal(got.Values(), testCase.want) {
				t.Fatalf("scanCleanUpgradeSources() = %+v, want %+v", got.Values(), testCase.want)
			}
		})
	}
}

func scanLandedCleanUpgradeDebt(root string) (cleanUpgradeDebtInventory, error) {
	paths, err := productionSourcePaths(root)
	if err != nil {
		return cleanUpgradeDebtInventory{}, err
	}
	sources := make([]realWorldSource, 0, len(paths))
	for _, path := range paths {
		owner, err := realWorldSourceOwner(path)
		if err != nil {
			return cleanUpgradeDebtInventory{}, err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return cleanUpgradeDebtInventory{}, errors.Join(ErrPrimitiveContract, err)
		}
		sources = append(sources, realWorldSource{owner: owner, name: path, source: source})
	}
	return scanCleanUpgradeSources(sources)
}

func scanCleanUpgradeSources(sources []realWorldSource) (cleanUpgradeDebtInventory, error) {
	var result cleanUpgradeDebtInventory
	for _, source := range sources {
		if err := scanCleanUpgradeSource(source, &result); err != nil {
			return cleanUpgradeDebtInventory{}, err
		}
	}
	result.Sort()
	return result, nil
}

func scanCleanUpgradeSource(source realWorldSource, result *cleanUpgradeDebtInventory) error {
	if err := source.owner.Validate(); err != nil || result == nil {
		return errors.Join(ErrPrimitiveContract, err)
	}
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, source.name, source.source, parser.ParseComments)
	if err != nil {
		return errors.Join(ErrPrimitiveContract, err)
	}
	name := filepath.Base(source.name)
	if cleanUpgradeCompatibilityFile(name) {
		if err := result.Add(cleanUpgradeDebt{owner: source.owner, kind: cleanUpgradeDebtCompatibilityFile, file: name}); err != nil {
			return err
		}
	}
	for _, declaration := range file.Decls {
		if err := scanCleanUpgradeDeclaration(cleanUpgradeDeclarationInput{
			owner: source.owner, file: name, declaration: declaration, result: result,
		}); err != nil {
			return err
		}
	}
	return nil
}

type cleanUpgradeDeclarationScan struct {
	result *cleanUpgradeDebtInventory
	file   string
	name   string
	owner  PackageIdentity
}

type cleanUpgradeDeclarationInput struct {
	declaration ast.Decl
	result      *cleanUpgradeDebtInventory
	file        string
	owner       PackageIdentity
}

func scanCleanUpgradeDeclaration(input cleanUpgradeDeclarationInput) error {
	scan := cleanUpgradeDeclarationScan{owner: input.owner, file: input.file, result: input.result}
	switch value := input.declaration.(type) {
	case *ast.FuncDecl:
		scan.name = value.Name.Name
		if err := scan.recordDeclarationDebt(value.Doc); err != nil {
			return err
		}
		if exportedPureForwarder(value) {
			return input.result.Add(cleanUpgradeDebt{
				owner: input.owner, kind: cleanUpgradeDebtExportedForwarder, file: input.file, name: scan.name,
			})
		}
	case *ast.GenDecl:
		for _, specification := range value.Specs {
			if err := scanCleanUpgradeSpecification(value.Doc, specification, scan); err != nil {
				return err
			}
		}
	}
	return nil
}

func scanCleanUpgradeSpecification(
	declarationDocumentation *ast.CommentGroup,
	specification ast.Spec,
	scan cleanUpgradeDeclarationScan,
) error {
	switch value := specification.(type) {
	case *ast.TypeSpec:
		scan.name = value.Name.Name
		if err := scan.recordDeclarationDebt(firstDocumentation(value.Doc, declarationDocumentation)); err != nil {
			return err
		}
		if value.Assign.IsValid() {
			return scan.result.Add(cleanUpgradeDebt{
				owner: scan.owner, kind: cleanUpgradeDebtTypeAlias, file: scan.file, name: scan.name,
			})
		}
	case *ast.ValueSpec:
		for _, name := range value.Names {
			scan.name = name.Name
			if err := scan.recordDeclarationDebt(firstDocumentation(value.Doc, declarationDocumentation)); err != nil {
				return err
			}
		}
	}
	return nil
}

func firstDocumentation(primary, fallback *ast.CommentGroup) *ast.CommentGroup {
	if primary != nil {
		return primary
	}
	return fallback
}

func (s cleanUpgradeDeclarationScan) recordDeclarationDebt(documentation *ast.CommentGroup) error {
	if cleanUpgradeDeprecatedDocumentation(documentation) {
		if err := s.result.Add(cleanUpgradeDebt{owner: s.owner, kind: cleanUpgradeDebtDeprecatedDeclaration, file: s.file, name: s.name}); err != nil {
			return err
		}
	}
	if cleanUpgradeCompatibilityName(s.name) {
		return s.result.Add(cleanUpgradeDebt{owner: s.owner, kind: cleanUpgradeDebtCompatibilityName, file: s.file, name: s.name})
	}
	return nil
}

func cleanUpgradeDeprecatedDocumentation(documentation *ast.CommentGroup) bool {
	if documentation == nil {
		return false
	}
	for line := range strings.SplitSeq(documentation.Text(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), cleanUpgradeDeprecatedPrefix) {
			return true
		}
	}
	return false
}

func cleanUpgradeCompatibilityName(name string) bool {
	for _, token := range [...]string{
		cleanUpgradeLegacyToken,
		cleanUpgradeDeprecatedToken,
		cleanUpgradeCompatibilityToken,
		cleanUpgradeCompatToken,
		cleanUpgradeShimToken,
		cleanUpgradeBackwardToken,
		cleanUpgradeFallbackToken,
	} {
		if strings.Contains(name, token) {
			return true
		}
		lowerName := strings.ToLower(name)
		lowerToken := strings.ToLower(token)
		if strings.HasPrefix(lowerName, lowerToken) || strings.HasSuffix(lowerName, lowerToken) {
			return true
		}
	}
	return false
}

func cleanUpgradeCompatibilityFile(name string) bool {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	for component := range strings.SplitSeq(stem, "_") {
		switch strings.ToLower(component) {
		case "legacy", "deprecated", "compat", "compatibility", "shim", "backward", "fallback":
			return true
		}
	}
	return false
}

func exportedPureForwarder(declaration *ast.FuncDecl) bool {
	if declaration == nil || !ast.IsExported(declaration.Name.Name) || declaration.Body == nil || len(declaration.Body.List) != 1 {
		return false
	}
	returned, ok := declaration.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(returned.Results) != 1 {
		return false
	}
	call, ok := returned.Results[0].(*ast.CallExpr)
	if !ok || !exportedLocalCall(call.Fun) {
		return false
	}
	parameters, ok := namedFunctionParameters(declaration.Type.Params)
	if !ok || len(parameters) != len(call.Args) {
		return false
	}
	for index, parameter := range parameters {
		argument, ok := call.Args[index].(*ast.Ident)
		if !ok || argument.Name != parameter {
			return false
		}
	}
	return true
}

func exportedLocalCall(expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)
	if !ok || identifier.Name == "" {
		return false
	}
	first, _ := utf8.DecodeRuneInString(identifier.Name)
	return unicode.IsUpper(first)
}

func namedFunctionParameters(fields *ast.FieldList) ([]string, bool) {
	if fields == nil {
		return nil, true
	}
	parameters := make([]string, 0, fields.NumFields())
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			return nil, false
		}
		for _, name := range field.Names {
			parameters = append(parameters, name.Name)
		}
	}
	return parameters, true
}

func (i *cleanUpgradeDebtInventory) Add(value cleanUpgradeDebt) error {
	if i == nil || value.kind <= cleanUpgradeDebtUnknown || value.kind >= cleanUpgradeDebtLimit {
		return architectureContractError("clean-upgrade debt is invalid")
	}
	if err := value.owner.Validate(); err != nil || value.file == "" {
		return errors.Join(architectureContractError("clean-upgrade debt is incomplete"), err)
	}
	if i.count >= cleanUpgradeDebtMaximum {
		return architectureContractError("clean-upgrade debt inventory exceeds its fixed capacity")
	}
	i.values[i.count] = value
	i.count++
	return nil
}

func (i *cleanUpgradeDebtInventory) Sort() {
	if i == nil {
		return
	}
	slices.SortFunc(i.values[:i.count], func(left, right cleanUpgradeDebt) int {
		if difference := int(left.owner) - int(right.owner); difference != 0 {
			return difference
		}
		if difference := int(left.kind) - int(right.kind); difference != 0 {
			return difference
		}
		if difference := strings.Compare(left.file, right.file); difference != 0 {
			return difference
		}
		return strings.Compare(left.name, right.name)
	})
}

func (i cleanUpgradeDebtInventory) Values() []cleanUpgradeDebt {
	return slices.Clone(i.values[:i.count])
}
