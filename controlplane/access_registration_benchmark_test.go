package controlplane_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
)

// Fixed six-field request, one trusted authority, one known verifier, no prior
// replay. This measures authentication plus exact replay commitment, not key
// generation, JSON input construction, networking or product policy.
func BenchmarkAccessRegistrationVerification(b *testing.B) {
	request, server := accessRegistrationFixture(b)
	verifier, err := request.Token.Verifier()
	if err != nil {
		b.Fatalf("Verifier() error = %v, want nil", err)
	}
	input := controlplane.AccessRegistrationVerification{Request: request, ExpectedVerifier: verifier}
	if err := input.Validate(); err != nil {
		b.Fatalf("input.Validate() error = %v, want nil", err)
	}
	canonical, err := request.MarshalJSON()
	if err != nil || len(canonical) == 0 {
		b.Fatalf("MarshalJSON(workload) = (%d bytes, %v), want nonempty and nil", len(canonical), err)
	}
	b.SetBytes(int64(len(canonical)))
	clear(canonical)
	wantIdentity, err := request.Identity()
	if err != nil {
		b.Fatalf("Identity() error = %v, want nil", err)
	}
	var observed controlplane.VerifiedRegistrationAuthority
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		observed, err = server.VerifyAccessRegistration(input)
		if err != nil {
			b.Fatalf("VerifyAccessRegistration() error = %v, want nil", err)
		}
	}
	b.StopTimer()
	gotIdentity, err := observed.Identity()
	if err != nil || gotIdentity != wantIdentity {
		b.Fatalf("observed identity = (%+v, %v), want (%+v, nil)", gotIdentity, err, wantIdentity)
	}
}
