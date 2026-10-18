package lineio

import (
	"context"
	"errors"
	"io"
	"iter"
	"math"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// RecordSource lends the same bytes through sequential and positional reads.
// Its lifetime remains inside the caller's native scope.
type RecordSource interface {
	io.ReadSeeker
	io.ReaderAt
}

// RecordFraming selects physical-line framing or an exact uninterpreted span.
type RecordFraming uint8

const (
	RecordFramingUnknown RecordFraming = iota
	RecordFramingLF
	RecordFramingEOF
	// RecordFramingOpaque admits the exact range independently of contained LF
	// bytes. Go still lends physical fragments; no source text is collected.
	RecordFramingOpaque
)

func (f RecordFraming) Validate() error {
	if f != RecordFramingLF && f != RecordFramingEOF && f != RecordFramingOpaque {
		return core.ErrLineIOContract
	}
	return nil
}
func (RecordFraming) OffWireEnum() {}

// RecordRange retains exact source coordinates, including LF when present.
type RecordRange struct {
	Offset  SourceByteOffset
	Bytes   core.ByteLength
	Framing RecordFraming
}

func (r RecordRange) Validate() error {
	if err := errors.Join(r.Offset.Validate(), r.Framing.Validate()); err != nil {
		return err
	}
	size, err := r.Bytes.Int64()
	if err != nil {
		return err
	}
	if size == 0 || int64(r.Offset) > math.MaxInt64-size {
		return core.ErrLineIOContract
	}
	return nil
}

// RecordRangeRequest selects a native source at its current byte position.
// BufferBytes selects working memory and imposes no input-size quota.
type RecordRangeRequest struct {
	Source      RecordSource
	BufferBytes core.ByteCount
}

func (r RecordRangeRequest) Validate() error {
	return (Request{Source: r.Source, BufferBytes: r.BufferBytes}).Validate()
}

// RangeReader owns one fixed native line buffer and its exact consumed offset.
// It borrows the source, starts no goroutine or coroutine, and has no input quota.
// The caller owns source cleanup and stops after a refusal.
type RangeReader struct {
	reader *Reader
	source *recordRangeReadGuard
	offset int64
}

func NewRangeReader(ctx context.Context, request RecordRangeRequest) (*RangeReader, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return nil, err
	}
	offset, err := request.Source.Seek(0, io.SeekCurrent)
	if err != nil || offset < 0 {
		return nil, errors.Join(core.ErrLineIOScan, err)
	}
	source := &recordRangeReadGuard{source: request.Source}
	reader, err := New(Request{Source: source, BufferBytes: request.BufferBytes})
	if err != nil {
		return nil, err
	}
	return &RangeReader{reader: reader, source: source, offset: offset}, nil
}

func (r *RangeReader) Validate() error {
	if r == nil || r.source == nil || r.offset < 0 {
		return core.ErrLineIOContract
	}
	return r.reader.Validate()
}

// ReadRecord returns the next complete physical span, or bare io.EOF when no
// record remains. A final unterminated record succeeds with RecordFramingEOF.
// No partial range escapes a source failure. Cancellation before a read leaves
// the cursor untouched; source interruption remains the caller's responsibility.
func (r *RangeReader) ReadRecord(ctx context.Context) (RecordRange, error) {
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return RecordRange{}, err
	}
	start := r.offset
	for {
		fragment, err := r.reader.ReadFragment(ctx)
		if r.source.readErr != nil {
			return RecordRange{}, errors.Join(core.ErrLineIOScan, err, r.source.readErr)
		}
		if err != nil && err != io.EOF {
			return RecordRange{}, err
		}
		if int64(len(fragment.Bytes)) > math.MaxInt64-r.offset {
			return RecordRange{}, core.ErrLineIOScan
		}
		r.offset += int64(len(fragment.Bytes))
		if fragment.More {
			continue
		}
		if r.offset == start {
			return RecordRange{}, io.EOF
		}
		size, sizeErr := core.NewByteLength(uint64(r.offset - start))
		if sizeErr != nil {
			return RecordRange{}, sizeErr
		}
		framing := RecordFramingLF
		if err == io.EOF {
			framing = RecordFramingEOF
		}
		record := RecordRange{Offset: SourceByteOffset(start), Bytes: size, Framing: framing}
		if err := record.Validate(); err != nil {
			return RecordRange{}, err
		}
		return record, nil
	}
}

