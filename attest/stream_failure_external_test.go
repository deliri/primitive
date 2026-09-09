package attest_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

type streamFailureBody struct {
	size        int
	callbackErr error
	writerErr   error
	retained    io.Writer
	calls       int
	accepted    int
}

func (*streamFailureBody) Validate() error               { return nil }
func (*streamFailureBody) AttestationDomain() testDomain { return testDomainPrimary }
func (b *streamFailureBody) WriteCanonical(destination io.Writer) error {
	b.calls++
	b.retained = destination
	var chunk [benchmarkCanonicalChunkBytes]byte
	for remaining := b.size; remaining > 0; {
		count := min(remaining, len(chunk))
		written, err := destination.Write(chunk[:count])
		b.accepted += written
		if err != nil {
			b.writerErr = err
			break
		}
		if written != count {
			return io.ErrShortWrite
		}
		remaining -= count
	}
	return b.callbackErr
}

func TestAttestStreamFailureIdentityAndClosureTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		size         int
		callbackErr  error
		wantErr      error
		wantOverflow bool
	}{
		{name: "minimum complete body retains exact proof", size: 1},
		{name: "one below ceiling retains exact proof", size: attest.CanonicalBodyMaximumBytes - 1},
		{name: "exact ceiling retains exact proof", size: attest.CanonicalBodyMaximumBytes},
		{name: "empty callback cannot manufacture proof", wantErr: core.ErrAttestContract},
		{name: "ignored overflow cannot manufacture proof", size: attest.CanonicalBodyMaximumBytes + 1, wantErr: core.ErrAttestContract, wantOverflow: true},
		{name: "cancellation before bytes remains cancellation", callbackErr: context.Canceled, wantErr: core.ErrAttestContract},
		{name: "cancellation after partial bytes remains cancellation", size: 1, callbackErr: context.Canceled, wantErr: core.ErrAttestContract},
		{name: "cancellation after complete maximum body prevents proof", size: attest.CanonicalBodyMaximumBytes, callbackErr: context.Canceled, wantErr: core.ErrAttestContract},
		{name: "overflow retains cancellation alongside writer refusal", size: attest.CanonicalBodyMaximumBytes + 1, callbackErr: context.Canceled, wantErr: core.ErrAttestContract, wantOverflow: true},
		{name: "partial deadline remains deadline", size: 1, callbackErr: context.DeadlineExceeded, wantErr: core.ErrAttestContract},
		{name: "overflow retains deadline alongside writer refusal", size: attest.CanonicalBodyMaximumBytes + 1, callbackErr: context.DeadlineExceeded, wantErr: core.ErrAttestContract, wantOverflow: true},
		{name: "overflow retains wrapped cancellation", size: attest.CanonicalBodyMaximumBytes + 1, callbackErr: fmt.Errorf("body source: %w", context.Canceled), wantErr: core.ErrAttestContract, wantOverflow: true},
		{name: "overflow retains independent producer failure", size: attest.CanonicalBodyMaximumBytes + 1, callbackErr: fixtureErrorWrite, wantErr: core.ErrAttestContract, wantOverflow: true},
		{name: "overflow retains joined producer failures", size: attest.CanonicalBodyMaximumBytes + 1, callbackErr: errors.Join(context.Canceled, fixtureErrorWrite), wantErr: core.ErrAttestContract, wantOverflow: true},
	}
	for _, verifying := range []bool{false, true} {
		operation := "sign"
		if verifying {
			operation = "verify"
		}
		t.Run(operation, func(t *testing.T) {
			t.Parallel()
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					body := &streamFailureBody{size: tc.size, callbackErr: tc.callbackErr}
					key := deterministicPrivateKey(t, "stream-failure")
					signerCalls := &externalSignerObservation{}
					var got attest.Envelope[testDomain]
					var proof attest.Verified[testDomain]
					var err error
					if verifying {
						baselineBody := &benchmarkCanonicalBody{size: max(1, min(tc.size, attest.CanonicalBodyMaximumBytes))}
						envelope := mustEnvelope(t, baselineBody, key)
						proof, err = attest.Verify(attest.VerifyRequest[testDomain]{Body: body, Envelope: envelope, TrustedKeys: mustTrustedKeys(t, mustPublicKey(t, key))})
						if err == nil {
							got, err = proof.Envelope()
						}
					} else {
						got, err = attest.Sign(attest.SignRequest[testDomain]{Body: body, Signer: externalSigner{key: key, mode: externalSignerValid, observation: signerCalls}})
					}
					if !errors.Is(err, tc.wantErr) || tc.callbackErr != nil && !errors.Is(err, tc.callbackErr) {
						t.Fatalf("operation error = %v; want %v preserving callback %v", err, tc.wantErr, tc.callbackErr)
					}
					if (body.writerErr != nil) != tc.wantOverflow || tc.wantOverflow && !errors.Is(err, body.writerErr) {
						t.Fatalf("writer refusal = %v, operation error = %v; want overflow %t with original refusal preserved", body.writerErr, err, tc.wantOverflow)
					}
					wantAccepted := min(tc.size, attest.CanonicalBodyMaximumBytes)
					wantSignCalls := 0
					if !verifying && tc.wantErr == nil {
						wantSignCalls = 1
					}
					if body.calls != 1 || body.accepted != wantAccepted || body.retained == nil || signerCalls.signCalls != wantSignCalls {
						t.Fatalf("effects = %d callbacks, %d bytes, writer %v, %d signing calls; want 1, %d, retained writer, %d", body.calls, body.accepted, body.retained, signerCalls.signCalls, wantAccepted, wantSignCalls)
					}
					written, lateErr := body.retained.Write([]byte{1})
					if written != 0 || !errors.Is(lateErr, core.ErrAttestContract) {
						t.Fatalf("late write = %d, %v; want zero and closed-writer refusal", written, lateErr)
					}
					if tc.wantErr != nil {
						if got != (attest.Envelope[testDomain]{}) || proof != (attest.Verified[testDomain]{}) {
							t.Fatalf("refused output = %+v, %+v; want zero envelope and proof", got, proof)
						}
						return
					}
					digest := sha256.New()
					oracleBody := &benchmarkCanonicalBody{size: tc.size}
					if err := oracleBody.WriteCanonical(digest); err != nil {
						t.Fatal(err)
					}
					var rawDigest [sha256.Size]byte
					copy(rawDigest[:], digest.Sum(nil))
					length, lengthErr := got.BodyLength.Uint64()
					if lengthErr != nil || length != uint64(tc.size) || got.BodySHA256 != core.NewSHA256Digest(rawDigest) || got.Signer != mustPublicKey(t, key) || got.Domain != testDomainPrimary {
						t.Fatalf("completed facts = %+v, length error %v; want exact domain, signer, %d zero bytes and independent digest", got, lengthErr, tc.size)
					}
				})
			}
		})
	}
}
