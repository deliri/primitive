package runprotocol

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"iter"
	"unicode"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// GoFuzzSignalRecord observes independent native marker positions. Zero ranges
// mean absence. Consumers choose precedence; no rendered text is authority.
type GoFuzzSignalRecord struct {
	Record        lineio.RecordRange
	Panic         lineio.RecordRange
	Deadline      lineio.RecordRange
	FailureHeader lineio.RecordRange
	FailingInput  lineio.RecordRange
	RerunHeader   lineio.RecordRange
	ReplayCommand lineio.RecordRange
	BuildHeader   lineio.RecordRange
}

func (r GoFuzzSignalRecord) Validate() error {
	if err := r.Record.Validate(); err != nil {
		return err
	}
	whole, _ := r.Record.Bytes.Int64()
	lengths := [...]int64{int64(len("panic:")), int64(len("context deadline exceeded")), int64(len("--- FAIL:")), int64(len("failing input")), int64(len("to re-run:")), int64(len("go test ")), int64(len("# "))}
	for i, span := range [...]lineio.RecordRange{r.Panic, r.Deadline, r.FailureHeader, r.FailingInput, r.RerunHeader, r.ReplayCommand, r.BuildHeader} {
		if span == (lineio.RecordRange{}) {
			continue
		}
		if err := span.Validate(); err != nil {
			return err
		}
		size, _ := span.Bytes.Int64()
		if size != lengths[i] || span.Framing != lineio.RecordFramingOpaque || span.Offset < r.Record.Offset || int64(span.Offset)-int64(r.Record.Offset) > whole-size {
			return core.ErrGoToolchainContract
		}
	}
	return nil
}

type GoFuzzSignalSourceRequest struct{ Source lineio.RecordSource }

func (r GoFuzzSignalSourceRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrGoToolchainContract
	}
	return nil
}

// GoFuzzSignalRecords consumes complete physical records with fixed working
// memory. Standard buffered reads and byte searches own marker observations.
// Native source completion is required before callers can seal facts.
func GoFuzzSignalRecords(ctx context.Context, request GoFuzzSignalSourceRequest) iter.Seq2[GoFuzzSignalRecord, error] {
	return func(yield func(GoFuzzSignalRecord, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(GoFuzzSignalRecord{}, goFuzzSignalRefusal(ctx, err))
			return
		}
		buffer, err := core.NewByteCount(4096)
		if err != nil {
			yield(GoFuzzSignalRecord{}, goFuzzSignalRefusal(ctx, err))
			return
		}
		for record, err := range lineio.RecordRanges(ctx, lineio.RecordRangeRequest{Source: request.Source, BufferBytes: buffer}) {
			if err != nil {
				yield(GoFuzzSignalRecord{}, goFuzzSignalRefusal(ctx, err))
				return
			}
			observation, err := readGoFuzzSignalRecord(ctx, request.Source, record)
			if err != nil {
				yield(GoFuzzSignalRecord{}, goFuzzSignalRefusal(ctx, err))
				return
			}
			if err := errors.Join(observation.Validate(), contextstate.Validate(ctx)); err != nil {
				yield(GoFuzzSignalRecord{}, goFuzzSignalRefusal(ctx, err))
				return
			}
			if !yield(observation, nil) {
				return
			}
		}
	}
}
func (GoFuzzSignalRecord) runProtocolSealedProjection()           {}
func (GoFuzzSignalSourceRequest) runProtocolInternalFlowCarrier() {}

func goFuzzSignalRefusal(ctx context.Context, cause error) error {
	if ctx == nil {
		return errors.Join(cause, core.ErrNilContext)
	}
	return errors.Join(cause, context.Cause(ctx))
}

func goFuzzSignalMarker(offset int64, length int) (lineio.RecordRange, error) {
	extent, err := core.NewByteLength(uint64(length))
	if err != nil {
		return lineio.RecordRange{}, err
	}
	marker := lineio.RecordRange{Offset: lineio.SourceByteOffset(offset), Bytes: extent, Framing: lineio.RecordFramingOpaque}
	return marker, marker.Validate()
}

