package filestore

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// SortedRecordReadView borrows immutable source bytes and their sorted native
// coordinates. The index preserves equal records in arrival order. Neither
// capability owns closure; both expire when the enclosing Use returns.
type SortedRecordReadView struct {
	Source lineio.RecordSource
	Index  RecordRangeIndexSource
}

func (v SortedRecordReadView) Validate() error {
	if core.ReaderIsNil(v.Source) || core.ReaderIsNil(v.Index) {
		return core.ErrFilestoreContract
	}
	return nil
}

// SortedRecordScopeRequest supplies framing-independent comparison policy.
// Produce writes physical JSONL/text records through a native bounded writer.
// Use runs only after production, framing admission, sorting and flush succeed.
// There is no input population, cardinality limit or durable publication claim.
type SortedRecordScopeRequest struct {
	Parent  core.AbsolutePath
	Produce func(context.Context, io.Writer) error
	Compare func(context.Context, lineio.RecordSource, lineio.RecordRange, lineio.RecordRange) (core.Comparison, error)
	Use     func(context.Context, SortedRecordReadView) error
}

func (r SortedRecordScopeRequest) Validate() error {
	if err := r.Parent.Validate(); err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	if r.Produce == nil || r.Compare == nil || r.Use == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// WithSortedRecordScope owns acquisition, buffered source production, external
// sorting, read-only index lending and cleanup. A late cleanup refusal remains
// a refusal even after Use; all consumer effects are provisional until return.
func WithSortedRecordScope(ctx context.Context, request SortedRecordScopeRequest) (resultErr error) {
	defer func() { resultErr = scopeCancellationError(ctx, errors.Join(resultErr, contextstate.Validate(ctx))) }()
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return err
	}
	return WithScratchScope(ctx, ScratchScopeRequest{Parent: request.Parent, Use: func(ctx context.Context, root *os.Root) (resultErr error) {
		source, err := root.OpenFile("source-records", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err != nil {
			return activationError(err)
		}
		defer func() {
			if err := source.Close(); err != nil {
				resultErr = errors.Join(resultErr, cleanupError(err))
			}
		}()
		index, err := root.OpenFile("sorted-coordinates", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if err != nil {
			return activationError(err)
		}
		defer func() {
			if err := index.Close(); err != nil {
				resultErr = errors.Join(resultErr, cleanupError(err))
			}
		}()
		writer := bufio.NewWriterSize(source, contentSortBufferBytes)
		if err := request.Produce(ctx, writer); err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return destinationError(err)
		}
		info, err := source.Stat()
		if err != nil {
			return sourceError(err)
		}
		view := SortedRecordReadView{Source: io.NewSectionReader(source, 0, info.Size()), Index: sortedRecordIndexReader{file: index}}
		coordinates := bufio.NewWriterSize(index, contentSortBufferBytes)
		if _, err := SortRecordStream(ctx, RecordRangeStreamRequest{Parent: request.Parent, Source: view.Source, Compare: request.Compare, Visit: func(record lineio.RecordRange) error { return WriteRecordRangeIndex(coordinates, record) }}); err != nil {
			return err
		}
		if err := coordinates.Flush(); err != nil {
			return destinationError(err)
		}
		if err := errors.Join(view.Validate(), contextstate.Validate(ctx)); err != nil {
			return err
		}
		if err := Rewind(ctx, RewindRequest{Source: view.Source}); err != nil {
			return err
		}
		if err := Rewind(ctx, RewindRequest{Source: view.Index}); err != nil {
			return err
		}
		return request.Use(ctx, view)
	}})
}

// Keep the mutable native file and Close out of the borrowed index contract.
type sortedRecordIndexReader struct{ file *os.File }

func (r sortedRecordIndexReader) Read(data []byte) (int, error) { return r.file.Read(data) }
func (r sortedRecordIndexReader) ReadAt(data []byte, offset int64) (int, error) {
	return r.file.ReadAt(data, offset)
}
func (r sortedRecordIndexReader) Seek(offset int64, whence int) (int64, error) {
	return r.file.Seek(offset, whence)
}
func (r sortedRecordIndexReader) Stat() (fs.FileInfo, error) { return r.file.Stat() }

var _ core.Validatable = SortedRecordReadView{}
var _ core.Validatable = SortedRecordScopeRequest{}
var _ RecordRangeIndexSource = sortedRecordIndexReader{}
