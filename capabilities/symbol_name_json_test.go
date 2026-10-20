package capabilities

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
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
