package providerwire

import (
	"bytes"
	"errors"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func FuzzPayPalOAuthResponseSemanticClosure(f *testing.F) {
	maximum, err := core.NewByteCount(payPalOAuthResponseMaximumBytes)
	if err != nil {
		f.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	limits := payPalOAuthTestLimits(maximum)
	seed := payPalOAuthResponse{
		Scope: "openid", AccessToken: "paypal_access_token", TokenType: payPalOAuthTokenTypeBearer,
		AppID: "APP-123", ExpiresIn: 28800, Nonce: "nonce-123",
	}
	canonical, err := core.EncodeValidatedJSON(seed, limits)
	if err != nil {
		f.Fatalf("core.EncodeValidatedJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"scope":"openid","access_token":"paypal_access_token","token_type":"Bearer","app_id":"APP-123","expires_in":28800,"nonce":"nonce-123","unknown":true}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got, gotErr := decodePayPalOAuthResponse(data, maximum)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrProviderWireContract) || got != (payPalOAuthResponse{}) {
				t.Fatalf("decodePayPalOAuthResponse(rejected) = (%+v, %v), want zero and %v", got, gotErr, core.ErrProviderWireContract)
			}
			return
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil {
			t.Fatalf("decodePayPalOAuthResponse(accepted).Validate() error = %v, want nil", gotValidateErr)
		}
		encoded, gotEncodeErr := core.EncodeValidatedJSON(got, limits)
		if gotEncodeErr != nil || len(encoded) > payPalOAuthResponseMaximumBytes {
			t.Fatalf("core.EncodeValidatedJSON(accepted) = (%d bytes, %v), want bounded and nil", len(encoded), gotEncodeErr)
		}
		roundTrip, gotRoundTripErr := decodePayPalOAuthResponse(encoded, maximum)
		if gotRoundTripErr != nil || roundTrip != got {
			t.Fatalf("PayPal OAuth canonical round trip = (%+v, %v), want (%+v, nil)", roundTrip, gotRoundTripErr, got)
		}
		second, gotSecondErr := core.EncodeValidatedJSON(roundTrip, limits)
		if gotSecondErr != nil || !slices.Equal(second, encoded) {
			t.Fatalf("PayPal OAuth second canonical projection = (%q, %v), want (%q, nil)", second, gotSecondErr, encoded)
		}
		if bytes.Contains(encoded, []byte(`"unknown"`)) {
			t.Fatalf("PayPal OAuth canonical projection = %q, want no unowned member", encoded)
		}
	})
}

func FuzzNewPayPalClientCredentialSemanticBoundary(f *testing.F) {
	f.Add("paypal-client", []byte("paypal-secret"))
	f.Add("", []byte("paypal-secret"))
	f.Add("paypal-client", []byte{})

	f.Fuzz(func(t *testing.T, clientID string, secret []byte) {
		identity, identityErr := exchange.ParseBasicAuthorizationIdentity(clientID)
		credential, gotErr := NewPayPalClientCredential(identity, secret)
		wantAccepted := identityErr == nil && len(clientID) > 0 && len(clientID) <= PayPalClientIDMaximumBytes &&
			len(secret) > 0 && len(secret) <= PayPalClientSecretMaximumBytes && utf8.Valid(secret)
		if !wantAccepted {
			if !errors.Is(errors.Join(identityErr, gotErr), core.ErrProviderWireContract) || credential.Validate() == nil {
				t.Fatalf("NewPayPalClientCredential(rejected) = (%v, %v/%v), want zero and %v", credential, identityErr, gotErr, core.ErrProviderWireContract)
			}
			return
		}
		mutateFirstByte(secret)
		if gotErr != nil || credential.Validate() != nil {
			t.Fatalf("NewPayPalClientCredential(accepted) = (%v, %v), want copied credential and nil", credential, gotErr)
		}
		if closeErr := credential.Close(); closeErr != nil || credential.Validate() == nil {
			t.Fatalf("PayPalClientCredential.Close() = %v, post-close Validate = %v, want nil then rejection", closeErr, credential.Validate())
		}
	})
}

func payPalOAuthTestLimits(maximum core.ByteCount) core.StrictJSONLimits {
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	limits.NestingDepthMaximum = payPalOAuthResponseNestingMaximum
	limits.ObjectFieldMaximum = payPalOAuthResponseFieldMaximum
	limits.ArrayItemMaximum = payPalOAuthResponseArrayItemMaximum
	return limits
}
