package standard

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
)

// TestDeriveRequestIdentityBindsCallerNonceToOrigin is a contract ratchet for
// the previously unpinned caller/server seam: the same retry facts must retain
// one identity, while either load-bearing fact must change it.
func TestDeriveRequestIdentityBindsCallerNonceToOrigin(t *testing.T) {
	t.Parallel()

	origin := OriginIdentity{Offering: core.Offering{Token: "blink-kernel"}}
	nonce := requestNonceFixture(t, "01890f2e-7b00-7000-8000-000000000001")
	first, firstErr := DeriveRequestIdentity(origin, nonce)
	second, secondErr := DeriveRequestIdentity(origin, nonce)
	if firstErr != nil || secondErr != nil || first != second {
		t.Fatalf("DeriveRequestIdentity(same origin and nonce) = (%v, %v) then (%v, %v), want one stable identity", first, firstErr, second, secondErr)
	}

	differentOrigin, originErr := DeriveRequestIdentity(OriginIdentity{Offering: core.Offering{Token: "forge"}}, nonce)
	if originErr != nil || differentOrigin == first {
		t.Fatalf("DeriveRequestIdentity(one-fact origin mutation) = (%v, %v), want identity different from %v", differentOrigin, originErr, first)
	}

	differentNonce, nonceErr := DeriveRequestIdentity(origin, requestNonceFixture(t, "01890f2e-7b00-7000-8000-000000000002"))
	if nonceErr != nil || differentNonce == first {
		t.Fatalf("DeriveRequestIdentity(one-fact nonce mutation) = (%v, %v), want identity different from %v", differentNonce, nonceErr, first)
	}

	if got, gotErr := DeriveRequestIdentity(OriginIdentity{}, nonce); got != (RequestIdentity{}) || !errors.Is(gotErr, core.ErrStandardContract) {
		t.Fatalf("DeriveRequestIdentity(zero origin) = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrStandardContract)
	}
	if got, gotErr := DeriveRequestIdentity(origin, RequestNonce{}); got != (RequestIdentity{}) || !errors.Is(gotErr, core.ErrStandardContract) {
		t.Fatalf("DeriveRequestIdentity(zero nonce) = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrStandardContract)
	}
}

func requestNonceFixture(t testing.TB, value string) RequestNonce {
	t.Helper()
	uuid, uuidErr := primitiveid.ParseUUIDv7(value)
	nonce, nonceErr := NewRequestNonce(uuid)
	if err := errors.Join(uuidErr, nonceErr); err != nil {
		t.Fatalf("NewRequestNonce(%q) setup error = %v, want nil", value, err)
	}
	return nonce
}
