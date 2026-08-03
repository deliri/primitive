package attest_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

type verifyMutation uint8

const (
	verifyMutationNone verifyMutation = iota
	verifyMutationBodyBytes
	verifyMutationBodyExtent
	verifyMutationBodyDomain
	verifyMutationEnvelopeDomain
	verifyMutationEnvelopeSigner
	verifyMutationEnvelopeLength
	verifyMutationEnvelopeDigest
	verifyMutationEnvelopeSignature
	verifyMutationUntrustedSigner
	verifyMutationZeroTrust
	verifyMutationZeroEnvelope
	verifyMutationNilBody
)

func TestVerifyPublicProductionPathHostileMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr      error
		wantContract error
		name         string
		body         []byte
		trustedIndex int
		mutation     verifyMutation
		domain       testDomain
	}{
		{name: "one byte body verifies with signer first", body: []byte("a"), domain: testDomainPrimary},
		{name: "binary body verifies with signer second", body: []byte{0, 1, 2, 0}, domain: testDomainPrimary, trustedIndex: 1},
		{name: "utf8 bytes verify without interpretation", body: []byte("世界"), domain: testDomainPrimary, trustedIndex: 2},
		{name: "alternate domain verifies", body: []byte("alternate"), domain: testDomainAlternate},
		{name: "page body verifies", body: make([]byte, 4096), domain: testDomainPrimary, trustedIndex: 3},
		{name: "prime extent verifies", body: make([]byte, 7919), domain: testDomainPrimary, trustedIndex: 4},
		{name: "sixty four kibibytes verifies", body: make([]byte, 64<<10), domain: testDomainPrimary, trustedIndex: 5},
		{name: "maximum minus one verifies", body: make([]byte, attest.CanonicalBodyMaximumBytes-1), domain: testDomainPrimary},
		{name: "exact maximum verifies", body: make([]byte, attest.CanonicalBodyMaximumBytes), domain: testDomainPrimary},
		{name: "sixteenth trust position verifies", body: []byte("last trusted key"), domain: testDomainPrimary, trustedIndex: attest.TrustedKeyMaximumCount - 1},
		{name: "same extent changed body rejects", body: []byte("body"), domain: testDomainPrimary, mutation: verifyMutationBodyBytes, wantErr: core.ErrAttestVerification},
		{name: "changed body extent rejects", body: []byte("body"), domain: testDomainPrimary, mutation: verifyMutationBodyExtent, wantErr: core.ErrAttestVerification},
		{name: "changed body domain rejects", body: []byte("body"), domain: testDomainPrimary, mutation: verifyMutationBodyDomain, wantErr: core.ErrAttestVerification},
		{name: "changed envelope domain rejects", body: []byte("body"), domain: testDomainPrimary, mutation: verifyMutationEnvelopeDomain, wantErr: core.ErrAttestVerification},
		{name: "changed envelope signer rejects", body: []byte("body"), domain: testDomainPrimary, mutation: verifyMutationEnvelopeSigner, wantErr: core.ErrAttestVerification},
		{name: "changed envelope length rejects", body: []byte("body"), domain: testDomainPrimary, mutation: verifyMutationEnvelopeLength, wantErr: core.ErrAttestVerification},
		{name: "changed envelope digest rejects", body: []byte("body"), domain: testDomainPrimary, mutation: verifyMutationEnvelopeDigest, wantErr: core.ErrAttestVerification},
		{name: "changed envelope signature rejects", body: []byte("body"), domain: testDomainPrimary, mutation: verifyMutationEnvelopeSignature, wantErr: core.ErrAttestVerification},
		{name: "valid untrusted signer rejects", body: []byte("body"), domain: testDomainPrimary, mutation: verifyMutationUntrustedSigner, wantErr: core.ErrAttestVerification},
		{name: "zero trust set rejects structurally", body: []byte("body"), domain: testDomainPrimary, mutation: verifyMutationZeroTrust, wantErr: core.ErrAttestContract, wantContract: core.ErrAttestContract},
		{name: "zero envelope rejects structurally", body: []byte("body"), domain: testDomainPrimary, mutation: verifyMutationZeroEnvelope, wantErr: core.ErrAttestContract, wantContract: core.ErrAttestContract},
		{name: "nil body rejects structurally", body: []byte("body"), domain: testDomainPrimary, mutation: verifyMutationNilBody, wantErr: core.ErrAttestContract, wantContract: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotRequest := verificationFixture(t, tc.body, tc.domain, tc.trustedIndex)
			applyVerifyMutation(t, &gotRequest, tc.mutation)
			gotVerified, gotErr := attest.Verify(gotRequest)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("attest.Verify() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantContract != nil && !errors.Is(gotErr, tc.wantContract) {
				t.Fatalf("attest.Verify() contract identity = %v, want %v", gotErr, tc.wantContract)
			}
			if tc.wantErr != nil {
				if gotVerified != (attest.Verified[testDomain]{}) {
					t.Fatalf("attest.Verify() proof = %+v, want zero", gotVerified)
				}
				return
			}
			gotEnvelope, gotEnvelopeErr := gotVerified.Envelope()
			if gotEnvelopeErr != nil {
				t.Fatalf("Verified.Envelope() error = %v, want nil", gotEnvelopeErr)
			}
			if gotEnvelope != gotRequest.Envelope {
				t.Fatalf("Verified.Envelope() = %+v, want %+v", gotEnvelope, gotRequest.Envelope)
			}
		})
	}
}

func TestAttestVerifierLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive signed terminal envelope verifies exactly", func(t *testing.T) {
		t.Parallel()

		request := verificationFixture(t, []byte("terminal evidence"), testDomainPrimary, 0)
		gotVerified, gotErr := attest.Verify(request)
		if gotErr != nil {
			t.Fatalf("attest.Verify() error = %v, want nil", gotErr)
		}
		gotEnvelope, gotEnvelopeErr := gotVerified.Envelope()
		if gotEnvelopeErr != nil || gotEnvelope != request.Envelope {
			t.Fatalf(
				"Verified.Envelope() = (%+v, %v), want (%+v, nil)",
				gotEnvelope,
				gotEnvelopeErr,
				request.Envelope,
			)
		}
	})

	t.Run("negative forged terminal digest fails closed", func(t *testing.T) {
		t.Parallel()

		request := verificationFixture(t, []byte("terminal evidence"), testDomainPrimary, 0)
		request.Envelope.BodySHA256 = mutateDigest(t, request.Envelope.BodySHA256)
		gotVerified, gotErr := attest.Verify(request)
		if !errors.Is(gotErr, core.ErrAttestVerification) {
			t.Fatalf("attest.Verify() error = %v, want %v", gotErr, core.ErrAttestVerification)
		}
		if gotVerified != (attest.Verified[testDomain]{}) {
			t.Fatalf("attest.Verify() proof = %+v, want zero", gotVerified)
		}
	})

	t.Run("neutral absent body creates no envelope or proof", func(t *testing.T) {
		t.Parallel()

		privateKey := deterministicPrivateKey(t, "layer-triad-neutral")
		gotEnvelope, gotSignErr := attest.Sign(attest.SignRequest[testDomain]{
			Body:   nil,
			Signer: privateKey,
		})
		if !errors.Is(gotSignErr, core.ErrAttestContract) {
			t.Fatalf("attest.Sign() error = %v, want %v", gotSignErr, core.ErrAttestContract)
		}
		if gotEnvelope != (attest.Envelope[testDomain]{}) {
			t.Fatalf("attest.Sign() envelope = %+v, want zero", gotEnvelope)
		}
		gotVerified, gotVerifyErr := attest.Verify(attest.VerifyRequest[testDomain]{})
		if !errors.Is(gotVerifyErr, core.ErrAttestContract) {
			t.Fatalf("attest.Verify() error = %v, want %v", gotVerifyErr, core.ErrAttestContract)
		}
		if gotVerified != (attest.Verified[testDomain]{}) {
			t.Fatalf("attest.Verify() proof = %+v, want zero", gotVerified)
		}
	})
}

func verificationFixture(
	t testing.TB,
	body []byte,
	domain testDomain,
	trustedIndex int,
) attest.VerifyRequest[testDomain] {
	t.Helper()
	signer := deterministicPrivateKey(t, "verify-signer")
	envelope := mustEnvelope(t, literalBody{value: body, domain: domain}, signer)
	publicKeys := deterministicPublicKeys(t, attest.TrustedKeyMaximumCount)
	publicKeys[trustedIndex] = mustPublicKey(t, signer)
	trusted := mustTrustedKeys(t, publicKeys...)
	return attest.VerifyRequest[testDomain]{
		Body:        literalBody{value: append([]byte(nil), body...), domain: domain},
		Envelope:    envelope,
		TrustedKeys: trusted,
	}
}

func applyVerifyMutation(
	t testing.TB,
	request *attest.VerifyRequest[testDomain],
	mutation verifyMutation,
) {
	t.Helper()
	switch mutation {
	case verifyMutationNone:
		return
	case verifyMutationBodyBytes:
		body := copyLiteralBody(request.Body.(literalBody))
		body.value[0] ^= 1
		request.Body = body
	case verifyMutationBodyExtent:
		body := copyLiteralBody(request.Body.(literalBody))
		body.value = append(body.value, 0)
		request.Body = body
	case verifyMutationBodyDomain:
		body := copyLiteralBody(request.Body.(literalBody))
		body.domain = alternateTestDomain(body.domain)
		request.Body = body
	case verifyMutationEnvelopeDomain:
		request.Envelope.Domain = alternateTestDomain(request.Envelope.Domain)
	case verifyMutationEnvelopeSigner:
		originalSigner := request.Envelope.Signer
		request.Envelope.Signer = mustPublicKey(t, deterministicPrivateKey(t, "different-signer"))
		request.TrustedKeys = mustTrustedKeys(t, originalSigner, request.Envelope.Signer)
	case verifyMutationEnvelopeLength:
		request.Envelope.BodyLength = mutateLength(t, request.Envelope.BodyLength)
	case verifyMutationEnvelopeDigest:
		request.Envelope.BodySHA256 = mutateDigest(t, request.Envelope.BodySHA256)
	case verifyMutationEnvelopeSignature:
		request.Envelope.Signature = mutateSignature(t, request.Envelope.Signature)
	case verifyMutationUntrustedSigner:
		other := deterministicPrivateKey(t, "untrusted-valid-signer")
		request.Envelope = mustEnvelope(t, request.Body, other)
	case verifyMutationZeroTrust:
		request.TrustedKeys = attest.TrustedKeys{}
	case verifyMutationZeroEnvelope:
		request.Envelope = attest.Envelope[testDomain]{}
	case verifyMutationNilBody:
		request.Body = nil
	default:
		t.Fatalf("verify mutation = %d, want admitted mutation", mutation)
	}
}

func alternateTestDomain(domain testDomain) testDomain {
	if domain == testDomainPrimary {
		return testDomainAlternate
	}
	return testDomainPrimary
}
