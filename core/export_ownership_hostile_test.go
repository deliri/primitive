package core

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	coreExportInventoryMaximum      = 512
	coreExportDependencyMaximum     = 32
	coreSpecialExportAdmissionCount = 65
)

type coreExportName string

type coreExportConsumerContract struct {
	name              coreExportName
	dependencies      [coreExportDependencyMaximum]coreExportName
	directConsumers   [PrimitivePackageCount + 1]bool
	consumers         [PrimitivePackageCount + 1]bool
	errorProducers    [PrimitivePackageCount + 1]bool
	errorDecisions    [PrimitivePackageCount + 1]bool
	dependencyCount   uint8
	stableErr         bool
	typedDomainMember bool
}

type coreExportInventory struct {
	values                [coreExportInventoryMaximum]coreExportConsumerContract
	packageErrorDecisions [PrimitivePackageCount + 1]bool
	count                 uint16
}

type coreSpecialExportAdmission struct {
	witness any
	name    coreExportName
	reason  coreSpecialExportAdmissionReason
}

type coreSpecialExportAdmissionReason uint8

const (
	coreSpecialExportAdmissionReasonUnknown coreSpecialExportAdmissionReason = iota
	coreSpecialExportAdmissionReasonArchitectureCatalog
	coreSpecialExportAdmissionReasonTestIsolationContract
	coreSpecialExportAdmissionReasonCoherentDomainContract
	coreSpecialExportAdmissionReasonLimit
)

func coreSpecialExportAdmissionReasonTexts() [coreSpecialExportAdmissionReasonLimit]string {
	return [...]string{
		coreSpecialExportAdmissionReasonArchitectureCatalog:    "compiler-owned architecture catalog is Core's self-projection",
		coreSpecialExportAdmissionReasonTestIsolationContract:  "external test-isolation analyzer ABI is Core's self-projection",
		coreSpecialExportAdmissionReasonCoherentDomainContract: "export is an invariant of its Core-owned coherent domain",
	}
}

func (r coreSpecialExportAdmissionReason) Validate() error {
	if r <= coreSpecialExportAdmissionReasonUnknown ||
		r >= coreSpecialExportAdmissionReasonLimit ||
		coreSpecialExportAdmissionReasonTexts()[r] == "" {
		return architectureContractError("Core special export admission reason is invalid")
	}
	return nil
}

func architectureCatalogAdmission(name coreExportName, witness any) coreSpecialExportAdmission {
	return coreSpecialExportAdmission{
		name: name, witness: witness,
		reason: coreSpecialExportAdmissionReasonArchitectureCatalog,
	}
}

func testIsolationContractAdmission(name coreExportName, witness any) coreSpecialExportAdmission {
	return coreSpecialExportAdmission{
		name: name, witness: witness,
		reason: coreSpecialExportAdmissionReasonTestIsolationContract,
	}
}

func coherentDomainContractAdmission(name coreExportName, witness any) coreSpecialExportAdmission {
	return coreSpecialExportAdmission{
		name: name, witness: witness,
		reason: coreSpecialExportAdmissionReasonCoherentDomainContract,
	}
}

