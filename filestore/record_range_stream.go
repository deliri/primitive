package filestore

import (
	"bufio"
	"context"
	"errors"
	"io"
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

// RecordRangeIndexStreamRequest borrows the caller's streaming framing
// producer. Index must write canonical tuples with WriteRecordRangeIndex;
// Primitive owns the same scratch and sort machinery as physical-line sorting.
type RecordRangeIndexStreamRequest struct {
	Records RecordRangeStreamRequest
	Index   func(context.Context, io.Writer) error
}

func (r RecordRangeIndexStreamRequest) Validate() error {
	if err := r.Records.Validate(); err != nil {
		return err
	}
	if r.Index == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// SortRecordStream captures only physical range tuples, never source text.
// Its fixed run buffers impose no cardinality or record extent limit. Visitor
// effects remain provisional until sorting and native cleanup both succeed.
func SortRecordStream(ctx context.Context, r RecordRangeStreamRequest) (observation RecordRangeSortObservation, resultErr error) {
	return SortRecordIndexStream(ctx, RecordRangeIndexStreamRequest{Records: r, Index: func(ctx context.Context, destination io.Writer) error {
		buffer, err := core.NewByteCount(contentSortBufferBytes)
		if err != nil {
			return err
		}
		for record, err := range lineio.RecordRanges(ctx, lineio.RecordRangeRequest{Source: r.Source, BufferBytes: buffer}) {
			if err != nil {
				return err
			}
			if err := WriteRecordRangeIndex(destination, record); err != nil {
				return err
			}
		}
		return nil
	}})
}

// SortRecordIndexStream stages only the caller's native source coordinates.
// Fixed run buffers impose no field, record or byte quota. Visitor effects
// remain provisional until source admission, sorting and native cleanup finish.
func SortRecordIndexStream(ctx context.Context, r RecordRangeIndexStreamRequest) (observation RecordRangeSortObservation, resultErr error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return RecordRangeSortObservation{}, err
	}
	defer func() {
		if resultErr != nil {
			observation = RecordRangeSortObservation{}
		}
	}()
	resultErr = WithScratchScope(ctx, ScratchScopeRequest{Parent: r.Records.Parent, Use: func(ctx context.Context, root *os.Root) error {
		var err error
		observation, err = sortRecordStreamInRoot(ctx, root, r)
		return err
	}})
	return observation, resultErr
}
func sortRecordStreamInRoot(ctx context.Context, root *os.Root, r RecordRangeIndexStreamRequest) (observation RecordRangeSortObservation, resultErr error) {
	defer func() {
		if resultErr != nil {
			observation = RecordRangeSortObservation{}
		}
	}()
	var files [2]*os.File
	for i, name := range [...]string{core.FilestoreRecordIndexFilename, core.FilestoreRecordMergeFilename} {
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
	writer := bufio.NewWriterSize(files[0], contentSortBufferBytes)
	if err := r.Index(ctx, writer); err != nil {
		return RecordRangeSortObservation{}, err
	}
	if err := writer.Flush(); err != nil {
		return RecordRangeSortObservation{}, err
	}
	return SortRecordRanges(ctx, RecordRangeSortRequest{Source: r.Records.Source, Files: files, Compare: r.Records.Compare, Visit: r.Records.Visit})
}

var _ core.Validatable = RecordRangeStreamRequest{}
var _ core.Validatable = RecordRangeSortRequest{}
var _ core.Validatable = RecordRangeIndexStreamRequest{}
