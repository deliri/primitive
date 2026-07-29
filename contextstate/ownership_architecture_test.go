package contextstate

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const (
	standardContextImportPath      = "context"
	standardErrorsImportPath       = "errors"
	retiredContextcheckPackageName = "contextcheck"
	ownershipViolationMaximum      = 64
)

type ownershipViolationKind uint8

const (
	ownershipViolationUnknown ownershipViolationKind = iota
	ownershipViolationRetiredImport
	ownershipViolationDirectClassification
	ownershipViolationRepeatedBoundary
	ownershipViolationCompatibility
)

type ownershipSymbol uint8

const (
	ownershipSymbolUnknown ownershipSymbol = iota
	ownershipSymbolRetiredContextcheck
	ownershipSymbolDirectClassification
	ownershipSymbolRepeatedContextBoundary
	ownershipSymbolClassify
	ownershipSymbolObserveAfterDone
	ownershipSymbolState
	ownershipSymbolValidate
)

type ownershipViolation struct {
	file   string
	line   uint32
	kind   ownershipViolationKind
	symbol ownershipSymbol
}

type ownershipViolationInventory struct {
	values [ownershipViolationMaximum]ownershipViolation
	count  uint8
}

func (s ownershipSymbol) String() string {
	switch s {
	case ownershipSymbolRetiredContextcheck:
		return "retired contextcheck import"
	case ownershipSymbolDirectClassification:
		return "errors.Is context terminal classification"
	case ownershipSymbolRepeatedContextBoundary:
		return "context nil and Err boundary"
	case ownershipSymbolClassify:
		return "contextstate.Classify"
	case ownershipSymbolObserveAfterDone:
		return "contextstate.ObserveAfterDone"
	case ownershipSymbolState:
		return "contextstate.State"
	case ownershipSymbolValidate:
		return "contextstate.Validate"
	default:
		return "unknown contextstate ownership symbol"
	}
}

func (i *ownershipViolationInventory) add(
	violation ownershipViolation,
) error {
	if int(i.count) >= len(i.values) {
		return core.ErrContextStateContract
	}
	i.values[i.count] = violation
	i.count++
	return nil
}

func (i ownershipViolationInventory) valuesView() []ownershipViolation {
	return i.values[:i.count]
}

type importBinding struct {
	name  string
	found bool
	dot   bool
}

type ownershipPaths struct {
	contextstate string
	retired      string
}