// These are PLAN's compiler-owned architecture catalog, not ordinary shared
// facts. Each name is paired with a live identifier reference, so a declaration
// rename breaks the test build instead of changing policy through a filename.
func coreSpecialExportAdmissions() [coreSpecialExportAdmissionCount]coreSpecialExportAdmission {
	return [...]coreSpecialExportAdmission{
		architectureCatalogAdmission("ArchitectureCatalog", ArchitectureCatalog{}),
		architectureCatalogAdmission("DirectImportContract", DirectImportContract{}),
		architectureCatalogAdmission("DirectTestImportContract", DirectTestImportContract{}),
		architectureCatalogAdmission("PackageContract", PackageContract{}),
		architectureCatalogAdmission("PackageIdentity", PackageIdentity(0)),
		architectureCatalogAdmission("PackageKind", PackageKind(0)),
		architectureCatalogAdmission("PackageKindUnknown", PackageKindUnknown),
		architectureCatalogAdmission("PackageKindProduction", PackageKindProduction),
		architectureCatalogAdmission("PackageKindTestSupport", PackageKindTestSupport),
		architectureCatalogAdmission("PackageUnknown", PackageUnknown),
		architectureCatalogAdmission("PackageCore", PackageCore),
		architectureCatalogAdmission("PackageAttest", PackageAttest),
		architectureCatalogAdmission("PackageContextState", PackageContextState),
		architectureCatalogAdmission("PackageCurrency", PackageCurrency),
		architectureCatalogAdmission("PackageKeygen", PackageKeygen),
		architectureCatalogAdmission("PackageTestSerial", PackageTestSerial),
		architectureCatalogAdmission("PackageFilestore", PackageFilestore),
		architectureCatalogAdmission("PackageHostFacts", PackageHostFacts),
		architectureCatalogAdmission("PackageTemporal", PackageTemporal),
		architectureCatalogAdmission("PackageExchange", PackageExchange),
		architectureCatalogAdmission("PackageFuzzFinder", PackageFuzzFinder),
		architectureCatalogAdmission("PackageLease", PackageLease),
		architectureCatalogAdmission("PackageGate", PackageGate),
		architectureCatalogAdmission("PackageReceipt", PackageReceipt),
		architectureCatalogAdmission("PackageProcess", PackageProcess),
		architectureCatalogAdmission("PackageRelease", PackageRelease),
		architectureCatalogAdmission("PackageShutdown", PackageShutdown),
		architectureCatalogAdmission("PackageObjectStore", PackageObjectStore),
		architectureCatalogAdmission("PackageTimeProof", PackageTimeProof),
		architectureCatalogAdmission("PackageCloudIdentity", PackageCloudIdentity),
		architectureCatalogAdmission("PackageUpgrade", PackageUpgrade),
		architectureCatalogAdmission("PackageWiring", PackageWiring),
		architectureCatalogAdmission("PackageLineIO", PackageLineIO),
		architectureCatalogAdmission("PackageManual", PackageManual),
		architectureCatalogAdmission("ParsePackageIdentity", ParsePackageIdentity),
		architectureCatalogAdmission("PrimitiveArchitecture", PrimitiveArchitecture),
		architectureCatalogAdmission("PrimitivePackageCount", PrimitivePackageCount),
		architectureCatalogAdmission("PrimitiveDirectImportCount", PrimitiveDirectImportCount),
		architectureCatalogAdmission("PrimitiveDirectTestImportCount", PrimitiveDirectTestImportCount),
		architectureCatalogAdmission("PrimitiveMaximumDirectImports", PrimitiveMaximumDirectImports),
		architectureCatalogAdmission("PrimitiveModulePath", PrimitiveModulePath),
		architectureCatalogAdmission("PrimitivePackagePathPrefix", PrimitivePackagePathPrefix),
		coherentDomainContractAdmission("SecretMaterialMaximumBytes", SecretMaterialMaximumBytes),
		testIsolationContractAdmission("TestIsolationCorePackagePath", TestIsolationCorePackagePath),
		testIsolationContractAdmission("TestIsolationDeclarationPackagePath", TestIsolationDeclarationPackagePath),
		testIsolationContractAdmission("TestIsolationDeclarationFunctionName", TestIsolationDeclarationFunctionName),
		testIsolationContractAdmission("TestIsolationDeclarationTypeName", TestIsolationDeclarationTypeName),
		testIsolationContractAdmission("TestIsolationDeclarationHazardFieldName", TestIsolationDeclarationHazardFieldName),
		testIsolationContractAdmission("TestIsolationDeclarationScopeFieldName", TestIsolationDeclarationScopeFieldName),
		testIsolationContractAdmission("TestIsolationHazard", TestIsolationHazard(0)),
		testIsolationContractAdmission("TestIsolationHazardUnknown", TestIsolationHazardUnknown),
		testIsolationContractAdmission("TestIsolationHazardProcessEnvironment", TestIsolationHazardProcessEnvironment),
		testIsolationContractAdmission("TestIsolationHazardProcessWorkingDirectory", TestIsolationHazardProcessWorkingDirectory),
		testIsolationContractAdmission("TestIsolationHazardProcessSignal", TestIsolationHazardProcessSignal),
		testIsolationContractAdmission("TestIsolationHazardProcessOutput", TestIsolationHazardProcessOutput),
		testIsolationContractAdmission("TestIsolationHazardProcessLogger", TestIsolationHazardProcessLogger),
		testIsolationContractAdmission("TestIsolationHazardGlobalRegistry", TestIsolationHazardGlobalRegistry),
		testIsolationContractAdmission("TestIsolationHazardRuntimeAllocation", TestIsolationHazardRuntimeAllocation),
		testIsolationContractAdmission("TestIsolationHazardSiblingOrder", TestIsolationHazardSiblingOrder),
		testIsolationContractAdmission("TestIsolationScope", TestIsolationScope(0)),
		testIsolationContractAdmission("TestIsolationScopeUnknown", TestIsolationScopeUnknown),
		testIsolationContractAdmission("TestIsolationScopeSiblingTable", TestIsolationScopeSiblingTable),
		testIsolationContractAdmission("TestIsolationScopePackageProcess", TestIsolationScopePackageProcess),
		testIsolationContractAdmission("TestIsolationDeclaration", TestIsolationDeclaration{}),
		testIsolationContractAdmission("ErrTestIsolationContract", ErrTestIsolationContract),
	}
}

