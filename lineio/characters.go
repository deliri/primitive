package lineio

import (
	"bufio"
	"context"
	"errors"
	"io"
	"iter"
	"text/scanner"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// SourceLine and RuneColumn use the units of Go's text/scanner. Lines and
// columns start at one; Offset is independently measured in UTF-8 bytes.
type SourceLine uint64
type RuneColumn uint64
type CharacterValue rune

// SourceByteOffset is a zero-based byte coordinate in Go's signed size domain.
// Its zero value is the valid beginning of a source, unlike a positive count.
type SourceByteOffset uint64

func (v SourceByteOffset) Validate() error {
	_, err := core.CheckedInt64FromUint64(uint64(v))
	return err
}

func (v SourceLine) Validate() error {
	if v == 0 {
		return core.ErrLineIOContract
	}
	return nil
}
func (v RuneColumn) Validate() error {
	if v == 0 {
		return core.ErrLineIOContract
	}
	return nil
}
func (v CharacterValue) Validate() error {
	if v == 0 || !utf8.ValidRune(rune(v)) {
		return core.ErrLineIOContract
	}
	return nil
}

type CharacterPosition struct {
	Offset SourceByteOffset
	Line   SourceLine
	Column RuneColumn
}

func (p CharacterPosition) Validate() error {
	return errors.Join(p.Offset.Validate(), p.Line.Validate(), p.Column.Validate())
}

type Character struct {
	Position CharacterPosition
	Value    CharacterValue
}

func (c Character) Validate() error { return errors.Join(c.Position.Validate(), c.Value.Validate()) }

// CharacterRequest borrows an interruptible UTF-8 text source. Its lifetime
// remains with the caller. Go's scanner refuses NUL and malformed UTF-8 and
// ignores an initial BOM. There is no line, token, or file-size ceiling.
type CharacterRequest struct{ Source io.Reader }

func (r CharacterRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrLineIOContract
	}
	return nil
}

// Characters delegates decoding and positions to Go's bounded character
// reader. Next deliberately disables token-text collection: Scan and TokenText
// would retain arbitrarily long tokens and must not replace this operation.
// One character is published synchronously. A prefix is provisional until EOF.
func Characters(ctx context.Context, request CharacterRequest) iter.Seq2[Character, error] {
	return func(yield func(Character, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(Character{}, err)
			return
		}
		// bufio owns the standard no-progress refusal; scanner owns UTF-8.
		source := &characterSource{source: bufio.NewReader(checkedReader{source: request.Source}), ctx: ctx}
		var decoder scanner.Scanner
		decoder.Init(source)
		var decodeErr error
		decoder.Error = func(*scanner.Scanner, string) { decodeErr = errors.Join(core.ErrLineIOScan, source.readErr) }
		for {
			character, done, err := readCharacter(ctx, &decoder, &decodeErr)
			if err != nil {
				yield(Character{}, err)
				return
			}
			if done {
				return
			}
			if !yield(character, nil) {
				return
			}
		}
	}
}

func readCharacter(ctx context.Context, decoder *scanner.Scanner, decodeErr *error) (Character, bool, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return Character{}, false, err
	}
	value := decoder.Peek()
	if err := *decodeErr; err != nil {
		return Character{}, false, err
	}
	if value == scanner.EOF {
		return Character{}, true, nil
	}
	position, err := nativeCharacterPosition(decoder.Pos())
	if err != nil {
		return Character{}, false, err
	}
	value = decoder.Next()
	if err := errors.Join(contextstate.Validate(ctx), *decodeErr); err != nil {
		return Character{}, false, err
	}
	character := Character{Position: position, Value: CharacterValue(value)}
	return character, false, character.Validate()
}

func nativeCharacterPosition(p scanner.Position) (CharacterPosition, error) {
	if p.Offset < 0 || p.Line < 1 || p.Column < 1 {
		return CharacterPosition{}, core.ErrLineIOContract
	}
	position := CharacterPosition{Offset: SourceByteOffset(p.Offset), Line: SourceLine(p.Line), Column: RuneColumn(p.Column)}
	return position, position.Validate()
}

// Go's error callback supplies prose. Retaining the reader's actual failure
// independently preserves cancellation and provider error identities.
type characterSource struct {
	source  *bufio.Reader
	readErr error
	ctx     context.Context
}

func (s *characterSource) Read(destination []byte) (int, error) {
	if err := contextstate.Validate(s.ctx); err != nil {
		s.readErr = err
		return 0, err
	}
	if len(destination) == 0 {
		return 0, nil
	}
	// Read alone may return (0, nil); Peek delegates repeated empty reads to
	// Go's bounded fill routine, without advancing the source coordinate.
	if _, err := s.source.Peek(1); err != nil {
		if err != io.EOF {
			s.readErr = err
		}
		return 0, err
	}
	n, err := s.source.Read(destination)
	if err != nil && err != io.EOF {
		s.readErr = err
	}
	return n, err
}

var (
	_ io.Reader        = (*characterSource)(nil)
	_ core.Validatable = SourceByteOffset(0)
	_ core.Validatable = SourceLine(0)
	_ core.Validatable = RuneColumn(0)
	_ core.Validatable = CharacterValue(0)
	_ core.Validatable = CharacterRequest{}
	_ core.Validatable = CharacterPosition{}
	_ core.Validatable = Character{}
)
