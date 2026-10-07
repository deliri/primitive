package lineio_test

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/lineio"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestCharactersPreserveRuneAndByteCoordinates(t *testing.T) {
	t.Parallel()
	want := []struct {
		value        rune
		offset       uint64
		line, column uint64
	}{
		{'a', 0, 1, 1}, {'é', 1, 1, 2}, {'\n', 3, 1, 3}, {'\t', 4, 2, 1}, {'界', 5, 2, 2},
	}
	index := 0
	for character, err := range lineio.Characters(t.Context(), lineio.CharacterRequest{Source: strings.NewReader("aé\n\t界")}) {
		if err != nil {
			t.Fatalf("Characters: got %v, want nil", err)
		}
		if index >= len(want) {
			t.Fatalf("character count: got more than %d, want %d", len(want), len(want))
		}
		offset := uint64(character.Position.Offset)
		expected := want[index]
		if rune(character.Value) != expected.value || offset != expected.offset || uint64(character.Position.Line) != expected.line || uint64(character.Position.Column) != expected.column {
			t.Fatalf("character %d: got %+v, want %+v", index, character, expected)
		}
		index++
	}
	if index != len(want) {
		t.Fatalf("character count: got %d, want %d", index, len(want))
	}
}

func TestCharactersIngressAndReaderFailures(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	var absent *strings.Reader
	cases := []struct {
		name   string
		ctx    context.Context
		source io.Reader
		want   error
	}{
		{"nil_context", nil, strings.NewReader("a"), core.ErrNilContext},
		{"canceled", canceled, strings.NewReader("a"), context.Canceled},
		{"nil_reader", t.Context(), nil, core.ErrLineIOContract},
		{"typed_nil_reader", t.Context(), absent, core.ErrLineIOContract},
		{"malformed_utf8", t.Context(), strings.NewReader("a\xff"), core.ErrLineIOScan},
		{"nul", t.Context(), strings.NewReader("a\x00"), core.ErrLineIOScan},
		{"provider_error", t.Context(), &terminalReader{Reader: strings.NewReader("a"), cause: hostileReaderError{}}, hostileReaderError{}},
		{"provider_canceled", t.Context(), &terminalReader{Reader: strings.NewReader("a"), cause: context.Canceled}, context.Canceled},
		{"wrapped_eof_is_failure", t.Context(), &terminalReader{Reader: strings.NewReader("a"), cause: errors.Join(io.EOF)}, io.EOF},
		{"negative_count", t.Context(), invalidCountReader(-1), bufio.ErrBadReadCount},
		{"excess_count", t.Context(), invalidCountReader(1), bufio.ErrBadReadCount},
		{"stalled_source", t.Context(), &stalledReader{source: strings.NewReader("a"), remaining: 200}, io.ErrNoProgress},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var failure error
			for _, err := range lineio.Characters(tc.ctx, lineio.CharacterRequest{Source: tc.source}) {
				if err != nil {
					failure = err
				}
			}
			if !errors.Is(failure, tc.want) {
				t.Fatalf("Characters: got %v, want %v", failure, tc.want)
			}
		})
	}
}

func TestCharactersConsumerOwnsStopAndReaderLifetime(t *testing.T) {
	t.Parallel()
	source := &observedReader{Reader: io.LimitReader(fillReader('a'), 64<<20)}
	for character, err := range lineio.Characters(t.Context(), lineio.CharacterRequest{Source: source}) {
		if err != nil || character.Value != 'a' {
			t.Fatalf("first character: got (%v, %v), want (a, nil)", character, err)
		}
		break
	}
	reads := source.calls
	if source.bytes > 4096 || source.closed {
		t.Fatalf("reader ownership: got bytes=%d closed=%v, want bounded read ahead and borrowed lifetime", source.bytes, source.closed)
	}
	if reads == 0 {
		t.Fatal("reader calls: got zero, want production source read")
	}
}