func TestContextOwnershipMatcherSyntheticRedGreenRatchet(t *testing.T) {
	t.Parallel()

	paths := mustOwnershipPaths(t)
	contextstateImport := paths.contextstate
	retiredImport := paths.retired
	cases := []struct {
		name     string
		source   string
		wantKind ownershipViolationKind
		want     int
	}{
		{
			name: "ordinary validated consumer is allowed",
			source: "package consumer\n" +
				"import (\n" +
				"\"context\"\n" +
				"contextstate \"" + contextstateImport + "\"\n" +
				")\n" +
				"func run(ctx context.Context) error {\n" +
				"if err := contextstate.Validate(ctx); err != nil { return err }\n" +
				"return nil\n" +
				"}\n",
		},
		{
			name: "direct cancellation classification is rejected",
			source: "package consumer\n" +
				"import (\"context\"; \"errors\")\n" +
				"func classify(err error) bool { return errors.Is(err, context.Canceled) }\n",
			wantKind: ownershipViolationDirectClassification,
			want:     1,
		},
		{
			name: "direct deadline classification is rejected",
			source: "package consumer\n" +
				"import (\"context\"; \"errors\")\n" +
				"func classify(err error) bool { return errors.Is(err, context.DeadlineExceeded) }\n",
			wantKind: ownershipViolationDirectClassification,
			want:     1,
		},
		{
			name: "aliased direct classification is rejected",
			source: "package consumer\n" +
				"import (ctxpkg \"context\"; errpkg \"errors\")\n" +
				"func classify(err error) bool { return errpkg.Is(err, ctxpkg.Canceled) }\n",
			wantKind: ownershipViolationDirectClassification,
			want:     1,
		},
		{
			name: "dot imported direct classification is rejected",
			source: "package consumer\n" +
				"import (. \"context\"; . \"errors\")\n" +
				"func classify(err error) bool { return Is(err, Canceled) }\n",
			wantKind: ownershipViolationDirectClassification,
			want:     1,
		},
		{
			name: "direct identity classification is rejected without the errors import",
			source: "package consumer\n" +
				"import \"context\"\n" +
				"func classify(err error) bool { return err == context.Canceled }\n",
			wantKind: ownershipViolationDirectClassification,
			want:     1,
		},
		{
			name: "negated identity classification is rejected",
			source: "package consumer\n" +
				"import \"context\"\n" +
				"func classify(err error) bool { return err != context.DeadlineExceeded }\n",
			wantKind: ownershipViolationDirectClassification,
			want:     1,
		},
		{
			name: "reversed identity classification is rejected",
			source: "package consumer\n" +
				"import \"context\"\n" +
				"func classify(err error) bool { return context.Canceled == err }\n",
			wantKind: ownershipViolationDirectClassification,
			want:     1,
		},
		{
			name: "dot imported identity classification is rejected",
			source: "package consumer\n" +
				"import . \"context\"\n" +
				"func classify(err error) bool { return err == Canceled }\n",
			wantKind: ownershipViolationDirectClassification,
			want:     1,
		},
		{
			name: "comparison against a non-terminal context symbol is allowed",
			source: "package consumer\n" +
				"import \"context\"\n" +
				"func same(ctx context.Context) bool { return ctx == context.TODO() }\n",
		},
		{
			name: "switch case identity classification is rejected",
			source: "package consumer\n" +
				"import \"context\"\n" +
				"func classify(err error) bool {\n" +
				"switch err { case context.Canceled: return true }\n" +
				"return false\n" +
				"}\n",
			wantKind: ownershipViolationDirectClassification,
			want:     1,
		},
		{
			name: "dot imported switch case identity classification is rejected",
			source: "package consumer\n" +
				"import . \"context\"\n" +
				"func classify(err error) bool {\n" +
				"switch err { case DeadlineExceeded: return true }\n" +
				"return false\n" +
				"}\n",
			wantKind: ownershipViolationDirectClassification,
			want:     1,
		},
		{
			name: "switch case against a non-terminal context symbol is allowed",
			source: "package consumer\n" +
				"import \"context\"\n" +
				"func same(ctx context.Context) bool {\n" +
				"switch ctx { case context.TODO(): return true }\n" +
				"return false\n" +
				"}\n",
		},
		{
			name: "similar local call is not mistaken for standard classification",
			source: "package consumer\n" +
				"import \"context\"\n" +
				"type localErrors struct{}\n" +
				"func (localErrors) Is(error, error) bool { return false }\n" +
				"func classify(err error) bool { return (localErrors{}).Is(err, context.Canceled) }\n",
		},
		{
			name: "nil check without Err call is allowed",
			source: "package consumer\n" +
				"import \"context\"\n" +
				"func validate(ctx context.Context) bool { return ctx == nil }\n",
		},
		{
			name: "Err call without nil check is allowed",
			source: "package consumer\n" +
				"import \"context\"\n" +
				"func observe(ctx context.Context) error { return ctx.Err() }\n",
		},
		{
			name: "repeated context boundary is rejected",
			source: "package consumer\n" +
				"import \"context\"\n" +
				"func validate(ctx context.Context) error {\n" +
				"if ctx == nil { return nil }\n" +
				"return ctx.Err()\n" +
				"}\n",
			wantKind: ownershipViolationRepeatedBoundary,
			want:     1,
		},
		{
			name: "aliased context parameter boundary is rejected",
			source: "package consumer\n" +
				"import ctxpkg \"context\"\n" +
				"func validate(ctx ctxpkg.Context) error {\n" +
				"if nil == ctx { return nil }\n" +
				"return ctx.Err()\n" +
				"}\n",
			wantKind: ownershipViolationRepeatedBoundary,
			want:     1,
		},
		{
			name: "different context parameters are not merged",
			source: "package consumer\n" +
				"import \"context\"\n" +
				"func observe(left, right context.Context) error {\n" +
				"if left == nil { return nil }\n" +
				"return right.Err()\n" +
				"}\n",
		},
		{
			name: "pure return forwarder is rejected",
			source: "package consumer\n" +
				"import (\"context\"; contextstate \"" + contextstateImport + "\")\n" +
				"func validate(ctx context.Context) error { return contextstate.Validate(ctx) }\n",
			wantKind: ownershipViolationCompatibility,
			want:     1,
		},
		{
			name: "pure expression forwarder is rejected",
			source: "package consumer\n" +
				"import (\"context\"; contextstate \"" + contextstateImport + "\")\n" +
				"func validate(ctx context.Context) { contextstate.Validate(ctx) }\n",
			wantKind: ownershipViolationCompatibility,
			want:     1,
		},
		{
			name: "type alias is rejected",
			source: "package consumer\n" +
				"import contextstate \"" + contextstateImport + "\"\n" +
				"type State = contextstate.State\n",
			wantKind: ownershipViolationCompatibility,
			want:     1,
		},
		{
			name: "function value alias is rejected",
			source: "package consumer\n" +
				"import contextstate \"" + contextstateImport + "\"\n" +
				"var Classify = contextstate.Classify\n",
			wantKind: ownershipViolationCompatibility,
			want:     1,
		},
		{
			name: "retired contextcheck import is rejected",
			source: "package consumer\n" +
				"import _ \"" + retiredImport + "\"\n",
			wantKind: ownershipViolationRetiredImport,
			want:     1,
		},
		{
			name: "comments and string literals are not source matches",
			source: "package consumer\n" +
				"const prose = \"errors.Is(err, context.Canceled)\"\n" +
				"// ctx == nil; ctx.Err()\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fileSet := token.NewFileSet()
			file, err := parser.ParseFile(
				fileSet,
				"synthetic.go",
				tc.source,
				parser.SkipObjectResolution,
			)
			if err != nil {
				t.Fatal(err)
			}
			var got ownershipViolationInventory
			if auditErr := auditOwnershipFile(
				fileSet,
				file,
				"synthetic.go",
				false,
				paths,
				&got,
			); auditErr != nil {
				t.Fatalf("auditOwnershipFile() error = %v, want nil", auditErr)
			}
			if int(got.count) != tc.want {
				t.Fatalf(
					"ownership violation count = %d, want %d; violations=%+v",
					got.count,
					tc.want,
					got.valuesView(),
				)
			}
			if tc.want > 0 && got.values[0].kind != tc.wantKind {
				t.Fatalf(
					"ownership violation kind = %d, want %d",
					got.values[0].kind,
					tc.wantKind,
				)
			}
		})
	}
}

