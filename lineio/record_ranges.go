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

// RecordFraming records the native terminator of one complete physical record.
type RecordFraming uint8

const (
	RecordFramingUnknown RecordFraming = iota
	RecordFramingLF
	RecordFramingEOF
)

func (f RecordFraming) Validate() error {
	if f != RecordFramingLF && f != RecordFramingEOF {
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

// RecordRanges yields complete physical records with constant working memory.
// Invalid UTF-8, NUL, whitespace and CR remain uninterpreted source bytes.
// A read failure never publishes an interrupted partial record. Clean EOF
// publishes a final unterminated record and produces no empty tail record.
func RecordRanges(ctx context.Context, request RecordRangeRequest) iter.Seq2[RecordRange, error] {
	return func(yield func(RecordRange, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(RecordRange{}, err)
			return
		}
		offset, err := request.Source.Seek(0, io.SeekCurrent)
		if err != nil || offset < 0 {
			yield(RecordRange{}, errors.Join(core.ErrLineIOScan, err))
			return
		}
		source := recordRangeReadGuard{source: request.Source}
		reader, err := New(Request{Source: &source, BufferBytes: request.BufferBytes})
		if err != nil {
			yield(RecordRange{}, err)
			return
		}
		for {
			start := offset
			for {
				fragment, err := reader.ReadFragment(ctx)
				if source.readErr != nil {
					yield(RecordRange{}, errors.Join(core.ErrLineIOScan, err, source.readErr))
					return
				}
				if err != nil && err != io.EOF {
					yield(RecordRange{}, err)
					return
				}
				if int64(len(fragment.Bytes)) > math.MaxInt64-offset {
					yield(RecordRange{}, core.ErrLineIOScan)
					return
				}
				offset += int64(len(fragment.Bytes))
				if fragment.More {
					continue
				}
				if offset == start {
					return
				}
				size, sizeErr := core.NewByteLength(uint64(offset - start))
				if sizeErr != nil {
					yield(RecordRange{}, sizeErr)
					return
				}
				framing := RecordFramingLF
				if err == io.EOF {
					framing = RecordFramingEOF
				}
				record := RecordRange{Offset: SourceByteOffset(start), Bytes: size, Framing: framing}
				if validation := record.Validate(); validation != nil {
					yield(RecordRange{}, validation)
					return
				}
				if !yield(record, nil) || err == io.EOF {
					return
				}
				break
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
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(Fragment{}, err)
			return
		}
		size, err := request.Record.Bytes.Int64()
		if err != nil {
			yield(Fragment{}, err)
			return
		}
		source := recordRangeReadGuard{source: io.NewSectionReader(request.Source, int64(request.Record.Offset), size)}
		reader, err := New(Request{Source: &source, BufferBytes: request.BufferBytes})
		if err != nil {
			yield(Fragment{}, err)
			return
		}
		var observed int64
		for {
			fragment, err := reader.ReadFragment(ctx)
			if source.readErr != nil {
				yield(Fragment{}, errors.Join(core.ErrLineIOScan, err, source.readErr))
				return
			}
			if err != nil && err != io.EOF {
				yield(Fragment{}, err)
				return
			}
			observed += int64(len(fragment.Bytes))
			if !fragment.More && err == nil && (observed != size || request.Record.Framing != RecordFramingLF) {
				yield(Fragment{}, core.ErrLineIOScan)
				return
			}
			if err == io.EOF && (observed != size || request.Record.Framing != RecordFramingEOF) {
				yield(Fragment{}, errors.Join(core.ErrLineIOScan, io.ErrUnexpectedEOF))
				return
			}
			if len(fragment.Bytes) > 0 && !yield(fragment, nil) {
				return
			}
			if err == io.EOF || !fragment.More {
				return
			}
		}
	}
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