func TestCharacterContractsOwnZeroAndNativeExtent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		value core.Validatable
		want  error
	}{
		{"origin_is_valid", lineio.SourceByteOffset(0), nil},
		{"native_last_offset", lineio.SourceByteOffset(math.MaxInt64), nil},
		{"offset_overflow", lineio.SourceByteOffset(math.MaxUint64), core.ErrNumericOverflow},
		{"zero_line", lineio.SourceLine(0), core.ErrLineIOContract},
		{"zero_column", lineio.RuneColumn(0), core.ErrLineIOContract},
		{"nul", lineio.CharacterValue(0), core.ErrLineIOContract},
		{"surrogate", lineio.CharacterValue(0xd800), core.ErrLineIOContract},
		{"negative_rune", lineio.CharacterValue(-1), core.ErrLineIOContract},
		{"replacement_rune_is_valid", lineio.CharacterValue(utf8.RuneError), nil},
		{"zero_character", lineio.Character{}, core.ErrLineIOContract},
		{"zero_position", lineio.CharacterPosition{}, core.ErrLineIOContract},
		{"first_character", lineio.Character{Position: lineio.CharacterPosition{Line: 1, Column: 1}, Value: 'a'}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.value.Validate()
			if !errors.Is(got, tc.want) {
				t.Fatalf("Validate: got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCharactersCancellationAfterPublicationRefusesFurtherRead(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	source := &observedReader{Reader: io.LimitReader(fillReader('a'), 64<<20)}
	observed, reads := 0, 0
	var failure error
	for _, err := range lineio.Characters(ctx, lineio.CharacterRequest{Source: source}) {
		if err != nil {
			failure = err
			break
		}
		observed++
		reads = source.calls
		cancel()
	}
	if observed != 1 || source.calls != reads || !errors.Is(failure, context.Canceled) {
		t.Fatalf("cancellation: got count=%d reads=%d error=%v, want count=1 reads=%d error=%v", observed, source.calls, failure, reads, context.Canceled)
	}
}

func TestCharactersRetainedMemoryDoesNotGrowWithSource(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	const allowance = 256 << 10
	for _, size := range [...]int64{256 << 10, 1 << 20, 64 << 20} {
		before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		var observed int64
		var retained int64
		for character, err := range lineio.Characters(t.Context(), lineio.CharacterRequest{Source: io.LimitReader(fillReader('a'), size)}) {
			if err != nil {
				t.Fatal(err)
			}
			if character.Value != 'a' {
				t.Fatalf("character: got %v, want a", character.Value)
			}
			observed++
			if observed == size {
				after, err := hostfacts.ObserveCollectedGoHeap(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				retained = int64(after.Uint64()) - int64(before.Uint64())
			}
		}
		t.Logf("source_bytes=%d observed=%d retained_bytes=%d allowance=%d", size, observed, retained, allowance)
		if observed != size || retained > allowance {
			t.Fatalf("stream retention: got count=%d bytes=%d, want count=%d bytes<=%d", observed, retained, size, allowance)
		}
	}
}

func FuzzCharactersMatchIndependentUTF8Oracle(f *testing.F) {
	for _, seed := range []string{"", "aé\n\t界", "\xef\xbb\xbfx", "x\xef\xbb\xbf", "a\xff", "a\x00", "\r\n", "\xf0\x9f\x98\x80"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		valid := utf8.Valid(input) && bytes.IndexByte(input, 0) < 0
		offset, line, column := 0, uint64(1), uint64(1)
		if bytes.HasPrefix(input, []byte("\xef\xbb\xbf")) {
			offset, column = 3, 2
		}
		var failure error
		for character, err := range lineio.Characters(t.Context(), lineio.CharacterRequest{Source: bytes.NewReader(input)}) {
			if err != nil {
				failure = err
				break
			}
			if offset >= len(input) {
				t.Fatal("character: got extra value, want EOF")
			}
			expected, width := utf8.DecodeRune(input[offset:])
			actualOffset := uint64(character.Position.Offset)
			if rune(character.Value) != expected || actualOffset != uint64(offset) || uint64(character.Position.Line) != line || uint64(character.Position.Column) != column {
				t.Fatalf("character: got %+v, want rune=%U offset=%d line=%d column=%d", character, expected, offset, line, column)
			}
			offset += width
			if expected == '\n' {
				line++
				column = 1
			} else {
				column++
			}
		}
		if valid {
			if failure != nil || offset != len(input) {
				t.Fatalf("valid text: got error=%v offset=%d, want nil and %d", failure, offset, len(input))
			}
			return
		}
		if !errors.Is(failure, core.ErrLineIOScan) {
			t.Fatalf("invalid text: got %v, want %v", failure, core.ErrLineIOScan)
		}
	})
}
