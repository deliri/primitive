package runprotocol

import (
	"bufio"
	"context"
	"errors"
	"io"
	"iter"
	"math"
	"unicode"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// GoTestRecordSyntax reports independent lexical observations. It does not
// select an outcome, precedence, severity, or failure owner for a consumer.
type GoTestRecordSyntax struct {
	Record            lineio.RecordRange
	FailureHeader     bool
	PackageFailure    bool
	Panic             bool
	FoldedPanic       bool
	RuntimeError      bool
	FatalError        bool
	ContextDeadline   bool
	Signal            bool
	DataRace          bool
	NonZeroExitStatus bool
}

func (r GoTestRecordSyntax) Validate() error { return r.Record.Validate() }

type GoTestTranscriptStreamRequest struct{ Source io.Reader }

func (r GoTestTranscriptStreamRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrGoToolchainContract
	}
	return nil
}

type GoTestTranscriptRequest struct{ Source lineio.RecordSource }

func (r GoTestTranscriptRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrGoToolchainContract
	}
	return nil
}
func (GoTestTranscriptRequest) runProtocolInternalFlowCarrier() {}

// GoTestTranscriptRecords observes complete borrowed positional records.
// Both native framing and positional rereads must finish before an observation
// escapes. A truncated declared extent refuses even if its prefix has a marker.
func GoTestTranscriptRecords(ctx context.Context, request GoTestTranscriptRequest) iter.Seq2[GoTestRecordSyntax, error] {
	return func(yield func(GoTestRecordSyntax, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(GoTestRecordSyntax{}, goFuzzSignalRefusal(ctx, err))
			return
		}
		window, err := core.NewByteCount(4096)
		if err != nil {
			yield(GoTestRecordSyntax{}, err)
			return
		}
		for record, err := range lineio.RecordRanges(ctx, lineio.RecordRangeRequest{Source: request.Source, BufferBytes: window}) {
			if err != nil {
				yield(GoTestRecordSyntax{}, goFuzzSignalRefusal(ctx, err))
				return
			}
			length, err := record.Bytes.Int64()
			if err != nil {
				yield(GoTestRecordSyntax{}, err)
				return
			}
			section := io.NewSectionReader(request.Source, int64(record.Offset), length)
			guard := &goFuzzFrameSource{ctx: ctx, source: section}
			observed, readErr := readGoTestRecordSyntax(bufio.NewReaderSize(guard, 4096))
			consumed, positionErr := section.Seek(0, io.SeekCurrent)
			refusal := errors.Join(readErr, guard.readErr, positionErr, contextstate.Validate(ctx))
			if consumed != length {
				refusal = errors.Join(refusal, io.ErrUnexpectedEOF)
			}
			if refusal != nil {
				yield(GoTestRecordSyntax{}, goFuzzSignalRefusal(ctx, refusal))
				return
			}
			observed.Record = record
			if err := observed.Validate(); err != nil {
				yield(GoTestRecordSyntax{}, err)
				return
			}
			if !yield(observed, nil) {
				return
			}
		}
	}
}

// GoTestTranscriptStreamRecords owns native forward-only framing and buffered
// rune decoding. A physical source refusal withdraws that record. Earlier
// records remain provisional until the caller consumes clean source exhaustion.
func GoTestTranscriptStreamRecords(ctx context.Context, request GoTestTranscriptStreamRequest) iter.Seq2[GoTestRecordSyntax, error] {
	return func(yield func(GoTestRecordSyntax, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(GoTestRecordSyntax{}, goFuzzSignalRefusal(ctx, err))
			return
		}
		window, err := core.NewByteCount(4096)
		if err != nil {
			yield(GoTestRecordSyntax{}, err)
			return
		}
		source, err := lineio.New(lineio.Request{Source: request.Source, BufferBytes: window})
		if err != nil {
			yield(GoTestRecordSyntax{}, err)
			return
		}
		var offset int64
		for {
			var observed GoTestRecordSyntax
			extent, err := source.WithRecord(ctx, lineio.RecordReadRequest{Use: func(ctx context.Context, record io.Reader) error {
				var err error
				observed, err = readGoTestRecordSyntax(bufio.NewReaderSize(record, 4096))
				return err
			}})
			if err == io.EOF {
				return
			}
			if err != nil {
				yield(GoTestRecordSyntax{}, goFuzzSignalRefusal(ctx, err))
				return
			}
			length, err := extent.Bytes.Int64()
			if err != nil || length > math.MaxInt64-offset {
				yield(GoTestRecordSyntax{}, errors.Join(core.ErrGoToolchainOutput, err))
				return
			}
			observed.Record = lineio.RecordRange{Offset: lineio.SourceByteOffset(offset), Bytes: extent.Bytes, Framing: extent.Framing}
			if err := errors.Join(observed.Validate(), contextstate.Validate(ctx)); err != nil {
				yield(GoTestRecordSyntax{}, goFuzzSignalRefusal(ctx, err))
				return
			}
			if !yield(observed, nil) {
				return
			}
			offset += length
		}
	}
}
func (GoTestRecordSyntax) runProtocolSealedProjection()               {}
func (GoTestTranscriptStreamRequest) runProtocolInternalFlowCarrier() {}

