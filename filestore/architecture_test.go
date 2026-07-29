package filestore

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
)

type (
	validatedRequest[T any]  struct{}
	capabilityWrapper[T any] struct{}
	ownershipReceipt[T any]  struct{}
)

type architectureScan struct {
	violations       []string
	primitiveImports []string
}

// filestoreContractInventory classifies every production struct by its real
// role. The generic arguments make every inventory entry compiler-visible.
type filestoreContractInventory struct {
	Location         capabilityWrapper[Location]
	DirectoryRequest validatedRequest[DirectoryRequest]
	ReadRequest      validatedRequest[ReadRequest]
	WriteRequest     validatedRequest[WriteRequest]
	StageRequest     validatedRequest[StageRequest]
	CommitRequest    validatedRequest[CommitRequest]
	AppendRequest    validatedRequest[AppendRequest]
	RotationRequest  validatedRequest[RotationRequest]
	RemovalRequest   validatedRequest[RemovalRequest]
	StagedFile       ownershipReceipt[StagedFile]
}

var _ = filestoreContractInventory{}

func TestFilestorePublicSurfaceIsExactRatchet(t *testing.T) {
	t.Parallel()

	files := parseProductionFiles(t)
	gotTypes := make([]string, 0)
	gotFunctions := make([]string, 0)
	gotConstants := make([]string, 0)
	gotMethods := make([]string, 0)
	for _, file := range files {
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.GenDecl:
				for _, specification := range value.Specs {
					switch typed := specification.(type) {
					case *ast.TypeSpec:
						if ast.IsExported(typed.Name.Name) {
							gotTypes = append(gotTypes, typed.Name.Name)
						}
					case *ast.ValueSpec:
						if value.Tok != token.CONST {
							continue
						}
						for _, name := range typed.Names {
							if ast.IsExported(name.Name) {
								gotConstants = append(gotConstants, name.Name)
							}
						}
					}
				}
			case *ast.FuncDecl:
				if !ast.IsExported(value.Name.Name) {
					continue
				}
				if value.Recv == nil {
					gotFunctions = append(gotFunctions, value.Name.Name)
					continue
				}
				gotMethods = append(gotMethods, receiverName(value.Recv.List[0].Type)+"."+value.Name.Name)
			}
		}
	}
	requireExactNames(t, "exported types", gotTypes, []string{
		"AppendMode",
		"AppendRequest",
		"CommitRequest",
		"DirectoryRequest",
		"InstallMode",
		"Location",
		"ReadRequest",
		"RemovalRequest",
		"RotationRequest",
		"StageRequest",
		"StagedFile",
		"WriteRequest",
	})
	requireExactNames(t, "exported functions", gotFunctions, []string{
		"Commit",
		"Discard",
		"EnsureDirectory",
		"OpenAppend",
		"Read",
		"Recover",
		"Remove",
		"RotateAppend",
		"Stage",
		"Write",
	})
	requireExactNames(t, "exported constants", gotConstants, []string{
		"AppendCreate",
		"AppendCreateOrOpen",
		"AppendExisting",
		"AppendUnknown",
		"InstallCreate",
		"InstallReplace",
		"InstallUnknown",
	})
	requireExactNames(t, "exported methods", gotMethods, []string{
		"AppendMode.Validate",
		"AppendRequest.Validate",
		"CommitRequest.Validate",
		"DirectoryRequest.Validate",
		"InstallMode.Validate",
		"Location.Validate",
		"ReadRequest.Validate",
		"RemovalRequest.Validate",
		"RotationRequest.Validate",
		"StageRequest.Validate",
		"StagedFile.BytesWritten",
		"StagedFile.Path",
		"StagedFile.Validate",
		"WriteRequest.Validate",
	})
}

func TestFilestoreDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	got := make([]string, 0)
	for _, file := range parseProductionFiles(t) {
		ast.Inspect(file, func(node ast.Node) bool {
			specification, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := specification.Type.(*ast.StructType); ok {
				got = append(got, specification.Name.Name)
			}
			return true
		})
	}
	want := classifiedFilestoreStructNames(t)
	requireExactNames(t, "production struct inventory", got, want)
}

func classifiedFilestoreStructNames(t *testing.T) []string {
	t.Helper()

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(
		fileSet,
		"architecture_test.go",
		nil,
		parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatalf("ParseFile(architecture_test.go) error = %v, want nil", err)
	}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			specification := raw.(*ast.TypeSpec)
			if specification.Name.Name != "filestoreContractInventory" {
				continue
			}
			structure := specification.Type.(*ast.StructType)
			names := make([]string, 0, len(structure.Fields.List))
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					names = append(names, name.Name)
				}
			}
			return names
		}
	}
	t.Fatal("filestoreContractInventory declarations found = 0, want 1")
	return nil
}

func TestFilestoreProductionUsesGoAndOSPrimitivesWithoutCoordinationMachineryRatchet(t *testing.T) {
	t.Parallel()

	got, err := scanProductionArchitecture(parseProductionFiles(t))
	if err != nil {
		t.Fatalf("scanProductionArchitecture() error = %v, want nil", err)
	}
	if len(got.violations) != 0 {
		t.Fatalf("production architecture violations = %v, want none", got.violations)
	}
	requireExactNames(t, "Primitive production imports", got.primitiveImports, []string{
		"github.com/deliri/primitive/v2026/contextstate",
		"github.com/deliri/primitive/v2026/core",
	})
}