// Read the physical record once after framing. Leading whitespace contributes
// only its native offset. The fixed overlap preserves markers across reads.
func readGoFuzzSignalRecord(ctx context.Context, source lineio.RecordSource, record lineio.RecordRange) (GoFuzzSignalRecord, error) {
	length, err := record.Bytes.Int64()
	if err != nil {
		return GoFuzzSignalRecord{}, err
	}
	guard := &goFuzzFrameSource{ctx: ctx, source: io.NewSectionReader(source, int64(record.Offset), length)}
	reader := bufio.NewReaderSize(guard, 4096)
	observation := GoFuzzSignalRecord{Record: record}
	var consumed int64
	for {
		value, width, err := reader.ReadRune()
		if err == io.EOF {
			if refusal := errors.Join(guard.readErr, contextstate.Validate(ctx)); refusal != nil {
				return GoFuzzSignalRecord{}, refusal
			}
			if consumed != length {
				return GoFuzzSignalRecord{}, io.ErrUnexpectedEOF
			}
			return observation, nil
		}
		if err != nil {
			return GoFuzzSignalRecord{}, errors.Join(err, guard.readErr)
		}
		if !unicode.IsSpace(value) {
			if err := reader.UnreadRune(); err != nil {
				return GoFuzzSignalRecord{}, err
			}
			break
		}
		consumed += int64(width)
	}
	prefix, peekErr := reader.Peek(len("to re-run:"))
	if peekErr != nil && peekErr != io.EOF {
		return GoFuzzSignalRecord{}, errors.Join(peekErr, guard.readErr)
	}
	var foldedPrefix [len("to re-run:")]byte
	for index, value := range prefix {
		if value >= 'A' && value <= 'Z' {
			value += 'a' - 'A'
		}
		foldedPrefix[index] = value
	}
	for index, literal := range [...]string{"--- FAIL:", "to re-run:", "go test ", "# "} {
		candidate := prefix
		if index == 1 {
			candidate = foldedPrefix[:len(prefix)]
		}
		if !bytes.HasPrefix(candidate, []byte(literal)) {
			continue
		}
		marker, err := goFuzzSignalMarker(int64(record.Offset)+consumed, len(literal))
		if err != nil {
			return GoFuzzSignalRecord{}, err
		}
		switch index {
		case 0:
			observation.FailureHeader = marker
		case 1:
			observation.RerunHeader = marker
		case 2:
			observation.ReplayCommand = marker
		case 3:
			observation.BuildHeader = marker
		}
	}
	const overlap = len("context deadline exceeded") - 1
	var buffer [32768 + overlap]byte
	var folded [len(buffer)]byte
	carry, empty := 0, 0
	for {
		n, readErr := reader.Read(buffer[carry:])
		if n < 0 || n > len(buffer)-carry || int64(n) > length-consumed {
			return GoFuzzSignalRecord{}, errors.Join(core.ErrGoToolchainOutput, readErr, guard.readErr)
		}
		if n > 0 {
			empty = 0
			window := buffer[:carry+n]
			for index, value := range window {
				if value >= 'A' && value <= 'Z' {
					value += 'a' - 'A'
				}
				folded[index] = value
			}
			for index, literal := range [...]string{"panic:", "context deadline exceeded", "failing input"} {
				destination := &observation.Panic
				candidate := window
				switch index {
				case 0:
				case 1:
					destination = &observation.Deadline
					candidate = folded[:len(window)]
				case 2:
					destination = &observation.FailingInput
					candidate = folded[:len(window)]
				}
				if *destination != (lineio.RecordRange{}) {
					continue
				}
				position := bytes.Index(candidate, []byte(literal))
				if position < 0 {
					continue
				}
				marker, err := goFuzzSignalMarker(int64(record.Offset)+consumed-int64(carry)+int64(position), len(literal))
				if err != nil {
					return GoFuzzSignalRecord{}, err
				}
				*destination = marker
			}
			consumed += int64(n)
			carry = min(overlap, len(window))
			copy(buffer[:carry], window[len(window)-carry:])
		} else if readErr == nil {
			empty++
			if empty == core.ReaderConsecutiveEmptyReadMaximum {
				return GoFuzzSignalRecord{}, io.ErrNoProgress
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				return GoFuzzSignalRecord{}, errors.Join(readErr, guard.readErr, contextstate.Validate(ctx))
			}
			if refusal := errors.Join(guard.readErr, contextstate.Validate(ctx)); refusal != nil {
				return GoFuzzSignalRecord{}, refusal
			}
			if consumed != length {
				return GoFuzzSignalRecord{}, io.ErrUnexpectedEOF
			}
			return observation, nil
		}
	}
}
