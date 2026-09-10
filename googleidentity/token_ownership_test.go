package googleidentity

import (
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const googleTestBearerGrammar = `\A[-A-Za-z0-9._~+/]+=*\z`

func TestGoogleTokenOwnershipAndRedactionLayerTriad(t *testing.T) {
	t.Parallel()
	duration, err := temporal.DurationFromSeconds(300)
	if err != nil {
		t.Fatal(err)
	}
	access, err := newGoogleAccessToken(googleTestToken, duration)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func([]byte)
	}{
		{name: "source_prefix_mutation", mutate: func(b []byte) { b[0] = 'x' }},
		{name: "source_suffix_mutation", mutate: func(b []byte) { b[len(b)-1] = 'x' }},
		{name: "source_erasure", mutate: func(b []byte) { clear(b) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := []byte(googleTestToken)
			token, err := ParseGoogleCloudCommandOutput(input)
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(input)
			value, err := token.BearerValue()
			if err != nil || value != bearerPrefix+googleTestToken {
				t.Fatalf("token owns original=%t error=%v, want true and nil", value == bearerPrefix+googleTestToken, err)
			}
		})
	}
	token, err := ParseGoogleCloudCommandOutput([]byte(googleTestToken))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		value fmt.Formatter
	}{
		{"identity_bearer", token}, {"access_bearer", access}, {"unset_identity", Token{}}, {"unset_access", AccessToken{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d", "%f", "%.1s"} {
				if got := fmt.Sprintf(format, tc.value); got != core.RedactedValueText {
					t.Fatalf("format %s got=%q, want %q", format, got, core.RedactedValueText)
				}
			}
		})
	}
	for _, tc := range []struct {
		name   string
		reveal func() (string, error)
	}{
		{"unset_identity_disclosure", (Token{}).BearerValue}, {"unset_access_disclosure", (AccessToken{}).BearerValue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			value, err := tc.reveal()
			if value != "" || !errors.Is(err, core.ErrGoogleIdentityContract) {
				t.Fatalf("unset disclosure=%q error=%v, want empty typed refusal", value, err)
			}
		})
	}
}
