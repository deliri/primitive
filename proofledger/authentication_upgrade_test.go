package proofledger

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

func TestProofLedgerReceiptBindingLayerTriad(t *testing.T) {
	t.Parallel()
	genesis, err := NewGenesisHead(fixtureLedger(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	first := fixtureEvent(t, genesis, 0, 1)
	event := fixtureEvent(t, first.Head(), 1, 2)
	document := fixtureReceiptDocument(t, event)
	foreign := fixtureReceiptDocument(t, first)
	trusted := fixtureTrustedKeys(t, document.Receipt.Producer)
	for _, tc := range []struct {
		name    string
		mutate  func(*AppendReceiptVerification[ledgerTestPayload])
		wantErr error
	}{
		{"exact_signed_receipt_exposes_exact_document", func(v *AppendReceiptVerification[ledgerTestPayload]) {}, nil},
		{"event_identity_is_bound", func(v *AppendReceiptVerification[ledgerTestPayload]) { v.Document.Receipt.Event = first.Event }, core.ErrProofLedgerAppendReceiptMismatch},
		{"request_identity_is_bound", func(v *AppendReceiptVerification[ledgerTestPayload]) { v.Document.Receipt.Request = first.Request }, core.ErrProofLedgerAppendReceiptMismatch},
		{"sequence_is_bound", func(v *AppendReceiptVerification[ledgerTestPayload]) { v.Document.Receipt.Sequence++ }, core.ErrProofLedgerAppendReceiptMismatch},
		{"predecessor_digest_is_bound", func(v *AppendReceiptVerification[ledgerTestPayload]) {
			v.Document.Receipt.PreviousHash = core.SHA256Of([]byte("other predecessor"))
		}, core.ErrProofLedgerAppendReceiptMismatch},
		{"event_digest_is_bound", func(v *AppendReceiptVerification[ledgerTestPayload]) { v.Document.Receipt.Hash = first.Hash }, core.ErrProofLedgerAppendReceiptMismatch},
		{"recording_time_is_bound", func(v *AppendReceiptVerification[ledgerTestPayload]) {
			v.Document.Receipt.RecordedAt = first.RecordedAt
		}, core.ErrProofLedgerAppendReceiptMismatch},
		{"authentic_foreign_signature_is_refused", func(v *AppendReceiptVerification[ledgerTestPayload]) {
			v.Document.Attestation.Signature = foreign.Attestation.Signature
		}, core.ErrAttestVerification},
		{"producer_nomination_is_bound", func(v *AppendReceiptVerification[ledgerTestPayload]) { v.Document.Receipt.Producer = fixtureKey(t, 3) }, core.ErrProofLedgerAppendReceiptMismatch},
		{"untrusted_signer_cannot_authenticate", func(v *AppendReceiptVerification[ledgerTestPayload]) {
			v.TrustedKeys = fixtureTrustedKeys(t, fixtureKey(t, 3))
		}, core.ErrAttestVerification},
		{"absent_event_exposes_no_proof", func(v *AppendReceiptVerification[ledgerTestPayload]) { v.Event = Envelope[ledgerTestPayload]{} }, core.ErrProofLedgerContract},
		{"absent_document_exposes_no_proof", func(v *AppendReceiptVerification[ledgerTestPayload]) { v.Document = AppendReceiptDocument{} }, core.ErrProofLedgerContract},
		{"absent_trust_exposes_no_proof", func(v *AppendReceiptVerification[ledgerTestPayload]) { v.TrustedKeys = attest.TrustedKeys{} }, core.ErrProofLedgerContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := AppendReceiptVerification[ledgerTestPayload]{Event: event, Document: document, TrustedKeys: trusted}
			tc.mutate(&input)
			if tc.wantErr != nil && input == (AppendReceiptVerification[ledgerTestPayload]{Event: event, Document: document, TrustedKeys: trusted}) {
				t.Fatalf("mutation input = baseline, want one changed fact")
			}
			// Bind the unsigned receipt layer independently of the authenticator.
			// Producer nomination belongs to the signed document, not this event match.
			matchWant := error(nil)
			receiptFacts := input.Document.Receipt
			receiptFacts.Producer = document.Receipt.Producer
			if input.Event == (Envelope[ledgerTestPayload]{}) || input.Document == (AppendReceiptDocument{}) {
				matchWant = core.ErrProofLedgerContract
			} else if receiptFacts != document.Receipt {
				matchWant = core.ErrProofLedgerAppendReceiptMismatch
			}
			if matchErr := VerifyAppendReceipt(input.Event, input.Document.Receipt); !errors.Is(matchErr, matchWant) {
				t.Fatalf("VerifyAppendReceipt() error = %v, want %v", matchErr, matchWant)
			}
			got, err := VerifyAppendReceiptDocument(input)
			if !errors.Is(err, tc.wantErr) || err != nil && (!errors.Is(err, core.ErrProofLedgerContract) || got != (VerifiedAppendReceipt{})) {
				t.Fatalf("VerifyAppendReceiptDocument() = (%v, %v), want zero on %v", got, err, tc.wantErr)
			}
			observed, documentErr := got.Document()
			if err == nil {
				if documentErr != nil || observed != document {
					t.Fatalf("Document() = (%v, %v), want exact signed receipt", observed, documentErr)
				}
			} else if observed != (AppendReceiptDocument{}) || !errors.Is(documentErr, core.ErrProofLedgerAppendReceiptMismatch) {
				t.Fatalf("Document(refused proof) = (%v, %v), want zero and typed refusal", observed, documentErr)
			}
		})
	}
}

func FuzzProofLedgerReceiptAuthentication(f *testing.F) {
	genesis, err := NewGenesisHead(fixtureLedger(f))
	if err != nil {
		f.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	event := fixtureEvent(f, genesis, 0, 1)
	document := fixtureReceiptDocument(f, event)
	foreign := fixtureReceiptDocument(f, fixtureEvent(f, genesis, 1, 2))
	trusted := fixtureTrustedKeys(f, document.Receipt.Producer)
	foreignKey := fixtureKey(f, 3)
	foreignTrust := fixtureTrustedKeys(f, foreignKey)
	canonical, err := document.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON() error = %v, want nil", err)
	}
	for selector := range uint8(8) {
		f.Add(canonical, selector)
	}
	f.Add([]byte{}, uint8(0))
	f.Add([]byte(`{}`), uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, selector uint8) {
		keys := trusted
		candidate := document
		switch selector % 8 {
		case 1:
			candidate.Attestation.Signature = foreign.Attestation.Signature
		case 2:
			candidate.Receipt.Hash = foreign.Receipt.Hash
		case 3:
			candidate.Attestation.Signer = foreignKey
		case 4:
			candidate.Receipt.Producer = foreignKey
		case 5:
			candidate.Receipt.Request = foreign.Receipt.Request
		case 6:
			keys = foreignTrust
			data = canonical
		case 7:
			keys = attest.TrustedKeys{}
			data = canonical
		}
		if selector%8 > 0 && selector%8 < 6 {
			if candidate == document {
				t.Fatalf("selector=%d changed no fact, want nonvacuous mutation", selector)
			}
			var err error
			data, err = core.MarshalCanonicalJSONDocument(appendReceiptDocumentWire(candidate))
			if err != nil {
				t.Fatalf("MarshalCanonicalJSONDocument(mutation) error = %v, want nil", err)
			}
		}
		got := document
		if err := got.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrProofLedgerContract) || got != document || bytes.Equal(data, canonical) {
				t.Fatalf("UnmarshalJSON() = (%v, %v), want preserved receiver and typed refusal", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Validate(admitted) error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON(admitted) error = %v, want nil", err)
		}
		var roundTrip AppendReceiptDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("UnmarshalJSON(roundtrip) error = %v, want exact admitted document", err)
		}
		proof, err := VerifyAppendReceiptDocument(AppendReceiptVerification[ledgerTestPayload]{Event: event, Document: got, TrustedKeys: keys})
		if got == document && selector%8 != 6 && selector%8 != 7 {
			observed, documentErr := proof.Document()
			if err != nil || documentErr != nil || observed != document {
				t.Fatalf("VerifyAppendReceiptDocument(authentic) errors=%v/%v, want exact signed document", err, documentErr)
			}
		} else if !errors.Is(err, core.ErrProofLedgerContract) || proof != (VerifiedAppendReceipt{}) {
			t.Fatalf("VerifyAppendReceiptDocument(mutated) = (%v, %v), want zero and typed refusal", proof, err)
		}
	})
}
