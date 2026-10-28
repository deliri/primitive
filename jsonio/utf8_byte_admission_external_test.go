package jsonio_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

type utf8AdmissionClass uint8

const (
	utf8AdmissionValid utf8AdmissionClass = iota + 1
	utf8AdmissionMalformedUTF8
	utf8AdmissionMalformedJSON
)

func TestValidateBytesUTF8ClassificationLayerTriadHostileTable(t *testing.T) {
	t.Parallel()
	// Each row crosses a native encoding boundary, continuation position or JSON refusal; none is an extent quota.
	valid := []struct {
		name string
		data []byte
	}{
		{"empty string", []byte(`""`)}, {"ASCII maximum", []byte{'"', 0x7f, '"'}},
		{"two-byte minimum", []byte{'"', 0xc2, 0x80, '"'}}, {"two-byte maximum", []byte{'"', 0xdf, 0xbf, '"'}},
		{"three-byte minimum", []byte{'"', 0xe0, 0xa0, 0x80, '"'}}, {"before surrogate range", []byte{'"', 0xed, 0x9f, 0xbf, '"'}},
		{"after surrogate range", []byte{'"', 0xee, 0x80, 0x80, '"'}}, {"three-byte maximum", []byte{'"', 0xef, 0xbf, 0xbf, '"'}},
		{"four-byte minimum", []byte{'"', 0xf0, 0x90, 0x80, 0x80, '"'}}, {"four-byte maximum", []byte{'"', 0xf4, 0x8f, 0xbf, 0xbf, '"'}},
		{"literal replacement rune", []byte{'"', 0xef, 0xbf, 0xbd, '"'}}, {"escaped control", []byte(`"\u0000"`)},
		{"null", []byte(`null`)}, {"whitespace around object", []byte(" \n{}\t")},
	}
	invalid := []struct {
		name string
		data []byte
	}{
		{"standalone continuation minimum", []byte{0x80}}, {"standalone continuation maximum", []byte{0xbf}},
		{"reserved leading C0", []byte{0xc0, 0x80}}, {"reserved leading C1", []byte{0xc1, 0xbf}},
		{"reserved leading F5", []byte{0xf5, 0x80, 0x80, 0x80}}, {"reserved leading FF", []byte{0xff}},
		{"two-byte missing continuation", []byte{0xc2}},
		{"three-byte missing both continuations", []byte{0xe1}}, {"three-byte missing final continuation", []byte{0xe1, 0x80}},
		{"four-byte missing three continuations", []byte{0xf1}}, {"four-byte missing two continuations", []byte{0xf1, 0x80}}, {"four-byte missing final continuation", []byte{0xf1, 0x80, 0x80}},
		{"two-byte continuation below range", []byte{0xc2, 0x7f}}, {"two-byte continuation above range", []byte{0xc2, 0xc0}},
		{"three-byte first continuation below range", []byte{0xe1, 0x7f, 0x80}}, {"three-byte first continuation above range", []byte{0xe1, 0xc0, 0x80}},
		{"three-byte final continuation below range", []byte{0xe1, 0x80, 0x7f}}, {"three-byte final continuation above range", []byte{0xe1, 0x80, 0xc0}},
		{"four-byte first continuation below range", []byte{0xf1, 0x7f, 0x80, 0x80}}, {"four-byte first continuation above range", []byte{0xf1, 0xc0, 0x80, 0x80}},
		{"four-byte middle continuation below range", []byte{0xf1, 0x80, 0x7f, 0x80}}, {"four-byte middle continuation above range", []byte{0xf1, 0x80, 0xc0, 0x80}},
		{"four-byte final continuation below range", []byte{0xf1, 0x80, 0x80, 0x7f}}, {"four-byte final continuation above range", []byte{0xf1, 0x80, 0x80, 0xc0}},
		{"three-byte overlong maximum", []byte{0xe0, 0x9f, 0xbf}}, {"four-byte overlong maximum", []byte{0xf0, 0x8f, 0xbf, 0xbf}},
		{"surrogate minimum", []byte{0xed, 0xa0, 0x80}}, {"surrogate maximum", []byte{0xed, 0xbf, 0xbf}},
		{"beyond Unicode maximum minimum", []byte{0xf4, 0x90, 0x80, 0x80}}, {"beyond Unicode maximum maximum", []byte{0xf4, 0xbf, 0xbf, 0xbf}},
		{"invalid byte after complete value", []byte{'"', ' ', '"', ' ', 0xff}},
	}
	grammar := []struct {
		name string
		data []byte
	}{
		{"no value", nil}, {"truncated JSON", []byte(`{"a":`)}, {"duplicate member", []byte(`{"a":1,"a":2}`)}, {"second value", []byte(`{} true`)}, {"unescaped ASCII control", []byte{'"', 0x00, '"'}},
	}
	for _, group := range []struct {
		rows []struct {
			name string
			data []byte
		}
		want utf8AdmissionClass
	}{{valid, utf8AdmissionValid}, {invalid, utf8AdmissionMalformedUTF8}, {grammar, utf8AdmissionMalformedJSON}} {
		for _, row := range group.rows {
			t.Run(row.name, func(t *testing.T) {
				t.Parallel()
				input := row.data
				// Raw-invalid fragments are made into JSON string candidates except the explicit trailing-byte case.
				if group.want == utf8AdmissionMalformedUTF8 && row.name != "invalid byte after complete value" {
					input = append(append([]byte{'"'}, input...), '"')
				}
				before := bytes.Clone(input)
				err := jsonio.ValidateBytes(input)
				got := utf8AdmissionValid
				if jsonio.IsInvalidUTF8(err) {
					got = utf8AdmissionMalformedUTF8
				} else if err != nil {
					got = utf8AdmissionMalformedJSON
				}
				if got != group.want || !bytes.Equal(input, before) {
					t.Fatalf("input=%q class=%d refusal=%v conserved=%v, want class%d and unchanged source", input, got, err, bytes.Equal(input, before), group.want)
				}
				if err != nil && !errors.Is(err, core.ErrJSONContract) {
					t.Fatalf("refusal=%v, want %v", err, core.ErrJSONContract)
				}
				if group.want == utf8AdmissionMalformedUTF8 && !jsonio.IsSyntacticError(err) {
					t.Fatalf("UTF8 refusal=%v, want retained native syntax cause", err)
				}
			})
		}
	}
}

