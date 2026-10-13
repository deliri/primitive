package runprotocol

import (
	"context"
	"errors"
	"iter"
	"regexp"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// GoFuzzInputRecord retains source coordinates, never a path list or a copied
// physical record. The source remains borrowed from the caller's native scope.
type GoFuzzInputRecord struct {
	record lineio.RecordRange
	path   lineio.RecordRange
}

func (r GoFuzzInputRecord) Record() lineio.RecordRange { return r.record }
func (r GoFuzzInputRecord) Path() lineio.RecordRange   { return r.path }
func (r GoFuzzInputRecord) Validate() error {
	if err := errors.Join(r.record.Validate(), r.path.Validate()); err != nil {
		return err
	}
	whole, _ := r.record.Bytes.Int64()
	size, _ := r.path.Bytes.Int64()
	if r.path.Framing != lineio.RecordFramingOpaque || r.path.Offset < r.record.Offset || int64(r.path.Offset)-int64(r.record.Offset) > whole-size {
		return core.ErrGoToolchainContract
	}
	return nil
}

type GoFuzzInputSourceRequest struct{ Source lineio.RecordSource }

func (r GoFuzzInputSourceRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrGoToolchainContract
	}
	return nil
}

// GoFuzzInputRecords reads every physical record without a flow quota. Go's
// regexp RuneReader supplies marker positions; native trims preserve exact
// path bytes. Empty markers are neutral. A late source refusal stays a refusal.
func GoFuzzInputRecords(ctx context.Context, request GoFuzzInputSourceRequest) iter.Seq2[GoFuzzInputRecord, error] {
	return func(yield func(GoFuzzInputRecord, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(GoFuzzInputRecord{}, err)
			return
		}
		buffer, err := core.NewByteCount(4096)
		if err != nil {
			yield(GoFuzzInputRecord{}, err)
			return
		}
		marker := regexp.MustCompile(`[fF][aA][iI][lL][iI][nN][gG] [iI][nN][pP][uU][tT]`)
		written := regexp.MustCompile(`[wW][rR][iI][tT][tT][eE][nN] [tT][oO]`)
		for record, err := range lineio.RecordRanges(ctx, lineio.RecordRangeRequest{Source: request.Source, BufferBytes: buffer}) {
			if err != nil {
				yield(GoFuzzInputRecord{}, err)
				return
			}
			length, err := record.Bytes.Int64()
			if err != nil {
				yield(GoFuzzInputRecord{}, err)
				return
			}
			span := goFuzzMetricSpan{offset: int64(record.Offset), bytes: length}
			match, err := goFuzzMetricMatch(ctx, request.Source, span, marker)
			if err != nil {
				yield(GoFuzzInputRecord{}, err)
				return
			}
			if len(match) == 0 {
				continue
			}
			span.offset += int64(match[1])
			span.bytes -= int64(match[1])
			match, err = goFuzzMetricMatch(ctx, request.Source, span, written)
			if err != nil {
				yield(GoFuzzInputRecord{}, err)
				return
			}
			if len(match) > 0 {
				span.offset += int64(match[1])
				span.bytes -= int64(match[1])
			}
			span, err = goFuzzMetricTrimmed(ctx, request.Source, span, goFuzzMetricTrimWhitespace)
			if err != nil {
				yield(GoFuzzInputRecord{}, err)
				return
			}
			if span.bytes == 0 {
				continue
			}
			extent, err := core.NewByteLength(uint64(span.bytes))
			if err != nil {
				yield(GoFuzzInputRecord{}, err)
				return
			}
			observation := GoFuzzInputRecord{record: record, path: lineio.RecordRange{Offset: lineio.SourceByteOffset(span.offset), Bytes: extent, Framing: lineio.RecordFramingOpaque}}
			if err := observation.Validate(); err != nil {
				yield(GoFuzzInputRecord{}, err)
				return
			}
			if !yield(observation, nil) {
				return
			}
		}
	}
}

func (GoFuzzInputRecord) runProtocolSealedProjection()           {}
func (GoFuzzInputSourceRequest) runProtocolInternalFlowCarrier() {}
