package filestore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
	"sort"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// RecordRangeIndexSource lends the same native object for positional reads and
// extent observation. Operations do not change its sequential cursor or close
// it. The caller owns its lifetime and exclusive, immutable custody.
type RecordRangeIndexSource interface {
	lineio.RecordSource
	Stat() (fs.FileInfo, error)
}

// RecordRangeIndexPosition is a zero-based position in the native tuple stream.
// Its only limit is the signed byte offset representable by Go's ReaderAt.
type RecordRangeIndexPosition uint64

func (p RecordRangeIndexPosition) Validate() error {
	if p > RecordRangeIndexPosition(math.MaxInt64/RecordRangeIndexBytes) {
		return core.ErrFilestoreContract
	}
	return nil
}

type RecordRangeIndexReadRequest struct {
	Source   RecordRangeIndexSource
	Position RecordRangeIndexPosition
}

func (r RecordRangeIndexReadRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrFilestoreContract
	}
	return r.Position.Validate()
}

// RecordRangeIndexReadObservation distinguishes an observed end from a native
// refusal. Absent is valid only with a zero record and a successful operation.
type RecordRangeIndexReadObservation struct {
	Record  lineio.RecordRange
	Present bool
}

func (o RecordRangeIndexReadObservation) Validate() error {
	if !o.Present {
		if o.Record != (lineio.RecordRange{}) {
			return core.ErrFilestoreContract
		}
		return nil
	}
	return o.Record.Validate()
}