func FuzzValidateBytesUTF8ClassificationMatchesCanonicalOwner(f *testing.F) {
	f.Add([]byte(`"世界"`))
	f.Add([]byte{'"', 0xff, '"'})
	f.Add([]byte(`{} true`))
	f.Fuzz(func(t *testing.T, input []byte) {
		before := bytes.Clone(input)
		err := jsonio.ValidateBytes(input)
		_, canonicalErr := jsonio.Canonicalize(input, jsonio.CanonicalPolicy{NestingDepthMaximum: core.JSONNestingDepthMaximum})
		if (err == nil) != (canonicalErr == nil) || jsonio.IsInvalidUTF8(err) != jsonio.IsInvalidUTF8(canonicalErr) || !bytes.Equal(input, before) {
			t.Fatalf("input=%q validation=%v canonical=%v UTF8=%t/%t conserved=%t, want matching acceptance and classification with source unchanged", input, err, canonicalErr, jsonio.IsInvalidUTF8(err), jsonio.IsInvalidUTF8(canonicalErr), bytes.Equal(input, before))
		}
		if err != nil && !errors.Is(err, core.ErrJSONContract) {
			var depth *jsonio.DepthError
			if !errors.As(err, &depth) {
				t.Fatalf("refusal=%v, want JSON contract or depth identity", err)
			}
		}
	})
}
