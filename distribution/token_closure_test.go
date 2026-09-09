package distribution

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"strconv"
	"testing"
)

func TestSigningDomainTokenClosureLayerTriad(t *testing.T) {
	t.Parallel()
	tokens := signingDomainTokens()
	for raw := range 256 {
		t.Run("underlying value "+strconv.Itoa(raw), func(t *testing.T) {
			t.Parallel()
			d := SigningDomain(raw)
			token := ""
			if raw > int(SigningDomainUnknown) && raw < len(tokens) {
				token = tokens[raw]
			}
			wantErr := error(nil)
			if token == "" {
				wantErr = core.ErrDistributionContract
			}

			switch d {
			case SigningDomainPublicationRequestV1, SigningDomainUpdateRequestV1, SigningDomainUpgradeRequestV1:
				commitment := RequestCommitment{domain: d, digest: core.SHA256Of([]byte("closed request token"))}
				if err := commitment.Validate(); !errors.Is(err, wantErr) {
					t.Fatalf("request token closure=%v, want %v", err, wantErr)
				}
			}
			gotErr := d.Validate()
			if !errors.Is(gotErr, wantErr) || d.IsValid() != (wantErr == nil) || d.String() != token {
				t.Fatalf("domain %d=(%q,%v,%t), want (%q,%v,%t)", raw, d.String(), gotErr, d.IsValid(), token, wantErr, wantErr == nil)
			}
			text, textErr := d.MarshalText()
			wire, wireErr := d.MarshalJSON()
			if wantErr != nil {
				if text != nil || wire != nil || !errors.Is(textErr, wantErr) || !errors.Is(wireErr, core.ErrJSONContract) {
					t.Fatalf("unset token emission=(%q,%v,%q,%v), want nil bytes and typed refusal", text, textErr, wire, wireErr)
				}
				return
			}
			if textErr != nil || string(text) != token || wireErr != nil {
				t.Fatalf("domain emission=(%q,%v,%v), want (%q,nil,nil)", text, textErr, wireErr, token)
			}
			parsed, err := ParseSigningDomain(token)
			if err != nil || parsed != d {
				t.Fatalf("parse token=(%v,%v), want (%v,nil)", parsed, err, d)
			}
		})
	}
}
func TestEmptySigningTokenNeverNamesATableHole(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		parse func() (SigningDomain, error)
	}{
		{"text parser", func() (SigningDomain, error) { return ParseSigningDomain("") }},
		{"canonical text parser", func() (SigningDomain, error) { return SigningDomainUnknown.ParseCanonicalText(nil) }},
		{"JSON parser", func() (SigningDomain, error) {
			var d SigningDomain
			err := d.UnmarshalJSON([]byte{'"', '"'})
			return d, err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.parse()
			if got != SigningDomainUnknown || !errors.Is(err, core.ErrDistributionContract) {
				t.Fatalf("empty token=(%v,%v), want unknown and Distribution refusal", got, err)
			}
		})
	}
}
