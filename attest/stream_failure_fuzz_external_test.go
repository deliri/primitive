package attest_test

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

func FuzzAttestStreamFailureClosure(f *testing.F) {
	canonical, err := (builtBody{commit: "stream-failure", count: 1}).canonical()
	if err != nil {
		f.Fatal(err)
	}
	for _, size := range []int{0, 1, len(canonical), attest.CanonicalBodyMaximumBytes - 1, attest.CanonicalBodyMaximumBytes, attest.CanonicalBodyMaximumBytes + 1} {
		for failure := range byte(4) {
			f.Add(uint32(size), failure)
		}
	}
	key := deterministicPrivateKey(f, "stream-failure-fuzz")
	f.Fuzz(func(t *testing.T, size uint32, failure byte) {
		callbackErrors := [...]error{nil, context.Canceled, context.DeadlineExceeded, errors.Join(context.Canceled, fixtureErrorWrite)}
		callbackErr := callbackErrors[int(failure)%len(callbackErrors)]
		extent := int(min(size, uint32(attest.CanonicalBodyMaximumBytes+1)))
		body := &streamFailureBody{size: extent, callbackErr: callbackErr}
		got, err := attest.Sign(attest.SignRequest[testDomain]{Body: body, Signer: key})
		wantRefused := extent == 0 || extent > attest.CanonicalBodyMaximumBytes || callbackErr != nil
		if (err != nil) != wantRefused || callbackErr != nil && !errors.Is(err, callbackErr) || body.writerErr != nil && !errors.Is(err, body.writerErr) {
			t.Fatalf("stream error = %v, writer error %v; want refusal %t retaining callback %v and writer identity", err, body.writerErr, wantRefused, callbackErr)
		}
		if body.calls != 1 || body.accepted != min(extent, attest.CanonicalBodyMaximumBytes) || body.retained == nil {
			t.Fatalf("stream effects = %d calls, %d bytes, retained %v; want one call, %d bytes and closed writer", body.calls, body.accepted, body.retained, min(extent, attest.CanonicalBodyMaximumBytes))
		}
		written, lateErr := body.retained.Write([]byte{1})
		if written != 0 || !errors.Is(lateErr, core.ErrAttestContract) {
			t.Fatalf("late write = %d, %v; want zero and typed refusal", written, lateErr)
		}
		if wantRefused {
			if !errors.Is(err, core.ErrAttestContract) || got != (attest.Envelope[testDomain]{}) {
				t.Fatalf("refused sign = %+v, %v; want zero envelope and typed contract error", got, err)
			}
			return
		}
		oracle := sha256.New()
		oracleBody := &benchmarkCanonicalBody{size: extent}
		if err := oracleBody.WriteCanonical(oracle); err != nil {
			t.Fatal(err)
		}
		var digest [sha256.Size]byte
		copy(digest[:], oracle.Sum(nil))
		length, lengthErr := got.BodyLength.Uint64()
		signature, signatureErr := got.Signature.Bytes()
		if lengthErr != nil || signatureErr != nil || length != uint64(extent) || got.BodySHA256 != core.NewSHA256Digest(digest) || got.Signer != mustPublicKey(t, key) || got.Domain != testDomainPrimary || !ed25519.Verify(key.Public().(ed25519.PublicKey), independentAttestationFrame(t, got), signature[:]) {
			t.Fatalf("stream envelope = %+v, length error %v, signature error %v; want exact extent, independent digest and authentic frame", got, lengthErr, signatureErr)
		}
	})
}
