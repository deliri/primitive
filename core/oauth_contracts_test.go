package core

import (
	"errors"
	"strings"
	"testing"
)

func TestOAuthPKCENativeChallengeAndTypedRefusals(t *testing.T) {
	t.Parallel()
	verifier, err := ParseOAuthPKCEVerifier("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	if err != nil {
		t.Fatalf("ParseOAuthPKCEVerifier(RFC7636) = %v, want nil", err)
	}
	challenge, err := verifier.Challenge()
	if err != nil || challenge.String() != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("RFC7636 challenge = (%q,%v), want canonical S256", challenge.String(), err)
	}
	for _, row := range []struct {
		name    string
		value   string
		wantErr error
	}{
		{name: "neutral/unset verifier refuses", wantErr: ErrOAuthContract},
		{name: "negative/one below RFC minimum", value: strings.Repeat("a", 42), wantErr: ErrOAuthContract},
		{name: "positive/exact RFC minimum", value: strings.Repeat("a", 43)},
		{name: "positive/exact RFC maximum", value: strings.Repeat("a", 128)},
		{name: "negative/one above RFC maximum", value: strings.Repeat("a", 129), wantErr: ErrOAuthContract},
		{name: "negative/space is outside unreserved alphabet", value: strings.Repeat("a", 42) + " ", wantErr: ErrOAuthContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := ParseOAuthPKCEVerifier(row.value)
			if !errors.Is(gotErr, row.wantErr) {
				t.Fatalf("PKCE verifier error = %v, want %v", gotErr, row.wantErr)
			}
			if gotErr != nil && got != (OAuthPKCEVerifier{}) {
				t.Fatalf("refused verifier = %+v, want zero", got)
			}
		})
	}
}
