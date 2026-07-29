package core

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"
)

type directImportInventory struct {
	values [PrimitiveDirectImportCount]DirectImportContract
	count  uint8
}

type architectureImportViolationKind uint8

const (
	architectureImportViolationUnknown architectureImportViolationKind = iota
	architectureImportViolationMissing
	architectureImportViolationExtra
)

type architectureImportViolation struct {
	contract DirectImportContract
	kind     architectureImportViolationKind
}

const architectureImportViolationMaximum = PrimitiveDirectImportCount * 2

type architectureImportViolationInventory struct {
	values [architectureImportViolationMaximum]architectureImportViolation
	count  uint8
}

type syntheticGoFile struct {
	name   string
	source string
}

type syntheticGoFileSet struct {
	values [3]syntheticGoFile
	count  uint8
}

func TestPrimitiveModulePathMatchesCompilerBuildInformation(t *testing.T) {
	t.Parallel()

	build, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("debug.ReadBuildInfo() available = false, want true")
	}
	if build.Main.Path != PrimitiveModulePath {
		t.Fatalf("debug.ReadBuildInfo().Main.Path = %q, want %q", build.Main.Path, PrimitiveModulePath)
	}
}

func TestLandedPackageImportsMatchPrimitiveArchitecture(t *testing.T) {
	t.Parallel()

	catalog := PrimitiveArchitecture()
	for packageContract := range catalog.Packages() {
		gotExists, gotViolations, gotErr := auditPackageImports("..", packageContract.Identity, catalog)
		if gotErr != nil {
			t.Fatalf("auditPackageImports(%v) error = %v, want nil", packageContract.Identity, gotErr)
		}
		if !gotExists {
			continue
		}
		for _, gotViolation := range gotViolations.Values() {
			t.Errorf(
				"landed package import violation kind=%v edge=%v -> %v",
				gotViolation.kind,
				gotViolation.contract.Importer,
				gotViolation.contract.Imported,
			)
		}
	}
}

