package googleidentity

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

func TestAudienceTextLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, input string
		wantErr     error
	}{
		{"minimum nonempty audience", "a", nil},
		{"OIDC audience without URL scheme", "api.tailscale.com/federated-client", nil},
		{"escaped text is preserved exactly", "a\"\\b", nil},
		{"multibyte UTF8 at former byte extent", strings.Repeat("é", googleFormerAudienceBytes/2), nil},
		{"below former byte extent", strings.Repeat("a", googleFormerAudienceBytes-1), nil},
		{"former byte extent", strings.Repeat("a", googleFormerAudienceBytes), nil},
		{"beyond former byte extent", strings.Repeat("a", googleFormerAudienceBytes+1), nil},
		{"invalid UTF8 cannot be repaired", string([]byte{0xff}), core.ErrGoogleIdentityContract},
		{"absent audience does not erase prior binding", "", core.ErrGoogleIdentityContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prior, err := ParseAudience("prior-binding")
			if err != nil {
				t.Fatal(err)
			}
			got := prior
			err = got.UnmarshalText([]byte(tc.input))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("UnmarshalText() = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != prior {
					t.Fatalf("refused receiver = %v, want %v", got, prior)
				}
				return
			}
			data, err := got.MarshalText()
			if err != nil || string(data) != tc.input {
				t.Fatalf("MarshalText() = (%q,%v), want (%q,nil)", data, err, tc.input)
			}
			wire, err := core.MarshalCanonicalJSONDocument(got)
			if err != nil {
				t.Fatal(err)
			}
			round, err := core.DecodeStrictJSONStructure[Audience](wire, core.DefaultStrictJSONLimits())
			if err != nil || round != got {
				t.Fatalf("JSON round trip = (%v,%v), want (%v,nil)", round, err, got)
			}
		})
	}
}

func TestAudienceEmptyBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		receiver *Audience
		wantErr  error
	}{
		{name: "nil_receiver_cannot_accept", wantErr: core.ErrGoogleIdentityContract},
		{name: "zero_receiver_accepts_valid_binding", receiver: &Audience{}},
		{name: "existing_receiver_replaced_only_on_admission", receiver: &Audience{value: "prior"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.receiver.UnmarshalText([]byte("next"))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("unmarshal error=%v, want %v", err, tc.wantErr)
			}
			if err == nil && tc.receiver.String() != "next" {
				t.Fatalf("binding=%v, want next", tc.receiver)
			}
		})
	}
	for _, tc := range []struct {
		name    string
		value   Audience
		want    string
		wantErr error
	}{
		{name: "zero_cannot_marshal_as_valid_binding", wantErr: core.ErrGoogleIdentityContract},
		{name: "valid_binding_marshal_is_exact", value: Audience{value: "next"}, want: "next"},
		{name: "corrupt_utf8_cannot_marshal_as_replacement_character", value: Audience{value: "\xff"}, wantErr: core.ErrGoogleIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data, err := tc.value.MarshalText()
			if !errors.Is(err, tc.wantErr) || string(data) != tc.want || (tc.wantErr != nil && data != nil) {
				t.Fatalf("text=%q error=%v, want %q and %v (nil bytes on refusal)", data, err, tc.want, tc.wantErr)
			}
		})
	}
}

// Inventory: ParseAudience and Audience.UnmarshalText admit the same external
// representation; both are exercised and compared within this callback.
func FuzzAudienceTextSemanticClosure(f *testing.F) {
	seed, err := ParseAudience("api.tailscale.com/federated-client")
	if err != nil {
		f.Fatal(err)
	}
	encoded, err := seed.MarshalText()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(encoded)
	f.Add([]byte{})
	f.Add([]byte{0xff})
	f.Add(bytes.Repeat([]byte{'a'}, googleFormerAudienceBytes+1))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		err := got.UnmarshalText(data)
		wantOK := len(data) > 0 && utf8.Valid(data)
		parsed, parseErr := ParseAudience(string(data))
		if (err == nil) != wantOK || (parseErr == nil) != wantOK {
			t.Fatalf("audience admission errors=%v/%v, want admitted=%t", err, parseErr, wantOK)
		}
		if err != nil {
			if !errors.Is(err, core.ErrGoogleIdentityContract) || got != seed || parsed != (Audience{}) || !errors.Is(parseErr, core.ErrGoogleIdentityContract) {
				t.Fatalf("refusal = (%v,%v), want preserved and typed", got, err)
			}
			return
		}
		if parseErr != nil || parsed != got || got.Validate() != nil || got.String() != string(data) {
			t.Fatalf("admitted audience = (%v,%v), want exact validated text", got, parseErr)
		}
		first, err := got.MarshalText()
		if err != nil || !bytes.Equal(first, data) {
			t.Fatalf("canonical text = (%q,%v), want exact input", first, err)
		}
		var round Audience
		if err := round.UnmarshalText(first); err != nil || round != got {
			t.Fatalf("round trip = (%v,%v), want (%v,nil)", round, err, got)
		}
		second, err := round.MarshalText()
		if err != nil || !bytes.Equal(first, second) {
			t.Fatalf("second marshal = (%q,%v), want %q", second, err, first)
		}
	})
}
