package gomodule

import (
	"bytes"
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// Requirement sets one canonical module version. An existing requirement keeps
// its indirect marking and comments; a newly added requirement is direct.
type Requirement struct {
	Path    Path
	Version string
}

func (r Requirement) Validate() error {
	if err := r.Path.Validate(); err != nil {
		return err
	}
	if err := module.Check(r.Path.String(), r.Version); err != nil {
		return errors.Join(core.ErrGoModuleContract, err)
	}
	if module.CanonicalVersion(r.Version) != r.Version {
		return core.ErrGoModuleContract
	}
	return nil
}

// RequireEditRequest borrows one caller-bounded go.mod source and its edits.
// Working memory is proportional to that source and the number of edits.
type RequireEditRequest struct {
	Source       []byte
	Requirements []Requirement
}

func (r RequireEditRequest) Validate() error {
	if len(r.Source) == 0 {
		return core.ErrGoModuleContract
	}
	seen := make(map[Path]struct{}, len(r.Requirements))
	for _, requirement := range r.Requirements {
		if err := requirement.Validate(); err != nil {
			return err
		}
		if _, exists := seen[requirement.Path]; exists {
			return core.ErrGoModuleContract
		}
		seen[requirement.Path] = struct{}{}
	}
	return nil
}

// EditRequirements uses Go's module parser and formatter without filesystem,
// process, or download effects. Refusals return nil and never alter the input.
// No edits returns independent, byte-identical validated source.
func EditRequirements(ctx context.Context, request RequireEditRequest) ([]byte, error) {
	if err := requireEditContext(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	file, err := modfile.Parse("go.mod", request.Source, nil)
	if err != nil {
		return nil, errors.Join(core.ErrGoModuleContract, err)
	}
	if file.Module == nil {
		return nil, core.ErrGoModuleContract
	}
	if _, err := parseDeclaredPath(file.Module.Mod.Path); err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(file.Require))
	for _, requirement := range file.Require {
		if _, exists := seen[requirement.Mod.Path]; exists {
			return nil, core.ErrGoModuleContract
		}
		seen[requirement.Mod.Path] = struct{}{}
		if err := module.Check(requirement.Mod.Path, requirement.Mod.Version); err != nil {
			return nil, errors.Join(core.ErrGoModuleContract, err)
		}
	}
	if err := requireEditContext(ctx); err != nil {
		return nil, err
	}
	if len(request.Requirements) == 0 {
		return bytes.Clone(request.Source), nil
	}
	for _, requirement := range request.Requirements {
		if err := requireEditContext(ctx); err != nil {
			return nil, err
		}
		if err := file.AddRequire(requirement.Path.String(), requirement.Version); err != nil {
			return nil, errors.Join(core.ErrGoModuleContract, err)
		}
	}
	formatted, err := file.Format()
	if err != nil {
		return nil, errors.Join(core.ErrGoModuleContract, err)
	}
	if err := requireEditContext(ctx); err != nil {
		return nil, err
	}
	return formatted, nil
}

func requireEditContext(ctx context.Context) error {
	if err := contextstate.Validate(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return errors.Join(core.ErrGoModuleContract, err, context.Cause(ctx))
		}
		return errors.Join(core.ErrGoModuleContract, err)
	}
	return nil
}
