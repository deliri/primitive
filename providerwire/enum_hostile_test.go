package providerwire

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestProviderWireClosedEnumsExhaustEveryUint8State(t *testing.T) {
	t.Parallel()

	for raw := range 256 {
		provider := Provider(raw)
		wantProviderValid := provider > ProviderUnknown && provider < providerLimit
		if provider.IsValid() != wantProviderValid || (provider.String() != "") != wantProviderValid {
			t.Fatalf("Provider(%d) = valid:%t text:%q, want valid:%t with matching diagnostic presence", raw, provider.IsValid(), provider.String(), wantProviderValid)
		}

		representation := TwilioWebhookRepresentation(raw)
		wantRepresentationValid := representation > TwilioWebhookRepresentationUnknown && representation < twilioWebhookRepresentationLimit
		if representation.IsValid() != wantRepresentationValid || (representation.String() != "") != wantRepresentationValid {
			t.Fatalf("TwilioWebhookRepresentation(%d) = valid:%t text:%q, want valid:%t with matching diagnostic presence", raw, representation.IsValid(), representation.String(), wantRepresentationValid)
		}
		if wantRepresentationValid {
			representation.OffWireEnum()
		}

		credentialKind := StripeCredentialKind(raw)
		wantCredentialValid := credentialKind > StripeCredentialUnknown && credentialKind < stripeCredentialLimit
		if credentialKind.IsValid() != wantCredentialValid || (credentialKind.String() != "") != wantCredentialValid {
			t.Fatalf("StripeCredentialKind(%d) = valid:%t text:%q, want valid:%t with matching diagnostic presence", raw, credentialKind.IsValid(), credentialKind.String(), wantCredentialValid)
		}

		status := PayPalWebhookVerificationStatus(raw)
		wantStatusValid := status > PayPalWebhookVerificationUnknown && status < payPalWebhookVerificationStatusLimit
		if status.IsValid() != wantStatusValid || (status.String() != "") != wantStatusValid {
			t.Fatalf("PayPalWebhookVerificationStatus(%d) = valid:%t text:%q, want valid:%t with matching wire presence", raw, status.IsValid(), status.String(), wantStatusValid)
		}
	}
}

func FuzzPayPalWebhookVerificationStatusJSONSemanticClosure(f *testing.F) {
	for status := PayPalWebhookVerificationSuccess; status < payPalWebhookVerificationStatusLimit; status++ {
		encoded, err := status.MarshalJSON()
		if err != nil {
			f.Fatalf("PayPalWebhookVerificationStatus.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add([]byte(`"UNKNOWN"`))
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		before := PayPalWebhookVerificationSuccess
		got := before
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
				t.Fatalf("PayPalWebhookVerificationStatus.UnmarshalJSON(rejected) = (%v, %v), want preserved %v and %v", got, gotErr, before, core.ErrJSONContract)
			}
			return
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil {
			t.Fatalf("PayPalWebhookVerificationStatus.UnmarshalJSON(accepted).Validate() error = %v, want nil", gotValidateErr)
		}
		encoded, gotEncodeErr := got.MarshalJSON()
		if gotEncodeErr != nil {
			t.Fatalf("PayPalWebhookVerificationStatus.MarshalJSON(accepted) error = %v, want nil", gotEncodeErr)
		}
		var roundTrip PayPalWebhookVerificationStatus
		gotRoundTripErr := roundTrip.UnmarshalJSON(encoded)
		if gotRoundTripErr != nil || roundTrip != got {
			t.Fatalf("PayPal webhook status canonical round trip = (%v, %v), want (%v, nil)", roundTrip, gotRoundTripErr, got)
		}
	})
}
