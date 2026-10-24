package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"sync"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// AppendSpoolScopeRequest exclusively creates one caller-named working file.
// Callers select its filesystem and own all writes to the file. Use owns its
// task lifetimes and must cancel and join them before
// returning. The scope also cancels and joins accepted reader callbacks before
// closing the original descriptor. It never removes the file.
type AppendSpoolScopeRequest struct {
	Use      func(context.Context, *AppendSpool) error
	Location Location
	Mode     fs.FileMode
}

func (r AppendSpoolScopeRequest) Validate() error {
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return (AppendRequest{Location: r.Location, Mode: r.Mode, Append: AppendCreate, Access: AppendAccessReadWrite}).Validate()
}

// AppendSpool owns a single append descriptor, its successful-write prefix
// and digest. Readers retain only their own cursor. A slow consumer never
// participates in Write; native working-file I/O is the producer's only sink.
// The owner must not be copied. No handle or deletion authority escapes.
type AppendSpool struct {
	self     *AppendSpool
	file     *os.File
	ctx      context.Context
	digest   *core.DigestWriter
	mutex    sync.Mutex
	readers  sync.WaitGroup
	changed  chan struct{}
	extent   int64
	terminal error
	sealed   bool
	closing  bool
}

// AppendSpoolResult preserves the ordinary file scope's operation and native
// close observations. Content is available only after successful production,
// synchronization, reader join and original-descriptor closure.
type AppendSpoolResult struct {
	scope   FileScopeResult
	content ContentIndexEntry
}

func (r AppendSpoolResult) Validate() error       { return r.scope.Validate() }
func (r AppendSpoolResult) OperationError() error { return r.scope.OperationError() }
func (r AppendSpoolResult) CleanupError() error   { return r.scope.CleanupError() }

func (r AppendSpoolResult) Content() (ContentIndexEntry, error) {
	if err := errors.Join(r.Validate(), r.OperationError(), r.CleanupError()); err != nil {
		return ContentIndexEntry{}, err
	}
	return r.content, r.content.Validate()
}

func WithAppendSpoolScope(ctx context.Context, request AppendSpoolScopeRequest) (result AppendSpoolResult, resultErr error) {
	if err := errors.Join(scopeCancellationError(ctx, contextstate.Validate(ctx)), request.Validate()); err != nil {
		return AppendSpoolResult{}, err
	}
	lifetime, cancel := context.WithCancelCause(ctx)
	defer cancel(fs.ErrClosed)
	file, err := OpenAppend(ctx, AppendRequest{Location: request.Location, Mode: request.Mode, Append: AppendCreate, Access: AppendAccessReadWrite})
	if err != nil {
		return AppendSpoolResult{}, err
	}
	spool := &AppendSpool{file: file, ctx: lifetime, digest: core.NewDigestWriter(), changed: make(chan struct{})}
	spool.self = spool
	result.scope.operationError = core.ErrFilestoreCallbackPanic
	defer func() {
		spool.mutex.Lock()
		spool.closing = true
		if !spool.sealed {
			spool.sealFailure(core.ErrFilestoreSource)
		}
		productionErr := spool.terminal
		spool.notify()
		spool.mutex.Unlock()
		cancel(fs.ErrClosed)
		spool.readers.Wait()
		result.scope.operationError = errors.Join(result.scope.operationError, productionErr, scopeCancellationError(ctx, contextstate.Validate(ctx)))
		if closeErr := file.Close(); closeErr != nil {
			result.scope.cleanupError = cleanupError(closeErr)
		}
		result.scope.completed = true
		if result.scope.operationError != nil || result.scope.cleanupError != nil {
			result.content = ContentIndexEntry{}
		}
		resultErr = result.Validate()
	}()
	result.scope.operationError = useFileScope(lifetime, file, fileScopeCallback{Use: func(ctx context.Context, _ *os.File) error {
		operationErr := request.Use(ctx, spool)
		spool.mutex.Lock()
		defer spool.mutex.Unlock()
		if spool.sealed && spool.terminal == nil {
			digest, extent, observationErr := spool.digest.Digest()
			if observationErr != nil {
				return errors.Join(operationErr, observationErr)
			}
			result.content = ContentIndexEntry{Digest: digest, Extent: extent}
		}
		return operationErr
	}})
	return result, nil
}

func (s *AppendSpool) Validate() error {
	if s == nil || s.self != s || s.file == nil || s.ctx == nil || s.digest == nil {
		return core.ErrFilestoreContract
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.closing {
		return contractError(fs.ErrClosed)
	}
	return nil
}

// notify and sealFailure require the native owner's mutex.
func (s *AppendSpool) notify() {
	close(s.changed)
	s.changed = make(chan struct{})
}

func (s *AppendSpool) sealFailure(cause error) {
	s.sealed = true
	s.terminal = errors.Join(s.terminal, cause)
	s.notify()
}

var (
	_ core.Validatable = AppendSpoolScopeRequest{}
	_ core.Validatable = AppendSpoolResult{}
	_ core.Validatable = (*AppendSpool)(nil)
)
