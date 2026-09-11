package core

import (
	"errors"
	"testing"
)

// Runtime graph and readiness policy belong to products. This deletion ratchet
// prevents the retired package from being admitted as a shared agreement again.
func TestRetiredRuntimeGraphPackageIsNotAnAgreement(t *testing.T) {
	t.Parallel()
	got, gotErr := ParsePackageIdentity("wiring")
	if got != PackageUnknown || !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf("ParsePackageIdentity(retired package) = (%v, %v), want unknown and %v", got, gotErr, ErrPrimitiveContract)
	}
	before := PackageCore
	got = before
	gotErr = got.UnmarshalJSON([]byte(`"wiring"`))
	if got != before || !errors.Is(gotErr, ErrJSONContract) || !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf("PackageIdentity.UnmarshalJSON(retired package) = (%v, %v), want preserved %v and typed JSON/Primitive refusal", got, gotErr, before)
	}
	for contract := range PrimitiveArchitecture().Packages() {
		if contract.Identity.String() == "wiring" {
			t.Fatalf("catalog contains retired package %v, want absence", contract.Identity)
		}
	}
}
