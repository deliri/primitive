package lineio_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/lineio"
)

// BenchmarkCharactersStreaming64KiB measures the per-rune publication cost of
// Characters over a fixed 64 KiB mixed ASCII and multibyte source with LF
// records. The rune and final-offset checks prove every character was decoded.
func BenchmarkCharactersStreaming64KiB(b *testing.B) {
	line := strings.Repeat("module example.com/app ", 3) + "ü€\n"
	source := strings.Repeat(line, (64<<10)/len(line))
	runes := utf8.RuneCountInString(source)
	if runes == 0 || runes == len(source) {
		b.Fatalf("fixture runes = %d over %d bytes, want a non-empty mixed-width source", runes, len(source))
	}
	reader := strings.NewReader(source)
	var count int
	var last lineio.Character
	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	for b.Loop() {
		reader.Reset(source)
		count = 0
		for character, err := range lineio.Characters(b.Context(), lineio.CharacterRequest{Source: reader}) {
			if err != nil {
				b.Fatalf("Characters() error = %v, want nil", err)
			}
			last = character
			count++
		}
	}
	if count != runes || int(last.Position.Offset) != len(source)-1 || last.Value != '\n' {
		b.Fatalf("characters = %d, last = %+v, want %d ending with LF at %d", count, last, runes, len(source)-1)
	}
}
