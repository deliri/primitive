package runprotocol

import (
	"bufio"
	"context"
	"errors"
	"io"
	"iter"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// GoBenchmarkRecordSource lends a seekable native source. ReaderAt admits
// individual field ranges without retaining a line; the caller owns lifetime.
// A source must present the same bytes through Read and ReadAt.
type GoBenchmarkRecordSource interface {
	io.ReadSeeker
	io.ReaderAt
}

// GoBenchmarkSourceRequest selects a source from its current native offset.
// No line, token, record-count or stream-size quota applies.
type GoBenchmarkSourceRequest struct{ Source GoBenchmarkRecordSource }

// Validate refuses a missing source.
func (r GoBenchmarkSourceRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrGoToolchainContract
	}
	return nil
}

// GoBenchmarkSourceExtent is the exact original record range, including its
// framing bytes. Native ranges can be copied or replayed without line buffers.
type GoBenchmarkSourceExtent struct {
	Offset lineio.SourceByteOffset
	Bytes  core.ByteLength
}

// Validate checks native source-coordinate representability.
func (s GoBenchmarkSourceExtent) Validate() error {
	if err := s.Offset.Validate(); err != nil {
		return err
	}
	size, err := s.Bytes.Int64()
	if err != nil {
		return err
	}
	if int64(s.Offset) > math.MaxInt64-size {
		return core.ErrGoToolchainOutput
	}
	return nil
}

// SourceExtent returns the exact source record range. Atomic borrowed-byte
// observations have the neutral range; streamed observations carry source bytes.
func (r GoBenchmarkRecord) SourceExtent() GoBenchmarkSourceExtent { return r.source }

// GoBenchmarkRecords scans native Go rows with fixed working buffers. Names
// are copied only after admission; the returned name is the sole payload-sized
// allocation. Numeric fields and unknown units never retain source-sized text.
// Complete malformed rows yield Refused observations and preserve their exact
// range. Read failures return errors and end the sequence. Each source range
// remains readable inside the caller's native scope.
func GoBenchmarkRecords(ctx context.Context, request GoBenchmarkSourceRequest) iter.Seq2[GoBenchmarkRecord, error] {
	return func(yield func(GoBenchmarkRecord, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(GoBenchmarkRecord{}, err)
			return
		}
		offset, err := request.Source.Seek(0, io.SeekCurrent)
		if err != nil || offset < 0 {
			yield(GoBenchmarkRecord{}, errors.Join(core.ErrGoToolchainOutput, err))
			return
		}
		input := goBenchmarkSourceReader{ctx: ctx, source: request.Source}
		cursor := goBenchmarkFieldCursor{ctx: ctx, source: request.Source, buffer: bufio.NewReader(&input), offset: offset}
		for {
			start := cursor.offset
			record, err := readGoBenchmarkSourceRecord(&cursor)
			if err == core.ErrGoToolchainOutput {
				for !cursor.ended {
					if _, _, drainErr := cursor.nextField(); drainErr != nil {
						err = drainErr
						break
					}
				}
				if err == core.ErrGoToolchainOutput {
					record, err = sourceGoBenchmarkRecord(&cursor, start, GoBenchmarkRecord{Presence: GoBenchmarkRecordRefused})
				}
			}
			if err == io.EOF {
				return
			}
			if err != nil {
				yield(GoBenchmarkRecord{}, err)
				return
			}
			if !yield(record, nil) {
				return
			}
		}
	}
}

type goBenchmarkSourceReader struct {
	ctx     context.Context
	source  io.Reader
	readErr error
}

func (r *goBenchmarkSourceReader) Read(destination []byte) (int, error) {
	if err := contextstate.Validate(r.ctx); err != nil {
		return 0, err
	}
	n, err := r.source.Read(destination)
	if n < 0 || n > len(destination) {
		r.readErr = errors.Join(core.ErrGoToolchainOutput, bufio.ErrBadReadCount, err)
		return 0, r.readErr
	}
	if observed := contextstate.Validate(r.ctx); observed != nil {
		r.readErr = errors.Join(observed, err)
		return n, r.readErr
	}
	if err != nil && err != io.EOF {
		err = errors.Join(core.ErrGoToolchainOutput, err)
		r.readErr = err
	}
	return n, err
}

