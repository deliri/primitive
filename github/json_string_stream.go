package github

import (
	"bufio"
	"errors"
	"io"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// jsonStringStream decodes one JSON string incrementally. The opening quote
// has already been consumed. Custody is one UTF-8 rune, independent of length.
type jsonStringStream struct {
	observe    func(rune) error
	source     *bufio.Reader
	pending    [utf8.UTFMax]byte
	begin, end int
	terminal   error
}

func (s *jsonStringStream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	written := 0
	for written < len(p) {
		if s.begin < s.end {
			n := copy(p[written:], s.pending[s.begin:s.end])
			s.begin += n
			written += n
			continue
		}
		if s.terminal != nil {
			return written, s.terminal
		}
		value, err := readJSONStringRune(s.source)
		if err != nil {
			s.terminal = err
			return written, err
		}
		if s.observe != nil {
			if err := s.observe(value); err != nil {
				s.terminal = err
				return written, err
			}
		}
		s.begin = 0
		s.end = utf8.EncodeRune(s.pending[:], value)
	}
	return written, nil
}
func readJSONStringRune(source *bufio.Reader) (rune, error) {
	first, err := source.ReadByte()
	if err != nil {
		return 0, jsonStreamError(err)
	}
	switch {
	case first == '"':
		return 0, io.EOF
	case first == '\\':
		return readJSONEscape(source)
	case first < 0x20:
		return 0, core.ErrGitHubResponse
	case first < utf8.RuneSelf:
		return rune(first), nil
	default:
		if err := source.UnreadByte(); err != nil {
			return 0, jsonStreamError(err)
		}
		value, width, runeErr := source.ReadRune()
		if runeErr != nil {
			return 0, jsonStreamError(runeErr)
		}
		if value == utf8.RuneError && width == 1 {
			return 0, core.ErrGitHubResponse
		}
		return value, nil
	}
}
func readJSONEscape(source *bufio.Reader) (rune, error) {
	value, err := source.ReadByte()
	if err != nil {
		return 0, jsonStreamError(err)
	}
	switch value {
	case '"', '\\', '/':
		return rune(value), nil
	case 'b':
		return '\b', nil
	case 'f':
		return '\f', nil
	case 'n':
		return '\n', nil
	case 'r':
		return '\r', nil
	case 't':
		return '\t', nil
	case 'u':
		first, hexErr := readJSONHexRune(source)
		if hexErr != nil {
			return 0, hexErr
		}
		if first >= 0xdc00 && first <= 0xdfff {
			return 0, core.ErrGitHubResponse
		}
		if first < 0xd800 || first > 0xdbff {
			return first, nil
		}
		slash, slashErr := source.ReadByte()
		if slashErr != nil {
			return 0, jsonStreamError(slashErr)
		}
		if slash != '\\' {
			return 0, core.ErrGitHubResponse
		}
		marker, markerErr := source.ReadByte()
		if markerErr != nil {
			return 0, jsonStreamError(markerErr)
		}
		if marker != 'u' {
			return 0, core.ErrGitHubResponse
		}
		second, secondErr := readJSONHexRune(source)
		if secondErr != nil {
			return 0, secondErr
		}
		if second < 0xdc00 || second > 0xdfff {
			return 0, core.ErrGitHubResponse
		}
		return utf16.DecodeRune(first, second), nil
	default:
		return 0, core.ErrGitHubResponse
	}
}
func readJSONHexRune(source *bufio.Reader) (rune, error) {
	var value rune
	for range 4 {
		digit, err := source.ReadByte()
		if err != nil {
			return 0, jsonStreamError(err)
		}
		value *= 16
		switch {
		case digit >= '0' && digit <= '9':
			value += rune(digit - '0')
		case digit >= 'a' && digit <= 'f':
			value += rune(digit - 'a' + 10)
		case digit >= 'A' && digit <= 'F':
			value += rune(digit - 'A' + 10)
		default:
			return 0, core.ErrGitHubResponse
		}
	}
	return value, nil
}

// EOF is reserved for a consumed closing delimiter. Truncation remains typed.
func jsonStreamError(err error) error {
	if errors.Is(err, io.EOF) {
		err = io.ErrUnexpectedEOF
	}
	return responseError(err)
}
