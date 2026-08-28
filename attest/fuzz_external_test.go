package attest_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

type signedFieldMutation uint8

const (
	signedFieldMutationDomain signedFieldMutation = iota
	signedFieldMutationSigner
	signedFieldMutationBodyLength
	signedFieldMutationBodyDigest
	signedFieldMutationSignature
	signedFieldMutationBodyBytes
	signedFieldMutationLimit
)

func FuzzEnvelopeJSONSemanticClosure(f *testing.F) {
	canonicalBody := literalBody{value: []byte("body"), domain: testDomainPrimary}
	canonicalSigner := deterministicPrivateKey(f, "canonical-json")
	canonicalEnvelope := mustEnvelope(f, canonicalBody, canonicalSigner)
	canonical, gotErr := canonicalEnvelope.MarshalJSON()
	if gotErr != nil {
		f.Fatalf("Envelope.MarshalJSON(seed) error = %v, want nil", gotErr)
	}
	canonicalTrust := mustTrustedKeys(f, mustPublicKey(f, canonicalSigner))
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{"domain":"test-primary-2026"}`))
	f.Add(append(bytes.Clone(canonical), 0))
	f.Add(reverseEnvelopeMembersFixture(f, canonical))

	f.Fuzz(func(t *testing.T, data []byte) {
		original := mustEnvelope(
			t,
			literalBody{value: []byte("fuzz receiver"), domain: testDomainAlternate},
			deterministicPrivateKey(t, "fuzz-json-receiver"),
		)
		var gotFresh attest.Envelope[testDomain]
		gotFreshErr := gotFresh.UnmarshalJSON(data)
		gotPopulated := original
		gotPopulatedErr := gotPopulated.UnmarshalJSON(data)
		if gotFreshErr != nil || gotPopulatedErr != nil {
			if !errors.Is(gotFreshErr, core.ErrJSONContract) ||
				!errors.Is(gotPopulatedErr, core.ErrJSONContract) {
				t.Fatalf(
					"Envelope.UnmarshalJSON(rejected) JSON errors = (%v, %v), want (%v, %v)",
					gotFreshErr,
					gotPopulatedErr,
					core.ErrJSONContract,
					core.ErrJSONContract,
				)
			}
			if !errors.Is(gotFreshErr, core.ErrAttestContract) ||
				!errors.Is(gotPopulatedErr, core.ErrAttestContract) {
				t.Fatalf(
					"Envelope.UnmarshalJSON(rejected) attest errors = (%v, %v), want (%v, %v)",
					gotFreshErr,
					gotPopulatedErr,
					core.ErrAttestContract,
					core.ErrAttestContract,
				)
			}
			if gotFresh != (attest.Envelope[testDomain]{}) {
				t.Fatalf("fresh Envelope after rejection = %+v, want zero", gotFresh)
			}
			if gotPopulated != original {
				t.Fatalf("populated Envelope after rejection = %+v, want preserved %+v", gotPopulated, original)
			}
			return
		}
		if gotFresh != gotPopulated {
			t.Fatalf("Envelope.UnmarshalJSON(accepted) receivers = (%+v, %+v), want equal", gotFresh, gotPopulated)
		}
		if gotErr := gotFresh.Validate(); gotErr != nil {
			t.Fatalf("Envelope.UnmarshalJSON() accepted invalid envelope: %v", gotErr)
		}
		gotCanonical, gotMarshalErr := gotFresh.MarshalJSON()
		if gotMarshalErr != nil {
			t.Fatalf("Envelope.MarshalJSON() error = %v, want nil", gotMarshalErr)
		}
		if len(gotCanonical) > attest.EnvelopeCanonicalJSONMaximumBytes {
			t.Fatalf(
				"len(Envelope.MarshalJSON()) = %d, want <= %d",
				len(gotCanonical),
				attest.EnvelopeCanonicalJSONMaximumBytes,
			)
		}
		var gotRoundTrip attest.Envelope[testDomain]
		gotRoundTripErr := gotRoundTrip.UnmarshalJSON(gotCanonical)
		if gotRoundTripErr != nil {
			t.Fatalf("canonical Envelope.UnmarshalJSON() error = %v, want nil", gotRoundTripErr)
		}
		if gotRoundTrip != gotFresh {
			t.Fatalf("canonical round trip = %+v, want %+v", gotRoundTrip, gotFresh)
		}
		gotSecond, gotSecondErr := gotRoundTrip.MarshalJSON()
		if gotSecondErr != nil {
			t.Fatalf("second Envelope.MarshalJSON() error = %v, want nil", gotSecondErr)
		}
		if !bytes.Equal(gotSecond, gotCanonical) {
			t.Fatalf("second canonical projection = %s, want %s", gotSecond, gotCanonical)
		}

		gotVerified, gotVerifyErr := attest.Verify(attest.VerifyRequest[testDomain]{
			Body:        copyLiteralBody(canonicalBody),
			Envelope:    gotFresh,
			TrustedKeys: canonicalTrust,
		})
		if gotFresh != canonicalEnvelope {
			if !errors.Is(gotVerifyErr, core.ErrAttestVerification) {
				t.Fatalf("attest.Verify(structurally accepted foreign envelope) error = %v, want %v", gotVerifyErr, core.ErrAttestVerification)
			}
			if gotVerified != (attest.Verified[testDomain]{}) {
				t.Fatalf("attest.Verify(structurally accepted foreign envelope) proof = %+v, want zero", gotVerified)
			}
			return
		}
		if gotVerifyErr != nil {
			t.Fatalf("attest.Verify(genuinely signed seed) error = %v, want nil", gotVerifyErr)
		}
		gotVerifiedEnvelope, gotVerifiedEnvelopeErr := gotVerified.Envelope()
		if gotVerifiedEnvelopeErr != nil || gotVerifiedEnvelope != canonicalEnvelope {
			t.Fatalf(
				"Verified.Envelope(genuinely signed seed) = (%+v, %v), want (%+v, nil)",
				gotVerifiedEnvelope,
				gotVerifiedEnvelopeErr,
				canonicalEnvelope,
			)
		}
	})
}

func FuzzVerifyRejectsEveryIndependentlyMutatedSignedField(f *testing.F) {
	for mutation := range signedFieldMutationLimit {
		f.Add(uint8(mutation), []byte("seed"))
	}

	f.Fuzz(func(t *testing.T, rawMutation uint8, fuzzBody []byte) {
		mutation := signedFieldMutation(rawMutation % uint8(signedFieldMutationLimit))
		body := boundedNonemptyFuzzBody(fuzzBody)
		signer := deterministicPrivateKey(t, "fuzz-signed-fields")
		originalBody := literalBody{value: body, domain: testDomainPrimary}
		envelope := mustEnvelope(t, originalBody, signer)
		request := attest.VerifyRequest[testDomain]{
			Body:        copyLiteralBody(originalBody),
			Envelope:    envelope,
			TrustedKeys: mustTrustedKeys(t, mustPublicKey(t, signer)),
		}
		applySignedFieldMutation(t, &request, mutation)
		proveSignedFieldMutationReachedIndependentOracle(t, originalBody, envelope, request, mutation)

		gotVerified, gotErr := attest.Verify(request)
		if !errors.Is(gotErr, core.ErrAttestVerification) {
			t.Fatalf("attest.Verify(mutated field %d) error = %v, want %v", mutation, gotErr, core.ErrAttestVerification)
		}
		if gotVerified != (attest.Verified[testDomain]{}) {
			t.Fatalf("attest.Verify(mutated field %d) proof = %+v, want zero", mutation, gotVerified)
		}
	})
}

func boundedNonemptyFuzzBody(input []byte) []byte {
	const fuzzBodyMaximum = 4096
	if len(input) == 0 {
		return []byte{0}
	}
	if len(input) > fuzzBodyMaximum {
		input = input[:fuzzBodyMaximum]
	}
	return bytes.Clone(input)
}

func applySignedFieldMutation(
	t testing.TB,
	request *attest.VerifyRequest[testDomain],
	mutation signedFieldMutation,
) {
	t.Helper()
	switch mutation {
	case signedFieldMutationDomain:
		request.Envelope.Domain = testDomainAlternate
	case signedFieldMutationSigner:
		other := deterministicPrivateKey(t, "fuzz-mutated-signer")
		request.Envelope.Signer = mustPublicKey(t, other)
		request.TrustedKeys = mustTrustedKeys(
			t,
			mustPublicKey(t, deterministicPrivateKey(t, "fuzz-signed-fields")),
			request.Envelope.Signer,
		)
	case signedFieldMutationBodyLength:
		request.Envelope.BodyLength = mutateLength(t, request.Envelope.BodyLength)
	case signedFieldMutationBodyDigest:
		request.Envelope.BodySHA256 = mutateDigest(t, request.Envelope.BodySHA256)
	case signedFieldMutationSignature:
		request.Envelope.Signature = mutateSignature(t, request.Envelope.Signature)
	case signedFieldMutationBodyBytes:
		body := copyLiteralBody(request.Body.(literalBody))
		body.value[0] ^= 1
		request.Body = body
	default:
		t.Fatalf("signed field mutation = %d, want admitted mutation", mutation)
	}
}

func proveSignedFieldMutationReachedIndependentOracle(
	t testing.TB,
	originalBody literalBody,
	original attest.Envelope[testDomain],
	mutated attest.VerifyRequest[testDomain],
	mutation signedFieldMutation,
) {
	t.Helper()
	switch mutation {
	case signedFieldMutationBodyBytes:
		gotBody := mutated.Body.(literalBody)
		if bytes.Equal(gotBody.value, originalBody.value) {
			t.Fatalf("mutated body bytes = %x, want different from original %x", gotBody.value, originalBody.value)
		}
		gotDigest := sha256.Sum256(gotBody.value)
		wantDigest, gotDigestErr := original.BodySHA256.Bytes()
		if gotDigestErr != nil {
			t.Fatalf("SHA256Digest.Bytes() error = %v, want nil", gotDigestErr)
		}
		if bytes.Equal(gotDigest[:], wantDigest[:]) {
			t.Fatalf("mutated body SHA-256 = %x, want different from %x", gotDigest, wantDigest)
		}
	default:
		gotFrame := independentAttestationFrame(t, mutated.Envelope)
		gotSignature, gotSignatureErr := mutated.Envelope.Signature.Bytes()
		if gotSignatureErr != nil {
			t.Fatalf("Signature.Bytes() error = %v, want nil", gotSignatureErr)
		}
		gotPublicKey, gotPublicKeyErr := mutated.Envelope.Signer.Bytes()
		if gotPublicKeyErr != nil {
			t.Fatalf("Ed25519PublicKey.Bytes() error = %v, want nil", gotPublicKeyErr)
		}
		if ed25519.Verify(gotPublicKey, gotFrame, gotSignature[:]) {
			t.Fatalf("ed25519.Verify(independently mutated field %d) = true, want false", mutation)
		}
	}
}
