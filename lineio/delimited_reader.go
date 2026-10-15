package lineio

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// DelimitedBoundary describes only native TAB/LF/EOF framing. Delimiters are
// excluded from Bytes. More continues the same field with another fixed window.
type DelimitedBoundary uint8

const (
	DelimitedBoundaryUnknown DelimitedBoundary = iota
	DelimitedBoundaryMore
	DelimitedBoundaryTab
	DelimitedBoundaryLF
	DelimitedBoundaryEOF
)

func (b DelimitedBoundary) Validate() error {
	if b < DelimitedBoundaryMore || b > DelimitedBoundaryEOF {
		return core.ErrLineIOContract
	}
	return nil
}
func (DelimitedBoundary) OffWireEnum() {}

// DelimitedFragment lends exact original bytes, including malformed UTF-8.
// A valid rune never straddles fragments. Field counts from one within a record.
// Empty fields and trailing TABs remain observations, not an omitted collection.
type DelimitedFragment struct {
	Bytes    []byte
	Offset   SourceByteOffset
	Field    uint64
	Boundary DelimitedBoundary
}

func (f DelimitedFragment) Validate() error {
	if err := errors.Join(f.Offset.Validate(), f.Boundary.Validate()); err != nil {
		return err
	}
	if f.Field == 0 || bytes.ContainsAny(f.Bytes, "\t\n") || f.Boundary == DelimitedBoundaryMore && len(f.Bytes) == 0 {
		return core.ErrLineIOContract
	}
	return nil
}

// DelimitedReader owns fixed native input and output windows. It accumulates
// neither a field nor a record. Its caller owns source interruption and closure.
type DelimitedReader struct {
	guard      recordRangeReadGuard
	checked    characterSource
	reader     *bufio.Reader
	window     []byte
	offset     int64
	field      uint64
	terminal   error
	recordOpen bool
}

func NewDelimitedReader(request Request) (*DelimitedReader, error) {
	size, err := request.bufferSize()
	if err != nil {
		return nil, err
	}
	result := &DelimitedReader{field: 1}
	result.guard.source = request.Source
	result.checked.source = bufio.NewReaderSize(&result.guard, size)
	result.reader = bufio.NewReaderSize(&result.checked, size)
	result.window = make([]byte, result.reader.Size())
	return result, nil
}

func (r *DelimitedReader) Validate() error {
	if r == nil || r.reader == nil || len(r.window) < utf8.UTFMax || r.field == 0 {
		return core.ErrLineIOContract
	}
	return nil
}

// ReadFragment returns a bounded original-byte window through Go's ReadRune.
// A malformed byte is lent unchanged; decoding is used only to preserve valid
// rune boundaries for caller whitespace policy. Only bare io.EOF exhausts the
// source. Any mechanical refusal withdraws this fragment and remains terminal.
func (r *DelimitedReader) ReadFragment(ctx context.Context) (result DelimitedFragment, resultErr error) {
	defer func() {
		if resultErr != nil && resultErr != io.EOF {
			result = DelimitedFragment{}
			if ctx != nil {
				resultErr = errors.Join(resultErr, context.Cause(ctx))
			}
			if r != nil && r.reader != nil {
				r.terminal = resultErr
			}
		}
	}()
	if err := errors.Join(contextstate.Validate(ctx), r.Validate()); err != nil {
		return DelimitedFragment{}, err
	}
	if r.terminal != nil {
		return DelimitedFragment{}, r.terminal
	}
	r.checked.ctx = ctx
	start, field := r.offset, r.field
	data := r.window[:0]
	boundary := DelimitedBoundaryMore
	for len(data) <= cap(data)-utf8.UTFMax {
		value, width, err := r.reader.ReadRune()
		if r.guard.readErr != nil || r.checked.readErr != nil {
			return DelimitedFragment{}, errors.Join(core.ErrLineIOScan, err, r.guard.readErr, r.checked.readErr, contextstate.Validate(ctx))
		}
		if err != nil {
			if err != io.EOF {
				return DelimitedFragment{}, errors.Join(core.ErrLineIOScan, err)
			}
			r.terminal = io.EOF
			if !r.recordOpen {
				return DelimitedFragment{}, io.EOF
			}
			r.recordOpen = false
			boundary = DelimitedBoundaryEOF
			break
		}
		if width < 1 || int64(width) > math.MaxInt64-r.offset {
			return DelimitedFragment{}, core.ErrLineIOScan
		}
		r.offset += int64(width)
		if value == '\t' {
			r.recordOpen = true
			r.field++
			boundary = DelimitedBoundaryTab
			break
		}
		if value == '\n' {
			r.recordOpen = false
			r.field = 1
			boundary = DelimitedBoundaryLF
			break
		}
		r.recordOpen = true
		if value == utf8.RuneError && width == 1 {
			if err := r.reader.UnreadRune(); err != nil {
				return DelimitedFragment{}, err
			}
			original, err := r.reader.ReadByte()
			if err != nil {
				return DelimitedFragment{}, err
			}
			data = append(data, original)
		} else {
			data = utf8.AppendRune(data, value)
		}
	}
	if err := contextstate.Validate(ctx); err != nil {
		return DelimitedFragment{}, err
	}
	result = DelimitedFragment{Bytes: data, Offset: SourceByteOffset(start), Field: field, Boundary: boundary}
	return result, result.Validate()
}
