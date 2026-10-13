package lineio

import (
	"context"
	"iter"
	"unicode"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// RecordFields observes strings.Fields whitespace semantics through native
// ReadRune coordinates. It retains no field text or field population. Ranges
// name exact original bytes, including malformed UTF-8; source lifetime remains
// caller-owned. Every yielded range is provisional until clean source completion.
func RecordFields(ctx context.Context, request RecordFragmentRequest) iter.Seq2[RecordRange, error] {
	return func(yield func(RecordRange, error) bool) {
		start, end := int64(-1), int64(0)
		for character, err := range RecordCharacters(ctx, request) {
			if err != nil {
				yield(RecordRange{}, err)
				return
			}
			if unicode.IsSpace(rune(character.Value)) {
				if start < 0 {
					continue
				}
				length, err := core.NewByteLength(uint64(end - start))
				if err != nil {
					yield(RecordRange{}, err)
					return
				}
				field := RecordRange{Offset: SourceByteOffset(start), Bytes: length, Framing: RecordFramingOpaque}
				if err := field.Validate(); err != nil {
					yield(RecordRange{}, err)
					return
				}
				if !yield(field, nil) {
					return
				}
				start = -1
				continue
			}
			if start < 0 {
				start = int64(character.Offset)
			}
			width, err := character.Bytes.Int64()
			if err != nil {
				yield(RecordRange{}, err)
				return
			}
			end = int64(character.Offset) + width
		}
		if start < 0 {
			return
		}
		length, err := core.NewByteLength(uint64(end - start))
		if err != nil {
			yield(RecordRange{}, err)
			return
		}
		field := RecordRange{Offset: SourceByteOffset(start), Bytes: length, Framing: RecordFramingOpaque}
		if err := field.Validate(); err != nil {
			yield(RecordRange{}, err)
			return
		}
		if !yield(field, nil) {
			return
		}
		if err := contextstate.Validate(ctx); err != nil {
			yield(RecordRange{}, err)
		}
	}
}
