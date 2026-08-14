package receipt

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestOfferingIdentityJSONProjectionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exhausts every offering and accepted JSON framing", func(t *testing.T) {
		t.Parallel()

		framings := []struct {
			name   string
			prefix []byte
			suffix []byte
		}{
			{name: "canonical"},
			{name: "one leading space", prefix: []byte{' '}},
			{name: "one trailing newline", suffix: []byte{'\n'}},
			{name: "tab and carriage return", prefix: []byte{'\t'}, suffix: []byte{'\r'}},
			{name: "mixed outer whitespace", prefix: []byte(" \n\t"), suffix: []byte("\r\n ")},
		}
		validCount := 0
		for raw := 0; raw <= 255; raw++ {
			offering := core.Offering(raw)
			if !offering.IsValid() {
				continue
			}
			validCount++
			want, err := OfferingIdentityFor(offering)
			if err != nil {
				t.Fatalf("OfferingIdentityFor(%v) error = %v, want nil", offering, err)
			}
			canonical, err := want.MarshalJSON()
			if err != nil {
				t.Fatalf("OfferingIdentity.MarshalJSON(%v) error = %v, want nil", offering, err)
			}
			for _, framing := range framings {
				t.Run(fmt.Sprintf("%s/%s", offering.String(), framing.name), func(t *testing.T) {
					t.Parallel()

					document := append(append(append([]byte{}, framing.prefix...), canonical...), framing.suffix...)
					got := OfferingIdentity{}
					if gotErr := got.UnmarshalJSON(document); gotErr != nil || got != want {
						t.Fatalf("OfferingIdentity.UnmarshalJSON(%q) = (%v, %v), want (%v, nil)", document, got, gotErr, want)
					}
					if gotErr := got.Validate(); gotErr != nil {
						t.Fatalf("OfferingIdentity.UnmarshalJSON(%q).Validate() error = %v, want nil", document, gotErr)
					}
					first, gotErr := got.MarshalJSON()
					if gotErr != nil || !bytes.Equal(first, canonical) {
						t.Fatalf("OfferingIdentity.MarshalJSON() = (%q, %v), want (%q, nil)", first, gotErr, canonical)
					}
					var roundTrip OfferingIdentity
					if gotErr := roundTrip.UnmarshalJSON(first); gotErr != nil || roundTrip != got {
						t.Fatalf("OfferingIdentity canonical round trip = (%v, %v), want (%v, nil)", roundTrip, gotErr, got)
					}
					second, gotErr := roundTrip.MarshalJSON()
					if gotErr != nil || !bytes.Equal(second, first) {
						t.Fatalf("OfferingIdentity second canonical projection = (%q, %v), want (%q, nil)", second, gotErr, first)
					}
				})
			}
		}
		if validCount != 3 {
			t.Fatalf("exhaustive core.Offering domain admitted %d values, want 3", validCount)
		}
	})

	t.Run("positive accepts exact global JSON byte ceiling", func(t *testing.T) {
		t.Parallel()

		want, err := OfferingIdentityFor(core.OfferingBug)
		if err != nil {
			t.Fatalf("OfferingIdentityFor(%v) error = %v, want nil", core.OfferingBug, err)
		}
		canonical, err := want.MarshalJSON()
		if err != nil {
			t.Fatalf("OfferingIdentity.MarshalJSON() error = %v, want nil", err)
		}
		for _, extent := range []int{core.JSONDocumentMaximumBytes - 1, core.JSONDocumentMaximumBytes} {
			t.Run(fmt.Sprintf("%d bytes", extent), func(t *testing.T) {
				t.Parallel()

				document := append(bytes.Repeat([]byte{' '}, extent-len(canonical)), canonical...)
				got := OfferingIdentity{}
				if gotErr := got.UnmarshalJSON(document); gotErr != nil || got != want {
					t.Fatalf("OfferingIdentity.UnmarshalJSON(%d bytes) = (%v, %v), want (%v, nil)", len(document), got, gotErr, want)
				}
			})
		}
	})

	t.Run("negative refuses every single bit near miss", func(t *testing.T) {
		t.Parallel()

		preserved, err := OfferingIdentityFor(core.OfferingWitness)
		if err != nil {
			t.Fatalf("OfferingIdentityFor(%v) error = %v, want nil", core.OfferingWitness, err)
		}
		for rawOffering := 0; rawOffering <= 255; rawOffering++ {
			offering := core.Offering(rawOffering)
			if !offering.IsValid() {
				continue
			}
			seed, gotErr := OfferingIdentityFor(offering)
			if gotErr != nil {
				t.Fatalf("OfferingIdentityFor(%v) error = %v, want nil", offering, gotErr)
			}
			seedBytes, gotErr := hex.DecodeString(seed.String())
			if gotErr != nil || len(seedBytes) != LifecycleIdentityBytes {
				t.Fatalf("hex.DecodeString(OfferingIdentityFor(%v)) = (%x, %v), want %d bytes and nil", offering, seedBytes, gotErr, LifecycleIdentityBytes)
			}
			for bit := range LifecycleIdentityBytes * 8 {
				mutated := append([]byte{}, seedBytes...)
				mutated[bit/8] ^= byte(1 << (bit % 8))
				if bytes.Equal(mutated, seedBytes) {
					t.Fatalf("single-bit mutation %d of %v did not change the load-bearing identity", bit, offering)
				}
				text := hex.EncodeToString(mutated)
				document, marshalErr := json.Marshal(text)
				if marshalErr != nil {
					t.Fatalf("json.Marshal(single-bit mutation %d of %v) error = %v, want nil", bit, offering, marshalErr)
				}
				got := preserved
				gotErr = got.UnmarshalJSON(document)
				if gotErr == nil {
					t.Fatalf("OfferingIdentity.UnmarshalJSON(single-bit mutation %d of %v) = (%v, nil), want refusal", bit, offering, got)
				}
				if !errors.Is(gotErr, core.ErrJSONContract) ||
					!errors.Is(gotErr, core.ErrLifecycleIdentityContract) ||
					!errors.Is(gotErr, core.ErrReceiptContract) {
					t.Fatalf("OfferingIdentity.UnmarshalJSON(single-bit mutation %d of %v) error = %v, want JSON+lifecycle+Receipt identities", bit, offering, gotErr)
				}
				if got != preserved {
					t.Fatalf("OfferingIdentity.UnmarshalJSON(single-bit mutation %d of %v) receiver = %v, want preserved %v", bit, offering, got, preserved)
				}
			}
		}
	})

	t.Run("negative pressures malformed type text and size boundaries", func(t *testing.T) {
		t.Parallel()

		preserved, err := OfferingIdentityFor(core.OfferingPeachfuzz)
		if err != nil {
			t.Fatalf("OfferingIdentityFor(%v) error = %v, want nil", core.OfferingPeachfuzz, err)
		}
		canonical, err := preserved.MarshalJSON()
		if err != nil {
			t.Fatalf("OfferingIdentity.MarshalJSON() error = %v, want nil", err)
		}
		uppercase := strings.ToUpper(preserved.String())
		if uppercase == preserved.String() {
			t.Fatalf("uppercase hostile fixture %q did not mutate the canonical identity", uppercase)
		}
		jsonText := func(value string) []byte {
			document, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				t.Fatalf("json.Marshal(%q) error = %v, want nil", value, marshalErr)
			}
			return document
		}
		cases := []struct {
			name     string
			document []byte
		}{
			{name: "nil document", document: nil},
			{name: "empty document", document: []byte{}},
			{name: "whitespace only", document: []byte(" \t\r\n")},
			{name: "null", document: []byte("null")},
			{name: "true boolean", document: []byte("true")},
			{name: "false boolean", document: []byte("false")},
			{name: "zero number", document: []byte("0")},
			{name: "negative number", document: []byte("-1")},
			{name: "fractional number", document: []byte("0.5")},
			{name: "empty object", document: []byte("{}")},
			{name: "object with identity member", document: append([]byte(`{"identity":`), append(canonical, '}')...)},
			{name: "empty array", document: []byte("[]")},
			{name: "array containing canonical identity", document: append([]byte{'['}, append(canonical, ']')...)},
			{name: "empty string", document: jsonText("")},
			{name: "one hexadecimal digit below exact width", document: jsonText(strings.Repeat("0", LifecycleIdentityHexBytes-1))},
			{name: "one hexadecimal digit above exact width", document: jsonText(strings.Repeat("0", LifecycleIdentityHexBytes+1))},
			{name: "uppercase canonical identity", document: jsonText(uppercase)},
			{name: "non hexadecimal exact width", document: jsonText(strings.Repeat("z", LifecycleIdentityHexBytes))},
			{name: "all zero identity", document: jsonText(strings.Repeat("0", LifecycleIdentityHexBytes))},
			{name: "all maximum identity", document: jsonText(strings.Repeat("f", LifecycleIdentityHexBytes))},
			{name: "lowest nonzero arbitrary identity", document: jsonText(strings.Repeat("0", LifecycleIdentityHexBytes-1) + "1")},
			{name: "highest identity below maximum", document: jsonText(strings.Repeat("f", LifecycleIdentityHexBytes-1) + "e")},
			{name: "truncated closing quote", document: canonical[:len(canonical)-1]},
			{name: "two adjacent documents", document: append(append([]byte{}, canonical...), canonical...)},
			{name: "trailing object", document: append(append([]byte{}, canonical...), []byte("{}")...)},
			{name: "unescaped nul in string", document: []byte{'"', 0, '"'}},
			{name: "invalid utf8 in string", document: []byte{'"', 0xff, '"'}},
			{name: "unpaired high surrogate", document: []byte(`"\ud800"`)},
			{name: "unpaired low surrogate", document: []byte(`"\udc00"`)},
			{name: "one byte above global JSON ceiling", document: bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes+1)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := preserved
				gotErr := got.UnmarshalJSON(tc.document)
				if !errors.Is(gotErr, core.ErrJSONContract) ||
					!errors.Is(gotErr, core.ErrLifecycleIdentityContract) ||
					!errors.Is(gotErr, core.ErrReceiptContract) {
					t.Fatalf("OfferingIdentity.UnmarshalJSON(%q) error = %v, want JSON+lifecycle+Receipt identities", tc.document, gotErr)
				}
				if got != preserved {
					t.Fatalf("OfferingIdentity.UnmarshalJSON(%q) receiver = %v, want preserved %v", tc.document, got, preserved)
				}
			})
		}
	})

	t.Run("neutral cannot emit or admit an unset identity", func(t *testing.T) {
		t.Parallel()

		zero := OfferingIdentity{}
		encoded, gotErr := zero.MarshalJSON()
		if encoded != nil || !errors.Is(gotErr, core.ErrJSONContract) ||
			!errors.Is(gotErr, core.ErrLifecycleIdentityContract) ||
			!errors.Is(gotErr, core.ErrReceiptContract) {
			t.Fatalf("OfferingIdentity{}.MarshalJSON() = (%q, %v), want nil and JSON+lifecycle+Receipt identities", encoded, gotErr)
		}
		var nilReceiver *OfferingIdentity
		gotErr = nilReceiver.UnmarshalJSON([]byte(`""`))
		if !errors.Is(gotErr, core.ErrJSONContract) ||
			!errors.Is(gotErr, core.ErrLifecycleIdentityContract) ||
			!errors.Is(gotErr, core.ErrReceiptContract) {
			t.Fatalf("(*OfferingIdentity)(nil).UnmarshalJSON() error = %v, want JSON+lifecycle+Receipt identities", gotErr)
		}
		gotErr = zero.UnmarshalJSON([]byte(`"00000000000000000000000000000000"`))
		if !errors.Is(gotErr, core.ErrJSONContract) ||
			!errors.Is(gotErr, core.ErrLifecycleIdentityContract) ||
			!errors.Is(gotErr, core.ErrReceiptContract) || zero != (OfferingIdentity{}) {
			t.Fatalf("zero OfferingIdentity rejected decode = (%v, %v), want zero and JSON+lifecycle+Receipt identities", zero, gotErr)
		}
	})
}