// TestOwnershipMatcherReportsEveryViolationInOneFile proves the matcher does not
// stop at its first finding and pins the reported order.
func TestOwnershipMatcherReportsEveryViolationInOneFile(t *testing.T) {
	t.Parallel()

	paths := mustOwnershipPaths(t)
	source := "package consumer\n" +
		"import (\n" +
		"\"context\"\n" +
		"\"errors\"\n" +
		"_ \"" + paths.retired + "\"\n" +
		"contextstate \"" + paths.contextstate + "\"\n" +
		")\n" +
		"func classify(err error) bool { return errors.Is(err, context.Canceled) }\n" +
		"func validate(ctx context.Context) error {\n" +
		"if ctx == nil { return nil }\n" +
		"return ctx.Err()\n" +
		"}\n" +
		"var Classify = contextstate.Classify\n"
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(
		fileSet,
		"synthetic.go",
		source,
		parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatal(err)
	}
	var got ownershipViolationInventory
	if auditErr := auditOwnershipFile(
		fileSet,
		file,
		"synthetic.go",
		false,
		paths,
		&got,
	); auditErr != nil {
		t.Fatalf("auditOwnershipFile() error = %v, want nil", auditErr)
	}
	want := []ownershipViolationKind{
		ownershipViolationRetiredImport,
		ownershipViolationDirectClassification,
		ownershipViolationRepeatedBoundary,
		ownershipViolationCompatibility,
	}
	gotKinds := make([]ownershipViolationKind, 0, len(want))
	for _, violation := range got.valuesView() {
		gotKinds = append(gotKinds, violation.kind)
	}
	if !slices.Equal(gotKinds, want) {
		t.Fatalf(
			"ownership violation kinds = %v, want %v; violations=%+v",
			gotKinds,
			want,
			got.valuesView(),
		)
	}
}

