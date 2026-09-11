package timeproof

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type responseFuzzAgreement struct {
	fixture authenticFixture
	proof   AuthoritativeTimestamp
	token   parsedToken
}

func responseAgreementForFuzz(t testing.TB, fixture authenticFixture) responseFuzzAgreement {
	t.Helper()
	proof, err := Verify(VerifyRequest{Response: fixture.response, Request: fixture.request, ExpectedDigest: fixture.digest})
	if err != nil || proof.Validate() != nil {
		t.Fatalf("Verify(signed seed) = (%v, %v), want valid proof and nil", proof, err)
	}
	der, _, err := parseTimestampResponse(fixture.response)
	if err != nil {
		t.Fatalf("parseTimestampResponse(seed) error = %v, want nil", err)
	}
	token, err := parseTimestampToken(der)
	if err != nil {
		t.Fatalf("parseTimestampToken(seed) error = %v, want nil", err)
	}
	return responseFuzzAgreement{fixture: fixture, proof: proof, token: token}
}

func FuzzVerifyFreeTSAResponse(f *testing.F) {
	agreement := responseAgreementForFuzz(f, loadAuthenticFixture(f))
	addRefusalResponseSeeds(f, agreement.fixture)
	canonical := agreement.proof.Evidence().ResponseBytes()
	f.Add(canonical)
	f.Add([]byte{})
	f.Add(canonical[:len(canonical)/2])
	f.Fuzz(func(t *testing.T, response []byte) {
		fuzzResponseAgreement(t, response, agreement)
	})
}

func FuzzVerifyDigiCertResponse(f *testing.F) {
	agreement := responseAgreementForFuzz(f, loadDigiCertAuthenticFixture(f))
	addRefusalResponseSeeds(f, agreement.fixture)
	canonical := agreement.proof.Evidence().ResponseBytes()
	f.Add(canonical)
	f.Add([]byte{})
	f.Add(canonical[:len(canonical)/2])
	f.Fuzz(func(t *testing.T, response []byte) {
		fuzzResponseAgreement(t, response, agreement)
	})
}

func fuzzResponseAgreement(t *testing.T, response []byte, agreement responseFuzzAgreement) {
	t.Helper()
	got, gotErr := Verify(VerifyRequest{Response: response, Request: agreement.fixture.request, ExpectedDigest: agreement.fixture.digest})
	if gotErr != nil {
		if !timestampHasNoProof(got) {
			t.Fatalf("Verify(rejected) proof = %+v, want every field zero", got)
		}
		if !errors.Is(gotErr, core.ErrTimeProofContract) && !errors.Is(gotErr, core.ErrTimeProofInvalid) && !errors.Is(gotErr, core.ErrTimeProofRefused) {
			t.Fatalf("Verify(rejected) error = %v, want typed Timeproof refusal", gotErr)
		}
		if errors.Is(gotErr, core.ErrTimeProofRefused) {
			var refusal Refusal
			if !errors.As(gotErr, &refusal) || refusal.Validate() != nil || refusal.Status().granted() {
				t.Fatalf("Verify(provider refusal) error = %v, want validated non-granting Refusal", gotErr)
			}
			fuzzRefusalSource(t, response, refusal)
		}
		return
	}
	if got.Validate() != nil || got.Time() != agreement.proof.Time() || got.Signer() != agreement.proof.Signer() || got.Serial() != agreement.proof.Serial() || got.Policy() != agreement.proof.Policy() {
		t.Fatalf("Verify(accepted) facts = %+v, want authentic signed facts %+v", got, agreement.proof)
	}
	if got.Evidence().Digest() != agreement.fixture.digest || got.Evidence().Nonce() != agreement.fixture.request.Nonce() || got.Evidence().Authority() != agreement.fixture.request.Authority() || !bytes.Equal(got.Evidence().ResponseBytes(), response) {
		t.Fatalf("Verify(accepted) evidence = %+v, want exact request and response custody", got.Evidence())
	}
	der, _, err := parseTimestampResponse(response)
	if err != nil {
		t.Fatalf("accepted response framing error = %v, want nil", err)
	}
	token, err := parseTimestampToken(der)
	if err != nil {
		t.Fatalf("accepted response token error = %v, want nil", err)
	}
	// The genuine signed agreement is the oracle. Unsigned CMS framing may
	// vary, but the signed content, attributes, signature and signer may not
	// become a new authentic agreement without the authority's private key.
	if !bytes.Equal(token.TSTDER, agreement.token.TSTDER) || !bytes.Equal(token.SignerInfo.SignedAttributes.FullBytes, agreement.token.SignerInfo.SignedAttributes.FullBytes) || !bytes.Equal(token.SignerInfo.Signature, agreement.token.SignerInfo.Signature) || !bytes.Equal(token.Signer.Raw, agreement.token.Signer.Raw) {
		t.Fatalf("accepted signed content/signature differs from genuine authority seed")
	}
}

