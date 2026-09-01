package providerwire

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestProviderCredentialParsersHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   []byte
		parse   func([]byte) error
		wantErr error
	}{
		{name: "Stripe restricted credential ordinary extent", value: []byte("rk_test_123"), parse: parseStripeCredentialForTest},
		{name: "Stripe secret credential ordinary extent", value: []byte("sk_test_123"), parse: parseStripeCredentialForTest},
		{name: "Stripe credential exact minimum extent", value: []byte("rk_a"), parse: parseStripeCredentialForTest},
		{name: "Stripe credential unknown publishable prefix", value: []byte("pk_test_123"), parse: parseStripeCredentialForTest, wantErr: core.ErrProviderWireContract},
		{name: "Stripe credential one below minimum extent", value: []byte("rk_"), parse: parseStripeCredentialForTest, wantErr: core.ErrProviderWireContract},
		{name: "Stripe credential whitespace is rejected", value: []byte("rk_test key"), parse: parseStripeCredentialForTest, wantErr: core.ErrProviderWireContract},
		{name: "Stripe credential control byte is rejected", value: []byte("rk_test\nkey"), parse: parseStripeCredentialForTest, wantErr: core.ErrProviderWireContract},
		{name: "Stripe credential exact maximum extent", value: []byte("rk_" + strings.Repeat("a", StripeCredentialMaximumBytes-3)), parse: parseStripeCredentialForTest},
		{name: "Stripe credential one above maximum extent", value: []byte("rk_" + strings.Repeat("a", StripeCredentialMaximumBytes-2)), parse: parseStripeCredentialForTest, wantErr: core.ErrProviderWireContract},
		{name: "Plunk secret ordinary extent", value: []byte("sk_test_123"), parse: parsePlunkCredentialForTest},
		{name: "Plunk secret exact minimum extent", value: []byte("sk_a"), parse: parsePlunkCredentialForTest},
		{name: "Plunk secret rejects Stripe restricted prefix", value: []byte("rk_test_123"), parse: parsePlunkCredentialForTest, wantErr: core.ErrProviderWireContract},
		{name: "Plunk secret rejects empty input", value: []byte{}, parse: parsePlunkCredentialForTest, wantErr: core.ErrProviderWireContract},
		{name: "Plunk secret exact maximum extent", value: []byte("sk_" + strings.Repeat("p", PlunkCredentialMaximumBytes-3)), parse: parsePlunkCredentialForTest},
		{name: "Plunk secret one above maximum extent", value: []byte("sk_" + strings.Repeat("p", PlunkCredentialMaximumBytes-2)), parse: parsePlunkCredentialForTest, wantErr: core.ErrProviderWireContract},
		{name: "Plunk webhook bearer exact one-byte minimum", value: []byte("p"), parse: parsePlunkWebhookSecretForTest},
		{name: "Plunk webhook bearer exact custody ceiling", value: []byte(strings.Repeat("p", PlunkWebhookSecretMaximumBytes)), parse: parsePlunkWebhookSecretForTest},
		{name: "Plunk webhook bearer empty value", value: []byte{}, parse: parsePlunkWebhookSecretForTest, wantErr: core.ErrProviderWireContract},
		{name: "Plunk webhook bearer one above custody ceiling", value: []byte(strings.Repeat("p", PlunkWebhookSecretMaximumBytes+1)), parse: parsePlunkWebhookSecretForTest, wantErr: core.ErrProviderWireContract},
		{name: "PayPal access token admits documented bearer grammar", value: []byte("A-._~+/="), parse: parsePayPalTokenForTest},
		{name: "PayPal access token rejects whitespace", value: []byte("token value"), parse: parsePayPalTokenForTest, wantErr: core.ErrProviderWireContract},
		{name: "PayPal access token rejects empty input", value: []byte{}, parse: parsePayPalTokenForTest, wantErr: core.ErrProviderWireContract},
		{name: "PayPal access token exact custody ceiling", value: []byte(strings.Repeat("p", PayPalAccessTokenMaximumBytes)), parse: parsePayPalTokenForTest},
		{name: "PayPal access token one above custody ceiling", value: []byte(strings.Repeat("p", PayPalAccessTokenMaximumBytes+1)), parse: parsePayPalTokenForTest, wantErr: core.ErrProviderWireContract},
		{name: "Stripe webhook secret ordinary extent", value: []byte("whsec_test_123"), parse: parseStripeWebhookSecretForTest},
		{name: "Stripe webhook secret exact minimum extent", value: []byte("whsec_a"), parse: parseStripeWebhookSecretForTest},
		{name: "Stripe webhook secret prefix without material", value: []byte("whsec_"), parse: parseStripeWebhookSecretForTest, wantErr: core.ErrProviderWireContract},
		{name: "Stripe webhook secret exact custody ceiling", value: []byte("whsec_" + strings.Repeat("s", StripeWebhookSecretMaximumBytes-6)), parse: parseStripeWebhookSecretForTest},
		{name: "Stripe webhook secret one above custody ceiling", value: []byte("whsec_" + strings.Repeat("s", StripeWebhookSecretMaximumBytes-5)), parse: parseStripeWebhookSecretForTest, wantErr: core.ErrProviderWireContract},
		{name: "Stripe webhook secret wrong prefix", value: []byte("sk_test_123"), parse: parseStripeWebhookSecretForTest, wantErr: core.ErrProviderWireContract},
		{name: "Twilio auth token exact documented extent", value: []byte(strings.Repeat("a", TwilioAuthTokenBytes)), parse: parseTwilioAuthTokenForTest},
		{name: "Twilio auth token one below documented extent", value: []byte(strings.Repeat("a", TwilioAuthTokenBytes-1)), parse: parseTwilioAuthTokenForTest, wantErr: core.ErrProviderWireContract},
		{name: "Twilio auth token one above documented extent", value: []byte(strings.Repeat("a", TwilioAuthTokenBytes+1)), parse: parseTwilioAuthTokenForTest, wantErr: core.ErrProviderWireContract},
		{name: "Twilio auth token punctuation is rejected", value: []byte(strings.Repeat("a", TwilioAuthTokenBytes-1) + "-"), parse: parseTwilioAuthTokenForTest, wantErr: core.ErrProviderWireContract},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotErr := testCase.parse(append([]byte(nil), testCase.value...))
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("credential parser error = %v, want %v", gotErr, testCase.wantErr)
			}
		})
	}
}

func TestProviderIdentityParsersHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		kind    uint8
		wantErr error
	}{
		{name: "Twilio account SID lowercase hex", value: "AC" + strings.Repeat("a", 32), kind: 1},
		{name: "Twilio account SID uppercase hex", value: "AC" + strings.Repeat("F", 32), kind: 1},
		{name: "Twilio account SID mixed hex", value: "AC0123456789abcdefABCDEF0123456789", kind: 1},
		{name: "Twilio account SID one below extent", value: "AC" + strings.Repeat("a", 31), kind: 1, wantErr: core.ErrProviderWireContract},
		{name: "Twilio account SID one above extent", value: "AC" + strings.Repeat("a", 33), kind: 1, wantErr: core.ErrProviderWireContract},
		{name: "Twilio account SID key prefix mismatch", value: "SK" + strings.Repeat("a", 32), kind: 1, wantErr: core.ErrProviderWireContract},
		{name: "Twilio account SID nonhex suffix", value: "AC" + strings.Repeat("z", 32), kind: 1, wantErr: core.ErrProviderWireContract},
		{name: "Twilio API key SID lowercase hex", value: "SK" + strings.Repeat("b", 32), kind: 2},
		{name: "Twilio API key SID account prefix mismatch", value: "AC" + strings.Repeat("b", 32), kind: 2, wantErr: core.ErrProviderWireContract},
		{name: "PayPal webhook ID ordinary documented identity", value: "WH123456789", kind: 3},
		{name: "PayPal webhook ID exact one byte minimum", value: "a", kind: 3},
		{name: "PayPal webhook ID empty", value: "", kind: 3, wantErr: core.ErrProviderWireContract},
		{name: "PayPal webhook ID newline", value: "WH-123\n", kind: 3, wantErr: core.ErrProviderWireContract},
		{name: "PayPal webhook ID punctuation is rejected", value: "WH-123456789", kind: 3, wantErr: core.ErrProviderWireContract},
		{name: "PayPal webhook ID exact maximum extent", value: strings.Repeat("a", PayPalWebhookIDMaximumBytes), kind: 3},
		{name: "PayPal webhook ID one above maximum extent", value: strings.Repeat("a", PayPalWebhookIDMaximumBytes+1), kind: 3, wantErr: core.ErrProviderWireContract},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotErr := parseProviderIdentityForTest(testCase.kind, testCase.value)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("identity parser error = %v, want %v", gotErr, testCase.wantErr)
			}
		})
	}
}

func parseStripeCredentialForTest(value []byte) error {
	credential, err := ParseStripeCredential(value)
	if err != nil {
		return err
	}
	if got := fmt.Sprint(credential); got != core.RedactedValueText {
		return core.ErrProviderWireContract
	}
	return errors.Join(credential.Validate(), credential.Close())
}

func parsePlunkCredentialForTest(value []byte) error {
	credential, err := ParsePlunkCredential(value)
	if err != nil {
		return err
	}
	if got := fmt.Sprint(credential); got != core.RedactedValueText {
		return core.ErrProviderWireContract
	}
	return errors.Join(credential.Validate(), credential.Close())
}

func parsePlunkWebhookSecretForTest(value []byte) error {
	secret, err := ParsePlunkWebhookSecret(value)
	if err != nil {
		return err
	}
	if got := fmt.Sprint(secret); got != core.RedactedValueText {
		return core.ErrProviderWireContract
	}
	return errors.Join(secret.Validate(), secret.Close())
}

func parsePayPalTokenForTest(value []byte) error {
	token, err := ParsePayPalAccessToken(value)
	if err != nil {
		return err
	}
	if got := fmt.Sprint(token); got != core.RedactedValueText {
		return core.ErrProviderWireContract
	}
	return errors.Join(token.Validate(), token.Close())
}

func parseStripeWebhookSecretForTest(value []byte) error {
	secret, err := ParseStripeWebhookSecret(value)
	if err != nil {
		return err
	}
	if got := fmt.Sprint(secret); got != core.RedactedValueText {
		return core.ErrProviderWireContract
	}
	return errors.Join(secret.Validate(), secret.Close())
}

func parseTwilioAuthTokenForTest(value []byte) error {
	token, err := ParseTwilioAuthToken(value)
	if err != nil {
		return err
	}
	if got := fmt.Sprint(token); got != core.RedactedValueText {
		return core.ErrProviderWireContract
	}
	return errors.Join(token.Validate(), token.Close())
}

func parseProviderIdentityForTest(kind uint8, value string) error {
	switch kind {
	case 1:
		_, err := ParseTwilioAccountSID(value)
		return err
	case 2:
		_, err := ParseTwilioAPIKeySID(value)
		return err
	case 3:
		_, err := ParsePayPalWebhookID(value)
		return err
	}
	return core.ErrProviderWireContract
}

func twilioAPIKeySIDTextForTest() string {
	return "SK" + strings.Repeat("a", 32)
}

func twilioAPIKeySecretForTest() []byte {
	return []byte(strings.Repeat("s", TwilioAPIKeySecretBytes))
}