func TestCoreTopLevelExportsHaveTwoNamedPrimitiveConsumers(t *testing.T) {
	t.Parallel()

	exports, err := collectCoreTopLevelExports(".")
	if err != nil {
		t.Fatalf("collectCoreTopLevelExports() error = %v, want nil", err)
	}
	if err := collectCoreExportConsumers("..", &exports); err != nil {
		t.Fatalf("collectCoreExportConsumers() error = %v, want nil", err)
	}
	connectDirectConsumerTypeDependencies(&exports)
	connectTypedDomainMemberConsumers(&exports)
	admissions := coreSpecialExportAdmissions()
	for index, admission := range admissions {
		if admission.witness == nil {
			t.Errorf("Core special admission %s has no compiler witness", admission.name)
		}
		if _, ok := exports.Lookup(admission.name); !ok {
			t.Errorf("Core special admission %s names no production export", admission.name)
		}
		if err := admission.reason.Validate(); err != nil {
			t.Errorf("Core special admission %s reason error = %v, want nil", admission.name, err)
		}
		for prior := range index {
			if admissions[prior].name == admission.name {
				t.Errorf("Core special admission %s is duplicated", admission.name)
			}
		}
	}
	for _, contract := range exports.Values() {
		if coreExportIsSpeciallyAdmitted(admissions, contract.name) {
			continue
		}
		if contract.stableErr {
			if !contract.hasErrorProducer() || !exports.hasErrorDecision(contract) {
				t.Errorf(
					"Core stable error %s producer=%t caller-decision=%t, want both",
					contract.name,
					contract.hasErrorProducer(),
					exports.hasErrorDecision(contract),
				)
			}
			continue
		}
		consumers := contract.ConsumerIdentities()
		if len(consumers) >= 2 {
			continue
		}
		t.Errorf("Core export %s has %d named Primitive consumers %v, want at least 2", contract.name, len(consumers), consumers)
	}
}

func TestTypedDomainMemberConsumerProjectionDoesNotLaunderUntypedExports(t *testing.T) {
	t.Parallel()

	var inventory coreExportInventory
	domain := coreExportConsumerContract{name: "SharedDomain"}
	domain.consumers[PackageRelease] = true
	domain.consumers[PackageUpgrade] = true
	if err := inventory.Add(domain); err != nil {
		t.Fatalf("Add(shared domain) error = %v, want nil", err)
	}
	for _, contract := range []coreExportConsumerContract{
		{name: "TypedMember", typedDomainMember: true},
		{name: "UntypedFact"},
	} {
		if err := inventory.Add(contract); err != nil {
			t.Fatalf("Add(%s) error = %v, want nil", contract.name, err)
		}
		added, found := inventory.Lookup(contract.name)
		if !found {
			t.Fatalf("Lookup(%s) found = false, want true", contract.name)
		}
		if err := added.AddDependency("SharedDomain"); err != nil {
			t.Fatalf("%s.AddDependency() error = %v, want nil", contract.name, err)
		}
	}

	connectTypedDomainMemberConsumers(&inventory)
	typed, found := inventory.Lookup("TypedMember")
	if !found {
		t.Fatal("Lookup(TypedMember) found = false, want true")
	}
	if got := typed.ConsumerIdentities(); !slices.Equal(got, []PackageIdentity{PackageRelease, PackageUpgrade}) {
		t.Fatalf("typed member consumers = %v, want [%v %v]", got, PackageRelease, PackageUpgrade)
	}
	untyped, found := inventory.Lookup("UntypedFact")
	if !found {
		t.Fatal("Lookup(UntypedFact) found = false, want true")
	}
	if got := untyped.ConsumerIdentities(); len(got) != 0 {
		t.Fatalf("untyped fact consumers = %v, want none", got)
	}
}

