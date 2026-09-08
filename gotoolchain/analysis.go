package gotoolchain

import (
	"context"
	"errors"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"golang.org/x/tools/go/packages"
)

// Test variants must not share concurrently checked named types. Go 1.27.1
// with x/tools v0.49.0 races inside Named.unpack during LoadSyntax(Tests:true).
// Each package owns its compiler and checks its units sequentially. A canonical
// importer and parsed syntax can be reused within that package; test-specific
// export selections get a separate importer. No checker/importer crosses
// concurrent package work. Keep the real race regression at this boundary.
func compilePackageAnalysis(ctx context.Context, loaded []*packages.Package, request AnalysisRequest, exports map[string]string) (PackageAnalysis, error) {
	metadata := selectAnalysisMetadata(loaded, request.Package.String(), request.IncludeTests)
	if len(metadata) == 0 {
		return PackageAnalysis{}, outputError("package analysis metadata does not contain the requested package", nil)
	}
	units := make([]*packages.Package, 0, len(metadata))
	compiler := analysisUnitCompiler{ctx: ctx, fset: token.NewFileSet(), exports: exports, syntax: make(map[string]*ast.File)}
	var failures []error
	for _, packageMetadata := range metadata {
		if err := ctx.Err(); err != nil {
			return PackageAnalysis{}, errors.Join(core.ErrGoToolchainExecution, err)
		}
		unit, err := compiler.compile(packageMetadata)
		if err != nil {
			compiler.canonical = nil
			failures = append(failures, outputError("package analysis compiler pass failed", err))
		}
		if unit != nil {
			units = append(units, unit)
		}
	}
	analysis := PackageAnalysis{
		WorkingDirectory: request.WorkingDirectory,
		Package:          request.Package,
		IncludeTests:     request.IncludeTests,
		Units:            units,
		Metadata:         metadata,
		Incomplete:       len(failures) != 0,
	}
	if len(failures) != 0 {
		return analysis, errors.Join(failures...)
	}
	if err := analysis.Validate(); err != nil {
		return PackageAnalysis{}, outputError("package analysis is incomplete", err)
	}
	return analysis, nil
}

func selectAnalysisMetadata(loaded []*packages.Package, requested string, includeTests bool) []*packages.Package {
	selected := make([]*packages.Package, 0, len(loaded))
	for _, unit := range loaded {
		if unit == nil || !analysisMetadataMatches(unit, requested, includeTests) {
			continue
		}
		selected = append(selected, unit)
	}
	sort.Slice(selected, func(left, right int) bool { return selected[left].ID < selected[right].ID })
	return selected
}

func analysisMetadataMatches(unit *packages.Package, requested string, includeTests bool) bool {
	if unit.PkgPath == requested {
		return unit.ForTest == "" || includeTests && unit.ForTest == requested
	}
	return includeTests && unit.ForTest == requested && unit.PkgPath == requested+"_test"
}

