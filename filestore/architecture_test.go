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
	validatedRequest[T any]    struct{}
	capabilityWrapper[T any]   struct{}
	ownershipReceipt[T any]    struct{}
	streamedObservation[T any] struct{}
	boundedFact[T any]         struct{}
)

type architectureScan struct {
	violations       []string
	primitiveImports []string
}

// filestoreContractInventory classifies every production struct by its real
// role. The generic arguments make every inventory entry compiler-visible.
type filestoreContractInventory struct {
	Location           capabilityWrapper[Location]
	DirectoryRequest   validatedRequest[DirectoryRequest]
	ReadRequest        validatedRequest[ReadRequest]
	ReadHandleRequest  validatedRequest[ReadHandleRequest]
	RenameRequest      validatedRequest[RenameRequest]
	WriteRequest       validatedRequest[WriteRequest]
	StageRequest       validatedRequest[StageRequest]
	CommitRequest      validatedRequest[CommitRequest]
	TouchRequest       validatedRequest[TouchRequest]
	DurabilityRequest  validatedRequest[DurabilityRequest]
	Permissions        boundedFact[Permissions]
	Ownership          boundedFact[Ownership]
	Allocation         boundedFact[Allocation]
	LockFileRequest    validatedRequest[LockFileRequest]
	AppendRequest      validatedRequest[AppendRequest]
	RotationRequest    validatedRequest[RotationRequest]
	RemovalRequest     validatedRequest[RemovalRequest]
	TreeRemovalRequest validatedRequest[TreeRemovalRequest]
	WalkRequest        validatedRequest[WalkRequest]
	WalkEntry          streamedObservation[WalkEntry]
	// One observation of a path, made before any effect and carrying no
	// capability over it.
	Inspection            streamedObservation[Inspection]
	DirectoryEntryMaximum boundedFact[DirectoryEntryMaximum]
	StagedFile            ownershipReceipt[StagedFile]
}

var _ = filestoreContractInventory{}

func TestFilestorePublicSurfaceIsExactRatchet(t *testing.T) {
	t.Parallel()

	productions := parseProductionFiles(t)
	gotTypes := make([]string, 0)
	gotFunctions := make([]string, 0)
	gotConstants := make([]string, 0)
	gotMethods := make([]string, 0)
	for _, production := range productions {
		for _, declaration := range production.file.Decls {
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
				receiver := receiverName(value.Recv.List[0].Type)
				if !ast.IsExported(receiver) {
					continue
				}
				gotMethods = append(gotMethods, receiver+"."+value.Name.Name)
			}
		}
	}
	requireExactNames(t, "exported types", gotTypes, []string{
		"AppendMode",
		"AppendRequest",
		"CommitRequest",
		"DirectoryRequest",
		"DirectoryEntryMaximum",
		"DurabilityRequest",
		"Ownership",
		"Allocation",
		"Permissions",
		"LockFileRequest",
		"TouchRequest",
		"InstallMode",
		"Inspection",
		"Location",
		"PathKind",
		"ReadHandleRequest",
		"ReadRequest",
		"RenameRequest",
		"RemovalRequest",
		"RotationRequest",
		"Sharing",
		"StageRequest",
		"StagedFile",
		"TreeRemovalRequest",
		"WalkDirective",
		"WalkEntry",
		"WalkOrder",
		"WalkRequest",
		"WriteRequest",
	})
	requireExactNames(t, "exported functions", gotFunctions, []string{
		"Canonicalize",
		"Commit",
		"ConfirmDurable",
		"Discard",
		"EnsureDirectory",
		"Inspect",
		"NewDirectoryEntryMaximum",
		"ObserveSharing",
		"OpenAppend",
		"OpenLockFile",
		"OpenParent",
		"OpenRead",
		"OpenRoot",
		"Read",
		"Recover",
		"Remove",
		"RemoveTree",
		"Rename",
		"RotateAppend",
		"Stage",
		"Touch",
		"Walk",
		"Write",
	})
	requireExactNames(t, "exported constants", gotConstants, []string{
		"AppendCreate",
		"AppendCreateOrOpen",
		"AppendExisting",
		"AppendUnknown",
		"DirectoryEntryMaximumLimit",
		"InstallCreate",
		"InstallReplace",
		"InstallUnknown",
		"PathKindAbsent",
		"PathKindDirectory",
		"PathKindOther",
		"PathKindRegularFile",
		"PathKindSymbolicLink",
		"PathKindUnknown",
		"PathKindUnreachable",
		"SharingAvailable",
		"SharingHeld",
		"SharingUnknown",
		"WalkContinue",
		"WalkDirectiveUnknown",
		"WalkOrderLexical",
		"WalkOrderNative",
		"WalkOrderUnknown",
		"WalkSkipDirectory",
	})
	requireExactNames(t, "exported methods", gotMethods, []string{
		"Allocation.Bytes",
		"Allocation.Reported",
		"Allocation.Validate",
		"AppendMode.IsValid",
		"AppendMode.OffWireEnum",
		"AppendMode.String",
		"AppendMode.Validate",
		"AppendRequest.Validate",
		"CommitRequest.Validate",
		"DirectoryRequest.Validate",
		"DirectoryEntryMaximum.Validate",
		"DurabilityRequest.Validate",
		"LockFileRequest.Validate",
		"TouchRequest.Validate",
		"Inspection.Allocation",
		"Inspection.Kind",
		"Inspection.Ownership",
		"Inspection.Permissions",
		"Ownership.GID",
		"Ownership.IsSet",
		"Ownership.UID",
		"Ownership.Validate",
		"Permissions.Bits",
		"Permissions.FileMode",
		"Permissions.IsSet",
		"Permissions.String",
		"Permissions.Validate",
		"Inspection.ModifiedAt",
		"Inspection.SizeBytes",
		"Inspection.Validate",
		"InstallMode.IsValid",
		"InstallMode.OffWireEnum",
		"InstallMode.String",
		"InstallMode.Validate",
		"Location.Validate",
		"PathKind.IsValid",
		"PathKind.OffWireEnum",
		"PathKind.String",
		"PathKind.Validate",
		"ReadHandleRequest.Validate",
		"ReadRequest.Validate",
		"RemovalRequest.Validate",
		"RenameRequest.Validate",
		"RotationRequest.Validate",
		"Sharing.IsValid",
		"Sharing.OffWireEnum",
		"Sharing.String",
		"Sharing.Validate",
		"StageRequest.Validate",
		"StagedFile.BytesWritten",
		"StagedFile.Path",
		"StagedFile.Validate",
		"TreeRemovalRequest.Validate",
		"WalkDirective.IsValid",
		"WalkDirective.OffWireEnum",
		"WalkDirective.String",
		"WalkDirective.Validate",
		"WalkEntry.Validate",
		"WalkOrder.IsValid",
		"WalkOrder.OffWireEnum",
		"WalkOrder.String",
		"WalkOrder.Validate",
		"WalkRequest.Validate",
		"WriteRequest.Validate",
	})
}

