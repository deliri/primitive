package lineio

import (
	"bufio"
	"context"
	"errors"
	"io"
	"unicode"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// TrimRecordWhitespace observes bytes.TrimSpace coordinates through native
// buffered ReadRune. It retains no record text. A blank record returns a zero
// range; a nonblank range names exact original bytes, including malformed UTF-8.
// Complete declared extent, physical framing and native source completion are
// required before a range escapes. The source and its cursor remain borrowed.
func TrimRecordWhitespace(ctx context.Context, request RecordFragmentRequest) (result RecordRange, resultErr error) {
	defer func() {
		if resultErr != nil {
			result = RecordRange{}
			if ctx != nil {
				resultErr = errors.Join(resultErr, context.Cause(ctx))
			}
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return RecordRange{}, err
	}
	length, err := request.Record.Bytes.Int64()
	if err != nil {
		return RecordRange{}, err
	}
	bufferSize, err := (Request{Source: request.Source, BufferBytes: request.BufferBytes}).bufferSize()
	if err != nil {
		return RecordRange{}, err
	}
	guard := recordRangeReadGuard{source: io.NewSectionReader(request.Source, int64(request.Record.Offset), length)}
	native := bufio.NewReaderSize(&guard, bufferSize)
	checked := characterSource{source: native, ctx: ctx}
	reader := bufio.NewReaderSize(&checked, bufferSize)
	var observed, end int64
	start := int64(-1)
	for {
		value, width, readErr := reader.ReadRune()
		if guard.readErr != nil || checked.readErr != nil {
			return RecordRange{}, errors.Join(core.ErrLineIOScan, readErr, guard.readErr, checked.readErr, contextstate.Validate(ctx))
		}
		if readErr != nil {
			if readErr != io.EOF || observed != length {
				return RecordRange{}, errors.Join(core.ErrLineIOScan, readErr, io.ErrUnexpectedEOF)
			}
			break
		}
		if width < 1 || int64(width) > length-observed {
			return RecordRange{}, core.ErrLineIOScan
		}
		next := observed + int64(width)
		if request.Record.Framing != RecordFramingOpaque && (value == '\n' && (next != length || request.Record.Framing != RecordFramingLF) || next == length && request.Record.Framing == RecordFramingLF && value != '\n') {
			return RecordRange{}, core.ErrLineIOScan
		}
		if !unicode.IsSpace(value) {
			if start < 0 {
				start = observed
			}
			end = next
		}
		observed = next
	}
	if err := contextstate.Validate(ctx); err != nil {
		return RecordRange{}, err
	}
	if start < 0 {
		return RecordRange{}, nil
	}
	size, err := core.NewByteLength(uint64(end - start))
	if err != nil {
		return RecordRange{}, err
	}
	result = RecordRange{Offset: SourceByteOffset(int64(request.Record.Offset) + start), Bytes: size, Framing: RecordFramingOpaque}
	return result, result.Validate()
}
