package filestore

import (
	"context"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
)

// OpenRead opens one existing regular file for reading below a rooted
// boundary and hands the caller the real OS handle.
//
// Read already streams a file into an io.Writer, which serves a caller that
// owns the destination. It does not serve a caller that must hand a reader to
// something else — a hasher, a bounded upload, a content-addressed ingest —
// because there is no way to turn a writer destination back into a source
// without buffering the whole file or standing up a pipe and a goroutine.
// Products facing that reach for os.Open, and reaching past the rooted
// boundary is exactly what this package exists to prevent. OpenAppend already
// returns a real handle for the write side; this is its read counterpart.
//
// The caller owns the returned handle and must close it. A path that is not a
// regular file is refused before any bytes are read, so a symbolic link,
// directory, or device planted at the name cannot answer for the file the
// caller asked for.
func OpenRead(ctx context.Context, request ReadHandleRequest) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return openRegularReadFile(
		request.Location.Root,
		request.Location.Path.String(),
	)
}
