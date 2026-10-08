package filestore

import (
	"embed"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type (
	validatedRequest[T any]    struct{}
	capabilityWrapper[T any]   struct{}
	ownershipReceipt[T any]    struct{}
	streamedObservation[T any] struct{}
	boundedFact[T any]         struct{}
	internalFlow[T any]        struct{}
)

type architectureScan struct {
	violations       []string
	primitiveImports []string
}

// filestoreContractInventory classifies every production struct by its real
// role. The generic arguments make every inventory entry compiler-visible.
type filestoreContractInventory struct {
	CopyContentRequest            validatedRequest[CopyContentRequest]
	ContentIndexInspectionRequest validatedRequest[ContentIndexInspectionRequest]
	ContentIndexEntry             boundedFact[ContentIndexEntry]
	ContentSortRequest            validatedRequest[ContentSortRequest]
	ContentStreamRequest          validatedRequest[ContentStreamRequest]
	ContentStreamSummary          streamedObservation[ContentStreamSummary]
	ContentIndexSummary           streamedObservation[ContentIndexSummary]
	ContentIndexConflictError     streamedObservation[ContentIndexConflictError]
	Pipe                          capabilityWrapper[Pipe]
	PipeTransferRequest           validatedRequest[PipeTransferRequest]
	PipeTransferResult            streamedObservation[PipeTransferResult]
	pipeProducerResult            internalFlow[pipeProducerResult]
	Location                      capabilityWrapper[Location]
	DirectoryRequest              validatedRequest[DirectoryRequest]
	ScratchRequest                validatedRequest[ScratchRequest]
	ScratchResetRequest           validatedRequest[ScratchResetRequest]
	RewindRequest                 validatedRequest[RewindRequest]
	ScratchScopeRequest           validatedRequest[ScratchScopeRequest]
	RootScopeRequest              validatedRequest[RootScopeRequest]
	RootScopeResult               streamedObservation[RootScopeResult]
	FileScopeResult               streamedObservation[FileScopeResult]
	ReadScopeRequest              validatedRequest[ReadScopeRequest]
	ScratchWriterScopeRequest     validatedRequest[ScratchWriterScopeRequest]
	ScratchReplayScopeRequest     validatedRequest[ScratchReplayScopeRequest]
	ReadRequest                   validatedRequest[ReadRequest]
	ReadHandleRequest             validatedRequest[ReadHandleRequest]
	HandleInspectionRequest       validatedRequest[HandleInspectionRequest]
	UpdateHandleRequest           validatedRequest[UpdateHandleRequest]
	RenameRequest                 validatedRequest[RenameRequest]
	WriteRequest                  validatedRequest[WriteRequest]
	StageRequest                  validatedRequest[StageRequest]
	StageDestinationRequest       validatedRequest[StageDestinationRequest]
	StageWriteRequest             validatedRequest[StageWriteRequest]
	StageWriteObservation         streamedObservation[StageWriteObservation]
	StageSyncRequest              validatedRequest[StageSyncRequest]
	StageSyncObservation          streamedObservation[StageSyncObservation]
	StageRestoreRequest           validatedRequest[StageRestoreRequest]
	StageRestoreObservation       streamedObservation[StageRestoreObservation]
	ActivationRequest             validatedRequest[ActivationRequest]
	CommitRequest                 validatedRequest[CommitRequest]
	TouchRequest                  validatedRequest[TouchRequest]
	DurabilityRequest             validatedRequest[DurabilityRequest]
	PermissionRequest             validatedRequest[PermissionRequest]
	Permissions                   boundedFact[Permissions]
	Ownership                     boundedFact[Ownership]
	Allocation                    boundedFact[Allocation]
	LockFileRequest               validatedRequest[LockFileRequest]
	AppendRequest                 validatedRequest[AppendRequest]
	RotationRequest               validatedRequest[RotationRequest]
	RemovalRequest                validatedRequest[RemovalRequest]
	TreeRemovalRequest            validatedRequest[TreeRemovalRequest]
	WalkRequest                   validatedRequest[WalkRequest]
	WalkEntry                     streamedObservation[WalkEntry]
	SymbolicLinkTarget            streamedObservation[SymbolicLinkTarget]
	FilesystemIdentity            streamedObservation[FilesystemIdentity]
	HeldDirectory                 capabilityWrapper[HeldDirectory]
	// One observation of a path, made before any effect and carrying no
	// capability over it.
	Inspection             streamedObservation[Inspection]
	StagedFile             ownershipReceipt[StagedFile]
	StageDestination       capabilityWrapper[StageDestination]
	directoryEntryEnsure   internalFlow[directoryEntryEnsure]
	streamCopyRequest      internalFlow[streamCopyRequest]
	streamReader           internalFlow[streamReader]
	streamWriter           internalFlow[streamWriter]
	stageSynchronization   internalFlow[stageSynchronization]
	createdFileAbandonment internalFlow[createdFileAbandonment]
	createdPathCleanup     internalFlow[createdPathCleanup]
	readDirectoryInput     internalFlow[readDirectoryInput]
	visitWalkEntryInput    internalFlow[visitWalkEntryInput]
	walkDirectoryInput     internalFlow[walkDirectoryInput]
	rootedOpenRequest      internalFlow[rootedOpenRequest]
}

var (
	_ = filestoreContractInventory{}
	_ = filestoreContractInventory{}.directoryEntryEnsure
	_ = filestoreContractInventory{}.streamCopyRequest
	_ = filestoreContractInventory{}.streamReader
	_ = filestoreContractInventory{}.streamWriter
	_ = filestoreContractInventory{}.stageSynchronization
	_ = filestoreContractInventory{}.createdFileAbandonment
	_ = filestoreContractInventory{}.createdPathCleanup
	_ = filestoreContractInventory{}.readDirectoryInput
	_ = filestoreContractInventory{}.visitWalkEntryInput
	_ = filestoreContractInventory{}.walkDirectoryInput
	_ = filestoreContractInventory{}.rootedOpenRequest
)

// Every platform source is checked, not just files selected on the host. The
// approved Primitive identities come from compiled types, and Go determines
// whether every remaining import belongs to its standard library.
func TestFilestoreImportsOnlyGoAndOwnedPrimitiveContracts(t *testing.T) {
	t.Parallel()
	approved := []string{
		reflect.TypeFor[core.Validatable]().PkgPath(),
		reflect.TypeFor[contextstate.State]().PkgPath(),
		reflect.TypeFor[temporal.Instant]().PkgPath(),
	}
	for _, production := range parseProductionFiles(t) {
		for _, specification := range production.file.Imports {
			path, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			t.Run(production.name+"/"+path, func(t *testing.T) {
				t.Parallel()
				if slices.Contains(approved, path) {
					return
				}
				got, err := build.Default.Import(path, "", build.FindOnly)
				if err != nil {
					t.Fatalf("production import %s lookup = %v, want Go standard library", path, err)
				}
				if !got.Goroot {
					t.Fatalf("production import %s is outside GOROOT, want Go standard library or an approved Primitive contract", path)
				}
			})
		}
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
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("production struct inventory = %v, want %v", got, want)
	}
}

func classifiedFilestoreStructNames(t *testing.T) []string {
	t.Helper()

	fileSet := token.NewFileSet()
	source, err := filestoreGoSources.ReadFile("architecture_test.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(
		fileSet,
		"architecture_test.go",
		source,
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
	want := []string{
		"github.com/deliri/primitive/v2026/contextstate",
		"github.com/deliri/primitive/v2026/core",
		"github.com/deliri/primitive/v2026/temporal",
	}
	slices.Sort(got.primitiveImports)
	slices.Sort(want)
	if !slices.Equal(got.primitiveImports, want) {
		t.Fatalf("Primitive production imports = %v, want %v", got.primitiveImports, want)
	}
}

func TestFilestoreProductionArchitectureMatcherDetectsForbiddenSyntheticShapes(t *testing.T) {
	t.Parallel()

	source := `package filestore
import (
	json "encoding/json/v2"
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
	_ = jsontext.Value{}
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
	want := []string{
		"RemoveAll: forbidden world-building function",
		"SyncDirectory: forbidden world-building function",
		"import encoding/json/v2",
		"import sync",
		"replacement.Read: forbidden file-lookalike method",
		"replacement: channel coordination",
		"replacement: loose map state",
		"violate: call io.ReadAll",
		"violate: call os.ReadFile",
		"violate: call filepath.Walk",
		"violate: goroutine",
	}
	slices.Sort(got.violations)
	slices.Sort(want)
	if !slices.Equal(got.violations, want) {
		t.Fatalf("synthetic architecture violations = %v, want %v", got.violations, want)
	}
}

func scanProductionArchitecture(files []productionFile) (architectureScan, error) {
	primitiveImports := make(map[string]struct{})
	forbiddenImports := map[string]struct{}{
		"encoding/json/v2": {},
		"sync":             {},
		"sync/atomic":      {},
		"unsafe":           {},
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
			nativePipeJoin := ownsNativePipeJoin(declaration)
			ast.Inspect(declaration, func(node ast.Node) bool {
				switch value := node.(type) {
				case *ast.GoStmt:
					if !nativePipeJoin {
						violations = append(violations, symbol+": goroutine")
					}
				case *ast.ChanType:
					if !nativePipeJoin {
						violations = append(violations, symbol+": channel coordination")
					}
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
						method := receiverName(value.Recv.List[0].Type) + "." + value.Name.Name
						if ownsGoInterfaceMethod(receiverName(value.Recv.List[0].Type), value.Name.Name) {
							return true
						}
						violations = append(
							violations,
							method+": forbidden file-lookalike method",
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

// Only the compiled native pipe capability may own one producer and one
// capacity-one completion channel. Actual endpoint/join tests prove lifetime;
// this structural check refuses expansion into a worker pool or queue.
func ownsNativePipeJoin(declaration ast.Decl) bool {
	function, ok := declaration.(*ast.FuncDecl)
	if !ok || function.Recv != nil || function.Name.Name != FilestoreIngressSymbolForTest(reflect.ValueOf(TransferPipe)) {
		return false
	}
	workers, channels, boundedCompletions, acquisitions, joins, loops := 0, 0, 0, 0, 0, 0
	ast.Inspect(function.Body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.GoStmt:
			workers++
		case *ast.ChanType:
			channels++
		case *ast.ForStmt, *ast.RangeStmt:
			loops++
		case *ast.UnaryExpr:
			if value.Op == token.ARROW {
				if _, ok := value.X.(*ast.Ident); ok {
					joins++
				}
			}
		case *ast.CallExpr:
			name, ok := value.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			if name.Name == FilestoreIngressSymbolForTest(reflect.ValueOf(OpenPipe)) {
				acquisitions++
			}
			if name.Name != "make" || len(value.Args) != 2 {
				return true
			}
			channel, ok := value.Args[0].(*ast.ChanType)
			if !ok {
				return true
			}
			item, ok := channel.Value.(*ast.Ident)
			if !ok || item.Name != reflect.TypeFor[pipeProducerResult]().Name() {
				return true
			}
			capacity, ok := value.Args[1].(*ast.BasicLit)
			if ok && capacity.Kind == token.INT && capacity.Value == "1" {
				boundedCompletions++
			}
		}
		return true
	})
	return workers == 1 && channels == 1 && boundedCompletions == 1 && acquisitions == 1 && joins == 1 && loops == 0
}

func TestNativePipeCoordinationAdmissionRejectsDetachedAndUnboundedWork(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, capacity, producer, join string
		want                           bool
	}{
		{name: "one native producer joined through bounded completion", capacity: ", 1", producer: "go func() {}()", join: "_ = <-completion", want: true},
		{name: "missing completion buffer is refused", producer: "go func() {}()", join: "_ = <-completion"},
		{name: "expanded completion buffer is refused", capacity: ", 2", producer: "go func() {}()", join: "_ = <-completion"},
		{name: "multiple producers are refused", capacity: ", 1", producer: "go func() {}(); go func() {}()", join: "_ = <-completion"},
		{name: "detached producer is refused", capacity: ", 1", producer: "go func() {}()"},
		{name: "repeated producer creation is refused", capacity: ", 1", producer: "for { go func() {}() }", join: "_ = <-completion"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := fmt.Sprintf("package filestore\nfunc %s() { _ = %s(); completion := make(chan %s%s); %s; %s }", FilestoreIngressSymbolForTest(reflect.ValueOf(TransferPipe)), FilestoreIngressSymbolForTest(reflect.ValueOf(OpenPipe)), reflect.TypeFor[pipeProducerResult]().Name(), tc.capacity, tc.producer, tc.join)
			file, err := parser.ParseFile(token.NewFileSet(), "native_join.go", source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			if got := ownsNativePipeJoin(file.Decls[0]); got != tc.want {
				t.Fatalf("native pipe coordination admission = %t, want %t", got, tc.want)
			}
		})
	}
}

// Each exception names a concrete compiler-visible receiver and a single-method
// Go interface. Adding a file-lookalike method does not expand this admission.
func ownsGoInterfaceMethod(receiver, method string) bool {
	return receiver == reflect.TypeFor[HeldDirectory]().Name() && method == reflect.TypeFor[io.Closer]().Method(0).Name ||
		receiver == reflect.TypeFor[streamReader]().Name() && method == reflect.TypeFor[io.Reader]().Method(0).Name ||
		receiver == reflect.TypeFor[streamWriter]().Name() && method == reflect.TypeFor[io.Writer]().Method(0).Name
}

var (
	_ io.Closer = (*HeldDirectory)(nil)
	_ io.Reader = (*streamReader)(nil)
	_ io.Writer = streamWriter{}
)

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

	entries, err := filestoreGoSources.ReadDir(".")
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
		source, readErr := filestoreGoSources.ReadFile(entry.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		file, parseErr := parser.ParseFile(fileSet, entry.Name(), source, parser.SkipObjectResolution)
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

// Compile the source inventory into this test build so Go overlays cannot make
// architecture checks inspect the unmodified working tree instead of the binary.
//
//go:embed *.go
var filestoreGoSources embed.FS
