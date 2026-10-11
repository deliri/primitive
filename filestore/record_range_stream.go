package filestore

import (
	"bufio"
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// RecordRangeStreamRequest borrows immutable native source bytes and policy
// comparison while Primitive owns its index scratch namespace and lifetimes.
// Source is observed from its current position; offsets refer to its native
// bytes. It must remain unchanged for the entire synchronous operation.
type RecordRangeStreamRequest struct {
	Parent  core.AbsolutePath
	Source  lineio.RecordSource
	Compare func(context.Context, lineio.RecordSource, lineio.RecordRange, lineio.RecordRange) (core.Comparison, error)
	Visit   func(lineio.RecordRange) error
}

func (r RecordRangeStreamRequest) Validate() error {
	if err := r.Parent.Validate(); err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	if core.ReaderIsNil(r.Source) || r.Compare == nil || r.Visit == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// SortRecordStream captures only physical range tuples, never source text.
// Its fixed run buffers impose no cardinality or record extent limit. Visitor
// effects remain provisional until sorting and native cleanup both succeed.
func SortRecordStream(ctx context.Context, r RecordRangeStreamRequest) (observation RecordRangeSortObservation, resultErr error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return RecordRangeSortObservation{}, err
	}
	defer func() {
		if resultErr != nil {
			observation = RecordRangeSortObservation{}
		}
	}()
	resultErr = WithScratchScope(ctx, ScratchScopeRequest{Parent: r.Parent, Use: func(ctx context.Context, root *os.Root) error {
		var err error
		observation, err = sortRecordStreamInRoot(ctx, root, r)
		return err
	}})
	return observation, resultErr
}
func sortRecordStreamInRoot(ctx context.Context, root *os.Root, r RecordRangeStreamRequest) (observation RecordRangeSortObservation, resultErr error) {
	defer func() {
		if resultErr != nil {
			observation = RecordRangeSortObservation{}
		}
	}()
	var files [2]*os.File
	for i, name := range [...]string{"index", "merge"} {
		file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err != nil {
			return RecordRangeSortObservation{}, activationError(err)
		}
		files[i] = file
		defer func() {
			if err := file.Close(); err != nil {
				resultErr = errors.Join(resultErr, cleanupError(err))
			}
		}()
	}
	buffer, err := core.NewByteCount(contentSortBufferBytes)
	if err != nil {
		return RecordRangeSortObservation{}, err
	}
	writer := bufio.NewWriterSize(files[0], contentSortBufferBytes)
	for record, err := range lineio.RecordRanges(ctx, lineio.RecordRangeRequest{Source: r.Source, BufferBytes: buffer}) {
		if err != nil {
			return RecordRangeSortObservation{}, err
		}
		if err := WriteRecordRangeIndex(writer, record); err != nil {
			return RecordRangeSortObservation{}, err
		}
	}
	if err := writer.Flush(); err != nil {
		return RecordRangeSortObservation{}, err
	}
	return SortRecordRanges(ctx, RecordRangeSortRequest{Source: r.Source, Files: files, Compare: r.Compare, Visit: r.Visit})
}

var _ core.Validatable = RecordRangeStreamRequest{}
var _ core.Validatable = RecordRangeSortRequest{}
