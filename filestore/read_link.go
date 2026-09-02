package filestore

import (
	"context"

	"github.com/deliri/primitive/v2026/contextstate"
)

// ReadSymbolicLink observes one bounded target without following the link.
func ReadSymbolicLink(ctx context.Context, location Location) (SymbolicLinkTarget, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return SymbolicLinkTarget{}, err
	}
	if err := location.Validate(); err != nil {
		return SymbolicLinkTarget{}, err
	}
	value, err := location.Root.Readlink(location.Path.String())
	if err != nil {
		return SymbolicLinkTarget{}, sourceError(err)
	}
	target := SymbolicLinkTarget{value: value}
	if err := target.Validate(); err != nil {
		return SymbolicLinkTarget{}, err
	}
	return target, nil
}