func coreExportIsSpeciallyAdmitted(
	admissions [coreSpecialExportAdmissionCount]coreSpecialExportAdmission,
	name coreExportName,
) bool {
	return slices.ContainsFunc(admissions[:], func(admission coreSpecialExportAdmission) bool {
		return admission.name == name && admission.witness != nil && admission.reason.Validate() == nil
	})
}

func collectCoreTopLevelExports(directory string) (coreExportInventory, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return coreExportInventory{}, err
	}
	files := token.NewFileSet()
	var exports coreExportInventory
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(files, filepath.Join(directory, entry.Name()), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return coreExportInventory{}, parseErr
		}
		for _, declaration := range file.Decls {
			if err := addCoreExportDeclaration(&exports, declaration); err != nil {
				return coreExportInventory{}, err
			}
		}
	}
	if err := collectCoreExportTypeDependencies(directory, &exports); err != nil {
		return coreExportInventory{}, err
	}
	if err := collectCoreLocalErrorProducers(directory, &exports); err != nil {
		return coreExportInventory{}, err
	}
	slices.SortFunc(exports.Values(), func(left, right coreExportConsumerContract) int {
		return strings.Compare(string(left.name), string(right.name))
	})
	return exports, nil
}

func collectCoreLocalErrorProducers(directory string, exports *coreExportInventory) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, entry.Name()), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil || function.Name.Name == "errorIdentityDiagnostics" {
				continue
			}
			ast.Inspect(function.Body, func(node ast.Node) bool {
				identifier, ok := node.(*ast.Ident)
				if !ok {
					return true
				}
				contract, found := exports.Lookup(coreExportName(identifier.Name))
				if found && contract.stableErr {
					contract.errorProducers[PackageCore] = true
				}
				return true
			})
		}
	}
	return nil
}

