package filestore

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"slices"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// These constants bound working memory, never input or output cardinality.
const contentSortRunEntries = 4096
const contentSortBufferBytes = 32 << 10

// ContentSortRequest borrows two distinct writable regular scratch files.
// Files[0] contains canonical ContentIndexEntry records, in any order. Both
// files may be overwritten. The caller owns their closure and removal.
// Destination receives sorted unique records and must not alias either file.
// The caller must retain exclusive custody of both scratch files throughout
// the operation, including through wrapped writers. All destination bytes
// are provisional until SortContentIndex succeeds.
type ContentSortRequest struct {
	Files       [2]*os.File
	Destination io.Writer
}

func (r ContentSortRequest) Validate() error {
	if r.Files[0] == nil || r.Files[1] == nil || core.WriterIsNil(r.Destination) {
		return core.ErrFilestoreContract
	}
	first, firstErr := r.Files[0].Stat()
	second, secondErr := r.Files[1].Stat()
	if err := errors.Join(firstErr, secondErr); err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	if !first.Mode().IsRegular() || !second.Mode().IsRegular() || os.SameFile(first, second) {
		return core.ErrFilestoreContract
	}
	if destination, ok := r.Destination.(*os.File); ok {
		info, err := destination.Stat()
		if err != nil {
			return errors.Join(core.ErrFilestoreContract, err)
		}
		if os.SameFile(first, info) || os.SameFile(second, info) {
			return core.ErrFilestoreContract
		}
	}
	return nil
}

// ContentIndexSummary is derived from the complete sorted unique stream.
// Empty input has zero entries and extent. No arrival-time or product state is
// inferred, and a failure returns no summary.
type ContentIndexSummary struct {
	Entries uint64
	Extent  core.ByteLength
}

func (s ContentIndexSummary) Validate() error {
	if err := s.Extent.Validate(); err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	if s.Entries == 0 && s.Extent.Uint64() != 0 {
		return core.ErrFilestoreContract
	}
	return nil
}

// ContentIndexConflictError refuses two individually valid extents for the
// same content digest. Identical repeated records are ordinary duplicates.
type ContentIndexConflictError struct {
	Digest core.SHA256Digest
	First  core.ByteLength
	Second core.ByteLength
}

func (e ContentIndexConflictError) Error() string {
	return "content index has conflicting extents for one digest"
}
func (ContentIndexConflictError) Unwrap() error { return core.ErrFilestoreContract }
func (e ContentIndexConflictError) Validate() error {
	if err := errors.Join(e.Digest.Validate(), e.First.Validate(), e.Second.Validate()); err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	if e.First == e.Second {
		return core.ErrFilestoreContract
	}
	return nil
}

// SortContentIndex externally sorts and deduplicates content records. It uses
// fixed-size sorted runs and two-way merge passes, O(1) resident memory and
// O(n log n) work. It preflights conflicts and total-extent overflow before
// writing to Destination, then verifies the held sorted stream again while
// emitting. It neither creates files nor publishes a durable receipt.
func SortContentIndex(ctx context.Context, request ContentSortRequest) (ContentIndexSummary, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return ContentIndexSummary{}, err
	}
	if err := request.Validate(); err != nil {
		return ContentIndexSummary{}, err
	}
	sorted, err := sortContentRuns(ctx, request.Files)
	if err != nil {
		return ContentIndexSummary{}, errors.Join(core.ErrFilestoreContract, err)
	}
	summary, digest, err := inspectContentIndex(ctx, sorted, nil)
	if err != nil {
		return ContentIndexSummary{}, errors.Join(core.ErrFilestoreContract, err)
	}
	emitted, emittedDigest, err := inspectContentIndex(ctx, sorted, request.Destination)
	if err != nil {
		return ContentIndexSummary{}, errors.Join(core.ErrFilestoreContract, err)
	}
	if emitted != summary || emittedDigest != digest {
		return ContentIndexSummary{}, core.ErrFilestoreContract
	}
	if err := contextstate.Validate(ctx); err != nil {
		return ContentIndexSummary{}, err
	}
	return summary, nil
}

func compareContentEntries(a, b ContentIndexEntry) (int, error) {
	left, leftErr := a.Digest.Bytes()
	right, rightErr := b.Digest.Bytes()
	if err := errors.Join(leftErr, rightErr); err != nil {
		return 0, err
	}
	return bytes.Compare(left[:], right[:]), nil
}

func sortContentRuns(ctx context.Context, files [2]*os.File) (*os.File, error) {
	count, err := writeInitialContentRuns(ctx, files[0], files[1])
	if err != nil {
		return nil, err
	}
	source, destination := files[1], files[0]
	for width := int64(contentSortRunEntries); width < count; width *= 2 {
		if err := mergeContentRuns(ctx, source, destination, count, width); err != nil {
			return nil, err
		}
		source, destination = destination, source
	}
	return source, nil
}

