package timeproof

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

func timeproofRefusalJSONFixture(t testing.TB, fixtures timeproofFuzzFixtures) []byte {
	t.Helper()
	status, err := RefusalStatusRejection.rfcValue()
	if err != nil {
		t.Fatalf("RefusalStatusRejection.rfcValue() error = %v, want nil", err)
	}
	// Standard-library ASN.1 builds the provider-owned unsigned refusal. Primitive
	// emits the enclosing typed custody JSON through its real marshaler.
	response := encodeSequence(encodeSequence(encodeStatusInteger(t, status)))
	evidence, err := newAuthorityEvidence(authorityEvidenceInput{Request: fixtures.request, Response: response})
	if err != nil {
		t.Fatalf("newAuthorityEvidence(refusal) error = %v, want nil", err)
	}
	proof, err := Verify(VerifyRequest{Request: fixtures.request, Response: response, ExpectedDigest: fixtures.request.Digest()})
	var refusal Refusal
	if !errors.Is(err, core.ErrTimeProofRefused) || !errors.As(err, &refusal) || refusal.Status() != RefusalStatusRejection || !timestampHasNoProof(proof) {
		t.Fatalf("Verify(provider refusal) = (%v, %v, %+v), want zero and typed rejection", proof, err, refusal)
	}
	encoded, err := evidence.MarshalJSON()
	if err != nil {
		t.Fatalf("AuthorityEvidence.MarshalJSON(refusal) error = %v, want nil", err)
	}
	return encoded
}

func TestEvidenceRefusalFuzzOraclePreservesProviderIdentity(t *testing.T) {
	t.Parallel()
	fixtures := timeproofFixturesForFuzz(t)
	fuzzTimeproofAuthorityEvidence(t, timeproofRefusalJSONFixture(t, fixtures), fixtures)
}