func TestFilestoreProductionArchitectureMatcherDetectsForbiddenSyntheticShapes(t *testing.T) {
	t.Parallel()

	source := `package filestore
import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
)
type replacement struct {
	state map[string]string
	ready chan struct{}
}
func RemoveAll() {}
func SyncDirectory() {}
func (replacement) Read([]byte) (int, error) { return 0, nil }
func violate() {
	go func() {}()
	_, _ = io.ReadAll(nil)
	_, _ = os.ReadFile("target")
	_ = filepath.Walk(".", nil)
	_ = json.RawMessage{}
	_ = sync.Mutex{}
}`
	file, err := parser.ParseFile(
		token.NewFileSet(),
		"synthetic_forbidden.go",
		source,
		parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatalf("ParseFile(synthetic_forbidden.go) error = %v, want nil", err)
	}
	got, err := scanProductionArchitecture([]*ast.File{file})
	if err != nil {
		t.Fatalf("scanProductionArchitecture(synthetic) error = %v, want nil", err)
	}
	requireExactNames(t, "synthetic architecture violations", got.violations, []string{
		"RemoveAll: forbidden world-building function",
		"SyncDirectory: forbidden world-building function",
		"import encoding/json",
		"import sync",
		"replacement.Read: forbidden file-lookalike method",
		"replacement: channel coordination",
		"replacement: loose map state",
		"violate: call io.ReadAll",
		"violate: call os.ReadFile",
		"violate: call filepath.Walk",
		"violate: goroutine",
	})
}

func scanProductionArchitecture(files []*ast.File) (architectureScan, error) {
	primitiveImports := make(map[string]struct{})
	forbiddenImports := map[string]struct{}{
		"encoding/json": {},
		"sync":          {},
		"sync/atomic":   {},
		"syscall":       {},
		"unsafe":        {},
	}
	forbiddenCalls := map[string]struct{}{
		"io.ReadAll":       {},
		"os.CreateTemp":    {},
		"os.ReadDir":       {},
		"os.ReadFile":      {},
		"os.RemoveAll":     {},
		"os.WriteFile":     {},
		"filepath.Walk":    {},
		"filepath.WalkDir": {},
	}
	violations := make([]string, 0)
	for _, file := range files {
		importNames := make(map[string]string)
		for _, specification := range file.Imports {
			path, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				return architectureScan{}, err
			}
			if _, forbidden := forbiddenImports[path]; forbidden {
				violations = append(violations, "import "+path)
			}
			if strings.HasPrefix(path, "github.com/deliri/primitive/v2026/") {
				primitiveImports[path] = struct{}{}
			}
			name := filepath.Base(path)
			if specification.Name != nil {
				name = specification.Name.Name
			}
			importNames[name] = path
		}
		for _, declaration := range file.Decls {
			symbol := declarationName(declaration)
			ast.Inspect(declaration, func(node ast.Node) bool {
				switch value := node.(type) {
				case *ast.GoStmt:
					violations = append(violations, symbol+": goroutine")
				case *ast.ChanType:
					violations = append(violations, symbol+": channel coordination")
				case *ast.MapType:
					violations = append(violations, symbol+": loose map state")
				case *ast.CallExpr:
					selector, ok := value.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					identifier, ok := selector.X.(*ast.Ident)
					if !ok {
						return true
					}
					path, imported := importNames[identifier.Name]
					if !imported {
						return true
					}
					call := filepath.Base(path) + "." + selector.Sel.Name
					if _, forbidden := forbiddenCalls[call]; forbidden {
						violations = append(violations, symbol+": call "+call)
					}
				case *ast.FuncDecl:
					if value.Recv == nil {
						if value.Name.Name == "RemoveAll" || value.Name.Name == "SyncDirectory" {
							violations = append(
								violations,
								value.Name.Name+": forbidden world-building function",
							)
						}
						return true
					}
					switch value.Name.Name {
					case "Close", "Read", "ReadAt", "Seek", "Stat", "Sync", "Write", "WriteAt":
						violations = append(
							violations,
							receiverName(value.Recv.List[0].Type)+"."+value.Name.Name+
								": forbidden file-lookalike method",
						)
					}
				}
				return true
			})
		}
	}
	gotPrimitiveImports := make([]string, 0, len(primitiveImports))
	for path := range primitiveImports {
		gotPrimitiveImports = append(gotPrimitiveImports, path)
	}
	return architectureScan{
		violations:       violations,
		primitiveImports: gotPrimitiveImports,
	}, nil
}

func declarationName(declaration ast.Decl) string {
	switch value := declaration.(type) {
	case *ast.FuncDecl:
		if value.Recv == nil {
			return value.Name.Name
		}
		return receiverName(value.Recv.List[0].Type) + "." + value.Name.Name
	case *ast.GenDecl:
		names := make([]string, 0, len(value.Specs))
		for _, raw := range value.Specs {
			switch specification := raw.(type) {
			case *ast.TypeSpec:
				names = append(names, specification.Name.Name)
			case *ast.ValueSpec:
				for _, name := range specification.Names {
					names = append(names, name.Name)
				}
			}
		}
		if len(names) > 0 {
			return strings.Join(names, ",")
		}
	}
	return "declaration"
}

func parseProductionFiles(t *testing.T) []*ast.File {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	files := make([]*ast.File, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fileSet, entry.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("ParseFile(%s) error = %v, want nil", entry.Name(), parseErr)
		}
		files = append(files, file)
	}
	return files
}

func receiverName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.StarExpr:
		return receiverName(value.X)
	case *ast.IndexExpr:
		return receiverName(value.X)
	default:
		return ""
	}
}

func requireExactNames(t *testing.T, label string, got, want []string) {
	t.Helper()

	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}