// TestOwnershipViolationInventorySaturates proves the fixed inventory refuses a
// violation once full instead of dropping it silently.
func TestOwnershipViolationInventorySaturates(t *testing.T) {
	t.Parallel()

	overflow := ownershipViolation{
		kind:   ownershipViolationRetiredImport,
		symbol: ownershipSymbolRetiredContextcheck,
	}
	var inventory ownershipViolationInventory
	for entry := 1; entry <= ownershipViolationMaximum; entry++ {
		if err := inventory.add(overflow); err != nil {
			t.Fatalf("add() entry %d error = %v, want nil", entry, err)
		}
	}
	if int(inventory.count) != ownershipViolationMaximum {
		t.Fatalf(
			"inventory count = %d, want %d",
			inventory.count,
			ownershipViolationMaximum,
		)
	}
	if err := inventory.add(overflow); !errors.Is(
		err,
		core.ErrContextStateContract,
	) {
		t.Fatalf(
			"saturated add() error = %v, want %v",
			err,
			core.ErrContextStateContract,
		)
	}
	if int(inventory.count) != ownershipViolationMaximum {
		t.Fatalf(
			"saturated inventory count = %d, want %d",
			inventory.count,
			ownershipViolationMaximum,
		)
	}
	if len(inventory.valuesView()) != ownershipViolationMaximum {
		t.Fatalf(
			"saturated inventory view = %d entries, want %d",
			len(inventory.valuesView()),
			ownershipViolationMaximum,
		)
	}
}

func TestLandedPackagesDoNotCloneContextstateOwnershipRatchet(t *testing.T) {
	t.Parallel()

	paths := mustOwnershipPaths(t)
	var violations ownershipViolationInventory
	var audited auditCoverage
	for contract := range core.PrimitiveArchitecture().Packages() {
		auditLandedPackage(t, contract, paths, &violations, &audited)
	}
	wantDirectories := catalogDirectoriesOnDisk(t)
	gotDirectories := slices.Clone(audited.directories)
	slices.Sort(gotDirectories)
	if !slices.Equal(gotDirectories, wantDirectories) {
		t.Fatalf(
			"audited catalog directories = %q, want %q",
			gotDirectories,
			wantDirectories,
		)
	}
	if audited.files == 0 {
		t.Fatalf("audited production files = %d, want at least one", audited.files)
	}
	for _, violation := range violations.valuesView() {
		t.Errorf(
			"contextstate ownership violation kind=%d file=%s line=%d symbol=%s",
			violation.kind,
			violation.file,
			violation.line,
			violation.symbol.String(),
		)
	}
}

// auditCoverage records what the live scan actually read. Without it a drifted
// directory derivation would skip every package and report a vacuous pass.
type auditCoverage struct {
	directories []string
	files       uint32
}

