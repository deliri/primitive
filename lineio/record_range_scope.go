package lineio

import (
	"context"
	"errors"
	"io"

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
// Every exit expires the reader, including panic and Goexit.
func WithRecordRange(ctx context.Context, request RecordRangeReadRequest) (resultErr error) {
	defer func() {
		if ctx != nil && (errors.Is(resultErr, context.Canceled) || errors.Is(resultErr, context.DeadlineExceeded)) {
			resultErr = errors.Join(resultErr, context.Cause(ctx))
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return err
	}
	reader, err := newRecordFragmentReader(ctx, request.Range)
	if err != nil {
		return err
	}
	source := recordRangeSource{context: ctx, reader: reader}
	defer func() {
		source.reader = nil
		source.fragment = Fragment{}
		source.context = nil
		source.readErr = nil
	}()
	useErr := request.Use(ctx, &source)
	_, drainErr := io.Copy(io.Discard, &source)
	return errors.Join(useErr, drainErr, contextstate.Validate(ctx))
}

// recordRangeSource retains one borrowed fragment; RecordFragments alone owns
// positioning, physical extent and framing. It starts no independent worker.
type recordRangeSource struct {
	context  context.Context
	reader   *recordFragmentReader
	fragment Fragment
	readErr  error
}

func (s *recordRangeSource) Read(destination []byte) (int, error) {
	if s.reader == nil {
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
		fragment, err := s.reader.readFragment(s.context)
		if err == io.EOF {
			return 0, io.EOF
		}
		if err != nil {
			s.readErr = err
			return 0, err
		}
		s.fragment = fragment
	}
	count := copy(destination, s.fragment.Bytes)
	s.fragment.Bytes = s.fragment.Bytes[count:]
	return count, nil
}

var _ core.Validatable = RecordRangeReadRequest{}
