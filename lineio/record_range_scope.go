package lineio

import (
	"context"
	"errors"
	"io"
	"iter"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// RecordRangeReadRequest lends one exact, natively framed source range. Use
// completes synchronously and must not retain or concurrently read its loan.
// Source remains inside its caller's native file or memory lifetime.
type RecordRangeReadRequest struct {
	Use   func(context.Context, io.Reader) error
	Range RecordFragmentRequest
}

func (r RecordRangeReadRequest) Validate() error {
	if r.Use == nil {
		return errors.Join(core.ErrLineIOContract, r.Range.Validate())
	}
	return r.Range.Validate()
}

// WithRecordRange lends existing RecordFragments as an io.Reader without
// collecting a record or changing the source's sequential position. It drains
// the range after every ordinary callback return, including refusal, so native
// framing, extent and read failures cannot disappear behind an early decode.
// Every exit expires the reader; panic and Goexit stop the native coroutine.
func WithRecordRange(ctx context.Context, request RecordRangeReadRequest) (resultErr error) {
	defer func() {
		if ctx != nil && (errors.Is(resultErr, context.Canceled) || errors.Is(resultErr, context.DeadlineExceeded)) {
			resultErr = errors.Join(resultErr, context.Cause(ctx))
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return err
	}
	next, stop := iter.Pull2(RecordFragments(ctx, request.Range))
	source := recordRangeSource{context: ctx, next: next}
	defer func() {
		source.next = nil
		source.fragment = Fragment{}
		source.context = nil
		source.readErr = nil
		stop()
	}()
	useErr := request.Use(ctx, &source)
	_, drainErr := io.Copy(io.Discard, &source)
	return errors.Join(useErr, drainErr, contextstate.Validate(ctx))
}

// recordRangeSource retains one borrowed fragment; RecordFragments alone owns
// positioning, physical extent and framing. It starts no independent worker.
type recordRangeSource struct {
	context  context.Context
	next     func() (Fragment, error, bool)
	fragment Fragment
	readErr  error
}

func (s *recordRangeSource) Read(destination []byte) (int, error) {
	if s.next == nil {
		return 0, core.ErrLineIOContract
	}
	if s.readErr != nil {
		return 0, s.readErr
	}
	if len(destination) == 0 {
		return 0, nil
	}
	if err := contextstate.Validate(s.context); err != nil {
		return 0, err
	}
	for len(s.fragment.Bytes) == 0 {
		fragment, err, ok := s.next()
		if err != nil {
			s.readErr = err
			return 0, err
		}
		if !ok {
			return 0, io.EOF
		}
		s.fragment = fragment
	}
	count := copy(destination, s.fragment.Bytes)
	s.fragment.Bytes = s.fragment.Bytes[count:]
	return count, nil
}

var _ core.Validatable = RecordRangeReadRequest{}