// catalogDirectoriesOnDisk lists the module directories that name a catalog
// package, derived from the repository tree rather than from import paths.
func catalogDirectoriesOnDisk(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir("..")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		identity, parseErr := core.ParsePackageIdentity(entry.Name())
		if parseErr != nil {
			continue
		}
		if validateErr := identity.Validate(); validateErr != nil {
			t.Fatal(validateErr)
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	return names
}

func auditLandedPackage(
	t *testing.T,
	contract core.PackageContract,
	paths ownershipPaths,
	violations *ownershipViolationInventory,
	audited *auditCoverage,
) {
	t.Helper()

	name, err := contract.Identity.Name()
	if err != nil {
		t.Fatal(err)
	}
	packageDirectory := filepath.Join("..", name)
	entries, readErr := os.ReadDir(packageDirectory)
	if errors.Is(readErr, fs.ErrNotExist) {
		return
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	audited.directories = append(audited.directories, name)
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		audited.files++
		auditLandedFile(
			t,
			packageDirectory,
			entry.Name(),
			contract.Identity == core.PackageContextState,
			paths,
			violations,
		)
	}
}

func auditLandedFile(
	t *testing.T,
	packageDirectory string,
	entryName string,
	isContextstate bool,
	paths ownershipPaths,
	violations *ownershipViolationInventory,
) {
	t.Helper()

	fileName := filepath.Join(packageDirectory, entryName)
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(
		fileSet,
		fileName,
		nil,
		parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := auditOwnershipFile(
		fileSet,
		file,
		fileName,
		isContextstate,
		paths,
		violations,
	); err != nil {
		t.Fatal(err)
	}
}

func auditOwnershipFile(
	fileSet *token.FileSet,
	file *ast.File,
	fileName string,
	isContextstate bool,
	paths ownershipPaths,
	violations *ownershipViolationInventory,
) error {
	contextBinding, err := findImportBinding(file, standardContextImportPath)
	if err != nil {
		return err
	}
	errorsBinding, err := findImportBinding(file, standardErrorsImportPath)
	if err != nil {
		return err
	}
	contextstateBinding, err := findImportBinding(file, paths.contextstate)
	if err != nil {
		return err
	}
	retiredBinding, err := findImportBinding(file, paths.retired)
	if err != nil {
		return err
	}
	if retiredBinding.found {
		if addErr := violations.add(ownershipViolation{
			kind:   ownershipViolationRetiredImport,
			file:   fileName,
			symbol: ownershipSymbolRetiredContextcheck,
		}); addErr != nil {
			return addErr
		}
	}
	if isContextstate {
		return nil
	}
	if err := auditDirectClassifiers(
		fileSet,
		file,
		fileName,
		contextBinding,
		errorsBinding,
		violations,
	); err != nil {
		return err
	}
	if err := auditRepeatedBoundaries(
		fileSet,
		file,
		fileName,
		contextBinding,
		violations,
	); err != nil {
		return err
	}
	return auditCompatibilitySurface(
		fileSet,
		file,
		fileName,
		contextstateBinding,
		violations,
	)
}

func auditDirectClassifiers(
	fileSet *token.FileSet,
	file *ast.File,
	fileName string,
	contextBinding importBinding,
	errorsBinding importBinding,
	violations *ownershipViolationInventory,
) error {
	if !contextBinding.found {
		return nil
	}
	var auditErr error
	ast.Inspect(file, func(node ast.Node) bool {
		if auditErr != nil {
			return false
		}
		if !isDirectClassification(node, contextBinding, errorsBinding) {
			return true
		}
		auditErr = violations.add(ownershipViolation{
			kind:   ownershipViolationDirectClassification,
			file:   fileName,
			line:   sourceLine(fileSet, node.Pos()),
			symbol: ownershipSymbolDirectClassification,
		})
		return auditErr == nil
	})
	return auditErr
}

// isDirectClassification reports whether node decides a context terminal state
// without contextstate. All recognized forms are covered: errors.Is against a
// terminal sentinel, direct identity comparison against one, and a switch case
// that names one. Equality and switch cases need no errors import, so the
// errors binding cannot gate this audit.
func isDirectClassification(
	node ast.Node,
	contextBinding importBinding,
	errorsBinding importBinding,
) bool {
	switch value := node.(type) {
	case *ast.CallExpr:
		return errorsBinding.found &&
			len(value.Args) == 2 &&
			bindingCalls(value.Fun, errorsBinding, "Is") &&
			isContextTerminal(value.Args[1], contextBinding)
	case *ast.BinaryExpr:
		return isIdentityClassification(value, contextBinding)
	case *ast.CaseClause:
		return caseClassifiesTerminal(value, contextBinding)
	default:
		return false
	}
}

func isIdentityClassification(
	comparison *ast.BinaryExpr,
	contextBinding importBinding,
) bool {
	if comparison.Op != token.EQL && comparison.Op != token.NEQ {
		return false
	}
	return isContextTerminal(comparison.X, contextBinding) ||
		isContextTerminal(comparison.Y, contextBinding)
}

func caseClassifiesTerminal(
	clause *ast.CaseClause,
	contextBinding importBinding,
) bool {
	for _, expression := range clause.List {
		if isContextTerminal(expression, contextBinding) {
			return true
		}
	}
	return false
}

func auditRepeatedBoundaries(
	fileSet *token.FileSet,
	file *ast.File,
	fileName string,
	contextBinding importBinding,
	violations *ownershipViolationInventory,
) error {
	if !contextBinding.found {
		return nil
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil || function.Type.Params == nil {
			continue
		}
		if err := auditFunctionBoundary(
			fileSet,
			function,
			fileName,
			contextBinding,
			violations,
		); err != nil {
			return err
		}
	}
	return nil
}

func auditFunctionBoundary(
	fileSet *token.FileSet,
	function *ast.FuncDecl,
	fileName string,
	contextBinding importBinding,
	violations *ownershipViolationInventory,
) error {
	for _, field := range function.Type.Params.List {
		if !bindingNamesType(field.Type, contextBinding, "Context") {
			continue
		}
		for _, name := range field.Names {
			if !functionChecksNil(function.Body, name.Name) ||
				!functionCallsErr(function.Body, name.Name) {
				continue
			}
			if err := violations.add(ownershipViolation{
				kind:   ownershipViolationRepeatedBoundary,
				file:   fileName,
				line:   sourceLine(fileSet, function.Pos()),
				symbol: ownershipSymbolRepeatedContextBoundary,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func auditCompatibilitySurface(
	fileSet *token.FileSet,
	file *ast.File,
	fileName string,
	contextstateBinding importBinding,
	violations *ownershipViolationInventory,
) error {
	if !contextstateBinding.found {
		return nil
	}
	for _, declaration := range file.Decls {
		symbol, position := compatibilitySymbol(declaration, contextstateBinding)
		if symbol == ownershipSymbolUnknown {
			continue
		}
		if err := violations.add(ownershipViolation{
			kind:   ownershipViolationCompatibility,
			file:   fileName,
			line:   sourceLine(fileSet, position),
			symbol: symbol,
		}); err != nil {
			return err
		}
	}
	return nil
}

func compatibilitySymbol(
	declaration ast.Decl,
	binding importBinding,
) (ownershipSymbol, token.Pos) {
	switch value := declaration.(type) {
	case *ast.FuncDecl:
		if symbol := forwardedFunction(
			value,
			binding,
		); symbol != ownershipSymbolUnknown {
			return symbol, value.Pos()
		}
	case *ast.GenDecl:
		return compatibilityGeneralSymbol(value, binding)
	}
	return ownershipSymbolUnknown, token.NoPos
}

func compatibilityGeneralSymbol(
	declaration *ast.GenDecl,
	binding importBinding,
) (ownershipSymbol, token.Pos) {
	for _, specification := range declaration.Specs {
		symbol, position := compatibilitySpecificationSymbol(
			specification,
			binding,
		)
		if symbol != ownershipSymbolUnknown {
			return symbol, position
		}
	}
	return ownershipSymbolUnknown, token.NoPos
}

func compatibilitySpecificationSymbol(
	specification ast.Spec,
	binding importBinding,
) (ownershipSymbol, token.Pos) {
	switch value := specification.(type) {
	case *ast.TypeSpec:
		if value.Assign.IsValid() {
			return boundSelectorName(value.Type, binding), value.Pos()
		}
	case *ast.ValueSpec:
		for _, expression := range value.Values {
			if symbol := boundSelectorName(
				expression,
				binding,
			); symbol != ownershipSymbolUnknown {
				return symbol, value.Pos()
			}
		}
	}
	return ownershipSymbolUnknown, token.NoPos
}

func forwardedFunction(
	function *ast.FuncDecl,
	binding importBinding,
) ownershipSymbol {
	if function.Body == nil || len(function.Body.List) != 1 {
		return ownershipSymbolUnknown
	}
	switch statement := function.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(statement.Results) == 1 {
			return forwardedCallName(statement.Results[0], binding)
		}
	case *ast.ExprStmt:
		return forwardedCallName(statement.X, binding)
	}
	return ownershipSymbolUnknown
}

func forwardedCallName(
	expression ast.Expr,
	binding importBinding,
) ownershipSymbol {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return ownershipSymbolUnknown
	}
	return boundSelectorName(call.Fun, binding)
}

func boundSelectorName(
	expression ast.Expr,
	binding importBinding,
) ownershipSymbol {
	if binding.dot {
		identifier, _ := expression.(*ast.Ident)
		if identifier != nil {
			return parseContextstateSymbol(identifier.Name)
		}
		return ownershipSymbolUnknown
	}
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || !isIdentifier(selector.X, binding.name) {
		return ownershipSymbolUnknown
	}
	return parseContextstateSymbol(selector.Sel.Name)
}

func parseContextstateSymbol(name string) ownershipSymbol {
	switch name {
	case "Classify":
		return ownershipSymbolClassify
	case "ObserveAfterDone":
		return ownershipSymbolObserveAfterDone
	case "State":
		return ownershipSymbolState
	case "Validate":
		return ownershipSymbolValidate
	default:
		return ownershipSymbolUnknown
	}
}

func functionChecksNil(body *ast.BlockStmt, name string) bool {
	found := false
	inspectFunctionBody(body, func(node ast.Node) {
		binary, ok := node.(*ast.BinaryExpr)
		if !ok || (binary.Op != token.EQL && binary.Op != token.NEQ) {
			return
		}
		if (isIdentifier(binary.X, name) && isIdentifier(binary.Y, "nil")) ||
			(isIdentifier(binary.Y, name) && isIdentifier(binary.X, "nil")) {
			found = true
		}
	})
	return found
}

func functionCallsErr(body *ast.BlockStmt, name string) bool {
	found := false
	inspectFunctionBody(body, func(node ast.Node) {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "Err" &&
			isIdentifier(selector.X, name) {
			found = true
		}
	})
	return found
}

func inspectFunctionBody(body *ast.BlockStmt, visit func(ast.Node)) {
	ast.Inspect(body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		visit(node)
		return true
	})
}

func bindingCalls(
	expression ast.Expr,
	binding importBinding,
	name string,
) bool {
	if binding.dot {
		return isIdentifier(expression, name)
	}
	selector, ok := expression.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == name &&
		isIdentifier(selector.X, binding.name)
}

func bindingNamesType(
	expression ast.Expr,
	binding importBinding,
	name string,
) bool {
	return bindingCalls(expression, binding, name)
}

func isContextTerminal(
	expression ast.Expr,
	binding importBinding,
) bool {
	return bindingCalls(expression, binding, "Canceled") ||
		bindingCalls(expression, binding, "DeadlineExceeded")
}

func isIdentifier(expression ast.Expr, name string) bool {
	identifier, _ := expression.(*ast.Ident)
	return identifier != nil && identifier.Name == name
}

func sourceLine(fileSet *token.FileSet, position token.Pos) uint32 {
	return uint32(fileSet.Position(position).Line)
}

func findImportBinding(
	file *ast.File,
	importPath string,
) (importBinding, error) {
	for _, specification := range file.Imports {
		path, err := strconv.Unquote(specification.Path.Value)
		if err != nil {
			return importBinding{}, err
		}
		if path != importPath {
			continue
		}
		name := filepath.Base(path)
		if specification.Name != nil {
			name = specification.Name.Name
		}
		return importBinding{
			name:  name,
			found: true,
			dot:   name == ".",
		}, nil
	}
	return importBinding{}, nil
}

func mustOwnershipPaths(t *testing.T) ownershipPaths {
	t.Helper()

	contextstatePath, err := core.PackageContextState.ImportPath()
	if err != nil {
		t.Fatal(err)
	}
	return ownershipPaths{
		contextstate: contextstatePath,
		retired: core.PrimitivePackagePathPrefix +
			retiredContextcheckPackageName,
	}
}
