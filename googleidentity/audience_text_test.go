package googleidentity

import (
	"bytes"
	"errors"
	"strings"
	"testing"

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
		{"multibyte UTF8 at byte ceiling", strings.Repeat("é", AudienceMaximumBytes/2), nil},
		{"one below byte ceiling", strings.Repeat("a", AudienceMaximumBytes-1), nil},
		{"exact byte ceiling", strings.Repeat("a", AudienceMaximumBytes), nil},
		{"one above byte ceiling", strings.Repeat("a", AudienceMaximumBytes+1), core.ErrGoogleIdentityContract},
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
	if data, err := (Audience{}).MarshalText(); data != nil || !errors.Is(err, core.ErrGoogleIdentityContract) {
		t.Fatalf("zero marshal = (%q,%v), want nil and identity contract", data, err)
	}
	var absent *Audience
	if err := absent.UnmarshalText([]byte("a")); !errors.Is(err, core.ErrGoogleIdentityContract) {
		t.Fatalf("nil receiver error = %v, want identity contract", err)
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
	f.Add(bytes.Repeat([]byte{'a'}, AudienceMaximumBytes+1))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		err := got.UnmarshalText(data)
		if err != nil {
			if !errors.Is(err, core.ErrGoogleIdentityContract) || got != seed {
				t.Fatalf("refusal = (%v,%v), want preserved and typed", got, err)
			}
			return
		}
		parsed, parseErr := ParseAudience(string(data))
		if parseErr != nil || parsed != got || got.Validate() != nil || got.String() != string(data) {
			t.Fatalf("admitted audience = (%v,%v), want exact validated text", got, parseErr)
		}
		first, err := got.MarshalText()
		if err != nil || len(first) > AudienceMaximumBytes || !bytes.Equal(first, data) {
			t.Fatalf("canonical text = (%q,%v), want exact bounded input", first, err)
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
