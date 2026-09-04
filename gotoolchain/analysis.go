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
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/tools/go/packages"
)

func compilePackageAnalysis(ctx context.Context, loaded []*packages.Package, request AnalysisRequest) (PackageAnalysis, error) {
	metadata := selectAnalysisMetadata(loaded, request.Package.String(), request.IncludeTests)
	if len(metadata) == 0 {
		return PackageAnalysis{}, outputError("package analysis metadata does not contain the requested package", nil)
	}
	exports := collectCanonicalExports(loaded)
	units := make([]*packages.Package, 0, len(metadata))
	for _, packageMetadata := range metadata {
		if err := ctx.Err(); err != nil {
			return PackageAnalysis{}, errors.Join(core.ErrGoToolchainExecution, err)
		}
		unit, err := compileAnalysisUnit(packageMetadata, exports)
		if err != nil {
			return PackageAnalysis{}, outputError("package analysis compiler pass failed", err)
		}
		units = append(units, unit)
	}
	analysis := PackageAnalysis{
		WorkingDirectory: request.WorkingDirectory,
		Package:          request.Package,
		IncludeTests:     request.IncludeTests,
		Units:            units,
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
		return includeTests || unit.ForTest == ""
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

func compileAnalysisUnit(metadata *packages.Package, exports map[string]string) (*packages.Package, error) {
	if err := validateAnalysisMetadata(metadata); err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	syntax, err := parseAnalysisSyntax(fset, metadata.CompiledGoFiles)
	if err != nil {
		return nil, err
	}
	information := newTypesInformation()
	configuration := types.Config{
		Context:   types.NewContext(),
		Importer:  importer.ForCompiler(fset, "gc", analysisExportLookup(metadata, exports)),
		Sizes:     metadata.TypesSizes,
		GoVersion: analysisGoVersion(metadata),
	}
	typedPackage, err := configuration.Check(metadata.PkgPath, fset, syntax, information)
	if err != nil {
		return nil, err
	}
	return &packages.Package{
		ID: metadata.ID, Name: metadata.Name, PkgPath: metadata.PkgPath, Dir: metadata.Dir,
		GoFiles: metadata.GoFiles, CompiledGoFiles: metadata.CompiledGoFiles,
		ExportFile: metadata.ExportFile, Module: metadata.Module, ForTest: metadata.ForTest,
		Types: typedPackage, Fset: fset, Syntax: syntax, TypesInfo: information,
		TypesSizes: metadata.TypesSizes,
	}, nil
}

func validateAnalysisMetadata(metadata *packages.Package) error {
	if metadata == nil || metadata.Name == "" || metadata.PkgPath == "" || len(metadata.CompiledGoFiles) == 0 {
		return errors.New("compiler metadata is incomplete")
	}
	if len(metadata.Errors) != 0 {
		return errors.New(metadata.Errors[0].Error())
	}
	if metadata.TypesSizes == nil {
		return errors.New("compiler metadata has no target sizes")
	}
	return nil
}

func parseAnalysisSyntax(fset *token.FileSet, paths []string) ([]*ast.File, error) {
	syntax := make([]*ast.File, 0, len(paths))
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		syntax = append(syntax, file)
	}
	return syntax, nil
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

func analysisExportLookup(metadata *packages.Package, exports map[string]string) importer.Lookup {
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
		return openAnalysisExport(exportPath)
	}
}

func openAnalysisExport(path string) (io.ReadCloser, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("compiler export path is not absolute")
	}
	return os.OpenInRoot(filepath.Dir(path), filepath.Base(path))
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
