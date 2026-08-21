package distribution_test

import (
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"encoding/json/jsontext"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
)

func TestSigningDomainExhaustsEveryUint8AndCanonicalToken(t *testing.T) {
	t.Parallel()

	valid := [...]struct {
		token  string
		domain distribution.SigningDomain
	}{
		{domain: distribution.SigningDomainPublicationRequestV1, token: distribution.SigningDomainPublicationRequestV1Token},
		{domain: distribution.SigningDomainPublicationGrantV1, token: distribution.SigningDomainPublicationGrantV1Token},
		{domain: distribution.SigningDomainPublicationCompletionV1, token: distribution.SigningDomainPublicationCompletionV1Token},
		{domain: distribution.SigningDomainUpdateRequestV1, token: distribution.SigningDomainUpdateRequestV1Token},
		{domain: distribution.SigningDomainUpdateResponseV1, token: distribution.SigningDomainUpdateResponseV1Token},
		{domain: distribution.SigningDomainUpgradeRequestV1, token: distribution.SigningDomainUpgradeRequestV1Token},
		{domain: distribution.SigningDomainUpgradeGrantV1, token: distribution.SigningDomainUpgradeGrantV1Token},
	}
	for raw := range 256 {
		domain := distribution.SigningDomain(raw)
		wantValid := false
		for _, candidate := range valid {
			if domain == candidate.domain {
				wantValid = true
			}
		}
		if got := domain.IsValid(); got != wantValid {
			t.Fatalf("distribution.SigningDomain(%d).IsValid() = %t, want %t", raw, got, wantValid)
		}
		if !wantValid && !errors.Is(domain.Validate(), core.ErrDistributionContract) {
			t.Fatalf("distribution.SigningDomain(%d).Validate() error = %v, want %v", raw, domain.Validate(), core.ErrDistributionContract)
		}
	}
	for _, tc := range valid {
		if got := tc.domain.String(); got != tc.token {
			t.Fatalf("distribution.SigningDomain(%d).String() = %q, want %q", tc.domain, got, tc.token)
		}
		encoded, err := json.Marshal(tc.domain)
		if err != nil {
			t.Fatalf("json.Marshal(distribution.SigningDomain(%d)) error = %v, want nil", tc.domain, err)
		}
		var decoded distribution.SigningDomain
		if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != tc.domain {
			t.Fatalf("json.Unmarshal(%q) = (%v, %v), want (%v, nil)", encoded, decoded, err, tc.domain)
		}
		parsed, err := distribution.ParseSigningDomain(tc.token)
		if err != nil || parsed != tc.domain {
			t.Fatalf("distribution.ParseSigningDomain(%q) = (%v, %v), want (%v, nil)", tc.token, parsed, err, tc.domain)
		}
	}
}

func TestSigningDomainHostileJSONSeparatesOwnerRefusalFromJSONV2TrailingDocumentRefusal(t *testing.T) {
	t.Parallel()

	canonical, err := json.Marshal(distribution.SigningDomainPublicationRequestV1)
	if err != nil {
		t.Fatalf("json.Marshal(valid signing domain) error = %v, want nil", err)
	}
	oversized := `"` + strings.Repeat("x", 1<<10) + `"`
	cases := []struct {
		name           string
		wire           []byte
		wantValue      distribution.SigningDomain
		wantTyped      bool
		wantFirstValue bool
	}{
		{name: "empty document is rejected by the stdlib parser", wire: nil},
		{name: "whitespace-only document is rejected by the stdlib parser", wire: []byte(" \t\n")},
		{name: "null is semantically rejected", wire: []byte("null"), wantTyped: true},
		{name: "boolean is semantically rejected", wire: []byte("true"), wantTyped: true},
		{name: "number is semantically rejected", wire: []byte("1"), wantTyped: true},
		{name: "array is semantically rejected", wire: []byte("[]"), wantTyped: true},
		{name: "object is semantically rejected", wire: []byte("{}"), wantTyped: true},
		{name: "empty token is semantically rejected", wire: []byte(`""`), wantTyped: true},
		{name: "future token is semantically rejected", wire: []byte(`"primitive-distribution-future-2099-1"`), wantTyped: true},
		{name: "token prefix is semantically rejected", wire: append([]byte(`"x`), canonical[1:]...), wantTyped: true},
		{name: "token suffix is semantically rejected", wire: append(append([]byte(nil), canonical[:len(canonical)-1]...), []byte(`x"`)...), wantTyped: true},
		{name: "uppercase token is semantically rejected", wire: []byte(strings.ToUpper(string(canonical))), wantTyped: true},
		{name: "leading whitespace is accepted by the JSON v2 boundary", wire: append([]byte(" "), canonical...), wantValue: distribution.SigningDomainPublicationRequestV1},
		{name: "trailing whitespace is accepted by the JSON v2 boundary", wire: append(append([]byte(nil), canonical...), ' '), wantValue: distribution.SigningDomainPublicationRequestV1},
		{name: "trailing value is rejected after JSON v2 admits the complete first value", wire: append(append([]byte(nil), canonical...), []byte(" true")...), wantFirstValue: true},
		{name: "missing opening quote is rejected by the JSON v2 parser", wire: append([]byte(nil), canonical[1:]...)},
		{name: "missing closing quote is rejected by the JSON v2 parser", wire: append([]byte(nil), canonical[:len(canonical)-1]...)},
		{name: "truncated token is semantically rejected", wire: append(append([]byte(nil), canonical[:len(canonical)-2]...), '"'), wantTyped: true},
		{name: "escaped canonical byte is semantically rejected", wire: []byte(`"\u0070rimitive-distribution-publication-request-2026-1"`), wantTyped: true},
		{name: "embedded null is rejected by the stdlib parser", wire: []byte{'"', 0, '"'}},
		{name: "unclosed escape is rejected by the stdlib parser", wire: []byte(`"\`)},
		{name: "oversized unknown token is semantically rejected", wire: []byte(oversized), wantTyped: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := distribution.SigningDomainUpgradeGrantV1
			gotErr := json.Unmarshal(tc.wire, &got)
			if tc.wantValue != distribution.SigningDomainUnknown {
				if gotErr != nil || got != tc.wantValue {
					t.Fatalf("json.Unmarshal(whitespace signing domain) = (%v, %v), want (%v, nil)", got, gotErr, tc.wantValue)
				}
				return
			}
			if tc.wantTyped {
				if !errors.Is(gotErr, core.ErrJSONContract) ||
					!errors.Is(gotErr, core.ErrDistributionContract) {
					t.Fatalf("json.Unmarshal(semantic signing-domain refusal) error = %v, want %v and %v", gotErr, core.ErrJSONContract, core.ErrDistributionContract)
				}
			} else {
				var syntax *jsontext.SyntacticError
				if !errors.As(gotErr, &syntax) {
					t.Fatalf("json.Unmarshal(malformed signing domain) error = %v, want *jsontext.SyntacticError", gotErr)
				}
			}
			wantReceiver := distribution.SigningDomainUpgradeGrantV1
			if tc.wantFirstValue {
				wantReceiver = distribution.SigningDomainPublicationRequestV1
			}
			if got != wantReceiver {
				t.Fatalf("rejected signing-domain receiver = %v, want %v", got, wantReceiver)
			}
		})
	}
}
