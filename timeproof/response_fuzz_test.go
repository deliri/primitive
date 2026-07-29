package timeproof

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzVerifyFreeTSAResponse(f *testing.F) {
	fixture := loadAuthenticFixture(f)
	f.Add(fixture.response)
	f.Add([]byte{})
	f.Add(fixture.response[:len(fixture.response)/2])

	f.Fuzz(func(t *testing.T, response []byte) {
		got, gotErr := Verify(VerifyRequest{
			Response: response, Request: fixture.request,
			ExpectedDigest: fixture.digest,
		})
		if gotErr != nil {
			if !got.isZero() {
				t.Fatalf(
					"Verify(rejected response) timestamp = %v, want zero",
					got,
				)
			}
			if !errors.Is(gotErr, core.ErrTimeProofContract) &&
				!errors.Is(gotErr, core.ErrTimeProofInvalid) &&
				!errors.Is(gotErr, core.ErrTimeProofRefused) {
				t.Fatalf(
					"Verify(rejected response) error = %v, want closed Timeproof identity",
					gotErr,
				)
			}
			var refusal Refusal
			if errors.As(gotErr, &refusal) &&
				(refusal.Validate() != nil || refusal.Status().granted()) {
				t.Fatalf(
					"Verify(rejected response) refusal = %v, want a validated non-granting authority conclusion",
					refusal,
				)
			}
			return
		}
		if got.Validate() != nil ||
			got.Evidence().Digest() != fixture.digest ||
			got.Evidence().Nonce() != fixture.request.Nonce() ||
			got.Evidence().Authority() != AuthorityFreeTSA ||
			!bytes.Equal(got.Evidence().ResponseBytes(), response) {
			t.Fatalf(
				"Verify(accepted response) timestamp = %v, want exact validated request/response binding",
				got,
			)
		}
	})
}
