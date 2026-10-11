package lineio

import (
	"bufio"
	"context"
	"errors"
	"io"
	"iter"
	"math"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// StreamCharacters lends each native ReadRune observation from a forward-only
// stream. Offsets start at zero; NUL, BOM, and malformed UTF-8 remain data.
// BufferBytes selects fixed working memory, never a line or stream quota.
// Published prefixes remain provisional until clean EOF. Source errors and
// cancellation refuse the next observation without manufacturing a character.
func StreamCharacters(ctx context.Context, request Request) iter.Seq2[RecordCharacter, error] {
	return func(yield func(RecordCharacter, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(RecordCharacter{}, err)
			return
		}
		size, err := request.bufferSize()
		if err != nil {
			yield(RecordCharacter{}, err)
			return
		}
		reader := bufio.NewReaderSize(checkedReader{source: request.Source}, size)
		var offset int64
		for {
			if err := contextstate.Validate(ctx); err != nil {
				yield(RecordCharacter{}, err)
				return
			}
			value, width, err := reader.ReadRune()
			if cancelErr := contextstate.Validate(ctx); cancelErr != nil {
				yield(RecordCharacter{}, errors.Join(cancelErr, err))
				return
			}
			if err != nil {
				// witness:waiver doctrine/error/sentinel_compare -- Native ReadRune's exact io.EOF marks clean stream exhaustion; wrapped EOF is a source refusal.
				if err == io.EOF {
					return
				}
				yield(RecordCharacter{}, errors.Join(core.ErrLineIOScan, err))
				return
			}
			if width < 1 || int64(width) > math.MaxInt64-offset {
				yield(RecordCharacter{}, core.ErrLineIOScan)
				return
			}
			extent, err := core.NewByteCount(uint64(width))
			if err != nil {
				yield(RecordCharacter{}, err)
				return
			}
			character := RecordCharacter{Offset: SourceByteOffset(offset), Value: RecordRune(value), Bytes: extent}
			if err := character.Validate(); err != nil {
				yield(RecordCharacter{}, err)
				return
			}
			if !yield(character, nil) {
				return
			}
			offset += int64(width)
		}
	}
}
