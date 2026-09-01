package providerwire

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzParseStripeCredentialSemanticBoundary(f *testing.F) {
	f.Add([]byte("rk_test_123"))
	f.Add([]byte("sk_test_123"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		credential, gotErr := ParseStripeCredential(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrProviderWireContract) || credential.Validate() == nil {
				t.Fatalf("ParseStripeCredential(rejected) = (%v, %v), want zero and %v", credential, gotErr, core.ErrProviderWireContract)
			}
			return
		}
		mutateFirstByte(data)
		if gotValidateErr := credential.Validate(); gotValidateErr != nil {
			t.Fatalf("ParseStripeCredential(accepted).Validate() error = %v, want nil after source mutation", gotValidateErr)
		}
		if gotCloseErr := credential.Close(); gotCloseErr != nil || credential.Validate() == nil {
			t.Fatalf("StripeCredential.Close() = %v, post-close Validate = %v, want nil then rejection", gotCloseErr, credential.Validate())
		}
	})
}

func FuzzParsePlunkCredentialSemanticBoundary(f *testing.F) {
	f.Add([]byte("sk_test_123"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		credential, gotErr := ParsePlunkCredential(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrProviderWireContract) || credential.Validate() == nil {
				t.Fatalf("ParsePlunkCredential(rejected) = (%v, %v), want zero and %v", credential, gotErr, core.ErrProviderWireContract)
			}
			return
		}
		mutateFirstByte(data)
		if gotValidateErr := credential.Validate(); gotValidateErr != nil {
			t.Fatalf("ParsePlunkCredential(accepted).Validate() error = %v, want nil after source mutation", gotValidateErr)
		}
		if gotCloseErr := credential.Close(); gotCloseErr != nil || credential.Validate() == nil {
			t.Fatalf("PlunkCredential.Close() = %v, post-close Validate = %v, want nil then rejection", gotCloseErr, credential.Validate())
		}
	})
}

func FuzzParsePlunkWebhookSecretSemanticBoundary(f *testing.F) {
	f.Add([]byte("plunk-shared-secret"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		secret, gotErr := ParsePlunkWebhookSecret(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrProviderWireContract) || secret.Validate() == nil {
				t.Fatalf("ParsePlunkWebhookSecret(rejected) = (%v, %v), want zero and %v", secret, gotErr, core.ErrProviderWireContract)
			}
			return
		}
		mutateFirstByte(data)
		if gotValidateErr := secret.Validate(); gotValidateErr != nil {
			t.Fatalf("ParsePlunkWebhookSecret(accepted).Validate() error = %v, want nil after source mutation", gotValidateErr)
		}
		if gotCloseErr := secret.Close(); gotCloseErr != nil || secret.Validate() == nil {
			t.Fatalf("PlunkWebhookSecret.Close() = %v, post-close Validate = %v, want nil then rejection", gotCloseErr, secret.Validate())
		}
	})
}

func FuzzParsePayPalAccessTokenSemanticBoundary(f *testing.F) {
	f.Add([]byte("A-._~+/="))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		token, gotErr := ParsePayPalAccessToken(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrProviderWireContract) || token.Validate() == nil {
				t.Fatalf("ParsePayPalAccessToken(rejected) = (%v, %v), want zero and %v", token, gotErr, core.ErrProviderWireContract)
			}
			return
		}
		mutateFirstByte(data)
		if gotValidateErr := token.Validate(); gotValidateErr != nil {
			t.Fatalf("ParsePayPalAccessToken(accepted).Validate() error = %v, want nil after source mutation", gotValidateErr)
		}
		if gotCloseErr := token.Close(); gotCloseErr != nil || token.Validate() == nil {
			t.Fatalf("PayPalAccessToken.Close() = %v, post-close Validate = %v, want nil then rejection", gotCloseErr, token.Validate())
		}
	})
}

func FuzzParseStripeWebhookSecretSemanticBoundary(f *testing.F) {
	f.Add([]byte("whsec_test_123"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		secret, gotErr := ParseStripeWebhookSecret(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrProviderWireContract) || secret.Validate() == nil {
				t.Fatalf("ParseStripeWebhookSecret(rejected) = (%v, %v), want zero and %v", secret, gotErr, core.ErrProviderWireContract)
			}
			return
		}
		mutateFirstByte(data)
		if gotValidateErr := secret.Validate(); gotValidateErr != nil {
			t.Fatalf("ParseStripeWebhookSecret(accepted).Validate() error = %v, want nil after source mutation", gotValidateErr)
		}
		if gotCloseErr := secret.Close(); gotCloseErr != nil || secret.Validate() == nil {
			t.Fatalf("StripeWebhookSecret.Close() = %v, post-close Validate = %v, want nil then rejection", gotCloseErr, secret.Validate())
		}
	})
}

