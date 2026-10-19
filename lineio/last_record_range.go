package lineio

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// LastRecordRangeRequest borrows one stable native byte window. Positional
// reads preserve the source's sequential cursor. BufferBytes controls working
// memory, not record length or total input. The caller owns source lifetime.
type LastRecordRangeRequest struct {
	Source      io.ReaderAt
	Extent      core.ByteLength
	BufferBytes core.ByteCount
}

func (r LastRecordRangeRequest) Validate() error {
	if core.IsNil(r.Source) {
		return core.ErrLineIOContract
	}
	return errors.Join(r.Extent.Validate(), r.BufferBytes.Validate())
}

// RecordRangeObservation distinguishes an observed empty window from refusal.
// An absent observation has no coordinates; a physical record is LF- or
// EOF-framed. Native read errors never manufacture an empty observation.
type RecordRangeObservation struct {
	Record  RecordRange
	Present bool
}

func (o RecordRangeObservation) Validate() error {
	if !o.Present {
		if o.Record != (RecordRange{}) {
			return core.ErrLineIOContract
		}
		return nil
	}
	if o.Record.Framing != RecordFramingLF && o.Record.Framing != RecordFramingEOF {
		return core.ErrLineIOContract
	}
	return o.Record.Validate()
}

// ObserveLastRecordRange scans backwards only as far as the preceding LF.
// It retains one fixed Go byte window and returns original byte coordinates,
// including a final LF when present. No record text or preceding population is
// copied. A file truncated beneath the admitted extent is a native refusal.
func ObserveLastRecordRange(ctx context.Context, request LastRecordRangeRequest) (observation RecordRangeObservation, resultErr error) {
	defer func() {
		if ctx != nil && (errors.Is(resultErr, context.Canceled) || errors.Is(resultErr, context.DeadlineExceeded)) {
			resultErr = errors.Join(resultErr, context.Cause(ctx))
		}
		if resultErr != nil {
			observation = RecordRangeObservation{}
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return observation, err
	}
	extent, err := request.Extent.Int64()
	if err != nil {
		return observation, err
	}
	if extent == 0 {
		return RecordRangeObservation{}, nil
	}
	width, err := request.BufferBytes.Int64()
	if err != nil {
		return observation, err
	}
	window := make([]byte, min(width, extent))
	framing := RecordFramingEOF
	for end := extent; end > 0; {
		if err := contextstate.Validate(ctx); err != nil {
			return observation, err
		}
		start := max(int64(0), end-int64(len(window)))
		want := int(end - start)
		n, readErr := request.Source.ReadAt(window[:want], start)
		if err := contextstate.Validate(ctx); err != nil {
			return observation, errors.Join(err, readErr)
		}
		if n != want {
			return observation, errors.Join(core.ErrLineIOScan, readErr, io.ErrUnexpectedEOF)
		}
		if readErr != nil && readErr != io.EOF {
			return observation, errors.Join(core.ErrLineIOScan, readErr)
		}
		if end == extent && window[n-1] == '\n' {
			framing = RecordFramingLF
		}
		for index := n - 1; index >= 0; index-- {
			offset := start + int64(index)
			if offset == extent-1 {
				continue
			}
			if window[index] == '\n' {
				size, err := core.NewByteLength(uint64(extent - offset - 1))
				if err != nil {
					return observation, err
				}
				observation = RecordRangeObservation{Record: RecordRange{Offset: SourceByteOffset(offset + 1), Bytes: size, Framing: framing}, Present: true}
				return observation, errors.Join(observation.Validate(), contextstate.Validate(ctx))
			}
		}
		end = start
	}
	size, err := core.NewByteLength(uint64(extent))
	if err != nil {
		return observation, err
	}
	observation = RecordRangeObservation{Record: RecordRange{Bytes: size, Framing: framing}, Present: true}
	return observation, errors.Join(observation.Validate(), contextstate.Validate(ctx))
}

var _ core.Validatable = LastRecordRangeRequest{}
var _ core.Validatable = RecordRangeObservation{}
