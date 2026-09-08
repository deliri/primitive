package gotoolchain

import (
	"context"
	"errors"
	"runtime"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"golang.org/x/sync/errgroup"
)

// AnalysisBatchRequest selects exact packages in one cmd/go working directory.
// Packages are strictly ordered and unique. The caller chooses a bounded batch;
// this is temporary compiler work, not a repository catalog or retained cache.
type AnalysisBatchRequest struct {
	WorkingDirectory core.AbsolutePath
	Packages         []gomodule.ImportPath
	IncludeTests     bool
}

func (r AnalysisBatchRequest) Validate() error {
	if err := r.WorkingDirectory.Validate(); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	if len(r.Packages) == 0 {
		return contractError("analysis batch has no packages")
	}
	previous := ""
	for _, pkg := range r.Packages {
		if err := pkg.Validate(); err != nil {
			return errors.Join(core.ErrGoToolchainContract, err)
		}
		if strings.Compare(previous, pkg.String()) >= 0 {
			return contractError("analysis batch packages are not strictly ordered")
		}
		previous = pkg.String()
	}
	return nil
}

// AnalysisResult preserves the exact request even when its compiler load fails.
// Err is the original typed error chain. Successful units in a partial Analysis
// remain usable; metadata is read-only while the batch callback is executing.
type AnalysisResult struct {
	Request  AnalysisRequest
	Analysis PackageAnalysis
	Err      error
}

func (r AnalysisResult) Validate() error {
	if err := r.Request.Validate(); err != nil {
		return err
	}
	if r.Err != nil && len(r.Analysis.Metadata) == 0 && len(r.Analysis.Units) == 0 {
		return r.validateEmptyAnalysis()
	}
	if r.Analysis.WorkingDirectory != r.Request.WorkingDirectory || r.Analysis.Package != r.Request.Package || r.Analysis.IncludeTests != r.Request.IncludeTests {
		return contractError("analysis result does not belong to its request")
	}
	if r.Err == nil && r.Analysis.Incomplete {
		return contractError("incomplete analysis has no compiler error")
	}
	return r.Analysis.Validate()
}

func (r AnalysisResult) validateEmptyAnalysis() error {
	if r.Analysis.WorkingDirectory != (core.AbsolutePath{}) || r.Analysis.Package != (gomodule.ImportPath{}) || r.Analysis.IncludeTests || r.Analysis.Incomplete {
		return contractError("failed empty analysis contains unowned metadata")
	}
	return nil
}

// AnalyzePackages loads export metadata once, then checks independent packages
// with separate Go checkers and importers. consume runs concurrently, at most
// GOMAXPROCS calls at a time. Callback failure cancels and joins all owned work;
// compiler failure is delivered for its package and does not cancel siblings.
func (c Capability) AnalyzePackages(ctx context.Context, request AnalysisBatchRequest, consume func(context.Context, AnalysisResult) error) error {
	if ctx == nil || consume == nil {
		return contractError("analysis batch context or consumer is nil")
	}
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return err
	}
	if uint64(len(request.Packages)) > uint64(c.configuration.Limits.PackageMaximum) {
		return contractError("analysis batch exceeds package bound")
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(core.ErrGoToolchainExecution, err)
	}
	loaded, loadErr := c.loadAnalysisMetadata(ctx, request.WorkingDirectory, request.Packages, request.IncludeTests)
	exports := collectCanonicalExports(loaded)
	group, work := errgroup.WithContext(ctx)
	group.SetLimit(runtime.GOMAXPROCS(0))
	for _, pkg := range request.Packages {
		group.Go(func() error {
			if err := work.Err(); err != nil {
				return errors.Join(core.ErrGoToolchainExecution, err)
			}
			selected := AnalysisRequest{WorkingDirectory: request.WorkingDirectory, Package: pkg, IncludeTests: request.IncludeTests}
			result := AnalysisResult{Request: selected}
			if loadErr != nil {
				// An early command refusal can obscure healthy siblings. Retry exact
				// packages only in this failure case; successful loads are never repeated.
				result.Analysis, result.Err = c.AnalyzePackage(work, selected)
			} else {
				result.Analysis, result.Err = compilePackageAnalysis(work, loaded, selected, exports)
			}
			if err := result.Validate(); err != nil {
				return err
			}
			return consume(work, result)
		})
	}
	return group.Wait()
}

func (AnalysisBatchRequest) goToolchainProtocolFact() {}
func (AnalysisResult) goToolchainProtocolFact()       {}
