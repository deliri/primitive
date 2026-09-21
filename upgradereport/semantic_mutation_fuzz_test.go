package upgradereport

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Structured mutations complement byte fuzzing: every non-baseline call reaches
// a structurally valid signed document with one changed load-bearing fact.
// The evidence mutation is deliberately re-signed by the device so its inner
// authority signature, not just the outer frame, must refuse it.
func FuzzSignedObservationSemanticMutation(f *testing.F) {
	seed, installation, keys, authority := reportFixture(f)
	for selector := range uint8(7) {
		f.Add(selector, int64(1))
	}
	f.Fuzz(func(t *testing.T, selector uint8, tick int64) {
		got := seed
		changed := selector%7 != 0
		switch selector % 7 {
		case 0: // Genuine baseline remains independently authenticated.
		case 1:
			got.Payload.ObservedAt = temporal.InstantFromNanoseconds(tick)
			if got.Payload.ObservedAt == seed.Payload.ObservedAt {
				got.Payload.ObservedAt = temporal.InstantFromNanoseconds(1)
			}
		case 2:
			got.Payload.Outcome = Cancelled
		case 3:
			got.Payload.Stage = Download
		case 4:
			got.Payload.Attempt = value[controlwire.RequestNonce](t)(controlwire.NewRequestNonce([32]byte{8}))
		case 5:
			b := value[[64]byte](t)(got.Attestation.Signature.Bytes())
			b[0] ^= 1
			var signature attest.Signature
			if err := signature.UnmarshalJSON(value[[]byte](t)(core.MarshalCanonicalJSONString(hex.EncodeToString(b[:])))); err != nil {
				t.Fatal(err)
			}
			got.Attestation.Signature = signature
		case 6:
			got.Payload.Evidence.Payload.Body.SHA256 = core.SHA256Of([]byte("different evidence bytes"))
			got = value[Request](t)(Sign(got.Payload, got.Certificate, installation.DevicePrivate))
		}
		if changed && got == seed {
			t.Fatalf("mutated request=%v, want a change from %v", got, seed)
		}
		encoded := value[[]byte](t)(got.MarshalJSON())
		var admitted Request
		if err := admitted.UnmarshalJSON(encoded); err != nil || admitted != got {
			t.Fatalf("structured mutation admission = (%v,%v), want exact structurally valid document", admitted, err)
		}
		err := admitted.Verify(authority, keys)
		if changed {
			if !errors.Is(err, core.ErrReportAuthentication) {
				t.Fatalf("changed signed fact verification = %v, want ErrReportAuthentication", err)
			}
		} else if err != nil {
			t.Fatalf("genuinely signed baseline = %v, want nil", err)
		}
	})
}
