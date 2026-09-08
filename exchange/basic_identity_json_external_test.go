package exchange_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestBasicIdentityJSONLayerTriad(t *testing.T) {
	t.Parallel()
	const retained exchange.BasicAuthorizationIdentity = "retained"
	maximum := strings.Repeat("i", exchange.BasicAuthorizationIdentityMaximumBytes)
	multibyteMaximum := strings.Repeat("é", exchange.BasicAuthorizationIdentityMaximumBytes/2) + "i"
	encode := func(value string) []byte {
		wire, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("Go JSON fixture encoding error = %v, want nil", err)
		}
		return wire
	}
	minimumWire := encode("i")
	boundedWire := func(size int) []byte {
		return append(bytes.Repeat([]byte{' '}, size-len(minimumWire)), minimumWire...)
	}
	cases := []struct {
		name    string
		wire    []byte
		want    exchange.BasicAuthorizationIdentity
		wantErr error
	}{
		{name: "positive minimum nonempty identity survives", wire: minimumWire, want: "i"},
		{name: "positive JSON quoting preserves a quote in identity", wire: encode("i\"d"), want: "i\"d"},
		{name: "positive JSON quoting preserves a backslash in identity", wire: encode("i\\d"), want: "i\\d"},
		{name: "positive two-byte UTF8 remains exact", wire: encode("é"), want: "é"},
		{name: "positive three-byte UTF8 remains exact", wire: encode("界"), want: "界"},
		{name: "positive surrogate pair becomes exact four-byte UTF8", wire: []byte(`"\ud83d\udd0c"`), want: "🔌"},
		{name: "positive printable whitespace is not a control character", wire: encode(" a\u00a0"), want: " a\u00a0"},
		{name: "positive identity one byte below ceiling survives", wire: encode(maximum[:len(maximum)-1]), want: exchange.BasicAuthorizationIdentity(maximum[:len(maximum)-1])},
		{name: "positive identity at byte ceiling survives", wire: encode(maximum), want: exchange.BasicAuthorizationIdentity(maximum)},
		{name: "positive multibyte identity at byte ceiling survives", wire: encode(multibyteMaximum), want: exchange.BasicAuthorizationIdentity(multibyteMaximum)},
		{name: "negative empty identity preserves receiver", wire: encode(""), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative one byte beyond identity ceiling preserves receiver", wire: encode(maximum + "i"), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative multibyte count cannot hide excess bytes", wire: encode(multibyteMaximum + "i"), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative colon cannot cross the identity delimiter", wire: encode("a:b"), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative escaped newline remains a forbidden control", wire: encode("a\n"), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative NUL remains forbidden after JSON decoding", wire: encode("a\x00"), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative DEL remains a forbidden control", wire: encode("a\x7f"), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative Unicode control cannot evade an ASCII-only check", wire: encode("a\u0085"), want: retained, wantErr: core.ErrJSONContract},
		{name: "neutral absent wire cannot overwrite retained identity", want: retained, wantErr: core.ErrJSONContract},
		{name: "neutral null cannot erase retained identity", wire: []byte("null"), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative non-string scalar cannot coerce to identity", wire: []byte("1"), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative container cannot replace the nominal string", wire: []byte(`{"identity":"i"}`), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative trailing document cannot be ignored", wire: []byte(`"i" "j"`), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative truncated token cannot partially replace receiver", wire: []byte(`"i`), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative invalid UTF8 cannot be repaired into another identity", wire: []byte{'"', 0xff, '"'}, want: retained, wantErr: core.ErrJSONContract},
		{name: "negative lone high surrogate cannot be repaired", wire: []byte(`"\ud800"`), want: retained, wantErr: core.ErrJSONContract},
		{name: "negative lone low surrogate cannot be repaired", wire: []byte(`"\udc00"`), want: retained, wantErr: core.ErrJSONContract},
		{name: "boundary final C0 control cannot enter identity", wire: encode("a\u001f"), want: retained, wantErr: core.ErrJSONContract},
		{name: "boundary first byte above C0 stays literal space", wire: encode("a "), want: "a "},
		{name: "boundary byte above space is not over-rejected", wire: encode("a!"), want: "a!"},
		{name: "boundary final printable ASCII byte remains exact", wire: encode("a~"), want: "a~"},
		{name: "boundary first C1 control cannot bypass UTF8 validation", wire: encode("a\u0080"), want: retained, wantErr: core.ErrJSONContract},
		{name: "boundary last C1 control remains forbidden", wire: encode("a\u009f"), want: retained, wantErr: core.ErrJSONContract},
		{name: "boundary first rune above C1 remains nonbreaking space", wire: encode("a\u00a0"), want: "a\u00a0"},
		{name: "boundary byte below credential delimiter stays literal", wire: encode("a9"), want: "a9"},
		{name: "boundary byte above credential delimiter stays literal", wire: encode("a;"), want: "a;"},
		{name: "boundary overlong UTF8 cannot normalize into identity", wire: []byte{'"', 0xc0, 0xaf, '"'}, want: retained, wantErr: core.ErrJSONContract},
		{name: "boundary truncated two-byte UTF8 cannot repair its tail", wire: []byte{'"', 0xc3, '"'}, want: retained, wantErr: core.ErrJSONContract},
		{name: "boundary truncated three-byte UTF8 cannot repair its tail", wire: []byte{'"', 0xe7, 0x95, '"'}, want: retained, wantErr: core.ErrJSONContract},
		{name: "boundary truncated four-byte UTF8 cannot repair its tail", wire: []byte{'"', 0xf0, 0x9f, 0x94, '"'}, want: retained, wantErr: core.ErrJSONContract},
		{name: "boundary whitespace document one below ceiling keeps exact value", wire: boundedWire(core.JSONDocumentMaximumBytes - 1), want: "i"},
		{name: "boundary whitespace document at ceiling keeps exact value", wire: boundedWire(core.JSONDocumentMaximumBytes), want: "i"},
		{name: "boundary document one above ceiling cannot hide behind tiny value", wire: boundedWire(core.JSONDocumentMaximumBytes + 1), want: retained, wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := retained
			gotErr := got.UnmarshalJSON(tc.wire)
			if !errors.Is(gotErr, tc.wantErr) || got != tc.want {
				t.Fatalf("UnmarshalJSON() = (%q, %v), want (%q, %v)", got, gotErr, tc.want, tc.wantErr)
			}
			if tc.wantErr != nil && !errors.Is(gotErr, core.ErrExchangeContract) {
				t.Fatalf("identity refusal error = %v, want JSON and Exchange identities", gotErr)
			}
			if tc.wantErr == nil {
				encoded, err := got.MarshalJSON()
				wantWire := encode(tc.want.String())
				if err != nil || !bytes.Equal(encoded, wantWire) {
					t.Fatalf("canonical identity wire/error = (%q, %v), want (%q, nil)", encoded, err, wantWire)
				}
			}
		})
	}
}