func TestArchitectureImportMatcherSyntheticRedGreenRatchet(t *testing.T) {
	t.Parallel()

	corePath := mustPackageImportPathForTest(t, PackageCore)
	contextStatePath := mustPackageImportPathForTest(t, PackageContextState)
	attestPath := mustPackageImportPathForTest(t, PackageAttest)
	unknownPath := PrimitivePackagePathPrefix + "notadmitted"
	nestedCorePath := corePath + "/internal"

	cases := []struct {
		wantErr     error
		name        string
		files       syntheticGoFileSet
		wantMissing int
		wantExtra   int
		identity    PackageIdentity
		wantExists  bool
	}{
		{name: "absent package is a neutral not-landed result", identity: PackageCore},
		{
			name:       "landed core with no imports matches its empty frontier",
			identity:   PackageCore,
			files:      oneSyntheticGoFile("core.go", goSourceWithImports()),
			wantExists: true,
		},
		{
			name:       "standard-library import is outside the sibling graph",
			identity:   PackageCore,
			files:      oneSyntheticGoFile("core.go", goSourceWithImports("errors")),
			wantExists: true,
		},
		{
			name:       "core importing attest is reported as one extra edge",
			identity:   PackageCore,
			files:      oneSyntheticGoFile("core.go", goSourceWithImports(attestPath)),
			wantExists: true,
			wantExtra:  1,
		},
		{
			name:       "attest exact core frontier is accepted",
			identity:   PackageAttest,
			files:      oneSyntheticGoFile("attest.go", goSourceWithImports(corePath)),
			wantExists: true,
		},
		{
			name:        "attest omitting core reports one missing edge",
			identity:    PackageAttest,
			files:       oneSyntheticGoFile("attest.go", goSourceWithImports()),
			wantExists:  true,
			wantMissing: 1,
		},
		{
			name:       "attest importing contextstate reports one extra edge",
			identity:   PackageAttest,
			files:      oneSyntheticGoFile("attest.go", goSourceWithImports(corePath, contextStatePath)),
			wantExists: true,
			wantExtra:  1,
		},
		{
			name:       "filestore exact two-edge frontier is accepted",
			identity:   PackageFilestore,
			files:      oneSyntheticGoFile("filestore.go", goSourceWithImports(corePath, contextStatePath)),
			wantExists: true,
		},
		{
			name:        "filestore missing contextstate reports the exact omission",
			identity:    PackageFilestore,
			files:       oneSyntheticGoFile("filestore.go", goSourceWithImports(corePath)),
			wantExists:  true,
			wantMissing: 1,
		},
		{
			name:       "filestore importing attest reports one extra edge",
			identity:   PackageFilestore,
			files:      oneSyntheticGoFile("filestore.go", goSourceWithImports(corePath, contextStatePath, attestPath)),
			wantExists: true,
			wantExtra:  1,
		},
		{
			name:     "test-only sibling import is a real extra coupling edge",
			identity: PackageAttest,
			files: syntheticGoFileSet{
				values: [3]syntheticGoFile{
					{name: "attest.go", source: goSourceWithImports(corePath)},
					{name: "attest_test.go", source: goSourceWithImports(contextStatePath)},
				},
				count: 2,
			},
			wantExists: true,
			wantExtra:  1,
		},
		{
			name:     "external-package test self import is not a sibling edge",
			identity: PackageAttest,
			files: syntheticGoFileSet{
				values: [3]syntheticGoFile{
					{name: "attest.go", source: goSourceWithImports(corePath)},
					{name: "attest_test.go", source: goSourceWithImports(attestPath)},
				},
				count: 2,
			},
			wantExists: true,
		},
		{
			name:     "duplicate edge across production files is one semantic import",
			identity: PackageAttest,
			files: syntheticGoFileSet{
				values: [3]syntheticGoFile{
					{name: "first.go", source: goSourceWithImports(corePath)},
					{name: "second.go", source: goSourceWithImports(corePath)},
				},
				count: 2,
			},
			wantExists: true,
		},
		{
			name:       "unknown Primitive package path fails with typed identity",
			identity:   PackageAttest,
			files:      oneSyntheticGoFile("attest.go", goSourceWithImports(corePath, unknownPath)),
			wantExists: true,
			wantErr:    ErrPrimitiveContract,
		},
		{
			name:       "nested path under admitted package fails as undeclared",
			identity:   PackageAttest,
			files:      oneSyntheticGoFile("attest.go", goSourceWithImports(corePath, nestedCorePath)),
			wantExists: true,
			wantErr:    ErrPrimitiveContract,
		},
		{
			name:        "empty landed filestore reports both required edges missing",
			identity:    PackageFilestore,
			files:       oneSyntheticGoFile("notes.txt", "not Go source"),
			wantExists:  true,
			wantMissing: 2,
		},
		{
			name:       "malformed production source fails with typed identity",
			identity:   PackageAttest,
			files:      oneSyntheticGoFile("attest.go", "package attest\nimport (\n"),
			wantExists: true,
			wantErr:    ErrPrimitiveContract,
		},
		{
			name:       "malformed test source fails with typed identity",
			identity:   PackageAttest,
			files:      oneSyntheticGoFile("attest_test.go", "package attest_test\nimport (\n"),
			wantExists: true,
			wantErr:    ErrPrimitiveContract,
		},
		{
			name:       "nested Go package is rejected as outside the exact catalog",
			identity:   PackageAttest,
			files:      oneSyntheticGoFile("internal/helper.go", goSourceWithImports(corePath)),
			wantExists: true,
			wantErr:    ErrPrimitiveContract,
		},
		{
			name:        "nested non-Go file is neutral but landed package still misses core",
			identity:    PackageAttest,
			files:       oneSyntheticGoFile("internal/README.txt", "not Go source"),
			wantExists:  true,
			wantMissing: 1,
		},
	}

	catalog := PrimitiveArchitecture()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			if tc.files.count > 0 {
				writeSyntheticPackage(t, root, tc.identity, tc.files)
			}
			gotExists, gotViolations, gotErr := auditPackageImports(root, tc.identity, catalog)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("auditPackageImports() error = %v, want %v", gotErr, tc.wantErr)
			}
			if gotExists != tc.wantExists {
				t.Fatalf("auditPackageImports() exists = %t, want %t", gotExists, tc.wantExists)
			}
			if gotErr != nil {
				return
			}
			gotMissing := gotViolations.CountKind(architectureImportViolationMissing)
			if gotMissing != tc.wantMissing {
				t.Fatalf("missing import violation count = %d, want %d", gotMissing, tc.wantMissing)
			}
			gotExtra := gotViolations.CountKind(architectureImportViolationExtra)
			if gotExtra != tc.wantExtra {
				t.Fatalf("extra import violation count = %d, want %d", gotExtra, tc.wantExtra)
			}
		})
	}
}