// RecordRanges yields complete physical records with constant working memory.
// Invalid UTF-8, NUL, whitespace and CR remain uninterpreted source bytes.
// A read failure never publishes an interrupted partial record. Clean EOF
// publishes a final unterminated record and produces no empty tail record.
func RecordRanges(ctx context.Context, request RecordRangeRequest) iter.Seq2[RecordRange, error] {
	return func(yield func(RecordRange, error) bool) {
		reader, err := NewRangeReader(ctx, request)
		if err != nil {
			yield(RecordRange{}, err)
			return
		}
		for {
			record, err := reader.ReadRecord(ctx)
			if err == io.EOF {
				return
			}
			if !yield(record, err) || err != nil {
				return
			}
		}
	}
}

// RecordFragmentRequest selects one complete range from its original source.
type RecordFragmentRequest struct {
	Source      RecordSource
	Record      RecordRange
	BufferBytes core.ByteCount
}

func (r RecordFragmentRequest) Validate() error {
	return errors.Join(r.Record.Validate(), (Request{Source: r.Source, BufferBytes: r.BufferBytes}).Validate())
}

// RecordFragments lends exact range bytes through the existing native fragment
// contract. No record materialization or source-length quota is introduced.
func RecordFragments(ctx context.Context, request RecordFragmentRequest) iter.Seq2[Fragment, error] {
	return func(yield func(Fragment, error) bool) {
		reader, err := newRecordFragmentReader(ctx, request)
		if err != nil {
			yield(Fragment{}, err)
			return
		}
		for {
			fragment, err := reader.readFragment(ctx)
			if err == io.EOF {
				return
			}
			if err != nil {
				yield(Fragment{}, err)
				return
			}
			if !yield(fragment, nil) {
				return
			}
		}
	}
}

// recordFragmentReader is the shared native fragment cursor. Iterator and
// borrowed-reader consumers use the same Go buffered reader and framing checks;
// adapting between those forms starts no coroutine or independent worker.
type recordFragmentReader struct {
	reader   *Reader
	source   recordRangeReadGuard
	observed int64
	size     int64
	framing  RecordFraming
	complete bool
}

func newRecordFragmentReader(ctx context.Context, request RecordFragmentRequest) (*recordFragmentReader, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return nil, err
	}
	size, err := request.Record.Bytes.Int64()
	if err != nil {
		return nil, err
	}
	cursor := &recordFragmentReader{source: recordRangeReadGuard{source: io.NewSectionReader(request.Source, int64(request.Record.Offset), size)}, size: size, framing: request.Record.Framing}
	cursor.reader, err = New(Request{Source: &cursor.source, BufferBytes: request.BufferBytes})
	if err != nil {
		return nil, err
	}
	return cursor, nil
}

func (r *recordFragmentReader) readFragment(ctx context.Context) (Fragment, error) {
	if r.complete {
		return Fragment{}, io.EOF
	}
	fragment, err := r.reader.ReadFragment(ctx)
	if r.source.readErr != nil {
		return Fragment{}, errors.Join(core.ErrLineIOScan, err, r.source.readErr)
	}
	if err != nil && err != io.EOF {
		return Fragment{}, err
	}
	r.observed += int64(len(fragment.Bytes))
	if !fragment.More && err == nil && r.framing != RecordFramingOpaque && (r.observed != r.size || r.framing != RecordFramingLF) {
		return Fragment{}, core.ErrLineIOScan
	}
	if err == io.EOF && (r.observed != r.size || r.framing != RecordFramingEOF && r.framing != RecordFramingOpaque) {
		return Fragment{}, errors.Join(core.ErrLineIOScan, io.ErrUnexpectedEOF)
	}
	r.complete = err == io.EOF || !fragment.More && r.framing != RecordFramingOpaque
	if len(fragment.Bytes) == 0 {
		return Fragment{}, io.EOF
	}
	return fragment, nil
}

var _ core.OffWireEnum = RecordFramingUnknown
var _ core.Validatable = RecordRange{}
var _ core.Validatable = RecordRangeRequest{}
var _ core.Validatable = RecordFragmentRequest{}

// recordRangeReadGuard retains an accompanying native error even when bufio
// finds LF in the same successful byte count.
type recordRangeReadGuard struct {
	source  io.Reader
	readErr error
}

func (r *recordRangeReadGuard) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	if n < 0 || n > len(p) {
		r.readErr = errors.Join(core.ErrLineIOScan, io.ErrShortBuffer, err)
		return 0, r.readErr
	}
	if err != nil && err != io.EOF {
		r.readErr = err
	}
	return n, err
}

var _ core.Validatable = (*RangeReader)(nil)
