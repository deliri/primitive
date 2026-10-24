package filestore

import (
	"context"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
)

// DuplicateBlockingFile deliberately applies Go's File.Fd blocking transition
// before duplicating the borrowed native descriptor on Darwin or Linux. This
// changes the shared open description, including the source's blocking flags
// and deadline behavior; it is not a non-mutating observation. Source and
// duplicate also share the native offset. Callers coordinate concurrent use.
//
// The source is never closed. Dup receives the descriptor held inside
// SyscallConn.Control, not an earlier numeric descriptor vulnerable to reuse.
// The caller owns and closes the returned ordinary Go file independently.
// Native Dup does not copy FD_CLOEXEC; no close-on-exec flag is added here.
// There is no retry, goroutine, buffer, or cancellation promise after native
// entry. Other platforms report errors.ErrUnsupported.
func DuplicateBlockingFile(ctx context.Context, request FileHandleRequest) (*os.File, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, scopeCancellationError(ctx, err)
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return duplicateBlockingFile(request.File)
}