func FuzzVerifyAuthorityBindingMutations(f *testing.F) {
	free := responseAgreementForFuzz(f, loadAuthenticFixture(f))
	digi := responseAgreementForFuzz(f, loadDigiCertAuthenticFixture(f))
	for provider := uint8(0); provider < 2; provider++ {
		for mutation := uint8(0); mutation < 5; mutation++ {
			f.Add(provider, mutation, uint16(0))
		}
	}
	f.Fuzz(func(t *testing.T, provider, mutation uint8, offset uint16) {
		agreement := free
		if provider%2 == 1 {
			agreement = digi
		}
		request := VerifyRequest{Response: agreement.fixture.response, Request: agreement.fixture.request, ExpectedDigest: agreement.fixture.digest}
		switch mutation % 5 {
		case 0:
			request.Response = bytes.Clone(request.Response)
			signature := agreement.token.SignerInfo.Signature
			index := bytes.Index(request.Response, signature)
			if len(signature) == 0 || index < 0 || index != bytes.LastIndex(request.Response, signature) {
				t.Fatalf("signature location = %d, want exactly one nonempty signed field", index)
			}
			request.Response[index+int(offset)%len(signature)] ^= 1
			if bytes.Equal(request.Response, agreement.fixture.response) {
				t.Fatalf("signature mutation byte at %d = %x, want different from %x", index+int(offset)%len(signature), request.Response[index+int(offset)%len(signature)], agreement.fixture.response[index+int(offset)%len(signature)])
			}
		case 1:
			nonce := agreement.fixture.request.Nonce()
			nonce.value[int(offset)%NonceBytes] ^= 1
			if nonce == agreement.fixture.request.Nonce() {
				t.Fatalf("mutated nonce = %v, want different from %v", nonce, agreement.fixture.request.Nonce())
			}
			var err error
			request.Request, err = newRequest(agreement.fixture.digest, nonce, agreement.fixture.request.Authority())
			if err != nil {
				t.Fatalf("newRequest(changed nonce) error = %v, want nil", err)
			}
		case 2:
			raw, err := agreement.fixture.digest.Bytes()
			if err != nil {
				t.Fatalf("Digest.Bytes() error = %v, want nil", err)
			}
			raw[int(offset)%len(raw)] ^= 1
			request.ExpectedDigest = core.NewSHA256Digest(raw)
			if request.ExpectedDigest == agreement.fixture.digest {
				t.Fatalf("mutated digest = %v, want different from %v", request.ExpectedDigest, agreement.fixture.digest)
			}
		case 3:
			authority := AuthorityDigiCert
			if agreement.fixture.request.Authority() == authority {
				authority = AuthorityFreeTSA
			}
			var err error
			request.Request, err = newRequest(agreement.fixture.digest, agreement.fixture.request.Nonce(), authority)
			if err != nil {
				t.Fatalf("newRequest(foreign authority) error = %v, want nil", err)
			}
		case 4:
			raw, err := agreement.fixture.digest.Bytes()
			if err != nil {
				t.Fatalf("Digest.Bytes() error = %v, want nil", err)
			}
			raw[int(offset)%len(raw)] ^= 1
			request.ExpectedDigest = core.NewSHA256Digest(raw)
			request.Request, err = newRequest(request.ExpectedDigest, agreement.fixture.request.Nonce(), agreement.fixture.request.Authority())
			if err != nil {
				t.Fatalf("newRequest(changed imprint) error = %v, want nil", err)
			}
		}
		got, gotErr := Verify(request)
		if !errors.Is(gotErr, core.ErrTimeProofInvalid) || !timestampHasNoProof(got) {
			t.Fatalf("Verify(binding mutation %d) = (%+v, %v), want zero and %v", mutation%5, got, gotErr, core.ErrTimeProofInvalid)
		}
	})
}
