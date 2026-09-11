package paymentauth

import (
	"errors"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

// Structurally valid signature substitutions complement arbitrary JSON grammar
// mutations. Every selector changes a fact actually consumed by the wall.
func FuzzPaymentCredentialSignedFacts(f *testing.F) {
	fixture := newPaymentQueryFixture(f, standardPaymentQueryFixtureRequest(f))
	otherSpec := standardPaymentQueryFixtureRequest(f)
	otherSpec.deviceByte++
	other := newPaymentQueryFixture(f, otherSpec)
	canonical, err := fixture.document.MarshalJSON()
	if err != nil {
		f.Fatalf("signed seed MarshalJSON error = %v, want nil", err)
	}
	for selector := range uint8(7) {
		f.Add(canonical, selector, uint8(0))
	}
	f.Fuzz(func(t *testing.T, data []byte, selector, marker uint8) {
		if selector%7 == 6 {
			got := fixture.document
			err := got.UnmarshalJSON(data)
			if err != nil {
				if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrControlPlaneContract) || got != fixture.document {
					t.Fatalf("decode refusal = %v, want preserved receiver and typed JSON/control-plane refusal", err)
				}
				return
			}
			proof, err := Verify(Verification{Document: got, Server: fixture.server})
			if err != nil {
				if !errors.Is(err, core.ErrAttestVerification) || proof != (Verified{}) {
					t.Fatalf("authentication refusal = (%v, %v), want zero and attestation refusal", proof, err)
				}
				return
			}
			payload, err := proof.Payload()
			if err != nil || got != fixture.document || payload != fixture.payload {
				t.Fatalf("authenticated payload = (%v, %v), want exact signed seed", payload, err)
			}
			return
		}
		changed := fixture.document
		wantErr := core.ErrAttestVerification
		switch selector % 7 {
		case 0:
			var raw [core.SHA256DigestBytes]byte
			raw[0] = marker
			raw[len(raw)-1] = 1
			nonce, err := controlwire.NewRequestNonce(raw)
			if err != nil {
				t.Fatalf("NewRequestNonce mutation error = %v, want nil", err)
			}
			changed.Request.Payload.Nonce = nonce
		case 1:
			changed.Request.Payload.Query.Selection = paymentQuerySpecificSelection(t)
		case 2:
			limit, err := core.NewCatalogPageLimit(1)
			if err != nil {
				t.Fatalf("NewCatalogPageLimit error = %v, want nil", err)
			}
			changed.Request.Payload.Query.Limit = limit
		case 3:
			changed.Request.Attestation.Signer = other.document.Certificate.Body.DeviceKey
			wantErr = core.ErrControlPlaneResponseBinding
		case 4:
			changed.Request.Attestation.BodySHA256 = core.SHA256Of([]byte{marker})
		case 5:
			size, err := changed.Request.Attestation.BodyLength.Uint64()
			if err != nil {
				t.Fatalf("BodyLength.Uint64 error = %v, want nil", err)
			}
			changed.Request.Attestation.BodyLength, err = core.NewByteCount(size + 1)
			if err != nil {
				t.Fatalf("NewByteCount error = %v, want nil", err)
			}
		}
		if changed == fixture.document {
			t.Fatal("signed mutation = baseline, want changed semantic fact")
		}
		wire, err := core.MarshalCanonicalJSONDocument(requestDocumentWire(changed))
		if err != nil {
			t.Fatalf("typed mutated wire error = %v, want nil", err)
		}
		received := fixture.document
		err = received.UnmarshalJSON(wire)
		if errors.Is(wantErr, core.ErrControlPlaneResponseBinding) {
			if !errors.Is(err, wantErr) || !errors.Is(err, core.ErrJSONContract) || received != fixture.document {
				t.Fatalf("nomination admission = (%v, %v), want preserved receiver and typed binding refusal", received, err)
			}
			return
		}
		if err != nil || received != changed {
			t.Fatalf("structural mutation admission = (%v, %v), want exact changed facts and nil", received, err)
		}
		proof, err := Verify(Verification{Document: received, Server: fixture.server})
		if !errors.Is(err, wantErr) || proof != (Verified{}) {
			t.Fatalf("mutated authentication = (%v, %v), want zero proof and %v", proof, err, wantErr)
		}
	})
}
