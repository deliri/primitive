package loops

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestProviderErrorRetainsLoopsIdentityWithoutDiagnosticCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("native diagnostic")
	cases := []struct {
		got   error
		want  error
		cause error
		name  string
	}{
		{name: "contract identity survives absent diagnostic", got: contractError(nil), want: core.ErrLoopsContract},
		{name: "authentication identity survives absent diagnostic", got: authenticationError(nil), want: core.ErrLoopsAuthentication},
		{name: "verification identity survives absent diagnostic", got: verificationError(nil), want: core.ErrLoopsVerification},
		{name: "binding identity survives absent diagnostic", got: bindingError(nil), want: core.ErrLoopsBinding},
		{name: "binding identity retains native cause", got: bindingError(cause), want: core.ErrLoopsBinding, cause: cause},
		{name: "verification identity retains native cause", got: verificationError(cause), want: core.ErrLoopsVerification, cause: cause},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !errors.Is(tc.got, tc.want) || !errors.Is(tc.got, core.ErrLoopsContract) {
				t.Fatalf("provider error = %v, want errors.Is(..., %v) and the Loops contract parent", tc.got, tc.want)
			}
			if tc.cause != nil && !errors.Is(tc.got, tc.cause) {
				t.Fatalf("provider error = %v, want retained native cause %v", tc.got, tc.cause)
			}
			if errors.Is(tc.got, core.ErrPlunkContract) || errors.Is(tc.got, core.ErrStripeContract) {
				t.Fatalf("provider error = %v, want no foreign provider family", tc.got)
			}
		})
	}
}