func writeInitialContentRuns(ctx context.Context, source, destination *os.File) (int64, error) {
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	if err := resetContentScratch(destination); err != nil {
		return 0, err
	}
	reader := bufio.NewReaderSize(source, contentSortBufferBytes)
	writer := bufio.NewWriterSize(destination, contentSortBufferBytes)
	var entries [contentSortRunEntries]ContentIndexEntry
	var total int64
	for {
		count := 0
		for count < len(entries) {
			if err := contextstate.Validate(ctx); err != nil {
				return 0, err
			}
			entry, err := ReadContentIndexEntry(reader)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return 0, err
			}
			entries[count] = entry
			count++
		}
		if count == 0 {
			break
		}
		if total > math.MaxInt64/ContentIndexRecordBytes-int64(count) {
			return 0, core.ErrFilestoreContract
		}
		var comparisonErr error
		slices.SortFunc(entries[:count], func(a, b ContentIndexEntry) int {
			order, err := compareContentEntries(a, b)
			if err != nil {
				comparisonErr = err
			}
			return order
		})
		if comparisonErr != nil {
			return 0, comparisonErr
		}
		for _, entry := range entries[:count] {
			if err := WriteContentIndexEntry(writer, entry); err != nil {
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

func resetContentScratch(file *os.File) error {
	if err := file.Truncate(0); err != nil {
		return err
	}
	_, err := file.Seek(0, io.SeekStart)
	return err
}

func mergeContentRuns(ctx context.Context, source, destination *os.File, count, width int64) error {
	if err := resetContentScratch(destination); err != nil {
		return err
	}
	left := bufio.NewReaderSize(nil, contentSortBufferBytes)
	right := bufio.NewReaderSize(nil, contentSortBufferBytes)
	writer := bufio.NewWriterSize(destination, contentSortBufferBytes)
	for offset := int64(0); offset < count; offset += 2 * width {
		middle := min(offset+width, count)
		end := min(middle+width, count)
		left.Reset(io.NewSectionReader(source, offset*ContentIndexRecordBytes, (middle-offset)*ContentIndexRecordBytes))
		right.Reset(io.NewSectionReader(source, middle*ContentIndexRecordBytes, (end-middle)*ContentIndexRecordBytes))
		if err := mergeContentPair(ctx, left, right, writer); err != nil {
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
	if info.Size() != count*ContentIndexRecordBytes {
		return errors.Join(core.ErrFilestoreContract, io.ErrUnexpectedEOF)
	}
	return contextstate.Validate(ctx)
}

func mergeContentPair(ctx context.Context, left, right io.Reader, destination io.Writer) error {
	a, aErr := ReadContentIndexEntry(left)
	b, bErr := ReadContentIndexEntry(right)
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		if aErr != nil && !errors.Is(aErr, io.EOF) {
			return aErr
		}
		if bErr != nil && !errors.Is(bErr, io.EOF) {
			return bErr
		}
		if errors.Is(aErr, io.EOF) && errors.Is(bErr, io.EOF) {
			return nil
		}
		order := 0
		if aErr == nil && bErr == nil {
			var err error
			order, err = compareContentEntries(a, b)
			if err != nil {
				return err
			}
		}
		if bErr != nil || (aErr == nil && order <= 0) {
			if err := WriteContentIndexEntry(destination, a); err != nil {
				return err
			}
			a, aErr = ReadContentIndexEntry(left)
		} else {
			if err := WriteContentIndexEntry(destination, b); err != nil {
				return err
			}
			b, bErr = ReadContentIndexEntry(right)
		}
	}
}

func inspectContentIndex(ctx context.Context, file *os.File, destination io.Writer) (ContentIndexSummary, core.SHA256Digest, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return ContentIndexSummary{}, core.SHA256Digest{}, err
	}
	digest := core.NewDigestWriter()
	reader := bufio.NewReaderSize(io.TeeReader(file, digest), contentSortBufferBytes)
	var prior ContentIndexEntry
	var count, total uint64
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return ContentIndexSummary{}, core.SHA256Digest{}, err
		}
		entry, err := ReadContentIndexEntry(reader)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return ContentIndexSummary{}, core.SHA256Digest{}, err
		}
		if count != 0 {
			order, err := compareContentEntries(prior, entry)
			if err != nil || order > 0 {
				return ContentIndexSummary{}, core.SHA256Digest{}, errors.Join(core.ErrFilestoreContract, err)
			}
		}
		if count != 0 && prior.Digest == entry.Digest {
			if prior.Extent != entry.Extent {
				first, second := prior.Extent, entry.Extent
				if first.Uint64() > second.Uint64() {
					first, second = second, first
				}
				return ContentIndexSummary{}, core.SHA256Digest{}, ContentIndexConflictError{Digest: entry.Digest, First: first, Second: second}
			}
			continue
		}
		if total > math.MaxUint64-entry.Extent.Uint64() {
			return ContentIndexSummary{}, core.SHA256Digest{}, core.ErrFilestoreContract
		}
		if destination != nil {
			if err := WriteContentIndexEntry(destination, entry); err != nil {
				return ContentIndexSummary{}, core.SHA256Digest{}, err
			}
		}
		count++
		total += entry.Extent.Uint64()
		prior = entry
	}
	extent, err := core.NewByteLength(total)
	if err != nil {
		return ContentIndexSummary{}, core.SHA256Digest{}, err
	}
	checksum, _, err := digest.Seal()
	if err != nil {
		return ContentIndexSummary{}, core.SHA256Digest{}, err
	}
	summary := ContentIndexSummary{Entries: count, Extent: extent}
	if err := errors.Join(summary.Validate(), contextstate.Validate(ctx)); err != nil {
		return ContentIndexSummary{}, core.SHA256Digest{}, err
	}
	return summary, checksum, nil
}
