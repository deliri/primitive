package lineio

import (
	"context"
	"errors"
	"io"
	"math"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// RecordReadRequest lends one physical record as an io.Reader. Use must finish
// synchronously. The native source remains owned by the caller.
type RecordReadRequest struct {
	Use     func(context.Context, io.Reader) error
	Capture io.Writer
}

func (r RecordReadRequest) Validate() error {
	if r.Use == nil {
		return core.ErrLineIOContract
	}
	if r.Capture != nil && core.WriterIsNil(r.Capture) {
		return core.ErrLineIOContract
	}
	return nil
}

// RecordReadObservation reports only consumed extent and physical framing.
// A sequential source supplies no absolute positional-read coordinate.
type RecordReadObservation struct {
	Bytes   core.ByteLength
	Framing RecordFraming
}

func (r RecordReadObservation) Validate() error {
	extent, err := r.Bytes.Int64()
	if err != nil || extent == 0 || (r.Framing != RecordFramingLF && r.Framing != RecordFramingEOF) {
		return errors.Join(core.ErrLineIOContract, err)
	}
	return nil
}

// WithRecord lends exactly one LF-framed record, including its delimiter, or
// the final unterminated record. A callback refusal drains this record before
// returning, so the next call starts at the next record. Only bare io.EOF means
// that no record remains. The existing fixed buffer owns all framing; neither
// a record nor the whole source is collected. A panic expires the borrowed
// reader and propagates immediately; the caller still owns source interruption.
func (r *Reader) WithRecord(ctx context.Context, request RecordReadRequest) (result RecordReadObservation, err error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate(), request.Validate()); err != nil {
		return RecordReadObservation{}, err
	}
	fragment, sourceErr := r.ReadFragment(ctx)
	if len(fragment.Bytes) == 0 && sourceErr == io.EOF {
		return RecordReadObservation{}, io.EOF
	}
	source := &recordReadSource{reader: r, context: ctx, fragment: fragment, sourceErr: sourceErr, capture: request.Capture}
	defer func() {
		source.reader = nil
		source.fragment = Fragment{}
		source.context = nil
		source.sourceErr = nil
		source.capture = nil
	}()
	useErr := request.Use(ctx, source)
	_, drainErr := io.Copy(io.Discard, source)
	if err := errors.Join(useErr, drainErr); err != nil {
		return RecordReadObservation{}, err
	}
	framing := RecordFramingLF
	if source.sourceErr == io.EOF {
		framing = RecordFramingEOF
	}
	extent, err := core.NewByteLength(uint64(source.consumed))
	if err != nil {
		return RecordReadObservation{}, err
	}
	result = RecordReadObservation{Bytes: extent, Framing: framing}
	if err := result.Validate(); err != nil {
		return RecordReadObservation{}, err
	}
	return result, nil
}

// recordReadSource adapts borrowed native fragments to io.Reader. It retains
// only the current fragment and byte cursor; Reader owns delimiter detection.
type recordReadSource struct {
	reader    *Reader
	context   context.Context
	fragment  Fragment
	sourceErr error
	consumed  int64
	capture   io.Writer
}

func (s *recordReadSource) Read(destination []byte) (int, error) {
	if s.reader == nil {
		return 0, core.ErrLineIOContract
	}
	if len(destination) == 0 {
		return 0, nil
	}
	if err := contextstate.Validate(s.context); err != nil {
		return 0, err
	}
	for len(s.fragment.Bytes) == 0 {
		if s.sourceErr != nil {
			return 0, s.sourceErr
		}
		if !s.fragment.More {
			return 0, io.EOF
		}
		s.fragment, s.sourceErr = s.reader.ReadFragment(s.context)
	}
	count := min(len(destination), len(s.fragment.Bytes))
	if int64(count) > math.MaxInt64-s.consumed {
		return 0, core.ErrLineIOContract
	}
	if s.capture != nil {
		written, writeErr := s.capture.Write(s.fragment.Bytes[:count])
		if written < 0 || written > count {
			writeErr = errors.Join(core.ErrLineIOContract, writeErr)
		} else if written != count && writeErr == nil {
			writeErr = io.ErrShortWrite
		}
		if err := errors.Join(writeErr, contextstate.Validate(s.context)); err != nil {
			s.sourceErr = errors.Join(core.ErrLineIOScan, s.sourceErr, err)
			s.fragment = Fragment{}
			return 0, s.sourceErr
		}
	}
	copy(destination, s.fragment.Bytes[:count])
	s.consumed += int64(count)
	s.fragment.Bytes = s.fragment.Bytes[count:]
	return count, nil
}
