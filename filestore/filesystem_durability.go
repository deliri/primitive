package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// FilesystemDurabilityRequest lends synchronous work a filesystem writeback
// window. Location must name a real directory; its root remains caller-owned.
// Use must finish and join all of its writers before returning. It must not
// retain work beyond the callback. The scope neither inspects the callback's
// entries nor follows their symbolic-link targets. It does not roll back effects.
type FilesystemDurabilityRequest struct {
	Location Location
	Use      func(context.Context) error
}

func (r FilesystemDurabilityRequest) Validate() error {
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return r.Location.Validate()
}

// WithFilesystemDurability acquires a directory descriptor before Use and
// synchronizes that same descriptor's entire filesystem after successful,
// joined work. This is filesystem-wide I/O, including unrelated dirty entries;
// it is not a directory-only flush. Namespace changes to the directory during
// Use do not replace the held descriptor. Callback refusal or cancellation
// closes it without starting synchronization; every path attempts closure.
// Panics propagate through closure.
//
// Linux amd64 and arm64 use syncfs. Linux 5.8+ reports filesystem writeback
// errors since this descriptor's acquisition; older kernels do not provide
// that error-window guarantee. Native synchronization acknowledges the OS,
// not immunity to hardware failure. Other platforms refuse before Use.
// Callback, native synchronization, close and context causes remain joined.
func WithFilesystemDurability(ctx context.Context, request FilesystemDurabilityRequest) (resultErr error) {
	defer func() { resultErr = scopeCancellationError(ctx, resultErr) }()
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := filesystemDurabilityPlatform(); err != nil {
		return err
	}
	info, err := request.Location.Root.Lstat(request.Location.Path.String())
	if err != nil {
		return sourceError(err)
	}
	if !info.IsDir() {
		return sourceError(fs.ErrInvalid)
	}
	file, err := openDirectory(request.Location.Root, request.Location.Path.String())
	if err != nil {
		return sourceError(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			resultErr = errors.Join(resultErr, cleanupError(err))
		}
		resultErr = errors.Join(resultErr, contextstate.Validate(ctx))
	}()
	opened, err := file.Stat()
	if err != nil {
		return sourceError(err)
	}
	if !opened.IsDir() || !os.SameFile(info, opened) {
		return sourceError(fs.ErrInvalid)
	}
	if err := request.Use(ctx); err != nil {
		return err
	}
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := synchronizeFilesystem(ctx, file); err != nil {
		return activationError(err)
	}
	return nil
}

var _ core.Validatable = FilesystemDurabilityRequest{}
