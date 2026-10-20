package gotoolchain

import (
	"context"
	"errors"
	"fmt"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// SSAUnitRequest borrows one exact unit selected by this PackageAnalysis. Use
// runs synchronously. The caller projects its policy from Go's native IR and
// must finish using the view before returning; no product call graph is built.
type SSAUnitRequest struct {
	Unit *packages.Package
	Use  func(context.Context, SSAUnitView) error
	// SourceValues requests Go's GlobalDebug source references. They preserve
	// native ValueForExpr and DebugRef placement, including
	// aliases and loops whose backedge was eliminated. Constants keep Go's
	// ordinary absence semantics; no product value table is constructed.
	SourceValues bool
}

func (r SSAUnitRequest) Validate() error {
	if r.Unit == nil || r.Use == nil {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAUnitRequest) goToolchainInternalFlow() {}

// SSAUnitView lends the Go team's actual objects for one native build. It is
// not a wire representation, persisted instruction model or runtime verdict.
// Source and IR retain their native pointer identity, including call operands,
// closure bindings, method wrappers, deferred calls and constant branch conditions.
type SSAUnitView struct {
	Source  *packages.Package
	Program *ssa.Program
	Package *ssa.Package
}

func (v SSAUnitView) Validate() error {
	if v.Source == nil || v.Source.Types == nil || v.Source.TypesInfo == nil || v.Source.Fset == nil || v.Program == nil || v.Package == nil {
		return core.ErrGoToolchainContract
	}
	if v.Package.Prog != v.Program || v.Program.Fset != v.Source.Fset || v.Package.Pkg != v.Source.Types || v.Program.Package(v.Source.Types) != v.Package {
		return core.ErrGoToolchainContract
	}
	if initial := v.Package.Func("init"); initial == nil || len(initial.Blocks) == 0 {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (SSAUnitView) goToolchainInternalFlow() {}

// WithSSAUnit owns one native Go SSA construction. That is an explicit
// whole-unit compiler allocation, not an O(1) compiler claim. Native construction
// is synchronous; cancellation is observed around the builder and consumer.
// The view adds no second function, alias or instruction catalog. Go's native
// structures and private compiler maps remain solely the Go team's authority.
func (a PackageAnalysis) WithSSAUnit(ctx context.Context, request SSAUnitRequest) error {
	if err := errors.Join(temporal.Wait(temporal.WaitRequest{Context: ctx}), a.Validate(), request.Validate()); err != nil {
		return err
	}
	if a.Incomplete {
		return core.ErrGoToolchainOutput
	}
	owns := false
	for _, unit := range a.Units {
		if unit == request.Unit {
			owns = true
			break
		}
	}
	if !owns {
		return core.ErrGoToolchainContract
	}
	program, compiled, err := buildSSAUnit(request.Unit, request.SourceValues)
	if err != nil {
		return err
	}
	view := SSAUnitView{Source: request.Unit, Program: program, Package: compiled}
	if err := errors.Join(view.Validate(), temporal.Wait(temporal.WaitRequest{Context: ctx})); err != nil {
		return err
	}
	err = request.Use(ctx, view)
	return errors.Join(err, temporal.Wait(temporal.WaitRequest{Context: ctx}))
}

// A native compiler assertion is a refused observation. Limit recovery to Go's
// builder: a panic in the caller's callback must retain ordinary Go semantics.
func buildSSAUnit(unit *packages.Package, sourceValues bool) (program *ssa.Program, compiled *ssa.Package, resultErr error) {
	defer func() {
		if failure := recover(); failure != nil {
			program, compiled = nil, nil
			if cause, ok := failure.(error); ok {
				resultErr = errors.Join(core.ErrGoToolchainOutput, cause)
			} else {
				resultErr = fmt.Errorf("%w: native SSA builder: %v", core.ErrGoToolchainOutput, failure)
			}
		}
	}()
	mode := ssa.BuilderMode(0)
	if sourceValues {
		mode = ssa.GlobalDebug
	}
	program = ssa.NewProgram(unit.Fset, mode)
	for _, imported := range unit.Types.Imports() {
		program.CreatePackage(imported, nil, nil, true)
	}
	compiled = program.CreatePackage(unit.Types, unit.Syntax, unit.TypesInfo, false)
	compiled.Build()
	return program, compiled, nil
}
