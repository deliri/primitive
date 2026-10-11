package filestore

import (
	"bufio"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"slices"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

// RecordRangeSortRequest borrows immutable source bytes and two distinct native
// scratch files. Files[0] contains canonical range-index tuples; both scratch
// files may be overwritten. Compare observes borrowed ranges and must provide
// a consistent ordering without external effects. Equal ranges retain arrival
// order. Visit runs only after complete index and ordering admission. The caller
// holds exclusive custody, including through wrappers, and owns every lifetime.
// Scratch files must not alias Source. No input or record extent quota applies.
type RecordRangeSortRequest struct {
	Source  lineio.RecordSource
	Files   [2]*os.File
	Compare func(context.Context, lineio.RecordSource, lineio.RecordRange, lineio.RecordRange) (core.Comparison, error)
	Visit   func(lineio.RecordRange) error
}

func (r RecordRangeSortRequest) Validate() error {
	if core.ReaderIsNil(r.Source) || r.Files[0] == nil || r.Files[1] == nil || r.Compare == nil || r.Visit == nil {
		return core.ErrFilestoreContract
	}
	first, e1 := r.Files[0].Stat()
	second, e2 := r.Files[1].Stat()
	if e1 != nil || e2 != nil {
		return errors.Join(core.ErrFilestoreContract, e1, e2)
	}
	if !first.Mode().IsRegular() || !second.Mode().IsRegular() || os.SameFile(first, second) {
		return core.ErrFilestoreContract
	}
	if file, ok := r.Source.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return errors.Join(core.ErrFilestoreContract, err)
		}
		if os.SameFile(first, info) || os.SameFile(second, info) {
			return core.ErrFilestoreContract
		}
	}
	return nil
}

// RecordRangeSortObservation reports the completely consumed native tuple count.
type RecordRangeSortObservation struct{ Records uint64 }