func TestOfferingIdentityForExhaustsClosedOfferingDomainAndKeepsNamespacesDistinct(t *testing.T) {
	t.Parallel()

	seen := make(map[OfferingIdentity]core.Offering)
	for raw := 0; raw <= 255; raw++ {
		offering := core.Offering(raw)
		got, gotErr := OfferingIdentityFor(offering)
		if !offering.IsValid() {
			if !errors.Is(gotErr, core.ErrPrimitiveContract) ||
				!errors.Is(gotErr, core.ErrLifecycleIdentityContract) ||
				!errors.Is(gotErr, core.ErrReceiptContract) || got != (OfferingIdentity{}) {
				t.Fatalf("OfferingIdentityFor(%d) = (%v, %v), want zero and Primitive+lifecycle+Receipt identities",
					raw, got, gotErr)
			}
			continue
		}
		if gotErr != nil || got.Validate() != nil {
			t.Fatalf("OfferingIdentityFor(%v) = (%v, %v), want valid identity and nil", offering, got, gotErr)
		}
		again, err := OfferingIdentityFor(offering)
		if err != nil || again != got {
			t.Fatalf("OfferingIdentityFor(%v) second derivation = (%v, %v), want (%v, nil)", offering, again, err, got)
		}
		if previous, exists := seen[got]; exists {
			t.Fatalf("OfferingIdentityFor(%v) = %v, already assigned to %v", offering, got, previous)
		}
		seen[got] = offering
	}
	if got, want := len(seen), 3; got != want {
		t.Fatalf("distinct derived offering identities = %d, want %d", got, want)
	}
}

