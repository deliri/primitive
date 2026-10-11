package lineio_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestStreamCharactersMatchesNativeUTF8AcrossFixedWindows(t *testing.T) {
	t.Parallel()
	for _, window := range []uint64{1, 2, 3, 4, 15, 16, 4095, 4096} {
		for mode, text := range []string{"", "plain\n", "\x00\xff\xfe\n", "\ufeffstart\ufeffend", "\u2003{}\u2003\n", "\xf0\x9f\x92", "\r\n\n", strings.Repeat("界", 65537) + "\n"} {
			t.Run(fmt.Sprintf("buffer_%d_encoding_%d", window, mode), func(t *testing.T) {
				t.Parallel()
				size, err := core.NewByteCount(window)
				if err != nil {
					t.Fatal(err)
				}
				data := []byte(text)
				observed := 0
				for character, err := range lineio.StreamCharacters(t.Context(), lineio.Request{Source: bytes.NewReader(data), BufferBytes: size}) {
					if err != nil {
						t.Fatal(err)
					}
					value, width := utf8.DecodeRune(data[observed:])
					extent, err := character.Bytes.Uint64()
					if err != nil || uint64(character.Offset) != uint64(observed) || rune(character.Value) != value || extent != uint64(width) {
						t.Fatalf("character=%+v want offset=%d value=%U width=%d", character, observed, value, width)
					}
					observed += width
				}
				if observed != len(data) {
					t.Fatalf("observed=%d want=%d", observed, len(data))
				}
			})
		}
	}
}

func TestStreamCharactersRefusalsAndConsumerBackpressure(t *testing.T) {
	t.Parallel()
	size, err := core.NewByteCount(16)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cases := []struct {
		name   string
		ctx    context.Context
		source io.Reader
		want   error
		prefix int
	}{
		{"nil context", nil, strings.NewReader("x"), core.ErrNilContext, 0},
		{"canceled ingress", ctx, strings.NewReader("x"), context.Canceled, 0},
		{"nil source", t.Context(), nil, core.ErrLineIOContract, 0},
		{"native reader refusal", t.Context(), iotest.ErrReader(io.ErrUnexpectedEOF), io.ErrUnexpectedEOF, 0},
		{"prefix before native refusal", t.Context(), io.MultiReader(strings.NewReader("a界"), iotest.ErrReader(io.ErrUnexpectedEOF)), io.ErrUnexpectedEOF, 2},
		{"wrapped EOF remains refusal", t.Context(), iotest.ErrReader(fmt.Errorf("native EOF: %w", io.EOF)), core.ErrLineIOScan, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			observed := 0
			var refusal error
			for _, err := range lineio.StreamCharacters(tc.ctx, lineio.Request{Source: tc.source, BufferBytes: size}) {
				if err != nil {
					refusal = err
					break
				}
				observed++
			}
			if observed != tc.prefix || !errors.Is(refusal, tc.want) {
				t.Fatalf("observed=%d refusal=%v want=%d/%v", observed, refusal, tc.prefix, tc.want)
			}
		})
	}
	t.Run("consumer stop leaves unread tail", func(t *testing.T) {
		t.Parallel()
		source := strings.NewReader(strings.Repeat("x", 8192))
		count := 0
		for _, err := range lineio.StreamCharacters(t.Context(), lineio.Request{Source: source, BufferBytes: size}) {
			if err != nil {
				t.Fatal(err)
			}
			count++
			break
		}
		if count != 1 || source.Len() < 8192-16 {
			t.Fatalf("count=%d unread=%d want 1 and native fixed-buffer tail", count, source.Len())
		}
	})
}

func FuzzStreamCharactersConservesNativeByteCoordinates(f *testing.F) {
	f.Add([]byte{0, 255, 239, 187, 191, 10}, uint16(1))
	f.Add([]byte("界\r\n"), uint16(4095))
	f.Fuzz(func(t *testing.T, data []byte, window uint16) {
		size, err := core.NewByteCount(uint64(window) + 1)
		if err != nil {
			t.Fatal(err)
		}
		observed := 0
		for character, err := range lineio.StreamCharacters(t.Context(), lineio.Request{Source: bytes.NewReader(data), BufferBytes: size}) {
			if err != nil {
				t.Fatal(err)
			}
			if observed >= len(data) {
				t.Fatal("extra character")
			}
			value, width := utf8.DecodeRune(data[observed:])
			extent, err := character.Bytes.Uint64()
			if err != nil || uint64(character.Offset) != uint64(observed) || rune(character.Value) != value || extent != uint64(width) {
				t.Fatalf("native observation=%+v at %d", character, observed)
			}
			observed += width
		}
		if observed != len(data) {
			t.Fatalf("extent=%d want=%d", observed, len(data))
		}
	})
}
