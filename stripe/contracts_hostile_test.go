package stripe

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	stripesdk "github.com/stripe/stripe-go/v86"
)

func TestPinnedStripeAPIVersionMatchesOfficialSDK(t *testing.T) {
	t.Parallel()

	if got, want := core.StripeAPIVersion, stripesdk.APIVersion; got != want {
		t.Fatalf("core.StripeAPIVersion = %q, want official SDK version %q", got, want)
	}
}

func TestCredentialHostileBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "restricted prefix with one material byte", value: "rk_a"},
		{name: "secret prefix with one material byte", value: "sk_a"},
		{name: "exact custody maximum", value: "rk_" + strings.Repeat("a", core.StripeCredentialCustodyMaximumBytes-3)},
		{name: "empty rejected", wantErr: core.ErrStripeContract},
		{name: "prefix without material rejected", value: "rk_", wantErr: core.ErrStripeContract},
		{name: "publishable prefix rejected", value: "pk_a", wantErr: core.ErrStripeContract},
		{name: "whitespace rejected", value: "rk_a b", wantErr: core.ErrStripeContract},
		{name: "one above custody maximum", value: "sk_" + strings.Repeat("a", core.StripeCredentialCustodyMaximumBytes-2), wantErr: core.ErrStripeContract},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseCredential([]byte(testCase.value))
			if testCase.wantErr != nil {
				if got.Validate() == nil || !errors.Is(err, testCase.wantErr) {
					t.Fatalf("ParseCredential() = (%v, %v), want rejected with %v", got, err, testCase.wantErr)
				}
				return
			}
			if err != nil || got.Validate() != nil {
				t.Fatalf("ParseCredential() error = %v validation = %v, want nil", err, got.Validate())
			}
			if closeErr := got.Close(); closeErr != nil || got.Validate() == nil {
				t.Fatalf("Credential.Close() = %v validation = %v, want destroyed", closeErr, got.Validate())
			}
		})
	}
}

func TestWebhookSecretHostileBoundaries(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "exact documented prefix plus material", value: "whsec_a"},
		{name: "exact custody maximum", value: "whsec_" + strings.Repeat("a", core.StripeWebhookSecretCustodyMaximumBytes-6)},
		{name: "prefix alone rejected", value: "whsec_", wantErr: core.ErrStripeContract},
		{name: "wrong prefix rejected", value: "secret_a", wantErr: core.ErrStripeContract},
		{name: "one above custody maximum", value: "whsec_" + strings.Repeat("a", core.StripeWebhookSecretCustodyMaximumBytes-5), wantErr: core.ErrStripeContract},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseWebhookSecret([]byte(testCase.value))
			if testCase.wantErr != nil && !errors.Is(err, testCase.wantErr) {
				t.Fatalf("ParseWebhookSecret() error = %v, want %v", err, testCase.wantErr)
			}
			if testCase.wantErr == nil && (err != nil || got.Validate() != nil) {
				t.Fatalf("ParseWebhookSecret() error = %v validation = %v, want nil", err, got.Validate())
			}
		})
	}
}