func TestScopeForBindsExactAccountAndDerivedOffering(t *testing.T) {
	t.Parallel()

	account, err := NewAccountIdentity([LifecycleIdentityBytes]byte{1})
	if err != nil {
		t.Fatalf("NewAccountIdentity() error = %v, want nil", err)
	}
	for _, offering := range []core.Offering{core.OfferingBug, core.OfferingWitness, core.OfferingPeachfuzz} {
		t.Run(offering.String(), func(t *testing.T) {
			t.Parallel()

			got, gotErr := ScopeFor(account, offering)
			wantOffering, wantErr := OfferingIdentityFor(offering)
			want := Scope{Account: account, Offering: wantOffering}
			if gotErr != nil || wantErr != nil || got != want {
				t.Fatalf("ScopeFor(%v) = (%v, %v), want (%v, nil); identity error = %v",
					offering, got, gotErr, want, wantErr)
			}
		})
	}
	if got, err := ScopeFor(AccountIdentity{}, core.OfferingWitness); !errors.Is(err, core.ErrReceiptContract) || got != (Scope{}) {
		t.Fatalf("ScopeFor(zero account) = (%v, %v), want zero and errors.Is %v", got, err, core.ErrReceiptContract)
	}
	if got, err := ScopeFor(account, core.OfferingUnknown); !errors.Is(err, core.ErrLifecycleIdentityContract) || got != (Scope{}) {
		t.Fatalf("ScopeFor(unknown offering) = (%v, %v), want zero and errors.Is %v", got, err, core.ErrLifecycleIdentityContract)
	}
}
