package lineio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestRecordCharactersConserveNativeBytePositionsAndWidths(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		"\n", "x", "x\n", "\r\n", "x\r", "\x00", "a\x00b\n", "\ufeffx\n", "x\ufeff\n", "λ\n", "界\n", "🦉\n", "λ界🦉\n", "e\u0301\n", "\u2028x\n", "\u0085\n", "\u00a0\n", "\u2003\n", "\u3000\n", "\xff\n", "\xc0\xaf\n", "\xed\xa0\x80\n", "\xf4\x90\x80\x80\n", "\xe2\x82", "\x7f\n",
	} {
		t.Run(strconv.Quote(body), func(t *testing.T) {
			t.Parallel()
			source := strings.NewReader("prefix\n" + body)
			if _, err := source.Seek(7, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			for record, err := range lineio.RecordRanges(t.Context(), lineio.RecordRangeRequest{Source: source, BufferBytes: recordRangeBuffer(t, 16)}) {
				if err != nil {
					t.Fatal(err)
				}
				checkRecordCharactersAgainstUTF8(t, source, record, []byte(body), 16)
			}
		})
	}
	for _, runeText := range []string{"x", "λ", "界", "🦉"} {
		for _, padding := range []int{15, 16, 17} {
			body := strings.Repeat("x", padding) + runeText + "\n"
			t.Run("decoder buffer boundary "+strconv.Itoa(padding)+runeText, func(t *testing.T) {
				t.Parallel()
				source := strings.NewReader(body)
				record := characterRecord(t, len(body), lineio.RecordFramingLF)
				checkRecordCharactersAgainstUTF8(t, source, record, []byte(body), 16)
			})
		}
	}
}

func characterRecord(t testing.TB, size int, framing lineio.RecordFraming) lineio.RecordRange {
	t.Helper()
	length, err := core.NewByteLength(uint64(size))
	if err != nil {
		t.Fatal(err)
	}
	return lineio.RecordRange{Bytes: length, Framing: framing}
}
func checkRecordCharactersAgainstUTF8(t *testing.T, source lineio.RecordSource, record lineio.RecordRange, body []byte, buffer uint64) {
	t.Helper()
	offset := 0
	for character, err := range lineio.RecordCharacters(t.Context(), lineio.RecordFragmentRequest{Source: source, Record: record, BufferBytes: recordRangeBuffer(t, buffer)}) {
		if err != nil {
			t.Fatal(err)
		}
		if offset >= len(body) {
			t.Fatal("extra character")
		}
		value, width := utf8.DecodeRune(body[offset:])
		actualWidth, widthErr := character.Bytes.Uint64()
		if widthErr != nil || character.Validate() != nil || rune(character.Value) != value || actualWidth != uint64(width) || uint64(character.Offset) != uint64(record.Offset)+uint64(offset) {
			t.Fatalf("character=%+v want rune%x width%d offset%d", character, value, width, offset)
		}
		offset += width
	}
	if offset != len(body) {
		t.Fatalf("consumed%d of%d source bytes", offset, len(body))
	}
}

func TestRecordCharactersPreserveReadCausesAndCancellation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                 string
		cause                error
		before, during, stop bool
	}{
		{name: "full-count source cause", cause: io.ErrUnexpectedEOF},
		{name: "wrapped EOF remains a cause", cause: errors.Join(core.ErrPrimitiveContract, io.EOF)},
		{name: "pre-canceled", before: true},
		{name: "cancel during positional read", during: true},
		{name: "read cause joins cancellation", during: true, cause: io.ErrUnexpectedEOF},
		{name: "consumer stops after first observation", stop: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := strings.Repeat("x", 8192) + "\n"
			source := &recordRangeSourcePressure{Reader: strings.NewReader(body), readAtErr: tc.cause}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.before {
				cancel()
			}
			if tc.during {
				source.cancel = cancel
			}
			observations, refusals := 0, 0
			for character, err := range lineio.RecordCharacters(ctx, lineio.RecordFragmentRequest{Source: source, Record: characterRecord(t, len(body), lineio.RecordFramingLF), BufferBytes: recordRangeBuffer(t, 16)}) {
				if err != nil {
					refusals++
					if character != (lineio.RecordCharacter{}) || tc.cause != nil && !errors.Is(err, tc.cause) || (tc.before || tc.during) && !errors.Is(err, context.Canceled) {
						t.Fatalf("refusal=%+v/%v", character, err)
					}
					continue
				}
				observations++
				if tc.stop {
					break
				}
			}
			if tc.stop {
				if observations != 1 || refusals != 0 || source.readAtCalls != 1 {
					t.Fatalf("stop=%d/%d/%d reads", observations, refusals, source.readAtCalls)
				}
			} else if observations != 0 || refusals != 1 {
				t.Fatalf("refusal events=%d/%d", observations, refusals)
			}
			if tc.before && source.readAtCalls != 0 {
				t.Fatal("canceled request read source")
			}
		})
	}
}

