package gotoolchain

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/packages"
)

// DependencyAnalysisRequest borrows one exact unit from the owning scope and
// names one of its direct cmd/go-selected imports, including a blank import.
type DependencyAnalysisRequest struct {
	Unit   *packages.Package
	Import gomodule.ImportPath
}

func (r DependencyAnalysisRequest) Validate() error {
	if r.Unit == nil {
		return core.ErrGoToolchainContract
	}
	if err := r.Import.Validate(); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	return nil
}

func (DependencyAnalysisRequest) goToolchainInternalFlow() {}

// DependencyAnalysisScope owns full source compiler results only during its
// synchronous callback. It is not safe for concurrent use. All source and build
// inputs and all borrowed compiler/metadata objects MUST remain immutable for
// the whole callback: mutation violates the caller contract and is not detected.
// Open a new scope after any source or build-input change.
// Borrowed units must not be retained beyond the callback. No reuse crosses it.
// Copied handles share the private owner and expire with the original handle.
type DependencyAnalysisScope struct {
	owner *dependencyAnalysisScopeOwner
}

type dependencyAnalysisScopeOwner struct {
	unit      *packages.Package
	metadata  *packages.Package
	directory core.AbsolutePath
	context   context.Context
	exports   map[string]string
	results   map[*packages.Package]AnalysisResult
	active    bool
}

func (*DependencyAnalysisScope) goToolchainCapabilityWrapper() {}
func (*dependencyAnalysisScopeOwner) goToolchainInternalFlow() {}

func (s *DependencyAnalysisScope) Validate() error {
	if s == nil || s.owner == nil || !s.owner.active || s.owner.context == nil || s.owner.unit == nil || s.owner.metadata == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}

// WithDependencyAnalysis opens its own complete production analysis, then lends
// its direct dependencies on demand. IncludeTests is refused: independent test
// variants are not this scope's extent. Every dependency is fully parsed and
// type checked once, including function bodies and private declarations. The
// original metadata/refusal is preserved; no eager dependency source check or
// callback runs after a root failure. There is no compiler-heap O(1) claim.
func (c Capability) WithDependencyAnalysis(ctx context.Context, request AnalysisRequest, use func(context.Context, PackageAnalysis, *DependencyAnalysisScope) error) error {
	if ctx == nil || use == nil || request.IncludeTests {
		return core.ErrGoToolchainContract
	}
	if err := dependencyAnalysisContext(ctx); err != nil {
		return err
	}
	analysis, err := c.AnalyzePackage(ctx, request)
	if err != nil {
		return err
	}
	if err := dependencyAnalysisContext(ctx); err != nil {
		return err
	}
	owner := &dependencyAnalysisScopeOwner{unit: analysis.Units[0], metadata: analysis.Metadata[0], directory: analysis.WorkingDirectory, context: ctx, active: true}
	scope := &DependencyAnalysisScope{owner: owner}
	defer owner.close()
	err = use(ctx, analysis, scope)
	return errors.Join(err, dependencyAnalysisContext(ctx))
}

// AnalyzeDependency checks ownership before returning a borrowed full analysis.
// Repeated physical import sites reuse compiler objects, not product findings;
// callers still apply their policy at each original site in their own order.
func (s *DependencyAnalysisScope) AnalyzeDependency(ctx context.Context, request DependencyAnalysisRequest) (PackageAnalysis, error) {
	if err := errors.Join(s.Validate(), request.Validate()); err != nil {
		return PackageAnalysis{}, err
	}
	owner := s.owner
	if err := errors.Join(dependencyAnalysisContext(owner.context), dependencyAnalysisContext(ctx)); err != nil {
		return PackageAnalysis{}, err
	}
	metadata, err := owner.dependencyMetadata(request)
	if err != nil {
		return PackageAnalysis{}, err
	}
	if result, exists := owner.results[metadata]; exists {
		return result.Analysis, errors.Join(result.Err, dependencyAnalysisContext(owner.context), dependencyAnalysisContext(ctx))
	}
	if owner.exports == nil {
		owner.exports = collectCanonicalExports([]*packages.Package{owner.metadata})
		owner.results = make(map[*packages.Package]AnalysisResult)
	}
	path, err := gomodule.ParseImportPath(metadata.PkgPath)
	if err != nil {
		return PackageAnalysis{}, errors.Join(core.ErrGoToolchainOutput, err)
	}
	selected := AnalysisRequest{WorkingDirectory: owner.directory, Package: path}
	analysis, compileErr := compilePackageAnalysis(ctx, []*packages.Package{metadata}, selected, owner.exports)
	result := AnalysisResult{Request: selected, Analysis: analysis, Err: compileErr}
	if err := result.Validate(); err != nil {
		return PackageAnalysis{}, err
	}
	owner.results[metadata] = result
	return analysis, errors.Join(compileErr, dependencyAnalysisContext(owner.context), dependencyAnalysisContext(ctx))
}

func (s *dependencyAnalysisScopeOwner) dependencyMetadata(request DependencyAnalysisRequest) (*packages.Package, error) {
	if request.Unit != s.unit {
		return nil, core.ErrGoToolchainContract
	}
	metadata := s.metadata.Imports[request.Import.String()]
	if metadata == nil {
		return nil, core.ErrGoToolchainContract
	}
	return metadata, nil
}

func (s *dependencyAnalysisScopeOwner) close() {
	s.active = false
	s.unit, s.metadata = nil, nil
	s.directory = core.AbsolutePath{}
	s.context = nil
	s.exports, s.results = nil, nil
}

func dependencyAnalysisContext(ctx context.Context) error {
	if ctx == nil {
		return core.ErrGoToolchainContract
	}
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return errors.Join(core.ErrGoToolchainExecution, err)
	}
	return nil
}