// ObserveRecordRangeIndexAt admits the native extent and reads exactly one
// tuple without moving the sequential cursor. An index position at or beyond
// the observed end returns absent; truncated tuples and all I/O or cancellation
// failures return a zero observation plus their original causes.
func ObserveRecordRangeIndexAt(ctx context.Context, request RecordRangeIndexReadRequest) (observation RecordRangeIndexReadObservation, resultErr error) {
	defer func() {
		resultErr = scopeCancellationError(ctx, errors.Join(resultErr, contextstate.Validate(ctx)))
		if resultErr != nil {
			observation = RecordRangeIndexReadObservation{}
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return observation, err
	}
	count, err := recordRangeIndexCount(request.Source)
	if err != nil {
		return observation, err
	}
	if uint64(request.Position) >= count {
		return observation, nil
	}
	record, err := readRecordRangeIndexPosition(ctx, request.Source, request.Position)
	if err != nil {
		return observation, err
	}
	return RecordRangeIndexReadObservation{Record: record, Present: true}, nil
}

// ReadRecordRangeIndexAt supplies Go's io.EOF convention for native io.Reader
// consumers. Typed application policy uses ObserveRecordRangeIndexAt so an
// error containing EOF can never be mistaken for an observed end.
func ReadRecordRangeIndexAt(ctx context.Context, request RecordRangeIndexReadRequest) (record lineio.RecordRange, resultErr error) {
	defer func() {
		resultErr = scopeCancellationError(ctx, errors.Join(resultErr, contextstate.Validate(ctx)))
		if resultErr != nil {
			record = lineio.RecordRange{}
		}
	}()
	observation, err := ObserveRecordRangeIndexAt(ctx, request)
	if err != nil {
		return record, err
	}
	if !observation.Present {
		return record, io.EOF
	}
	return observation.Record, nil
}

// Compare reports the ordering of the probed record against a caller-owned
// target. It must be monotonic (less, equal, greater) over the held index. This
// search proves only observed tuples, not global ordering or unprobed content.
type RecordRangeIndexSearchRequest struct {
	Source  RecordRangeIndexSource
	Compare func(context.Context, lineio.RecordRange) (core.Comparison, error)
}

func (r RecordRangeIndexSearchRequest) Validate() error {
	if core.ReaderIsNil(r.Source) || r.Compare == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// RecordRangeIndexSearchObservation contains the first record which is not
// less than the target. Present says a candidate exists, not that it is equal.
// A failed operation returns zero even if comparison has already run.
type RecordRangeIndexSearchObservation struct {
	Record   lineio.RecordRange
	Position RecordRangeIndexPosition
	Present  bool
}

func (o RecordRangeIndexSearchObservation) Validate() error {
	if !o.Present {
		if o.Record != (lineio.RecordRange{}) || o.Position != 0 {
			return core.ErrFilestoreContract
		}
		return nil
	}
	return errors.Join(o.Record.Validate(), o.Position.Validate())
}

// LowerBoundRecordRangeIndex delegates logarithmic probing to Go's sort.Search
// and fixed tuple reads to ReaderAt. Retained memory is one tuple; no index or
// source population is collected. Native I/O and callback causes are preserved.
func LowerBoundRecordRangeIndex(ctx context.Context, request RecordRangeIndexSearchRequest) (observation RecordRangeIndexSearchObservation, resultErr error) {
	defer func() {
		resultErr = scopeCancellationError(ctx, errors.Join(resultErr, contextstate.Validate(ctx)))
		if resultErr != nil {
			observation = RecordRangeIndexSearchObservation{}
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return observation, err
	}
	count, err := recordRangeIndexCount(request.Source)
	if err != nil {
		return observation, err
	}
	nativeCount := int(count)
	if nativeCount < 0 || uint64(nativeCount) != count {
		return observation, core.ErrFilestoreContract
	}
	var refusal error
	position := sort.Search(nativeCount, func(index int) bool {
		if refusal != nil {
			return true
		}
		record, err := readRecordRangeIndexPosition(ctx, request.Source, RecordRangeIndexPosition(index))
		if err != nil {
			refusal = err
			return true
		}
		comparison, err := request.Compare(ctx, record)
		if err != nil {
			refusal = err
			return true
		}
		if err := comparison.Validate(); err != nil {
			refusal = errors.Join(core.ErrFilestoreContract, err)
			return true
		}
		refusal = contextstate.Validate(ctx)
		return comparison != core.ComparisonLess
	})
	if err := errors.Join(refusal, contextstate.Validate(ctx)); err != nil {
		return observation, err
	}
	if position == nativeCount {
		return observation, nil
	}
	record, err := readRecordRangeIndexPosition(ctx, request.Source, RecordRangeIndexPosition(position))
	if err != nil {
		return observation, err
	}
	return RecordRangeIndexSearchObservation{Record: record, Position: RecordRangeIndexPosition(position), Present: true}, nil
}

func recordRangeIndexCount(source RecordRangeIndexSource) (uint64, error) {
	info, err := source.Stat()
	if err != nil {
		return 0, sourceError(err)
	}
	if info == nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size()%RecordRangeIndexBytes != 0 {
		return 0, core.ErrFilestoreContract
	}
	return uint64(info.Size() / RecordRangeIndexBytes), nil
}

func readRecordRangeIndexPosition(ctx context.Context, source RecordRangeIndexSource, position RecordRangeIndexPosition) (lineio.RecordRange, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return lineio.RecordRange{}, err
	}
	var body [RecordRangeIndexBytes]byte
	n, err := source.ReadAt(body[:], int64(position)*RecordRangeIndexBytes)
	if n < 0 || n > len(body) {
		return lineio.RecordRange{}, errors.Join(core.ErrFilestoreContract, io.ErrShortBuffer, err)
	}
	if n != len(body) {
		return lineio.RecordRange{}, sourceError(errors.Join(io.ErrUnexpectedEOF, err))
	}
	if err != nil && err != io.EOF {
		return lineio.RecordRange{}, sourceError(err)
	}
	record, err := ReadRecordRangeIndex(bytes.NewReader(body[:]))
	if err != nil {
		return lineio.RecordRange{}, err
	}
	if err := contextstate.Validate(ctx); err != nil {
		return lineio.RecordRange{}, err
	}
	return record, nil
}

var _ core.Validatable = RecordRangeIndexReadObservation{}
var _ core.Validatable = RecordRangeIndexPosition(0)
var _ core.Validatable = RecordRangeIndexReadRequest{}
var _ core.Validatable = RecordRangeIndexSearchRequest{}
var _ core.Validatable = RecordRangeIndexSearchObservation{}