func auditPackageImports(
	root string,
	identity PackageIdentity,
	catalog ArchitectureCatalog,
) (bool, architectureImportViolationInventory, error) {
	exists, imports, err := readPackageImports(root, identity)
	if err != nil || !exists {
		return exists, architectureImportViolationInventory{}, err
	}
	var violations architectureImportViolationInventory
	for _, gotImport := range imports.Values() {
		if !catalogContainsDirectImport(catalog, gotImport) {
			if addErr := violations.Add(architectureImportViolation{
				contract: gotImport,
				kind:     architectureImportViolationExtra,
			}); addErr != nil {
				return true, architectureImportViolationInventory{}, addErr
			}
		}
	}
	for wantImport := range catalog.DirectImports() {
		if wantImport.Importer == identity && !imports.Contains(wantImport) {
			if addErr := violations.Add(architectureImportViolation{
				contract: wantImport,
				kind:     architectureImportViolationMissing,
			}); addErr != nil {
				return true, architectureImportViolationInventory{}, addErr
			}
		}
	}
	return true, violations, nil
}

func readPackageImports(root string, identity PackageIdentity) (bool, directImportInventory, error) {
	name, err := identity.Name()
	if err != nil {
		return false, directImportInventory{}, err
	}
	packageDirectory := filepath.Join(root, name)
	entries, err := os.ReadDir(packageDirectory)
	if errors.Is(err, os.ErrNotExist) {
		return false, directImportInventory{}, nil
	}
	if err != nil {
		return false, directImportInventory{}, errors.Join(
			architectureContractError("landed package directory cannot be read"),
			err,
		)
	}
	var imports directImportInventory
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() {
			if nestedErr := rejectNestedGoSources(filepath.Join(packageDirectory, entry.Name())); nestedErr != nil {
				return true, directImportInventory{}, nestedErr
			}
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		filename := filepath.Join(packageDirectory, entry.Name())
		if readErr := readGoFileImports(files, filename, identity, &imports); readErr != nil {
			return true, directImportInventory{}, readErr
		}
	}
	return true, imports, nil
}

func rejectNestedGoSources(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return errors.Join(architectureContractError("nested package directory cannot be read"), err)
	}
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		if entry.IsDir() {
			if nestedErr := rejectNestedGoSources(path); nestedErr != nil {
				return nestedErr
			}
			continue
		}
		if strings.HasSuffix(entry.Name(), ".go") {
			return architectureContractError("landed package contains an undeclared nested Go package")
		}
	}
	return nil
}

func readGoFileImports(
	files *token.FileSet,
	filename string,
	identity PackageIdentity,
	imports *directImportInventory,
) error {
	file, err := parser.ParseFile(files, filename, nil, parser.ImportsOnly|parser.SkipObjectResolution)
	if err != nil {
		return errors.Join(architectureContractError("landed package source cannot be parsed"), err)
	}
	for _, imported := range file.Imports {
		if err := addPrimitiveFileImport(imported.Path.Value, identity, imports); err != nil {
			return err
		}
	}
	return nil
}

func addPrimitiveFileImport(
	quotedImportPath string,
	identity PackageIdentity,
	imports *directImportInventory,
) error {
	importPath, err := strconv.Unquote(quotedImportPath)
	if err != nil {
		return errors.Join(
			architectureContractError("landed package import path cannot be decoded"),
			err,
		)
	}
	importedIdentity, isPrimitive, err := parsePrimitiveImportPath(importPath)
	if err != nil || !isPrimitive || importedIdentity == identity {
		return err
	}
	return imports.Add(DirectImportContract{Importer: identity, Imported: importedIdentity})
}