func FuzzParseTwilioAuthTokenSemanticBoundary(f *testing.F) {
	f.Add([]byte("12345678901234567890123456789012"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		token, gotErr := ParseTwilioAuthToken(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrProviderWireContract) || token.Validate() == nil {
				t.Fatalf("ParseTwilioAuthToken(rejected) = (%v, %v), want zero and %v", token, gotErr, core.ErrProviderWireContract)
			}
			return
		}
		mutateFirstByte(data)
		if gotValidateErr := token.Validate(); gotValidateErr != nil {
			t.Fatalf("ParseTwilioAuthToken(accepted).Validate() error = %v, want nil after source mutation", gotValidateErr)
		}
		if gotCloseErr := token.Close(); gotCloseErr != nil || token.Validate() == nil {
			t.Fatalf("TwilioAuthToken.Close() = %v, post-close Validate = %v, want nil then rejection", gotCloseErr, token.Validate())
		}
	})
}

func FuzzParseTwilioAccountSIDSemanticBoundary(f *testing.F) {
	f.Add("AC0123456789abcdefABCDEF0123456789")
	f.Add("")

	f.Fuzz(func(t *testing.T, data string) {
		got, gotErr := ParseTwilioAccountSID(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrProviderWireContract) || got != "" {
				t.Fatalf("ParseTwilioAccountSID(rejected) = (%q, %v), want zero and %v", got, gotErr, core.ErrProviderWireContract)
			}
			return
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil || got.String() != data {
			t.Fatalf("ParseTwilioAccountSID(accepted) = (%q, %v), want exact input and nil", got, gotValidateErr)
		}
	})
}

func FuzzParseTwilioAPIKeySIDSemanticBoundary(f *testing.F) {
	f.Add(twilioAPIKeySIDTextForTest())
	f.Add("")

	f.Fuzz(func(t *testing.T, data string) {
		got, gotErr := ParseTwilioAPIKeySID(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrProviderWireContract) || got != "" {
				t.Fatalf("ParseTwilioAPIKeySID(rejected) = (%q, %v), want zero and %v", got, gotErr, core.ErrProviderWireContract)
			}
			return
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil || got.String() != data {
			t.Fatalf("ParseTwilioAPIKeySID(accepted) = (%q, %v), want exact input and nil", got, gotValidateErr)
		}
	})
}

func FuzzNewTwilioCredentialSemanticBoundary(f *testing.F) {
	f.Add("AC0123456789abcdefABCDEF0123456789", twilioAPIKeySIDTextForTest(), twilioAPIKeySecretForTest())
	f.Add("", "", []byte{})

	f.Fuzz(func(t *testing.T, accountText, keyText string, secret []byte) {
		account, accountErr := ParseTwilioAccountSID(accountText)
		key, keyErr := ParseTwilioAPIKeySID(keyText)
		credential, gotErr := NewTwilioCredential(account, key, secret)
		wantSecret := len(secret) == TwilioAPIKeySecretBytes
		for _, character := range secret {
			wantSecret = wantSecret && (character >= '0' && character <= '9' || character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z')
		}
		wantAccepted := accountErr == nil && keyErr == nil && wantSecret
		if !wantAccepted {
			if !errors.Is(errors.Join(accountErr, keyErr, gotErr), core.ErrProviderWireContract) || credential.Validate() == nil {
				t.Fatalf("NewTwilioCredential(rejected) = (%v, %v/%v/%v), want zero and %v", credential, accountErr, keyErr, gotErr, core.ErrProviderWireContract)
			}
			return
		}
		mutateFirstByte(secret)
		if gotErr != nil || credential.Validate() != nil || credential.AccountSID != account || credential.APIKeySID != key {
			t.Fatalf("NewTwilioCredential(accepted) = (%v, %v), want copied exact identity and nil", credential, gotErr)
		}
		if closeErr := credential.Close(); closeErr != nil || credential.Validate() == nil {
			t.Fatalf("TwilioCredential.Close() = %v, post-close Validate = %v, want nil then rejection", closeErr, credential.Validate())
		}
	})
}

func FuzzParsePayPalWebhookIDSemanticBoundary(f *testing.F) {
	f.Add("WH123456789")
	f.Add("")
	f.Add("WH-123456789")

	f.Fuzz(func(t *testing.T, data string) {
		got, gotErr := ParsePayPalWebhookID(data)
		wantAccepted := len(data) > 0 && len(data) <= PayPalWebhookIDMaximumBytes
		for index := range len(data) {
			wantAccepted = wantAccepted && (data[index] >= '0' && data[index] <= '9' || data[index] >= 'A' && data[index] <= 'Z' || data[index] >= 'a' && data[index] <= 'z')
		}
		if !wantAccepted {
			if !errors.Is(gotErr, core.ErrProviderWireContract) || got != "" {
				t.Fatalf("ParsePayPalWebhookID(rejected) = (%q, %v), want zero and %v", got, gotErr, core.ErrProviderWireContract)
			}
			return
		}
		gotValidateErr := got.Validate()
		if gotErr != nil || gotValidateErr != nil || got.String() != data {
			t.Fatalf("ParsePayPalWebhookID(accepted) = (%q, %v, validate:%v), want exact input and nil", got, gotErr, gotValidateErr)
		}
	})
}

func mutateFirstByte(data []byte) {
	if len(data) != 0 {
		data[0] ^= 0xff
	}
}
