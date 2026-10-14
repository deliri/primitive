package runprotocol

import (
	"bufio"
	"context"
	"errors"
	"io"
	"iter"
	"regexp"

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
// memory. Go regexp owns matching; native record/rune scopes own framing and
// late refusal. Callers own the source and must complete it before sealing facts.
func GoFuzzSignalRecords(ctx context.Context, request GoFuzzSignalSourceRequest) iter.Seq2[GoFuzzSignalRecord, error] {
	return func(yield func(GoFuzzSignalRecord, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(GoFuzzSignalRecord{}, err)
			return
		}
		buffer, err := core.NewByteCount(4096)
		if err != nil {
			yield(GoFuzzSignalRecord{}, err)
			return
		}
		patterns := [...]string{`panic:`, `[cC][oO][nN][tT][eE][xX][tT] [dD][eE][aA][dD][lL][iI][nN][eE] [eE][xX][cC][eE][eE][dD][eE][dD]`, `^[\x09-\x0D\x{85}\p{Z}]*(--- FAIL:)`, `[fF][aA][iI][lL][iI][nN][gG] [iI][nN][pP][uU][tT]`, `^[\x09-\x0D\x{85}\p{Z}]*([tT][oO] [rR][eE]-[rR][uU][nN]:)`, `^[\x09-\x0D\x{85}\p{Z}]*(go test )`, `^[\x09-\x0D\x{85}\p{Z}]*(# )`}
		var compiled [len(patterns)]*regexp.Regexp
		for i, pattern := range patterns {
			compiled[i] = regexp.MustCompile(pattern)
		}
		for record, err := range lineio.RecordRanges(ctx, lineio.RecordRangeRequest{Source: request.Source, BufferBytes: buffer}) {
			if err != nil {
				yield(GoFuzzSignalRecord{}, err)
				return
			}
			// Validate native framing through the complete declared source span.
			for _, err := range lineio.RecordFragments(ctx, lineio.RecordFragmentRequest{Source: request.Source, Record: record, BufferBytes: buffer}) {
				if err != nil {
					yield(GoFuzzSignalRecord{}, err)
					return
				}
			}
			observation := GoFuzzSignalRecord{Record: record}
			spans := [...]*lineio.RecordRange{&observation.Panic, &observation.Deadline, &observation.FailureHeader, &observation.FailingInput, &observation.RerunHeader, &observation.ReplayCommand, &observation.BuildHeader}
			for i, pattern := range compiled {
				match, err := goFuzzSignalMatch(ctx, request.Source, record, pattern)
				if err != nil {
					yield(GoFuzzSignalRecord{}, err)
					return
				}
				if len(match) == 0 {
					continue
				}
				extent, err := core.NewByteLength(uint64(match[1] - match[0]))
				if err != nil {
					yield(GoFuzzSignalRecord{}, err)
					return
				}
				*spans[i] = lineio.RecordRange{Offset: lineio.SourceByteOffset(int64(record.Offset) + int64(match[0])), Bytes: extent, Framing: lineio.RecordFramingOpaque}
			}
			if err := errors.Join(observation.Validate(), contextstate.Validate(ctx)); err != nil {
				yield(GoFuzzSignalRecord{}, err)
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

// Capturing only the fixed marker excludes arbitrary leading whitespace from
// its source coordinate. regexp reads native runes; draining retains errors.
func goFuzzSignalMatch(ctx context.Context, source lineio.RecordSource, record lineio.RecordRange, pattern *regexp.Regexp) ([]int, error) {
	length, _ := record.Bytes.Int64()
	guard := &goFuzzFrameSource{ctx: ctx, source: io.NewSectionReader(source, int64(record.Offset), length)}
	reader := bufio.NewReader(guard)
	match := pattern.FindReaderSubmatchIndex(reader)
	_, drainErr := io.Copy(io.Discard, reader)
	if err := errors.Join(guard.readErr, drainErr, contextstate.Validate(ctx)); err != nil {
		return nil, err
	}
	if len(match) > 2 {
		return match[2:4], nil
	}
	return match, nil
}