type goBenchmarkFieldCursor struct {
	ctx    context.Context
	source GoBenchmarkRecordSource
	buffer *bufio.Reader
	offset int64
	ended  bool
}
type goBenchmarkFieldSpan struct{ offset, length int64 }

func (c *goBenchmarkFieldCursor) readRune() (rune, int, error) {
	if err := contextstate.Validate(c.ctx); err != nil {
		return 0, 0, err
	}
	value, size, err := c.buffer.ReadRune()
	if observed := contextstate.Validate(c.ctx); observed != nil {
		return 0, 0, errors.Join(observed, err)
	}
	if err != nil && err != io.EOF {
		return 0, 0, errors.Join(core.ErrGoToolchainOutput, err)
	}
	if size < 0 || c.offset > math.MaxInt64-int64(size) {
		return 0, 0, core.ErrGoToolchainOutput
	}
	c.offset += int64(size)
	return value, size, err
}

func (c *goBenchmarkFieldCursor) nextField() (goBenchmarkFieldSpan, bool, error) {
	if c.ended {
		return goBenchmarkFieldSpan{}, false, nil
	}
	var field goBenchmarkFieldSpan
	for {
		value, size, err := c.readRune()
		if err == io.EOF {
			c.ended = true
			return field, field.length > 0, nil
		}
		if err != nil {
			return goBenchmarkFieldSpan{}, false, err
		}
		if value == '\n' {
			c.ended = true
			return field, field.length > 0, nil
		}
		if unicode.IsSpace(value) {
			if field.length != 0 {
				return field, true, nil
			}
			continue
		}
		if field.length == 0 {
			field.offset = c.offset - int64(size)
		}
		field.length += int64(size)
	}
}

func (c *goBenchmarkFieldCursor) fieldReader(field goBenchmarkFieldSpan) *goBenchmarkSourceReader {
	return &goBenchmarkSourceReader{ctx: c.ctx, source: io.NewSectionReader(c.source, field.offset, field.length)}
}

func (c *goBenchmarkFieldCursor) smallField(field goBenchmarkFieldSpan) ([9]byte, error) {
	var text [9]byte
	if field.length < 0 || field.length > int64(len(text)) {
		return text, core.ErrGoToolchainOutput
	}
	source := c.fieldReader(field)
	_, err := io.ReadFull(source, text[:int(field.length)])
	if err = errors.Join(err, source.readErr); err != nil {
		return [9]byte{}, errors.Join(core.ErrGoToolchainOutput, err)
	}
	return text, nil
}

func (c *goBenchmarkFieldCursor) metricUnit(field goBenchmarkFieldSpan) (GoBenchmarkMetricFields, error) {
	if field.length != 4 && field.length != 5 && field.length != 9 {
		return GoBenchmarkMetricFieldsNone, nil
	}
	text, err := c.smallField(field)
	if err != nil {
		return GoBenchmarkMetricFieldsNone, err
	}
	return goBenchmarkUnit(text[:int(field.length)]), nil
}

func (c *goBenchmarkFieldCursor) admittedName(field goBenchmarkFieldSpan) (bool, error) {
	if field.length < 9 {
		return false, nil
	}
	var prefix [9]byte
	prefixSource := c.fieldReader(goBenchmarkFieldSpan{offset: field.offset, length: 9})
	_, err := io.ReadFull(prefixSource, prefix[:])
	if err = errors.Join(err, prefixSource.readErr); err != nil {
		return false, errors.Join(core.ErrGoToolchainOutput, err)
	}
	if string(prefix[:9]) != "Benchmark" {
		return false, nil
	}
	nameSource := c.fieldReader(field)
	source := bufio.NewReader(nameSource)
	for {
		value, size, err := source.ReadRune()
		if err == io.EOF {
			return true, nil
		}
		if err != nil {
			return false, errors.Join(core.ErrGoToolchainOutput, err)
		}
		if value == utf8.RuneError && size == 1 || unicode.IsSpace(value) || unicode.IsControl(value) {
			if nameSource.readErr != nil {
				return false, errors.Join(core.ErrGoToolchainOutput, nameSource.readErr)
			}
			return false, core.ErrGoToolchainOutput
		}
	}
}

