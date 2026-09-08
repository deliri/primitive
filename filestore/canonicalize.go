package filestore

import (
	"context"
	"path/filepath"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// Canonicalize resolves an existing absolute path through Go's
// filepath.EvalSymlinks and admits the result as an absolute path. It follows
// links, including links outside the original directory; it does not reserve
// the observed identity or guarantee filesystem case normalization.
func Canonicalize(ctx context.Context, path core.AbsolutePath) (core.AbsolutePath, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return core.AbsolutePath{}, err
	}
	if err := path.Validate(); err != nil {
		return core.AbsolutePath{}, contractError(err)
	}
	resolved, err := filepath.EvalSymlinks(path.String())
	if err != nil {
		return core.AbsolutePath{}, sourceError(err)
	}
	canonical, err := core.ParseAbsolutePath(resolved)
	if err != nil {
		return core.AbsolutePath{}, contractError(err)
	}
	return canonical, nil
}
