package core_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestDecodeCanonicalHexAdmitsOnlyTheOneCanonicalSpelling pins the repository's
// single hex admission rule. Seven packages used to restate the
// decode-and-re-encode comparison; this table is the shared proof they now
// delegate to, so it pressures every way a spelling can be almost right:
// case, extent, prefixes, separators, and characters that only look like hex.
func TestDecodeCanonicalHexAdmitsOnlyTheOneCanonicalSpelling(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		value           string
		wantBytes       []byte
		destinationSize int
		wantErr         bool
	}{
		{name: "one byte lowercase is canonical", destinationSize: 1, value: "0a", wantBytes: []byte{0x0a}},
		{name: "all zero bytes are a legal spelling", destinationSize: 2, value: "0000", wantBytes: []byte{0, 0}},
		{name: "the full byte alphabet round trips", destinationSize: 8, value: "0123456789abcdef", wantBytes: []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}},
		{name: "an empty destination admits only the empty text", destinationSize: 0, value: ""},
		{name: "a thirty two byte digest width decodes exactly", destinationSize: 32, value: strings.Repeat("ff", 32), wantBytes: bytes.Repeat([]byte{0xff}, 32)},
		{name: "uppercase is not canonical", destinationSize: 1, value: "0A", wantErr: true},
		{name: "mixed case is not canonical", destinationSize: 2, value: "aBcd", wantErr: true},
		{name: "text one byte short is the wrong extent", destinationSize: 2, value: "ab", wantErr: true},
		{name: "text one byte long is the wrong extent", destinationSize: 1, value: "abcd", wantErr: true},
		{name: "odd length text can never be byte exact", destinationSize: 1, value: "abc", wantErr: true},
		{name: "empty text cannot fill a nonempty destination", destinationSize: 1, value: "", wantErr: true},
		{name: "a 0x prefix is a different language", destinationSize: 2, value: "0xab", wantErr: true},
		{name: "letters beyond f only look like hex", destinationSize: 1, value: "zz", wantErr: true},
		{name: "interior whitespace is refused", destinationSize: 2, value: "ab cd", wantErr: true},
		{name: "unicode digits that render like hex are refused", destinationSize: 2, value: "abсd", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			destination := make([]byte, tc.destinationSize)
			err := core.DecodeCanonicalHex(destination, tc.value)
			if tc.wantErr {
				if !errors.Is(err, core.ErrPrimitiveContract) {
					t.Fatalf("DecodeCanonicalHex(%q) error = %v, want errors.Is %v", tc.value, err, core.ErrPrimitiveContract)
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeCanonicalHex(%q) error = %v, want nil", tc.value, err)
			}
			if !bytes.Equal(destination, tc.wantBytes) && tc.destinationSize != 0 {
				t.Fatalf("DecodeCanonicalHex(%q) = %x, want %x", tc.value, destination, tc.wantBytes)
			}
		})
	}
}