func (c *goBenchmarkFieldCursor) sealName(field goBenchmarkFieldSpan) (GoBenchmarkName, error) {
	if field.length > math.MaxInt {
		return GoBenchmarkName{}, core.ErrGoToolchainOutput
	}
	var name strings.Builder
	name.Grow(int(field.length))
	if _, err := io.Copy(&name, c.fieldReader(field)); err != nil {
		return GoBenchmarkName{}, errors.Join(core.ErrGoToolchainOutput, err)
	}
	return GoBenchmarkName{value: name.String()}, nil
}

func readGoBenchmarkSourceRecord(c *goBenchmarkFieldCursor) (GoBenchmarkRecord, error) {
	start := c.offset
	c.ended = false
	name, found, err := c.nextField()
	if err != nil {
		return GoBenchmarkRecord{}, err
	}
	if !found {
		if c.offset == start {
			return GoBenchmarkRecord{}, io.EOF
		}
		return sourceGoBenchmarkRecord(c, start, GoBenchmarkRecord{Presence: GoBenchmarkRecordAbsent})
	}
	admitted, err := c.admittedName(name)
	if err != nil {
		return GoBenchmarkRecord{}, err
	}
	if !admitted {
		for !c.ended {
			if _, _, err := c.nextField(); err != nil {
				return GoBenchmarkRecord{}, err
			}
		}
		return sourceGoBenchmarkRecord(c, start, GoBenchmarkRecord{Presence: GoBenchmarkRecordAbsent})
	}
	count, found, err := c.nextField()
	if err != nil {
		return GoBenchmarkRecord{}, err
	}
	if !found {
		return GoBenchmarkRecord{}, core.ErrGoToolchainOutput
	}
	countSource := c.fieldReader(count)
	iterations, err := goBenchmarkInteger(countSource)
	if countSource.readErr != nil {
		err = errors.Join(err, countSource.readErr)
	}
	if err != nil {
		return GoBenchmarkRecord{}, err
	}
	record := GoBenchmarkRecord{Iterations: iterations, Presence: GoBenchmarkRecordPresent}
	pairs := false
	for {
		value, found, err := c.nextField()
		if err != nil {
			return GoBenchmarkRecord{}, err
		}
		if !found {
			break
		}
		unit, found, err := c.nextField()
		if err != nil {
			return GoBenchmarkRecord{}, err
		}
		if !found {
			return GoBenchmarkRecord{}, core.ErrGoToolchainOutput
		}
		pairs = true
		kind, err := c.metricUnit(unit)
		if err != nil {
			return GoBenchmarkRecord{}, err
		}
		valueSource := c.fieldReader(value)
		err = projectGoBenchmarkMetric(&record, valueSource, kind)
		if valueSource.readErr != nil {
			err = errors.Join(err, valueSource.readErr)
		}
		if err != nil {
			return GoBenchmarkRecord{}, err
		}
	}
	if !pairs {
		return GoBenchmarkRecord{}, core.ErrGoToolchainOutput
	}
	record.name, err = c.sealName(name)
	if err != nil {
		return GoBenchmarkRecord{}, err
	}
	return sourceGoBenchmarkRecord(c, start, record)
}

func sourceGoBenchmarkRecord(c *goBenchmarkFieldCursor, start int64, record GoBenchmarkRecord) (GoBenchmarkRecord, error) {
	extent, err := core.NewByteLength(uint64(c.offset - start))
	if err != nil {
		return GoBenchmarkRecord{}, err
	}
	record.source = GoBenchmarkSourceExtent{Offset: lineio.SourceByteOffset(start), Bytes: extent}
	if err := record.Validate(); err != nil {
		return GoBenchmarkRecord{}, err
	}
	if err := contextstate.Validate(c.ctx); err != nil {
		return GoBenchmarkRecord{}, err
	}
	return record, nil
}

func (GoBenchmarkSourceRequest) runProtocolFact()               {}
func (GoBenchmarkSourceExtent) runProtocolFact()                {}
func (goBenchmarkSourceReader) runProtocolInternalFlowCarrier() {}
func (goBenchmarkFieldCursor) runProtocolInternalFlowCarrier()  {}
func (goBenchmarkFieldSpan) runProtocolInternalFlowCarrier()    {}