// TestOnlyTheOwnerIdentityLeafNamesThePlatformStatusStructure proves the
// scoped import is exactly one file wide. A second file naming syscall would
// mean a platform detail had started spreading through the package, which is
// the reason the blanket ban existed before owner identity needed the one
// assertion the standard library offers no other route to.
func TestOnlyTheOwnerIdentityLeafNamesThePlatformStatusStructure(t *testing.T) {
	t.Parallel()

	naming := make([]string, 0)
	for _, production := range parseProductionFiles(t) {
		for _, specification := range production.file.Imports {
			path, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				t.Fatalf("Unquote(%s) error = %v, want nil", specification.Path.Value, err)
			}
			if path == "syscall" && !slices.Contains(naming, production.name) {
				naming = append(naming, production.name)
			}
		}
	}
	requireExactNames(t, "production files naming syscall", naming, []string{ownerIdentityLeafFile})
}

// TestNoProductionFileInvokesASyscall is the rule the scoped import rests on,
// and it is stricter than the ban it replaced. Importing syscall to read a
// structure this package was handed is not the hazard; calling into the kernel
// is, because every such call is either an unrooted path access or a second
// observation of an entry that may no longer be the one already described.
// This holds for the owner-identity leaf too, so the exemption buys a type
// assertion and nothing else.
func TestNoProductionFileInvokesASyscall(t *testing.T) {
	t.Parallel()

	for _, production := range parseProductionFiles(t) {
		ast.Inspect(production.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if ok && qualifier.Name == "syscall" {
				t.Errorf(
					"production call syscall.%s in %s, want a value this package was already handed",
					selector.Sel.Name,
					production.name,
				)
			}
			return true
		})
	}
}

func TestFilestoreDataFlowStructInventoryRatchet(t *testing.T) {
	t.Parallel()

	got := make([]string, 0)
	for _, production := range parseProductionFiles(t) {
		ast.Inspect(production.file, func(node ast.Node) bool {
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
		"github.com/deliri/primitive/v2026/temporal",
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
	got, err := scanProductionArchitecture([]productionFile{{file: file, name: "synthetic_forbidden.go"}})
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

// ownerIdentityLeafFile is the one production file permitted to name the
// platform status structure. Owner identifiers exist nowhere else in the
// standard library: fs.FileInfo.Sys returns an `any`, and reading it is a type
// assertion on a value Lstat already returned, not a call. What must never
// exist is a syscall invocation, which would be an unrooted path access or a
// second observation of an entry that may have changed; scanProductionArchitecture
// rejects one in every file including this leaf.
const ownerIdentityLeafFile = "attributes_unix.go"

func scanProductionArchitecture(files []productionFile) (architectureScan, error) {
	primitiveImports := make(map[string]struct{})
	forbiddenImports := map[string]struct{}{
		"encoding/json": {},
		"sync":          {},
		"sync/atomic":   {},
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
	for _, production := range files {
		file := production.file
		importNames := make(map[string]string)
		if production.name != ownerIdentityLeafFile {
			forbiddenImports["syscall"] = struct{}{}
		} else {
			delete(forbiddenImports, "syscall")
		}
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

// productionFile pairs one parsed production file with its name, so a rule can
// name the single leaf that owns an effect instead of banning the effect from
// the package that exists to own it.
type productionFile struct {
	file *ast.File
	name string
}

func parseProductionFiles(t *testing.T) []productionFile {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fileSet := token.NewFileSet()
	files := make([]productionFile, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fileSet, entry.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("ParseFile(%s) error = %v, want nil", entry.Name(), parseErr)
		}
		files = append(files, productionFile{file: file, name: entry.Name()})
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
