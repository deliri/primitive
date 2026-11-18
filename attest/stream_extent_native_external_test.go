package attest_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

func TestCanonicalSigningStreamContinuesAcrossFormerQuota(t *testing.T) {
	t.Parallel()
	for _, size := range []int{(1 << 20) - 1, 1 << 20, (1 << 20) + 1, (17 << 20) + 37} {
		t.Run(fmt.Sprintf("canonical_bytes_%d", size), func(t *testing.T) {
			t.Parallel()
			body := sizedBody{size: size, chunkSize: 8191, domain: testDomainPrimary}
			key := deterministicPrivateKey(t, "stream-extent")
			got, err := attest.Sign(attest.SignRequest[testDomain]{Body: body, Signer: key})
			if err != nil {
				t.Fatalf("Sign(%d streamed bytes) error = %v, want nil", size, err)
			}
			length, err := got.BodyLength.Uint64()
			if err != nil || length != uint64(size) {
				t.Fatalf("signed length = %d/%v, want %d/nil", length, err, size)
			}
			hash := sha256.New()
			if err := body.WriteCanonical(hash); err != nil {
				t.Fatal(err)
			}
			var digest [sha256.Size]byte
			copy(digest[:], hash.Sum(nil))
			if got.BodySHA256 != core.NewSHA256Digest(digest) {
				t.Fatalf("signed digest = %v, want independent streamed digest %x", got.BodySHA256, digest)
			}
			signature, err := got.Signature.Bytes()
			if err != nil || !ed25519.Verify(key.Public().(ed25519.PublicKey), independentAttestationFrame(t, got), signature[:]) {
				t.Fatalf("signature error = %v, want exact independently verified frame", err)
			}
			encoded, err := got.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var decoded attest.Envelope[testDomain]
			if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != got {
				t.Fatalf("envelope round trip = %+v/%v, want %+v/nil", decoded, err, got)
			}
			request := attest.VerifyRequest[testDomain]{Body: body, Envelope: decoded, TrustedKeys: mustTrustedKeys(t, mustPublicKey(t, key))}
			proof, err := attest.Verify(request)
			if err != nil || proof.Validate() != nil {
				t.Fatalf("Verify(%d streamed bytes) = %+v/%v, want valid proof", size, proof, err)
			}
			request.Envelope.BodyLength, err = core.NewByteCount(uint64(size + 1))
			if err != nil {
				t.Fatal(err)
			}
			refused, err := attest.Verify(request)
			if !errors.Is(err, core.ErrAttestVerification) || refused != (attest.Verified[testDomain]{}) {
				t.Fatalf("changed-length verification = %+v/%v, want zero proof and %v", refused, err, core.ErrAttestVerification)
			}
		})
	}
}
