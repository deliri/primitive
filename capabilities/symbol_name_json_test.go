package capabilities

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"go/token"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestSymbolNameJSONPreservesNativeIdentifier(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"Export", "_value", "Ω", "界", "𐐀", "Value١"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			name, err := ParseSymbolName(source)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(name)
			if err != nil {
				t.Fatal(err)
			}
			want, err := json.Marshal(name.String())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(encoded, want) {
				t.Fatalf("native identifier wire=%q want%q", encoded, want)
			}
			var decoded SymbolName
			if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != name {
				t.Fatalf("native identifier round trip=%v/%v want%v", decoded, err, name)
			}
		})
	}
}

func TestSymbolNameJSONRejectsObjectShape(t *testing.T) {
	t.Parallel()
	before, err := ParseSymbolName("Keep")
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{[]byte(`{}`), []byte(`{"value":"Other"}`), []byte(`[]`), []byte(`true`), []byte(`1`), []byte(`null`)} {
		t.Run(string(data), func(t *testing.T) {
			t.Parallel()
			got := before
			err := json.Unmarshal(data, &got)
			if !errors.Is(err, core.ErrCapabilitiesContract) || got != before {
				t.Fatalf("native identifier refusal=%v/%v want unchanged%v", got, err, before)
			}
		})
	}
}

func TestSymbolNameJSONNativeLexicalAndRepresentationBoundaries(t *testing.T) {
	t.Parallel()
	before, err := ParseSymbolName("Keep")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"a", "__", "Value0", "Ω", "界", "𐐀", "Value١", strings.Repeat("Name", 16385), "", "_", "0x", "a.b", "F[T]", "F()", "a ", "a\x00", "a\u200d"} {
		t.Run(fmt.Sprintf("identifier bytes%d prefix%q", len(source), source[:min(len(source), 16)]), func(t *testing.T) {
			t.Parallel()
			wire, err := json.Marshal(source)
			if err != nil {
				t.Fatal(err)
			}
			got := before
			err = got.UnmarshalJSON(wire)
			want := token.IsIdentifier(source) && !token.Lookup(source).IsKeyword() && source != "_"
			if !want {
				if !errors.Is(err, core.ErrCapabilitiesContract) || got != before {
					t.Fatalf("rejected identifier=%v/%v", got, err)
				}
				return
			}
			if err != nil || got.String() != source {
				t.Fatalf("native identifier=%q/%v want%q", got.String(), err, source)
			}
			encoded, err := got.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var roundtrip SymbolName
			if err := roundtrip.UnmarshalJSON(encoded); err != nil || roundtrip != got {
				t.Fatalf("native JSON round trip=%v/%v", roundtrip, err)
			}
		})
	}
	for keyword := token.BREAK; keyword <= token.VAR; keyword++ {
		t.Run("keyword "+keyword.String(), func(t *testing.T) {
			t.Parallel()
			wire, err := json.Marshal(keyword.String())
			if err != nil {
				t.Fatal(err)
			}
			got := before
			if err := got.UnmarshalJSON(wire); !errors.Is(err, core.ErrCapabilitiesContract) || got != before {
				t.Fatalf("keyword identifier=%v/%v", got, err)
			}
		})
	}
}

func TestSymbolNameJSONRefusesZeroAndNilWithoutMutation(t *testing.T) {
	t.Parallel()
	for _, name := range []SymbolName{{}, {value: "0bad"}, {value: "for"}, {value: "a.b"}, {value: "\xff"}} {
		wire, err := name.MarshalJSON()
		if !errors.Is(err, core.ErrCapabilitiesContract) || len(wire) != 0 {
			t.Fatalf("invalid identifier emitted%q/%v", wire, err)
		}
	}
	var receiver *SymbolName
	if err := receiver.UnmarshalJSON([]byte(`"Export"`)); !errors.Is(err, core.ErrCapabilitiesContract) {
		t.Fatalf("nil identifier receiver=%v", err)
	}
	before, err := ParseSymbolName("Keep")
	if err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{nil, []byte(`"unterminated`), []byte(`"Export" "Other"`), []byte("\"bad\xff\""), []byte(`"\ud800"`), []byte(`"\u0000"`)} {
		got := before
		if err := got.UnmarshalJSON(data); !errors.Is(err, core.ErrCapabilitiesContract) || got != before {
			t.Fatalf("malformed identifier=%v/%v", got, err)
		}
	}
}

func FuzzSymbolNameJSONNativeIdentifierClosure(f *testing.F) {
	for _, source := range []string{"Export", "_value", "Ω", "界", "𐐀"} {
		name, err := ParseSymbolName(source)
		if err != nil {
			f.Fatal(err)
		}
		wire, err := name.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(wire)
	}
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add([]byte(`"a.b"`))
	f.Add([]byte(`"for"`))
	f.Fuzz(func(t *testing.T, data []byte) {
		before, err := ParseSymbolName("Keep")
		if err != nil {
			t.Fatal(err)
		}
		got := before
		err = got.UnmarshalJSON(data)
		var text string
		decodeErr := json.Unmarshal(data, &text)
		want := decodeErr == nil && token.IsIdentifier(text) && !token.Lookup(text).IsKeyword() && text != "_"
		if !want {
			if !errors.Is(err, core.ErrCapabilitiesContract) || got != before {
				t.Fatalf("invalid JSON identifier=%v/%v", got, err)
			}
			return
		}
		if err != nil || got.String() != text || got.Validate() != nil {
			t.Fatalf("native JSON identifier=%q/%v want%q", got.String(), err, text)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var again SymbolName
		if err := again.UnmarshalJSON(encoded); err != nil || again != got {
			t.Fatalf("native JSON round trip=%v/%v", again, err)
		}
		second, err := again.MarshalJSON()
		if err != nil || !bytes.Equal(encoded, second) {
			t.Fatalf("canonical second identifier=%q/%v want%q", second, err, encoded)
		}
	})
}
