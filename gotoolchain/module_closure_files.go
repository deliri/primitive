package gotoolchain

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Borrow one regular, non-symlink entry through Filestore's existing native
// custody boundary. The parent and file close before the loan returns, even
// after cancellation; every acquisition/consumer/close cause is retained.
func withModuleCacheFile(ctx context.Context, path core.AbsolutePath, use func(io.Reader) error) (resultErr error) {
	location, err := filestore.OpenParent(ctx, path)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, filestore.CloseRoot(filestore.RootHandleRequest{Root: location.Root}))
	}()
	file, err := filestore.OpenCustodyRead(ctx, filestore.ReadHandleRequest{Location: location})
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, filestore.CloseFile(filestore.FileHandleRequest{File: file}))
	}()
	return use(moduleContextReader{ctx: ctx, source: file})
}

func moduleCachePathKind(ctx context.Context, path core.AbsolutePath) (filestore.PathKind, error) {
	observed, err := filestore.Inspect(ctx, path)
	if err != nil {
		return filestore.PathKindUnknown, err
	}
	return observed.Kind()
}