func parsePrimitiveImportPath(importPath string) (PackageIdentity, bool, error) {
	if !strings.HasPrefix(importPath, PrimitivePackagePathPrefix) {
		return PackageUnknown, false, nil
	}
	for identity := PackageCore; identity < packageIdentityLimit; identity++ {
		admittedPath, err := identity.ImportPath()
		if err != nil {
			return PackageUnknown, true, err
		}
		if importPath == admittedPath {
			return identity, true, nil
		}
	}
	return PackageUnknown, true, architectureContractError("landed package imports an undeclared Primitive package path")
}

func catalogContainsDirectImport(catalog ArchitectureCatalog, target DirectImportContract) bool {
	for candidate := range catalog.DirectImports() {
		if candidate == target {
			return true
		}
	}
	return false
}

func mustPackageImportPathForTest(t *testing.T, identity PackageIdentity) string {
	t.Helper()

	path, err := identity.ImportPath()
	if err != nil {
		t.Fatalf("PackageIdentity(%v).ImportPath() error = %v, want nil", identity, err)
	}
	return path
}

func oneSyntheticGoFile(name, source string) syntheticGoFileSet {
	return syntheticGoFileSet{
		values: [3]syntheticGoFile{{name: name, source: source}},
		count:  1,
	}
}

func goSourceWithImports(importPaths ...string) string {
	var source strings.Builder
	source.WriteString("package synthetic\n")
	for _, importPath := range importPaths {
		source.WriteString("import _ ")
		source.WriteString(strconv.Quote(importPath))
		source.WriteByte('\n')
	}
	return source.String()
}

func writeSyntheticPackage(
	t *testing.T,
	root string,
	identity PackageIdentity,
	files syntheticGoFileSet,
) {
	t.Helper()

	name, err := identity.Name()
	if err != nil {
		t.Fatalf("PackageIdentity(%v).Name() error = %v, want nil", identity, err)
	}
	packageDirectory := filepath.Join(root, name)
	if gotErr := os.MkdirAll(packageDirectory, 0o700); gotErr != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", packageDirectory, gotErr)
	}
	for _, file := range files.Values() {
		filename := filepath.Join(packageDirectory, file.name)
		if gotErr := os.MkdirAll(filepath.Dir(filename), 0o700); gotErr != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v, want nil", filepath.Dir(filename), gotErr)
		}
		if gotErr := os.WriteFile(filename, []byte(file.source), 0o600); gotErr != nil {
			t.Fatalf("os.WriteFile(%q) error = %v, want nil", filename, gotErr)
		}
	}
}

func (i *directImportInventory) Add(contract DirectImportContract) error {
	if i.Contains(contract) {
		return nil
	}
	if int(i.count) >= len(i.values) {
		return architectureContractError("landed import graph exceeds the catalog capacity")
	}
	i.values[i.count] = contract
	i.count++
	return nil
}

func (i directImportInventory) Contains(contract DirectImportContract) bool {
	for _, candidate := range i.Values() {
		if candidate == contract {
			return true
		}
	}
	return false
}

func (i directImportInventory) Values() []DirectImportContract {
	return i.values[:i.count]
}

func (i *architectureImportViolationInventory) Add(violation architectureImportViolation) error {
	if violation.kind == architectureImportViolationUnknown {
		return architectureContractError("architecture import violation kind is unset")
	}
	if int(i.count) >= len(i.values) {
		return architectureContractError("architecture import violation inventory exceeds its fixed capacity")
	}
	i.values[i.count] = violation
	i.count++
	return nil
}

func (i architectureImportViolationInventory) CountKind(kind architectureImportViolationKind) int {
	count := 0
	for _, violation := range i.Values() {
		if violation.kind == kind {
			count++
		}
	}
	return count
}

func (i architectureImportViolationInventory) Values() []architectureImportViolation {
	return i.values[:i.count]
}

func (s syntheticGoFileSet) Values() []syntheticGoFile {
	return s.values[:s.count]
}
