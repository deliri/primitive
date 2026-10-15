package runprotocol

import (
	"bufio"
	"context"
	"errors"
	"io"
	"iter"
	"regexp"
	"strconv"
	"unicode"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// GoFuzzMetricKind selects a compiler-owned counter spelling. It carries no
// product meaning: callers decide how signed observations affect policy.
type GoFuzzMetricKind uint8

const (
	GoFuzzMetricUnknown GoFuzzMetricKind = iota
	GoFuzzMetricExecutions
	GoFuzzMetricExecs
	GoFuzzMetricCrashers
	GoFuzzMetricInteresting
)

func (k GoFuzzMetricKind) Validate() error {
	if k < GoFuzzMetricExecutions || k > GoFuzzMetricInteresting {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (GoFuzzMetricKind) OffWireEnum() {}

type GoFuzzMetricPresence uint8

const (
	GoFuzzMetricAbsent GoFuzzMetricPresence = iota
	GoFuzzMetricPresent
	GoFuzzMetricMalformed
)

func (p GoFuzzMetricPresence) Validate() error {
	if p > GoFuzzMetricMalformed {
		return core.ErrGoToolchainOutput
	}
	return nil
}
func (GoFuzzMetricPresence) OffWireEnum() {}

// GoFuzzMetricField seals a signed native integer or a grammar refusal. Native
// strconv causes are retained, while their normalized input is fixed in size.
type GoFuzzMetricField struct {
	value    int64
	presence GoFuzzMetricPresence
	refusal  error
}

func (f GoFuzzMetricField) Presence() GoFuzzMetricPresence { return f.presence }
func (f GoFuzzMetricField) Value() (int64, bool)           { return f.value, f.presence == GoFuzzMetricPresent }
func (f GoFuzzMetricField) Refusal() error                 { return f.refusal }
func (f GoFuzzMetricField) Validate() error {
	if err := f.presence.Validate(); err != nil {
		return err
	}
	switch f.presence {
	case GoFuzzMetricAbsent:
		if f.value != 0 || f.refusal != nil {
			return core.ErrGoToolchainOutput
		}
	case GoFuzzMetricPresent:
		if f.refusal != nil {
			return core.ErrGoToolchainOutput
		}
	case GoFuzzMetricMalformed:
		if f.value != 0 || !errors.Is(f.refusal, core.ErrGoToolchainOutput) {
			return core.ErrGoToolchainOutput
		}
	default:
		return core.ErrGoToolchainOutput
	}
	return nil
}

// GoFuzzMetricRecord retains fixed scalar observations and native coordinates.
// It never collects a physical line, field payload, record list or source handle.
type GoFuzzMetricRecord struct {
	record       lineio.RecordRange
	fields       [4]GoFuzzMetricField
	cumulative   bool
	failingInput bool
}

func (r GoFuzzMetricRecord) Record() lineio.RecordRange  { return r.record }
func (r GoFuzzMetricRecord) InterestingCumulative() bool { return r.cumulative }
func (r GoFuzzMetricRecord) FailingInput() bool          { return r.failingInput }
func (r GoFuzzMetricRecord) Field(kind GoFuzzMetricKind) (GoFuzzMetricField, error) {
	if err := errors.Join(kind.Validate(), r.Validate()); err != nil {
		return GoFuzzMetricField{}, err
	}
	return r.fields[int(kind)-1], nil
}
func (r GoFuzzMetricRecord) Validate() error {
	if err := r.record.Validate(); err != nil {
		return err
	}
	for _, field := range r.fields {
		if err := field.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type GoFuzzMetricSourceRequest struct{ Source lineio.RecordSource }

func (r GoFuzzMetricSourceRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrGoToolchainContract
	}
	return nil
}

// GoFuzzMetricRecords lends one completed native physical record at a time.
// A later source refusal ends the sequence; earlier values are provisional
// until the caller has consumed the full source and closed its native scope.
func GoFuzzMetricRecords(ctx context.Context, request GoFuzzMetricSourceRequest) iter.Seq2[GoFuzzMetricRecord, error] {
	return func(yield func(GoFuzzMetricRecord, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(GoFuzzMetricRecord{}, goFuzzMetricRefusal(ctx, err))
			return
		}
		buffer, err := core.NewByteCount(4096)
		if err != nil {
			yield(GoFuzzMetricRecord{}, goFuzzMetricRefusal(ctx, err))
			return
		}
		patterns := newGoFuzzMetricPatterns()
		for record, err := range lineio.RecordRanges(ctx, lineio.RecordRangeRequest{Source: request.Source, BufferBytes: buffer}) {
			if err != nil {
				yield(GoFuzzMetricRecord{}, goFuzzMetricRefusal(ctx, err))
				return
			}
			observed, err := readGoFuzzMetricRecord(ctx, request.Source, record, patterns)
			if err != nil {
				yield(GoFuzzMetricRecord{}, goFuzzMetricRefusal(ctx, err))
				return
			}
			if !yield(observed, nil) {
				return
			}
		}
	}
}

func goFuzzMetricRefusal(ctx context.Context, cause error) error {
	if ctx == nil {
		return errors.Join(cause, core.ErrNilContext)
	}
	return errors.Join(cause, context.Cause(ctx))
}

type goFuzzMetricPatterns struct {
	prefixes            [4][3]*regexp.Regexp
	interestingPrefixes [3]*regexp.Regexp
}

func newGoFuzzMetricPatterns() goFuzzMetricPatterns {
	var patterns goFuzzMetricPatterns
	for i, prefix := range [...]string{"executions:", "execs:", "crashers:", "new interesting:"} {
		quoted := regexp.QuoteMeta(prefix)
		patterns.prefixes[i] = [3]*regexp.Regexp{regexp.MustCompile(`\A` + quoted), regexp.MustCompile(`,` + quoted), regexp.MustCompile(`, ` + quoted)}
	}
	patterns.interestingPrefixes = [3]*regexp.Regexp{regexp.MustCompile(`\Anew interesting`), regexp.MustCompile(`,new interesting`), regexp.MustCompile(`, new interesting`)}
	return patterns
}

type goFuzzMetricSpan struct{ offset, bytes int64 }
type goFuzzMetricTrim uint8

const (
	goFuzzMetricTrimWhitespace goFuzzMetricTrim = iota
	goFuzzMetricTrimCommas
	goFuzzMetricTrimCommasColons
)

func goFuzzMetricTrimmed(ctx context.Context, source lineio.RecordSource, span goFuzzMetricSpan, selection goFuzzMetricTrim) (goFuzzMetricSpan, error) {
	if span.bytes == 0 {
		return span, nil
	}
	if selection > goFuzzMetricTrimCommasColons {
		return goFuzzMetricSpan{}, core.ErrGoToolchainContract
	}
	guard := &goFuzzFrameSource{ctx: ctx, source: io.NewSectionReader(source, span.offset, span.bytes)}
	reader := bufio.NewReaderSize(guard, 4096)
	var first, end, offset int64
	seen := false
	for {
		value, width, err := reader.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return goFuzzMetricSpan{}, errors.Join(err, guard.readErr, contextstate.Validate(ctx))
		}
		trim := unicode.IsSpace(value)
		if selection == goFuzzMetricTrimCommas {
			trim = value == ','
		}
		if selection == goFuzzMetricTrimCommasColons {
			trim = value == ',' || value == ':'
		}
		if !trim {
			if !seen {
				first = span.offset + offset
				seen = true
			}
			end = span.offset + offset + int64(width)
		}
		offset += int64(width)
	}
	if err := errors.Join(guard.readErr, contextstate.Validate(ctx)); err != nil {
		return goFuzzMetricSpan{}, err
	}
	if offset != span.bytes {
		return goFuzzMetricSpan{}, io.ErrUnexpectedEOF
	}
	if !seen {
		return goFuzzMetricSpan{offset: span.offset + span.bytes}, nil
	}
	return goFuzzMetricSpan{offset: first, bytes: end - first}, nil
}

// regexp reads Go's bounded RuneReader. Drain and the guard preserve errors
// that regexp alone would otherwise treat as end of input.
func goFuzzMetricMatch(ctx context.Context, source lineio.RecordSource, span goFuzzMetricSpan, pattern *regexp.Regexp) ([]int, error) {
	guard := &goFuzzFrameSource{ctx: ctx, source: io.NewSectionReader(source, span.offset, span.bytes)}
	reader := bufio.NewReader(guard)
	match := pattern.FindReaderIndex(reader)
	_, drainErr := io.Copy(io.Discard, reader)
	if err := errors.Join(guard.readErr, drainErr, contextstate.Validate(ctx)); err != nil {
		return nil, err
	}
	return match, nil
}

func goFuzzMetricInteger(ctx context.Context, source lineio.RecordSource, span goFuzzMetricSpan) (GoFuzzMetricField, error) {
	guard := &goFuzzFrameSource{ctx: ctx, source: io.NewSectionReader(source, span.offset, span.bytes)}
	reader := bufio.NewReader(guard)
	var digits [20]byte
	used := 0
	negative, seen, invalid, overflow := false, false, false, false
	first := true
	for {
		value, err := reader.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return GoFuzzMetricField{}, errors.Join(core.ErrGoToolchainOutput, err, guard.readErr)
		}
		if first && (value == '+' || value == '-') {
			negative = value == '-'
			first = false
			continue
		}
		first = false
		if value < '0' || value > '9' {
			invalid = true
			continue
		}
		seen = true
		if used == 0 && value == '0' {
			continue
		}
		if used == 19 {
			overflow = true
			continue
		}
		digits[used] = value
		used++
	}
	if err := errors.Join(guard.readErr, contextstate.Validate(ctx)); err != nil {
		return GoFuzzMetricField{}, err
	}
	if !seen || invalid || overflow {
		return GoFuzzMetricField{presence: GoFuzzMetricMalformed, refusal: core.ErrGoToolchainOutput}, nil
	}
	if used == 0 {
		digits[0] = '0'
		used = 1
	}
	var normalized [21]byte
	length := 0
	if negative {
		normalized[0] = '-'
		length = 1
	}
	length += copy(normalized[length:], digits[:used])
	value, err := strconv.ParseInt(string(normalized[:length]), 10, 64)
	if err != nil {
		return GoFuzzMetricField{presence: GoFuzzMetricMalformed, refusal: errors.Join(core.ErrGoToolchainOutput, err)}, nil
	}
	return GoFuzzMetricField{presence: GoFuzzMetricPresent, value: value}, nil
}

func readGoFuzzMetricField(ctx context.Context, source lineio.RecordSource, span goFuzzMetricSpan, patterns [3]*regexp.Regexp) (GoFuzzMetricField, error) {
	var match []int
	for _, pattern := range patterns {
		found, err := goFuzzMetricMatch(ctx, source, span, pattern)
		if err != nil {
			return GoFuzzMetricField{}, err
		}
		if found != nil {
			match = found
			break
		}
	}
	if match == nil {
		return GoFuzzMetricField{}, nil
	}
	value := goFuzzMetricSpan{offset: span.offset + int64(match[1]), bytes: span.bytes - int64(match[1])}
	value, err := goFuzzMetricTrimmed(ctx, source, value, goFuzzMetricTrimWhitespace)
	if err != nil {
		return GoFuzzMetricField{}, err
	}
	space, err := goFuzzMetricMatch(ctx, source, value, regexp.MustCompile(` `))
	if err != nil {
		return GoFuzzMetricField{}, err
	}
	if space != nil {
		value.bytes = int64(space[0])
	}
	// Only suffix commas are ignored; a leading comma is invalid numeric data.
	trimmed, err := goFuzzMetricTrimmed(ctx, source, value, goFuzzMetricTrimCommas)
	if err != nil {
		return GoFuzzMetricField{}, err
	}
	if trimmed.bytes > 0 && trimmed.offset != value.offset {
		return GoFuzzMetricField{presence: GoFuzzMetricMalformed, refusal: core.ErrGoToolchainOutput}, nil
	}
	return goFuzzMetricInteger(ctx, source, trimmed)
}

func readGoFuzzMetricRecord(ctx context.Context, source lineio.RecordSource, record lineio.RecordRange, patterns goFuzzMetricPatterns) (GoFuzzMetricRecord, error) {
	length, err := record.Bytes.Int64()
	if err != nil {
		return GoFuzzMetricRecord{}, err
	}
	hints, err := goFuzzMetricRecordHints(ctx, source, goFuzzMetricSpan{offset: int64(record.Offset), bytes: length})
	if err != nil {
		return GoFuzzMetricRecord{}, err
	}
	present := false
	for _, hint := range hints {
		present = present || hint
	}
	if !present {
		return GoFuzzMetricRecord{record: record}, nil
	}
	span, err := goFuzzMetricTrimmed(ctx, source, goFuzzMetricSpan{offset: int64(record.Offset), bytes: length}, goFuzzMetricTrimWhitespace)
	if err != nil {
		return GoFuzzMetricRecord{}, err
	}
	observed := GoFuzzMetricRecord{record: record}
	for i, prefixes := range patterns.prefixes {
		if !hints[i] {
			continue
		}
		field, err := readGoFuzzMetricField(ctx, source, span, prefixes)
		if err != nil {
			return GoFuzzMetricRecord{}, err
		}
		observed.fields[i] = field
	}
	if observed.fields[3].presence != GoFuzzMetricPresent && hints[goFuzzMetricHintInteresting] {
		cursorSource := &goBenchmarkSourceReader{ctx: ctx, source: io.NewSectionReader(source, span.offset, span.bytes)}
		cursor := goBenchmarkFieldCursor{ctx: ctx, source: source, buffer: bufio.NewReader(cursorSource), offset: span.offset}
		for !cursor.ended {
			field, found, err := cursor.nextField()
			if err != nil {
				return GoFuzzMetricRecord{}, err
			}
			if !found {
				break
			}
			candidate, err := goFuzzMetricTrimmed(ctx, source, goFuzzMetricSpan{offset: field.offset, bytes: field.length}, goFuzzMetricTrimCommasColons)
			if err != nil {
				return GoFuzzMetricRecord{}, err
			}
			value, err := goFuzzMetricInteger(ctx, source, candidate)
			if err != nil {
				return GoFuzzMetricRecord{}, err
			}
			if value.presence == GoFuzzMetricPresent {
				observed.fields[3] = value
				break
			}
		}
		if cursorSource.readErr != nil {
			return GoFuzzMetricRecord{}, cursorSource.readErr
		}
		if observed.fields[3].presence != GoFuzzMetricPresent {
			for _, pattern := range patterns.interestingPrefixes {
				found, err := goFuzzMetricMatch(ctx, source, span, pattern)
				if err != nil {
					return GoFuzzMetricRecord{}, err
				}
				if found != nil {
					observed.fields[3] = GoFuzzMetricField{presence: GoFuzzMetricMalformed, refusal: core.ErrGoToolchainOutput}
					break
				}
			}
		}
	}
	observed.cumulative = hints[goFuzzMetricHintCumulative] || hints[goFuzzMetricHintTotal]
	observed.failingInput = hints[goFuzzMetricHintFailing]
	if err := observed.Validate(); err != nil {
		return GoFuzzMetricRecord{}, err
	}
	return observed, nil
}

func (GoFuzzMetricField) runProtocolSealedProjection()       {}
func (GoFuzzMetricRecord) runProtocolSealedProjection()      {}
func (GoFuzzMetricSourceRequest) runProtocolFact()           {}
func (goFuzzMetricPatterns) runProtocolInternalFlowCarrier() {}
func (goFuzzMetricSpan) runProtocolInternalFlowCarrier()     {}
