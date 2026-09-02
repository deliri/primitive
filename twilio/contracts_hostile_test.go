package twilio

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestSIDHostileBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr error
		name    string
		value   string
		account bool
	}{
		{name: "exact account SID", value: "AC" + strings.Repeat("a", 32), account: true},
		{name: "exact API key SID", value: "SK" + strings.Repeat("F", 32)},
		{name: "account one short", value: "AC" + strings.Repeat("a", 31), account: true, wantErr: core.ErrTwilioContract},
		{name: "account one long", value: "AC" + strings.Repeat("a", 33), account: true, wantErr: core.ErrTwilioContract},
		{name: "account wrong prefix", value: "SK" + strings.Repeat("a", 32), account: true, wantErr: core.ErrTwilioContract},
		{name: "key non-hex", value: "SK" + strings.Repeat("z", 32), wantErr: core.ErrTwilioContract},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			var err error
			if testCase.account {
				_, err = ParseAccountSID(testCase.value)
			} else {
				_, err = ParseAPIKeySID(testCase.value)
			}
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("SID parse error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestCredentialSecretHasIndependentBoundedCustody(t *testing.T) {
	t.Parallel()
	account, _ := ParseAccountSID("AC" + strings.Repeat("a", 32))
	key, _ := ParseAPIKeySID("SK" + strings.Repeat("b", 32))
	for _, testCase := range []struct {
		wantErr error
		name    string
		secret  string
	}{
		{name: "minimum nonempty custody", secret: "c"},
		{name: "exact custody ceiling", secret: strings.Repeat("c", core.TwilioAPIKeySecretCustodyMaximumBytes)},
		{name: "empty secret", wantErr: core.ErrTwilioContract},
		{name: "one beyond custody ceiling", secret: strings.Repeat("c", core.TwilioAPIKeySecretCustodyMaximumBytes+1), wantErr: core.ErrTwilioContract},
		{name: "space is not credential material", secret: "secret value", wantErr: core.ErrTwilioContract},
		{name: "visible punctuation remains opaque", secret: "secret-value!"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			credential, err := NewCredential(account, key, []byte(testCase.secret))
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("NewCredential() error = %v, want %v", err, testCase.wantErr)
			}
			if err == nil {
				_ = credential.Close()
			}
		})
	}
}
