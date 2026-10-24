package filestore

import (
	"context"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// NamedPipeRequest names one exclusively created native FIFO. Path is an
// authorized absolute name, not a held-root capability. The caller coordinates
// its parent and namespace. Native umask applies to Mode, including mode 0000.
type NamedPipeRequest struct {
	Path core.AbsolutePath
	Mode fs.FileMode
}

// Validate admits an absolute name and only native permission bits.
func (r NamedPipeRequest) Validate() error {
	if err := r.Path.Validate(); err != nil {
		return contractError(err)
	}
	if r.Mode != r.Mode.Perm() {
		return core.ErrFilestoreContract
	}
	return nil
}

// CreateNamedPipe delegates one exclusive FIFO creation to Darwin or Linux.
// Existing entries are preserved. Absolute path resolution has ordinary native
// namespace semantics, with no held-root confinement or rename reservation.
// This is not a durability acknowledgement. Context is admitted before the
// synchronous syscall; cancellation cannot interrupt it after entry.
func CreateNamedPipe(ctx context.Context, request NamedPipeRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return scopeCancellationError(ctx, err)
	}
	if err := request.Validate(); err != nil {
		return err
	}
	return createNamedPipe(request)
}

// OpenNamedPipeReadWrite opens an existing FIFO read/write through Go's rooted
// boundary, then verifies the kind on the acquired handle. It creates nothing,
// truncates nothing and reads no bytes. The caller owns the returned Go file.
// O_RDWR FIFO behavior is native Darwin/Linux behavior, not a POSIX portability
// guarantee. Callers coordinate namespace replacement; kind is checked after
// native open, so a racing replacement may be opened before it is refused.
// Context is checked before the synchronous operation, not used to interrupt
// native open or later reads. Other platforms report errors.ErrUnsupported.
func OpenNamedPipeReadWrite(ctx context.Context, location Location) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, scopeCancellationError(ctx, err)
	}
	if err := location.Validate(); err != nil {
		return nil, err
	}
	return openNamedPipeReadWrite(location)
}

var _ core.Validatable = NamedPipeRequest{}
