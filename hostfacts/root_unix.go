//go:build darwin || linux

package hostfacts

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type platformRoot struct {
	directory *filestore.HeldDirectory
	dev       uint64
}

func diskOpenIdentity() core.ErrorIdentity {
	return core.ErrHostFactsObservation
}

func openRoot(ctx context.Context, path core.AbsolutePath) (*platformRoot, error) {
	directory, err := filestore.OpenDirectory(ctx, path)
	if err != nil {
		return nil, err
	}
	filesystem, err := directory.Filesystem()
	if err != nil {
		return nil, errors.Join(err, directory.Close())
	}
	identity, err := filesystem.Uint64()
	if err != nil {
		return nil, errors.Join(err, directory.Close())
	}
	return &platformRoot{directory: directory, dev: identity}, nil
}

func (r *platformRoot) close() error {
	if r == nil || r.directory == nil {
		return core.ErrHostFactsContract
	}
	directory := r.directory
	r.directory = nil
	return directory.Close()
}
