package lineio

import (
	"bufio"
	"context"
	"errors"
	"io"
	"iter"
	"math"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// RecordRune is the rune returned by Go's ReadRune, including NUL and RuneError.
// A malformed UTF-8 byte is RuneError with width one, as defined by Go.
type RecordRune rune

func (r RecordRune) Validate() error {
	if !utf8.ValidRune(rune(r)) {
		return core.ErrLineIOContract
	}
	return nil
}

// RecordCharacter preserves a character's exact source byte position and width.
// It contains no token text, reader handle, or copied source payload.
type RecordCharacter struct {
	Offset SourceByteOffset
	Value  RecordRune
	Bytes  core.ByteCount
}

func (c RecordCharacter) Validate() error {
	if err := errors.Join(c.Offset.Validate(), c.Value.Validate(), c.Bytes.Validate()); err != nil {
		return err
	}
	width, _ := c.Bytes.Uint64()
	if width > utf8.UTFMax || int64(c.Offset) > math.MaxInt64-int64(width) {
		return core.ErrLineIOContract
	}
	if c.Value == RecordRune(utf8.RuneError) {
		if width != 1 && width != 3 {
			return core.ErrLineIOContract
		}
	} else if width != uint64(utf8.RuneLen(rune(c.Value))) {
		return core.ErrLineIOContract
	}
	return nil
}

// RecordCharacters replays one physical range through Go's bounded ReadRune.
// Unicode, NUL, malformed bytes, and BOM remain observations for caller policy.
// The declared framing must still agree with the source. No input quota applies.
func RecordCharacters(ctx context.Context, request RecordFragmentRequest) iter.Seq2[RecordCharacter, error] {
	return func(yield func(RecordCharacter, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(RecordCharacter{}, err)
			return
		}
		size, err := request.Record.Bytes.Int64()
		if err != nil {
			yield(RecordCharacter{}, err)
			return
		}
		source := recordRangeReadGuard{source: io.NewSectionReader(request.Source, int64(request.Record.Offset), size)}
		bufferSize, err := (Request{Source: &source, BufferBytes: request.BufferBytes}).bufferSize()
		if err != nil {
			yield(RecordCharacter{}, err)
			return
		}
		reader := bufio.NewReaderSize(&source, bufferSize)
		var observed int64
		for {
			if err := contextstate.Validate(ctx); err != nil {
				yield(RecordCharacter{}, err)
				return
			}
			value, width, err := reader.ReadRune()
			if source.readErr != nil {
				yield(RecordCharacter{}, errors.Join(core.ErrLineIOScan, err, source.readErr, contextstate.Validate(ctx)))
				return
			}
			if ctxErr := contextstate.Validate(ctx); ctxErr != nil {
				yield(RecordCharacter{}, errors.Join(ctxErr, err))
				return
			}
			if err != nil {
				if err == io.EOF && observed == size {
					return
				}
				yield(RecordCharacter{}, errors.Join(core.ErrLineIOScan, err, io.ErrUnexpectedEOF))
				return
			}
			if int64(width) > size-observed {
				yield(RecordCharacter{}, core.ErrLineIOScan)
				return
			}
			end := observed + int64(width)
			if value == '\n' && (end != size || request.Record.Framing != RecordFramingLF) || end == size && request.Record.Framing == RecordFramingLF && value != '\n' {
				yield(RecordCharacter{}, core.ErrLineIOScan)
				return
			}
			count, err := core.NewByteCount(uint64(width))
			if err != nil {
				yield(RecordCharacter{}, err)
				return
			}
			character := RecordCharacter{Offset: SourceByteOffset(int64(request.Record.Offset) + observed), Value: RecordRune(value), Bytes: count}
			if err := character.Validate(); err != nil {
				yield(RecordCharacter{}, err)
				return
			}
			if !yield(character, nil) {
				return
			}
			observed = end
		}
	}
}

var _ core.Validatable = RecordCharacter{}
var _ core.Validatable = RecordRune(0)
