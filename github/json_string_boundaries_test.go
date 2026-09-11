package github

import (
	"bufio"
	"bytes"
	json "encoding/json/v2"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"testing"
)

func TestJSONStringStreamingEscapesAndUnicodeBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, want, rewrite string }{
		{name: "empty string"},
		{name: "escaped quotation", want: "\""},
		{name: "escaped reverse solidus", want: "\\"},
		{name: "escaped solidus", want: "/", rewrite: `"\/"`},
		{name: "escaped backspace", want: "\b"},
		{name: "escaped form feed", want: "\f"},
		{name: "escaped newline", want: "\n"},
		{name: "escaped carriage return", want: "\r"},
		{name: "escaped tab", want: "\t"},
		{name: "escaped NUL is a JSON string fact", want: "\x00"},
		{name: "last ASCII codepoint", want: "\u007f"},
		{name: "first two-byte codepoint", want: "\u0080"},
		{name: "last two-byte codepoint", want: "\u07ff"},
		{name: "first three-byte codepoint", want: "\u0800"},
		{name: "last codepoint before surrogate domain", want: "\ud7ff"},
		{name: "first codepoint after surrogate domain", want: "\ue000"},
		{name: "last three-byte codepoint", want: "\uffff"},
		{name: "first four-byte codepoint", want: "\U00010000"},
		{name: "maximum Unicode codepoint", want: "\U0010ffff"},
		{name: "minimum surrogate pair", want: "\U00010000", rewrite: `"\ud800\udc00"`},
		{name: "maximum surrogate pair", want: "\U0010ffff", rewrite: `"\udbff\udfff"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			wire, err := json.Marshal(tc.want)
			if err != nil {
				t.Fatal(err)
			}
			if tc.rewrite != "" {
				if bytes.Equal(wire, []byte(tc.rewrite)) {
					t.Fatal("representation mutation changed=false, want true")
				}
				wire = []byte(tc.rewrite)
			}
			decoder := treeDecoder{source: bufio.NewReader(bytes.NewReader(wire))}
			if err := decoder.expect('"'); err != nil {
				t.Fatal(err)
			}
			stream := jsonStringStream{source: decoder.source}
			var got bytes.Buffer
			var scratch [1]byte // Every multibyte rune must survive caller read fragmentation.
			for {
				n, err := stream.Read(scratch[:])
				if _, writeErr := got.Write(scratch[:n]); writeErr != nil {
					t.Fatal(writeErr)
				}
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if got.String() != tc.want {
				t.Fatalf("decoded=%q, want %q", got.String(), tc.want)
			}
		})
	}
}
func TestJSONStringStreamingRejectsMalformedRepresentations(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		wire []byte
	}{
		{name: "raw control byte", wire: []byte{'"', 1, '"'}},
		{name: "lone UTF8 continuation", wire: []byte{'"', 0x80, '"'}},
		{name: "overlong UTF8 encoding", wire: []byte{'"', 0xc0, 0xaf, '"'}},
		{name: "truncated UTF8 encoding", wire: []byte{'"', 0xe2, 0x82}},
		{name: "UTF8 encoded surrogate", wire: []byte{'"', 0xed, 0xa0, 0x80, '"'}},
		{name: "UTF8 above Unicode maximum", wire: []byte{'"', 0xf4, 0x90, 0x80, 0x80, '"'}},
		{name: "missing closing quote", wire: []byte(`"text`)},
		{name: "unfinished escape", wire: []byte(`"\`)},
		{name: "unknown escape arm", wire: []byte(`"\x"`)},
		{name: "nonhex escape digit", wire: []byte(`"\u00xz"`)},
		{name: "incomplete hex escape", wire: []byte(`"\u001`)},
		{name: "unpaired low surrogate", wire: []byte(`"\udc00"`)},
		{name: "unpaired high surrogate", wire: []byte(`"\ud800"`)},
		{name: "high surrogate followed by scalar", wire: []byte(`"\ud800\ud7ff"`)},
		{name: "high surrogate followed by another high", wire: []byte(`"\ud800\udbff"`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := bufio.NewReader(bytes.NewReader(tc.wire))
			if _, err := source.ReadByte(); err != nil {
				t.Fatal(err)
			}
			stream := jsonStringStream{source: source}
			if _, err := io.Copy(io.Discard, &stream); !errors.Is(err, core.ErrGitHubResponse) {
				t.Fatalf("malformed string error=%v, want typed refusal", err)
			}
		})
	}
}
