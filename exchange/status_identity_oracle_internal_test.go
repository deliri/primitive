package exchange

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The independent agreement table partitions the finite admitted status
// domain. It never calls StatusError.Unwrap or a production classifier.
func statusIdentityOracle(raw int) core.ErrorIdentity {
	var identities [600]core.ErrorIdentity
	for i := range identities {
		identities[i] = core.ErrExchangeResponse
	}
	for i := 400; i < 500; i++ {
		identities[i] = core.ErrExchangeRejected
	}
	for i := 500; i < 600; i++ {
		identities[i] = core.ErrExchangeUnavailable
	}
	identities[401], identities[403] = core.ErrExchangeAuthentication, core.ErrExchangeAuthentication
	identities[408], identities[425] = core.ErrExchangeUnavailable, core.ErrExchangeUnavailable
	identities[429] = core.ErrExchangeRateLimited
	if raw < 0 || raw >= len(identities) {
		return core.ErrExchangeResponse
	}
	return identities[raw]
}

func TestStatusErrorHasExactlyOneProviderFailureIdentity(t *testing.T) {
	t.Parallel()
	for raw := 100; raw < 600; raw++ {
		var status core.HTTPStatusCode
		if err := status.AdmitInt(raw); err != nil {
			continue
		}
		got := StatusError{status: status, expected: core.HTTPStatusOK()}
		want := statusIdentityOracle(raw)
		for _, identity := range [...]core.ErrorIdentity{core.ErrExchangeResponse, core.ErrExchangeAuthentication, core.ErrExchangeRateLimited, core.ErrExchangeUnavailable, core.ErrExchangeRejected} {
			if errors.Is(got, identity) != (identity == want) {
				t.Fatalf("status %d identity %v = %t, want %t", raw, identity, errors.Is(got, identity), identity == want)
			}
		}
		if want != core.ErrExchangeResponse && errors.Is(got, core.ErrExchangeContract) {
			t.Fatalf("provider status %d = %v, want no malformed local contract identity", raw, got)
		}
	}
}
