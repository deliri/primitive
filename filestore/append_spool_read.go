package filestore

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// AppendSpoolReadRequest lends one independent cursor until Use returns.
// The callback must honor its context and never retain the reader. Its own
// error remains consumer meaning; this operation does not cancel production.
type AppendSpoolReadRequest struct {
	Use func(context.Context, io.Reader) error
}

func (r AppendSpoolReadRequest) Validate() error {
	if r.Use == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

func (s *AppendSpool) WithReader(ctx context.Context, request AppendSpoolReadRequest) (resultErr error) {
	if err := errors.Join(s.Validate(), scopeCancellationError(ctx, contextstate.Validate(ctx)), request.Validate()); err != nil {
		return err
	}
	s.mutex.Lock()
	if s.closing {
		s.mutex.Unlock()
		return contractError(fs.ErrClosed)
	}
	s.readers.Add(1)
	s.mutex.Unlock()
	defer s.readers.Done()
	lifetime, cancel := context.WithCancelCause(ctx)
	joined := make(chan struct{})
	stop := context.AfterFunc(s.ctx, func() {
		defer close(joined)
		cancel(context.Cause(s.ctx))
	})
	defer func() {
		if !stop() {
			<-joined
		}
		resultErr = scopeCancellationError(lifetime, errors.Join(resultErr, contextstate.Validate(lifetime)))
		cancel(fs.ErrClosed)
		resultErr = scopeCancellationError(ctx, resultErr)
	}()
	return useFileScope(lifetime, s.file, fileScopeCallback{Use: func(ctx context.Context, _ *os.File) error {
		return request.Use(ctx, &appendSpoolReader{owner: s, ctx: ctx})
	}})
}

type appendSpoolReader struct {
	owner  *AppendSpool
	ctx    context.Context
	offset int64
}

func (r *appendSpoolReader) Read(data []byte) (int, error) {
	for {
		if err := scopeCancellationError(r.ctx, contextstate.Validate(r.ctx)); err != nil {
			return 0, err
		}
		if len(data) == 0 {
			return 0, nil
		}
		r.owner.mutex.Lock()
		remaining := r.owner.extent - r.offset
		sealed, closing, terminal, changed := r.owner.sealed, r.owner.closing, r.owner.terminal, r.owner.changed
		r.owner.mutex.Unlock()
		if closing {
			return 0, contractError(fs.ErrClosed)
		}
		if remaining > 0 {
			window := data[:min(int64(len(data)), remaining)]
			read, err := r.owner.file.ReadAt(window, r.offset)
			r.offset += int64(read)
			if err != nil {
				return read, sourceError(err)
			}
			return read, nil
		}
		if sealed || terminal != nil {
			if terminal != nil {
				return 0, terminal
			}
			return 0, io.EOF
		}
		select {
		case <-changed:
		case <-r.ctx.Done():
			return 0, errors.Join(r.ctx.Err(), context.Cause(r.ctx))
		}
	}
}

var (
	_ core.Validatable = AppendSpoolReadRequest{}
	_ io.Reader        = (*appendSpoolReader)(nil)
)
