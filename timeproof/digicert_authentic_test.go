package timeproof

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestDigiCertVerifierLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := loadDigiCertAuthenticFixture(t)
	t.Run("positive authentic DigiCert response projects its closed authority", func(t *testing.T) {
		t.Parallel()

		got, gotErr := Verify(VerifyRequest{
			Response: fixture.response, Request: fixture.request,
			ExpectedDigest: fixture.digest,
		})
		if gotErr != nil {
			t.Fatalf("Verify(authentic DigiCert) error = %v, want nil", gotErr)
		}
		if got.Policy() != TimestampPolicyDigiCert ||
			got.Evidence().Authority() != AuthorityDigiCert {
			t.Fatalf(
				"Verify(authentic DigiCert) policy/authority = (%v, %v), want (%v, %v)",
				got.Policy(), got.Evidence().Authority(),
				TimestampPolicyDigiCert, AuthorityDigiCert,
			)
		}
		gotNanoseconds, gotInstantErr := got.Instant().Nanoseconds()
		const wantNanoseconds = int64(1785739051 * 1_000_000_000)
		if gotInstantErr != nil || gotNanoseconds != wantNanoseconds {
			t.Fatalf(
				"Verify(authentic DigiCert) instant = (%d, %v), want (%d, nil)",
				gotNanoseconds, gotInstantErr, wantNanoseconds,
			)
		}
	})

	t.Run("negative DigiCert response bound to FreeTSA is refused", func(t *testing.T) {
		t.Parallel()

		wrongRequest, err := newRequest(
			fixture.digest,
			fixture.request.Nonce(),
			AuthorityFreeTSA,
		)
		if err != nil {
			t.Fatalf("newRequest(FreeTSA mismatch) error = %v, want nil", err)
		}
		got, gotErr := Verify(VerifyRequest{
			Response: fixture.response, Request: wrongRequest,
			ExpectedDigest: fixture.digest,
		})
		if !errors.Is(gotErr, core.ErrTimeProofInvalid) || !got.isZero() {
			t.Fatalf(
				"Verify(DigiCert response as FreeTSA) = (%v, %v), want (zero, %v)",
				got, gotErr, core.ErrTimeProofInvalid,
			)
		}
	})

	t.Run("neutral unknown authority is refused before request construction", func(t *testing.T) {
		t.Parallel()

		got, gotErr := Prepare(PrepareRequest{Digest: fixture.digest})
		if !errors.Is(gotErr, core.ErrTimeProofContract) ||
			len(got.body) != 0 || got.authority != AuthorityUnknown {
			t.Fatalf(
				"Prepare(unknown authority) = (%v, %v), want (zero, %v)",
				got, gotErr, core.ErrTimeProofContract,
			)
		}
	})
}

func TestDigiCertAuthorityOwnsEndpointPolicyAndPinnedRoot(t *testing.T) {
	t.Parallel()

	endpoint, err := AuthorityDigiCert.Endpoint()
	if err != nil {
		t.Fatalf("AuthorityDigiCert.Endpoint() error = %v, want nil", err)
	}
	const wantEndpoint = "http://timestamp.digicert.com"
	if endpoint.String() != wantEndpoint {
		t.Fatalf(
			"AuthorityDigiCert.Endpoint() = %q, want %q",
			endpoint.String(), wantEndpoint,
		)
	}
	contract, err := authorityRegistry(AuthorityDigiCert)
	if err != nil {
		t.Fatalf("authorityRegistry(DigiCert) error = %v, want nil", err)
	}
	if contract.policy != TimestampPolicyDigiCert {
		t.Fatalf(
			"authorityRegistry(DigiCert).policy = %v, want %v",
			contract.policy, TimestampPolicyDigiCert,
		)
	}
	if err := verifyDigiCertRoot(contract.root); err != nil {
		t.Fatalf("verifyDigiCertRoot(pinned root) error = %v, want nil", err)
	}
	if err := verifyDigiCertRoot(nil); !errors.Is(err, core.ErrTimeProofInvalid) {
		t.Fatalf("verifyDigiCertRoot(nil) error = %v, want %v", err, core.ErrTimeProofInvalid)
	}
	mutated := *contract.root
	mutated.Raw = append([]byte(nil), contract.root.Raw...)
	mutated.Raw[len(mutated.Raw)-1] ^= 0x01
	if err := verifyDigiCertRoot(&mutated); !errors.Is(err, core.ErrTimeProofInvalid) {
		t.Fatalf("verifyDigiCertRoot(mutated) error = %v, want %v", err, core.ErrTimeProofInvalid)
	}
}

func loadDigiCertAuthenticFixture(t testing.TB) authenticFixture {
	t.Helper()

	encoded, err := authenticFixtureFiles.ReadFile("testdata/digicert_2026_response.b64")
	if err != nil {
		t.Fatalf("read authentic DigiCert response fixture error = %v, want nil", err)
	}
	response := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	count, err := base64.StdEncoding.Decode(response, encoded)
	if err != nil {
		t.Fatalf("decode authentic DigiCert response fixture error = %v, want nil", err)
	}
	response = response[:count]
	nonce, err := parseNonce("f3a9697c07c4ec264d5db21ed77e3d73")
	if err != nil {
		t.Fatalf("parseNonce(authentic DigiCert) error = %v, want nil", err)
	}
	digest := core.NewSHA256Digest([sha256.Size]byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
		0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f,
	})
	request, err := newRequest(digest, nonce, AuthorityDigiCert)
	if err != nil {
		t.Fatalf("newRequest(authentic DigiCert) error = %v, want nil", err)
	}
	evidence, err := newAuthorityEvidence(authorityEvidenceInput{
		Request: request, Response: response,
	})
	if err != nil {
		t.Fatalf("newAuthorityEvidence(authentic DigiCert) error = %v, want nil", err)
	}
	return authenticFixture{
		response: response, evidence: evidence, request: request, digest: digest,
	}
}
