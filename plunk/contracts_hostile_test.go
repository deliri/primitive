package plunk

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCredentialHostileBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "documented secret prefix admitted", value: "sk_a"},
		{name: "exact custody maximum", value: "sk_" + strings.Repeat("a", core.PlunkCredentialCustodyMaximumBytes-3)},
		{name: "public key rejected at server boundary", value: "pk_a", wantErr: core.ErrPlunkContract},
		{name: "prefix only rejected", value: "sk_", wantErr: core.ErrPlunkContract},
		{name: "control byte rejected", value: "sk_a\n", wantErr: core.ErrPlunkContract},
		{name: "one above custody maximum", value: "sk_" + strings.Repeat("a", core.PlunkCredentialCustodyMaximumBytes-2), wantErr: core.ErrPlunkContract},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseCredential([]byte(testCase.value))
			if testCase.wantErr != nil {
				if !errors.Is(err, testCase.wantErr) || got.Validate() == nil {
					t.Fatalf("ParseCredential() = (%v, %v), want rejected with %v", got, err, testCase.wantErr)
				}
				return
			}
			if err != nil || got.Validate() != nil {
				t.Fatalf("ParseCredential() error = %v validation = %v, want nil", err, got.Validate())
			}
			_ = got.Close()
		})
	}
}

func TestWebhookSecretHasIndependentNominalBound(t *testing.T) {
	t.Parallel()
	secret, err := ParseWebhookSecret([]byte(strings.Repeat("w", core.PlunkWebhookSecretCustodyMaximumBytes)))
	if err != nil || secret.Validate() != nil {
		t.Fatalf("ParseWebhookSecret(exact maximum) error = %v validation = %v", err, secret.Validate())
	}
	_, err = ParseWebhookSecret([]byte(strings.Repeat("w", core.PlunkWebhookSecretCustodyMaximumBytes+1)))
	if !errors.Is(err, core.ErrPlunkContract) {
		t.Fatalf("ParseWebhookSecret(one above maximum) error = %v, want %v", err, core.ErrPlunkContract)
	}
}
