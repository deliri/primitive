package gotoolchain

import (
	"errors"
	"go/version"

	"github.com/deliri/primitive/v2026/core"
)

// Compare delegates language, release, prerelease and custom suffix ordering
// to Go. Invalid nominal operands never inherit Go's fallback text ordering.
func (v ToolchainVersion) Compare(other ToolchainVersion) (core.Comparison, error) {
	if err := errors.Join(v.Validate(), other.Validate()); err != nil {
		return core.ComparisonUnknown, err
	}
	order := version.Compare(v.value, other.value)
	switch {
	case order < 0:
		return core.ComparisonLess, nil
	case order > 0:
		return core.ComparisonGreater, nil
	default:
		return core.ComparisonEqual, nil
	}
}