func TestRecordCharactersRefuseChangedPhysicalFraming(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		size         int
		framing      lineio.RecordFraming
	}{
		{"LF replaced by byte", "xx", 2, lineio.RecordFramingLF},
		{"EOF replaced by LF", "x\n", 2, lineio.RecordFramingEOF},
		{"early LF", "x\ny\n", 4, lineio.RecordFramingLF},
		{"premature source EOF", "x", 2, lineio.RecordFramingEOF},
		{"incomplete UTF8 is still one source byte", "\xe2", 2, lineio.RecordFramingEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var gotErr error
			for character, err := range lineio.RecordCharacters(t.Context(), lineio.RecordFragmentRequest{Source: strings.NewReader(tc.source), Record: characterRecord(t, tc.size, tc.framing), BufferBytes: recordRangeBuffer(t, 16)}) {
				if err != nil {
					gotErr = err
					if character != (lineio.RecordCharacter{}) {
						t.Fatal("refusal leaked character")
					}
				}
			}
			if !errors.Is(gotErr, core.ErrLineIOScan) {
				t.Fatalf("changed source admitted: %v", gotErr)
			}
		})
	}
}

func TestRecordCharactersWorkingMemoryDoesNotFollowRecordSize(t *testing.T) {
	// witness:waiver test/parallel/default -- process-wide allocation accounting.
	for _, size := range []int{4095, 4096, 4097, 1 << 20, 7 << 20} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			// witness:waiver test/parallel/default -- serial allocation counters.
			body := strings.Repeat("x", size) + "\n"
			source := strings.NewReader(body)
			request := lineio.RecordFragmentRequest{Source: source, Record: characterRecord(t, len(body), lineio.RecordFramingLF), BufferBytes: recordRangeBuffer(t, 4096)}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			count := 0
			for _, err := range lineio.RecordCharacters(t.Context(), request) {
				if err != nil {
					t.Fatal(err)
				}
				count++
			}
			runtime.ReadMemStats(&after)
			if count != len(body) {
				t.Fatalf("characters=%d want%d", count, len(body))
			}
			if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 128<<10 {
				t.Fatalf("record characters allocated%d bytes", allocated)
			}
		})
	}
}

func FuzzRecordCharactersMatchIndependentUTF8Bytes(f *testing.F) {
	for _, body := range [][]byte{[]byte("λ界🦉\nfinal"), {0, 0xff, '\n'}, []byte("\ufeff\u2003x\r\n"), {0xed, 0xa0, 0x80}, []byte(""), []byte("\n\n")} {
		f.Add(body)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		source := bytes.NewReader(data)
		for record, err := range lineio.RecordRanges(t.Context(), lineio.RecordRangeRequest{Source: source, BufferBytes: recordRangeBuffer(t, 17)}) {
			if err != nil {
				t.Fatal(err)
			}
			start := int(record.Offset)
			end := start + int(record.Bytes.Uint64())
			checkRecordCharactersAgainstUTF8(t, source, record, data[start:end], 16)
		}
	})
}

func TestRecordCharacterRefusesForgedCoordinatesAndWidths(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		offset lineio.SourceByteOffset
		value  lineio.RecordRune
		width  uint64
	}{
		{"negative rune", 0, -1, 1}, {"surrogate rune", 0, 0xd800, 3}, {"past Unicode range", 0, 0x110000, 4},
		{"ASCII wrong width", 0, 'x', 2}, {"two-byte rune wrong width", 0, 'λ', 1}, {"three-byte rune wrong width", 0, '界', 4}, {"four-byte rune wrong width", 0, '🦉', 3},
		{"replacement rune wrong width", 0, utf8.RuneError, 2}, {"byte position cannot represent its end", lineio.SourceByteOffset(math.MaxInt64), 'x', 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			character := lineio.RecordCharacter{Offset: tc.offset, Value: tc.value, Bytes: recordRangeBuffer(t, tc.width)}
			if character.Validate() == nil {
				t.Fatalf("forged character admitted: %+v", character)
			}
		})
	}
}