const (
	goTestSyntaxFuzzFailLinePrefix          = "--- FAIL:"
	goTestSyntaxPackageFailLinePrefix       = "FAIL\t"
	goTestSyntaxPanicDiagnosticFragment     = "panic:"
	goTestSyntaxGoRuntimeErrorFragment      = "runtime error:"
	goTestSyntaxGoFatalErrorFragment        = "fatal error:"
	goTestSyntaxContextDeadlineExceededText = "context deadline exceeded"
	goTestSyntaxGoSignalLinePrefix          = "signal: "
	goTestSyntaxGoDataRaceLinePrefix        = "warning: data race"
	goTestSyntaxExitStatusPrefix            = "exit status "
)

const goTestSyntaxWindowRunes = len(goTestSyntaxContextDeadlineExceededText)

func readGoTestRecordSyntax(reader *bufio.Reader) (GoTestRecordSyntax, error) {
	var facts GoTestRecordSyntax
	var raw, lower [goTestSyntaxWindowRunes]rune
	var count int64
	var trimmedCount int64
	var exitPrefix, firstSpace, delimiter, restNonspace, afterDelimiterNonspace bool
	var tokenCount, tokenEffective int64
	var tokenFirst rune
	for {
		value, _, err := reader.ReadRune()
		if err == io.EOF {
			break
		}
		if err != nil {
			return GoTestRecordSyntax{}, err
		}
		if count == 0 && unicode.IsSpace(value) {
			continue
		}
		slot := count % int64(goTestSyntaxWindowRunes)
		raw[slot], lower[slot] = value, unicode.ToLower(value)
		count++
		if !unicode.IsSpace(value) {
			trimmedCount = count
		}
		if count == int64(len(goTestSyntaxFuzzFailLinePrefix)) && goTestSyntaxSuffix(&raw, count, goTestSyntaxFuzzFailLinePrefix) {
			facts.FailureHeader = true
		}
		if count == int64(len(goTestSyntaxPackageFailLinePrefix)) && goTestSyntaxSuffix(&raw, count, goTestSyntaxPackageFailLinePrefix) {
			facts.PackageFailure = true
		}
		if goTestSyntaxSuffix(&lower, count, goTestSyntaxPanicDiagnosticFragment) {
			facts.FoldedPanic = true
		}
		if goTestSyntaxSuffix(&raw, count, goTestSyntaxPanicDiagnosticFragment) {
			facts.Panic = true
		}
		if goTestSyntaxSuffix(&lower, count, goTestSyntaxGoRuntimeErrorFragment) {
			facts.RuntimeError = true
		}
		if goTestSyntaxSuffix(&lower, count, goTestSyntaxGoFatalErrorFragment) {
			facts.FatalError = true
		}
		if goTestSyntaxSuffix(&lower, count, goTestSyntaxContextDeadlineExceededText) {
			facts.ContextDeadline = true
		}
		if count == int64(len(goTestSyntaxGoSignalLinePrefix)) && goTestSyntaxSuffix(&lower, count, goTestSyntaxGoSignalLinePrefix) {
			facts.Signal = true
		}
		if count == int64(len(goTestSyntaxGoDataRaceLinePrefix)) && goTestSyntaxSuffix(&lower, count, goTestSyntaxGoDataRaceLinePrefix) {
			facts.DataRace = true
		}
		if count == int64(len(goTestSyntaxExitStatusPrefix)) {
			exitPrefix = goTestSyntaxSuffix(&lower, count, goTestSyntaxExitStatusPrefix)
		}
		if exitPrefix && count > int64(len(goTestSyntaxExitStatusPrefix)) {
			if !unicode.IsSpace(value) {
				restNonspace = true
				if delimiter {
					afterDelimiterNonspace = true
				}
			}
			if !delimiter {
				if value == ' ' {
					firstSpace = tokenCount == 0
					delimiter = true
				} else {
					if tokenCount == 0 {
						tokenFirst = value
					}
					tokenCount++
					if !unicode.IsSpace(value) {
						tokenEffective = tokenCount
					}
				}
			}
		}
	}
	// Native exit-status syntax cuts at the first ASCII space only. A leading space
	// leaves the entire nonempty remainder as the status token. Unicode space
	// at the record's end is trimmed; interior tabs remain token data.
	statusCount := tokenEffective
	if delimiter && afterDelimiterNonspace {
		statusCount = tokenCount
	}
	if exitPrefix && restNonspace && (firstSpace || statusCount != 1 || tokenFirst != '0') {
		facts.NonZeroExitStatus = true
	}
	if trimmedCount < int64(len(goTestSyntaxPackageFailLinePrefix)) {
		facts.PackageFailure = false
	}
	if trimmedCount < int64(len(goTestSyntaxGoSignalLinePrefix)) {
		facts.Signal = false
	}

	return facts, nil
}

func goTestSyntaxSuffix(window *[goTestSyntaxWindowRunes]rune, count int64, pattern string) bool {
	if count < int64(len(pattern)) {
		return false
	}
	start := count - int64(len(pattern))
	for index := range len(pattern) {
		if window[(start+int64(index))%int64(goTestSyntaxWindowRunes)] != rune(pattern[index]) {
			return false
		}
	}
	return true
}
