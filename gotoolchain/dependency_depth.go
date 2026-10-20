package gotoolchain

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/packages"
)

// PackageDependencyDepthRequest borrows one exact cmd/go-selected metadata
// root. Within restricts traversed edges to that import subtree; the source
// itself may be outside it, for example a command importing internal packages.
type PackageDependencyDepthRequest struct {
	Unit   *packages.Package
	Within gomodule.ImportPath
}

func (r PackageDependencyDepthRequest) Validate() error {
	if r.Unit == nil {
		return core.ErrGoToolchainContract
	}
	if err := r.Within.Validate(); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	return nil
}

func (PackageDependencyDepthRequest) goToolchainInternalFlow() {}

// PackageDependencyDepthObservation is an exact longest-path count over the
// native selected root's cmd/go import closure, including blank imports.
// It is neither a copied import graph nor an architecture-policy verdict.
type PackageDependencyDepthObservation struct {
	Source gomodule.ImportPath
	Within gomodule.ImportPath
	Depth  uint64
}

func (o PackageDependencyDepthObservation) Validate() error {
	if err := errors.Join(o.Source.Validate(), o.Within.Validate()); err != nil {
		return errors.Join(core.ErrGoToolchainContract, err)
	}
	// A path cannot contain more vertices than Go can represent in its native
	// package containers. This is a representation bound, not an input quota.
	if o.Depth > uint64(math.MaxInt) {
		return core.ErrGoToolchainContract
	}
	return nil
}

func (PackageDependencyDepthObservation) goToolchainInternalFlow() {}

// DependencyDepth reads the existing cmd/go graph, which remains the sole
// import authority. Primitive owns traversal and temporary numeric memoization;
// no nodes, edges, aliases or source files are copied into another graph.
func (a PackageAnalysis) DependencyDepth(ctx context.Context, request PackageDependencyDepthRequest) (PackageDependencyDepthObservation, error) {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), a.Validate(), request.Validate()); err != nil {
		return PackageDependencyDepthObservation{}, err
	}
	if a.Incomplete {
		return PackageDependencyDepthObservation{}, core.ErrGoToolchainOutput
	}
	owned := false
	for _, metadata := range a.Metadata {
		if metadata == request.Unit {
			owned = true
			break
		}
	}
	if !owned {
		return PackageDependencyDepthObservation{}, core.ErrGoToolchainContract
	}
	source, err := gomodule.ParseImportPath(request.Unit.PkgPath)
	if err != nil {
		return PackageDependencyDepthObservation{}, errors.Join(core.ErrGoToolchainOutput, err)
	}
	depth, err := nativePackageDependencyDepth(ctx, request.Unit, request.Within, make(map[*packages.Package]uint64), make(map[*packages.Package]bool))
	if err != nil {
		return PackageDependencyDepthObservation{}, err
	}
	observation := PackageDependencyDepthObservation{Source: source, Within: request.Within, Depth: depth}
	if err := errors.Join(observation.Validate(), temporal.Wait(temporal.WaitRequest{Context: ctx})); err != nil {
		return PackageDependencyDepthObservation{}, err
	}
	return observation, nil
}

func nativePackageDependencyDepth(ctx context.Context, unit *packages.Package, within gomodule.ImportPath, memo map[*packages.Package]uint64, active map[*packages.Package]bool) (uint64, error) {
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx}); err != nil {
		return 0, err
	}
	if unit == nil || unit.PkgPath == "" || unit.IllTyped || len(unit.Errors) != 0 {
		return 0, core.ErrGoToolchainOutput
	}
	if depth, ok := memo[unit]; ok {
		return depth, nil
	}
	if active[unit] {
		return 0, core.ErrGoToolchainOutput
	}
	active[unit] = true
	var depth uint64
	for _, imported := range unit.Imports {
		if imported == nil || imported.PkgPath == "" {
			return 0, core.ErrGoToolchainOutput
		}
		if imported.PkgPath != within.String() && !strings.HasPrefix(imported.PkgPath, within.String()+"/") {
			continue
		}
		child, err := nativePackageDependencyDepth(ctx, imported, within, memo, active)
		if err != nil {
			return 0, err
		}
		if child == uint64(math.MaxInt) {
			return 0, core.ErrGoToolchainOutput
		}
		depth = max(depth, child+1)
	}
	delete(active, unit)
	memo[unit] = depth
	return depth, nil
}