func collectCanonicalExports(roots []*packages.Package) map[string]string {
	exports := make(map[string]string)
	identities := make(map[string]string)
	visited := make(map[string]struct{})
	var visit func(*packages.Package)
	visit = func(unit *packages.Package) {
		if unit == nil {
			return
		}
		if _, ok := visited[unit.ID]; ok {
			return
		}
		visited[unit.ID] = struct{}{}
		if unit.ExportFile != "" && canonicalExportPrecedes(unit, identities[unit.PkgPath]) {
			exports[unit.PkgPath] = unit.ExportFile
			identities[unit.PkgPath] = unit.ID
		}
		for _, dependency := range unit.Imports {
			visit(dependency)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	return exports
}

func canonicalExportPrecedes(unit *packages.Package, current string) bool {
	if current == "" {
		return true
	}
	if unit.ID == unit.PkgPath {
		return true
	}
	return current != unit.PkgPath && unit.ID < current
}

type analysisUnitCompiler struct {
	ctx       context.Context
	fset      *token.FileSet
	exports   map[string]string
	syntax    map[string]*ast.File
	canonical types.Importer
}

func (analysisUnitCompiler) goToolchainInternalFlow() {}

func (c *analysisUnitCompiler) compile(metadata *packages.Package) (*packages.Package, error) {
	if err := validateAnalysisMetadata(metadata); err != nil {
		return nil, err
	}
	diagnostics := analysisDiagnostics{}
	for _, diagnostic := range metadata.Errors {
		diagnostics.add(diagnostic)
	}
	files := analysisSyntaxFiles(metadata)
	syntax := c.parse(files, &diagnostics)
	information := newTypesInformation()
	configuration := types.Config{
		Context:   types.NewContext(),
		Importer:  c.importer(metadata),
		Sizes:     metadata.TypesSizes,
		GoVersion: analysisGoVersion(metadata),
		Error:     diagnostics.add,
	}
	if analysisImportsCgo(syntax) && !configureCgo(&configuration) {
		return nil, outputError("Go checker does not support original cgo syntax", nil)
	}
	// A test-only directory has a valid empty production package. Its name
	// comes from cmd/go even when there is no production AST to declare it.
	typedPackage := types.NewPackage(metadata.PkgPath, metadata.Name)
	checkable := slices.DeleteFunc(slices.Clone(syntax), func(file *ast.File) bool { return file == nil })
	before := len(diagnostics.records)
	checkErr := types.NewChecker(&configuration, c.fset, typedPackage, information).Files(checkable)
	if checkErr != nil && len(diagnostics.records) == before {
		diagnostics.add(checkErr)
	}
	return &packages.Package{
		ID: metadata.ID, Name: metadata.Name, PkgPath: metadata.PkgPath, Dir: metadata.Dir,
		GoFiles: metadata.GoFiles, CompiledGoFiles: files,
		ExportFile: metadata.ExportFile, Module: metadata.Module, ForTest: metadata.ForTest,
		Types: typedPackage, Fset: c.fset, Syntax: syntax, TypesInfo: information,
		TypesSizes: metadata.TypesSizes,
		IllTyped:   len(diagnostics.failures) != 0, Errors: diagnostics.records, TypeErrors: diagnostics.typeErrors,
	}, diagnostics.failure()
}

// Export failure may leave CompiledGoFiles empty even though cmd/go selected
// ordinary GoFiles. Check those selected inputs as explicitly partial syntax;
// keep the original metadata untouched and never invent cgo-generated files.
func analysisSyntaxFiles(metadata *packages.Package) []string {
	if len(metadata.CompiledGoFiles) != 0 || len(metadata.Errors) == 0 {
		return metadata.CompiledGoFiles
	}
	return metadata.GoFiles
}

func analysisImportsCgo(files []*ast.File) bool {
	for _, file := range files {
		if file == nil {
			continue
		}
		for _, imported := range file.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err == nil && name == core.GoCgoImportPath {
				return true
			}
		}
	}
	return false
}

func (c *analysisUnitCompiler) importer(metadata *packages.Package) types.Importer {
	if !canonicalAnalysisImports(metadata, c.exports) {
		return importer.ForCompiler(c.fset, "gc", analysisExportLookup(c.ctx, metadata, c.exports))
	}
	if c.canonical == nil {
		c.canonical = importer.ForCompiler(c.fset, "gc", analysisExportLookup(c.ctx, metadata, c.exports))
	}
	return c.canonical
}

func canonicalAnalysisImports(metadata *packages.Package, exports map[string]string) bool {
	for path, dependency := range metadata.Imports {
		if dependency == nil || path != dependency.PkgPath || dependency.ID != dependency.PkgPath || dependency.ExportFile != exports[path] {
			return false
		}
	}
	return true
}

func validateAnalysisMetadata(metadata *packages.Package) error {
	if metadata == nil || metadata.Name == "" || metadata.PkgPath == "" {
		return errors.New("compiler metadata is incomplete")
	}
	if metadata.TypesSizes == nil {
		return errors.New("compiler metadata has no target sizes")
	}
	return nil
}

func (c *analysisUnitCompiler) parse(paths []string, diagnostics *analysisDiagnostics) []*ast.File {
	syntax := make([]*ast.File, 0, len(paths))
	for _, path := range paths {
		file := c.syntax[path]
		if file == nil {
			var err error
			file, err = c.parseFile(path)
			if err != nil {
				diagnostics.add(err)
			} else {
				c.syntax[path] = file
			}
		}
		syntax = append(syntax, file)
	}
	return syntax
}

func (c *analysisUnitCompiler) parseFile(path string) (*ast.File, error) {
	input, err := openAnalysisInput(c.ctx, path)
	if err != nil {
		return nil, err
	}
	file, parseErr := parser.ParseFile(c.fset, path, input, parser.ParseComments|parser.SkipObjectResolution|parser.AllErrors)
	if closeErr := input.Close(); closeErr != nil {
		return file, errors.Join(parseErr, closeErr)
	}
	return file, parseErr
}

func newTypesInformation() *types.Info {
	return &types.Info{
		Types:        make(map[ast.Expr]types.TypeAndValue),
		Instances:    make(map[*ast.Ident]types.Instance),
		Defs:         make(map[*ast.Ident]types.Object),
		Uses:         make(map[*ast.Ident]types.Object),
		Implicits:    make(map[ast.Node]types.Object),
		Selections:   make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:       make(map[ast.Node]*types.Scope),
		FileVersions: make(map[*ast.File]string),
	}
}

func analysisExportLookup(ctx context.Context, metadata *packages.Package, exports map[string]string) importer.Lookup {
	direct := make(map[string]string, len(metadata.Imports))
	for path, dependency := range metadata.Imports {
		if dependency != nil && dependency.ExportFile != "" {
			direct[path] = dependency.ExportFile
		}
	}
	return func(path string) (io.ReadCloser, error) {
		exportPath := direct[path]
		if exportPath == "" {
			exportPath = exports[path]
		}
		if exportPath == "" {
			return nil, errors.New("compiler export is unavailable for " + path)
		}
		return openAnalysisInput(ctx, exportPath)
	}
}

func openAnalysisInput(ctx context.Context, path string) (io.ReadCloser, error) {
	absolute, err := core.ParseAbsolutePath(path)
	if err != nil {
		return nil, err
	}
	location, err := filestore.OpenParent(ctx, absolute)
	if err != nil {
		return nil, err
	}
	file, readErr := filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: location})
	if err := errors.Join(readErr, location.Root.Close()); err != nil {
		if file != nil {
			err = errors.Join(err, file.Close())
		}
		return nil, err
	}
	if file == nil {
		return nil, outputError("compiler input handle is absent", nil)
	}
	return file, nil
}

func analysisGoVersion(metadata *packages.Package) string {
	if metadata.Module == nil || metadata.Module.GoVersion == "" {
		return ""
	}
	if strings.HasPrefix(metadata.Module.GoVersion, "go") {
		return metadata.Module.GoVersion
	}
	return "go" + metadata.Module.GoVersion
}
