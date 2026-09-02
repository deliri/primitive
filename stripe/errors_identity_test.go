package stripe

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestProviderErrorRetainsStripeIdentityWithoutDiagnosticCause(t *testing.T) {
	t.Parallel()

	cases := []struct {
		got  error
		want error
		name string
	}{
		{name: "contract identity survives absent diagnostic", got: contractError(nil), want: core.ErrStripeContract},
		{name: "authentication identity survives absent diagnostic", got: authenticationError(nil), want: core.ErrStripeAuthentication},
		{name: "verification identity survives absent diagnostic", got: verificationError(nil), want: core.ErrStripeVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tc.got, tc.want) {
				t.Fatalf("provider error = %v, want errors.Is(..., %v)", tc.got, tc.want)
			}
		})
	}
}