func addCoreExportDeclaration(exports *coreExportInventory, declaration ast.Decl) error {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		if typed.Recv == nil && typed.Name.IsExported() {
			return exports.Add(coreExportConsumerContract{name: coreExportName(typed.Name.Name)})
		}
	case *ast.GenDecl:
		stableErrorIdentity := false
		for _, rawSpec := range typed.Specs {
			switch spec := rawSpec.(type) {
			case *ast.TypeSpec:
				if spec.Name.IsExported() {
					if err := exports.Add(coreExportConsumerContract{name: coreExportName(spec.Name.Name)}); err != nil {
						return err
					}
				}
			case *ast.ValueSpec:
				if spec.Type != nil {
					identity, ok := spec.Type.(*ast.Ident)
					stableErrorIdentity = ok && identity.Name == "ErrorIdentity"
				}
				for _, name := range spec.Names {
					if name.IsExported() {
						if err := exports.Add(coreExportConsumerContract{
							name:      coreExportName(name.Name),
							stableErr: stableErrorIdentity && name.Name != "ErrUnknown",
						}); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func collectCoreExportTypeDependencies(directory string, exports *coreExportInventory) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), filepath.Join(directory, entry.Name()), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		for _, declaration := range file.Decls {
			if err := addCoreExportTypeDependencies(exports, declaration); err != nil {
				return err
			}
		}
	}
	return nil
}

func addCoreExportTypeDependencies(exports *coreExportInventory, declaration ast.Decl) error {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		owner, ok := coreExportFunctionOwner(typed)
		if !ok {
			return nil
		}
		return addCoreExportTypeExpression(exports, owner, typed.Type)
	case *ast.GenDecl:
		var inheritedType ast.Expr
		for _, rawSpec := range typed.Specs {
			switch spec := rawSpec.(type) {
			case *ast.TypeSpec:
				if spec.Name.IsExported() {
					if err := addCoreExportTypeExpression(exports, coreExportName(spec.Name.Name), spec.Type); err != nil {
						return err
					}
				}
			case *ast.ValueSpec:
				if spec.Type != nil {
					inheritedType = spec.Type
				} else if typed.Tok != token.CONST {
					inheritedType = nil
				}
				for _, name := range spec.Names {
					if name.IsExported() && inheritedType != nil {
						if typed.Tok == token.CONST {
							identifier, ok := inheritedType.(*ast.Ident)
							if ok && identifier.IsExported() {
								if contract, found := exports.Lookup(coreExportName(name.Name)); found {
									contract.typedDomainMember = true
								}
							}
						}
						if err := addCoreExportTypeExpression(exports, coreExportName(name.Name), inheritedType); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func coreExportFunctionOwner(function *ast.FuncDecl) (coreExportName, bool) {
	if function.Recv == nil {
		return coreExportName(function.Name.Name), function.Name.IsExported()
	}
	if !function.Name.IsExported() || len(function.Recv.List) != 1 {
		return "", false
	}
	receiver := function.Recv.List[0].Type
	if pointer, ok := receiver.(*ast.StarExpr); ok {
		receiver = pointer.X
	}
	identifier, ok := receiver.(*ast.Ident)
	if !ok || !identifier.IsExported() {
		return "", false
	}
	return coreExportName(identifier.Name), true
}

func addCoreExportTypeExpression(
	exports *coreExportInventory,
	owner coreExportName,
	expression ast.Expr,
) error {
	contract, found := exports.Lookup(owner)
	if !found {
		return nil
	}
	var addErr error
	ast.Inspect(expression, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok || !identifier.IsExported() || addErr != nil {
			return addErr == nil
		}
		dependency := coreExportName(identifier.Name)
		if dependency != owner && exports.Contains(dependency) {
			addErr = contract.AddDependency(dependency)
		}
		return addErr == nil
	})
	return addErr
}

func connectDirectConsumerTypeDependencies(exports *coreExportInventory) {
	for index := range int(exports.count) {
		source := &exports.values[index]
		for _, dependencyName := range source.Dependencies() {
			dependency, ok := exports.Lookup(dependencyName)
			if !ok {
				continue
			}
			for identity := PackageCore; identity < packageIdentityLimit; identity++ {
				if source.directConsumers[identity] {
					dependency.consumers[identity] = true
				}
			}
		}
	}
}

// connectTypedDomainMemberConsumers projects a shared named enum's consumers
// onto its explicitly typed constants. A package accepting the enum accepts
// every admitted member even when it does not spell each constant in source.
// Untyped constants and arbitrary declaration dependencies receive no such
// projection.
func connectTypedDomainMemberConsumers(exports *coreExportInventory) {
	for index := range int(exports.count) {
		member := &exports.values[index]
		if !member.typedDomainMember || len(member.Dependencies()) != 1 {
			continue
		}
		domain, ok := exports.Lookup(member.Dependencies()[0])
		if !ok {
			continue
		}
		for identity := PackageCore; identity < packageIdentityLimit; identity++ {
			if domain.consumers[identity] {
				member.consumers[identity] = true
			}
		}
	}
}

func collectCoreExportConsumers(root string, exports *coreExportInventory) error {
	for packageContract := range PrimitiveArchitecture().Packages() {
		if packageContract.Identity == PackageCore {
			continue
		}
		name, err := packageContract.Identity.Name()
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(filepath.Join(root, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			if err := collectCoreFileConsumers(filepath.Join(root, name, entry.Name()), packageContract.Identity, exports); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectCoreFileConsumers(filename string, consumer PackageIdentity, exports *coreExportInventory) error {
	file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	aliases, err := coreImportAliases(file)
	if err != nil || len(aliases) == 0 {
		return err
	}
	errorAliases, err := packageImportAliases(file, "errors")
	if err != nil {
		return err
	}
	if coreFileCallsErrorsIs(file, errorAliases) {
		exports.packageErrorDecisions[consumer] = true
	}
	decisionPositions := coreStableErrorDecisionPositions(
		file,
		aliases,
		errorAliases,
		consumer,
		exports,
	)
	production := !strings.HasSuffix(filename, "_test.go")
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		owner, ok := selector.X.(*ast.Ident)
		if !ok || !slices.Contains(aliases, owner.Name) {
			return true
		}
		if contract, found := exports.Lookup(coreExportName(selector.Sel.Name)); found {
			contract.directConsumers[consumer] = true
			contract.consumers[consumer] = true
			if contract.stableErr && production && !slices.Contains(decisionPositions, selector.Pos()) {
				contract.errorProducers[consumer] = true
			}
		}
		return true
	})
	return nil
}

func coreFileCallsErrorsIs(file *ast.File, errorAliases []string) bool {
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		function, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || function.Sel.Name != "Is" {
			return true
		}
		owner, ok := function.X.(*ast.Ident)
		if ok && slices.Contains(errorAliases, owner.Name) {
			found = true
			return false
		}
		return true
	})
	return found
}

func coreStableErrorDecisionPositions(
	file *ast.File,
	coreAliases []string,
	errorAliases []string,
	consumer PackageIdentity,
	exports *coreExportInventory,
) []token.Pos {
	positions := make([]token.Pos, 0, 16)
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			return true
		}
		function, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || function.Sel.Name != "Is" {
			return true
		}
		packageName, ok := function.X.(*ast.Ident)
		if !ok || !slices.Contains(errorAliases, packageName.Name) {
			return true
		}
		target, ok := call.Args[1].(*ast.SelectorExpr)
		if !ok {
			return true
		}
		owner, ok := target.X.(*ast.Ident)
		if !ok || !slices.Contains(coreAliases, owner.Name) {
			return true
		}
		contract, found := exports.Lookup(coreExportName(target.Sel.Name))
		if found && contract.stableErr {
			contract.errorDecisions[consumer] = true
			positions = append(positions, target.Pos())
		}
		return true
	})
	return positions
}

func coreImportAliases(file *ast.File) ([]string, error) {
	return packageImportAliases(file, PrimitivePackagePathPrefix+"core")
}

func packageImportAliases(file *ast.File, wantedPath string) ([]string, error) {
	var aliases []string
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return nil, err
		}
		if path != wantedPath {
			continue
		}
		alias := filepath.Base(wantedPath)
		if imported.Name != nil {
			alias = imported.Name.Name
		}
		if alias == "." || alias == "_" {
			return nil, architectureContractError("Core ownership audit refuses dot or blank imports")
		}
		aliases = append(aliases, alias)
	}
	return aliases, nil
}

func (i *coreExportInventory) Add(contract coreExportConsumerContract) error {
	if contract.name == "" {
		return architectureContractError("Core export name is empty")
	}
	if i.Contains(contract.name) {
		return architectureContractError("Core export is declared more than once: " + string(contract.name))
	}
	if int(i.count) >= len(i.values) {
		return architectureContractError("Core export inventory capacity exceeded")
	}
	i.values[i.count] = contract
	i.count++
	return nil
}

func (i coreExportInventory) Contains(name coreExportName) bool {
	_, ok := i.Lookup(name)
	return ok
}

func (i *coreExportInventory) Lookup(name coreExportName) (*coreExportConsumerContract, bool) {
	for index := range int(i.count) {
		if i.values[index].name == name {
			return &i.values[index], true
		}
	}
	return nil, false
}

func (i *coreExportInventory) Values() []coreExportConsumerContract {
	return i.values[:i.count]
}

func (c *coreExportConsumerContract) AddDependency(dependency coreExportName) error {
	if dependency == "" {
		return ErrPrimitiveContract
	}
	if slices.Contains(c.Dependencies(), dependency) {
		return nil
	}
	if int(c.dependencyCount) >= len(c.dependencies) {
		return ErrPrimitiveContract
	}
	c.dependencies[c.dependencyCount] = dependency
	c.dependencyCount++
	return nil
}

func (c *coreExportConsumerContract) Dependencies() []coreExportName {
	return c.dependencies[:c.dependencyCount]
}

func (c coreExportConsumerContract) ConsumerIdentities() []PackageIdentity {
	consumers := make([]PackageIdentity, 0, PrimitivePackageCount)
	for identity := PackageCore; identity < packageIdentityLimit; identity++ {
		if c.consumers[identity] {
			consumers = append(consumers, identity)
		}
	}
	return consumers
}

func (c coreExportConsumerContract) hasErrorProducer() bool {
	return slices.Contains(c.errorProducers[:], true)
}

func (i coreExportInventory) hasErrorDecision(contract coreExportConsumerContract) bool {
	if slices.Contains(contract.errorDecisions[:], true) {
		return true
	}
	for identity := PackageCore; identity < packageIdentityLimit; identity++ {
		if contract.consumers[identity] && i.packageErrorDecisions[identity] {
			return true
		}
	}
	return false
}