// SortRecordRanges uses fixed 4096-tuple runs and native two-way merge passes.
// Working memory is constant and sorting work is O(n log n), excluding the
// caller's comparison cost. It neither collects source text nor deduplicates,
// infers product meaning, creates files or publishes a durable receipt. A
// refusal returns zero even if Visit has already consumed admitted records.
func SortRecordRanges(ctx context.Context, r RecordRangeSortRequest) (RecordRangeSortObservation, error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return RecordRangeSortObservation{}, err
	}
	count, err := writeInitialRecordRangeRuns(ctx, r)
	if err != nil {
		return RecordRangeSortObservation{}, err
	}
	source, destination := r.Files[1], r.Files[0]
	for width := int64(contentSortRunEntries); width < count; width *= 2 {
		if err := mergeRecordRangeRuns(ctx, r, source, destination, count, width); err != nil {
			return RecordRangeSortObservation{}, err
		}
		source, destination = destination, source
	}
	if err := walkSortedRecordRanges(ctx, r, source, count, nil); err != nil {
		return RecordRangeSortObservation{}, err
	}
	if err := walkSortedRecordRanges(ctx, r, source, count, r.Visit); err != nil {
		return RecordRangeSortObservation{}, err
	}
	return RecordRangeSortObservation{Records: uint64(count)}, nil
}
func compareRecordRanges(ctx context.Context, r RecordRangeSortRequest, a, b lineio.RecordRange) (int, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return 0, err
	}
	order, err := r.Compare(ctx, r.Source, a, b)
	if err != nil {
		return 0, err
	}
	if err := order.Validate(); err != nil {
		return 0, errors.Join(core.ErrFilestoreContract, err)
	}
	switch order {
	case core.ComparisonLess:
		return -1, nil
	case core.ComparisonEqual:
		return 0, nil
	case core.ComparisonGreater:
		return 1, nil
	default:
		return 0, core.ErrFilestoreContract
	}
}
func admitRecordRangeSource(ctx context.Context, source lineio.RecordSource, record lineio.RecordRange) error {
	buffer, err := core.NewByteCount(4096)
	if err != nil {
		return err
	}
	for _, err := range lineio.RecordFragments(ctx, lineio.RecordFragmentRequest{Source: source, Record: record, BufferBytes: buffer}) {
		if err != nil {
			return errors.Join(core.ErrFilestoreContract, err)
		}
	}
	return nil
}
func writeInitialRecordRangeRuns(ctx context.Context, r RecordRangeSortRequest) (int64, error) {
	if _, err := r.Files[0].Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	if err := resetContentScratch(r.Files[1]); err != nil {
		return 0, err
	}
	reader := bufio.NewReaderSize(r.Files[0], contentSortBufferBytes)
	writer := bufio.NewWriterSize(r.Files[1], contentSortBufferBytes)
	var entries [contentSortRunEntries]lineio.RecordRange
	var total int64
	for {
		count := 0
		for count < len(entries) {
			if err := contextstate.Validate(ctx); err != nil {
				return 0, err
			}
			record, err := ReadRecordRangeIndex(reader)
			if err == io.EOF {
				break
			}
			if err != nil {
				return 0, err
			}
			if err := admitRecordRangeSource(ctx, r.Source, record); err != nil {
				return 0, err
			}
			entries[count] = record
			count++
		}
		if count == 0 {
			break
		}
		if total > math.MaxInt64/RecordRangeIndexBytes-int64(count) {
			return 0, core.ErrFilestoreContract
		}
		var comparisonErr error
		slices.SortStableFunc(entries[:count], func(a, b lineio.RecordRange) int {
			if comparisonErr != nil {
				return 0
			}
			order, err := compareRecordRanges(ctx, r, a, b)
			if err != nil {
				comparisonErr = err
			}
			return order
		})
		if comparisonErr != nil {
			return 0, comparisonErr
		}
		for _, record := range entries[:count] {
			if err := WriteRecordRangeIndex(writer, record); err != nil {
				return 0, err
			}
		}
		total += int64(count)
	}
	if err := writer.Flush(); err != nil {
		return 0, err
	}
	return total, contextstate.Validate(ctx)
}
func mergeRecordRangeRuns(ctx context.Context, r RecordRangeSortRequest, source, destination *os.File, count, width int64) error {
	if err := resetContentScratch(destination); err != nil {
		return err
	}
	left := bufio.NewReaderSize(nil, contentSortBufferBytes)
	right := bufio.NewReaderSize(nil, contentSortBufferBytes)
	writer := bufio.NewWriterSize(destination, contentSortBufferBytes)
	for offset := int64(0); offset < count; offset += 2 * width {
		middle := min(offset+width, count)
		end := min(middle+width, count)
		left.Reset(io.NewSectionReader(source, offset*RecordRangeIndexBytes, (middle-offset)*RecordRangeIndexBytes))
		right.Reset(io.NewSectionReader(source, middle*RecordRangeIndexBytes, (end-middle)*RecordRangeIndexBytes))
		if err := mergeRecordRangePair(ctx, r, left, right, writer); err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	info, err := destination.Stat()
	if err != nil {
		return err
	}
	if info.Size() != count*RecordRangeIndexBytes {
		return errors.Join(core.ErrFilestoreContract, io.ErrUnexpectedEOF)
	}
	return contextstate.Validate(ctx)
}
func mergeRecordRangePair(ctx context.Context, r RecordRangeSortRequest, left, right io.Reader, writer io.Writer) error {
	a, ae := ReadRecordRangeIndex(left)
	b, be := ReadRecordRangeIndex(right)
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		if ae != nil && ae != io.EOF {
			return ae
		}
		if be != nil && be != io.EOF {
			return be
		}
		if ae == io.EOF && be == io.EOF {
			return nil
		}
		order := 0
		if ae == nil && be == nil {
			var err error
			order, err = compareRecordRanges(ctx, r, a, b)
			if err != nil {
				return err
			}
		}
		if be == io.EOF || ae == nil && order <= 0 {
			if err := WriteRecordRangeIndex(writer, a); err != nil {
				return err
			}
			a, ae = ReadRecordRangeIndex(left)
		} else {
			if err := WriteRecordRangeIndex(writer, b); err != nil {
				return err
			}
			b, be = ReadRecordRangeIndex(right)
		}
	}
}
func walkSortedRecordRanges(ctx context.Context, r RecordRangeSortRequest, file *os.File, count int64, visit func(lineio.RecordRange) error) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(file, contentSortBufferBytes)
	var previous lineio.RecordRange
	var observed int64
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		record, err := ReadRecordRangeIndex(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := admitRecordRangeSource(ctx, r.Source, record); err != nil {
			return err
		}
		if observed > 0 {
			order, err := compareRecordRanges(ctx, r, previous, record)
			if err != nil {
				return err
			}
			if order > 0 {
				return core.ErrFilestoreContract
			}
		}
		if observed >= count {
			return core.ErrFilestoreContract
		}
		observed++
		if visit != nil {
			if err := visit(record); err != nil {
				return err
			}
		}
		previous = record
	}
	if observed != count {
		return core.ErrFilestoreContract
	}
	return contextstate.Validate(ctx)
}
