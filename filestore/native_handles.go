package filestore

import (
	"context"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// FileHandleRequest borrows an acquired Go file. Presence is admission;
// Go observes whether its native handle is still open when used.
type FileHandleRequest struct{ File *os.File }

func (r FileHandleRequest) Validate() error {
	if r.File == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// RootHandleRequest borrows an acquired Go root without tracking another
// lifetime. Closing a root does not close files previously opened through it.
type RootHandleRequest struct{ Root *os.Root }

func (r RootHandleRequest) Validate() error {
	if r.Root == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// CloseFile attempts native closure without a live-context prerequisite.
// Cleanup after cancellation remains possible; native closed errors survive.
func CloseFile(request FileHandleRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if err := request.File.Close(); err != nil {
		return cleanupError(err)
	}
	return nil
}

// CloseRoot delegates root closure to Go, including repeated-close semantics.
// Callers separately own every child file acquired from that root.
func CloseRoot(request RootHandleRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if err := request.Root.Close(); err != nil {
		return cleanupError(err)
	}
	return nil
}

// SyncFile admits a live context before Go's blocking Sync operation. It
// neither flushes another buffered writer nor closes the borrowed file.
func SyncFile(ctx context.Context, request FileHandleRequest) error {
	if err := contextstate.Validate(ctx); err != nil {
		return scopeCancellationError(ctx, err)
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := request.File.Sync(); err != nil {
		return destinationError(err)
	}
	return nil
}
