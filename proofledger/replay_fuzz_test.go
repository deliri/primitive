package proofledger

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzProofLedgerReplayAndAppendHead(f *testing.F) {
	ledger := fixtureLedger(f)
	actor := fixtureKey(f, 1)
	identity := fixtureEventIdentity(f, 0)
	nonce := fixtureNonce(f, 1)
	instant := fixtureInstant(f, 1)
	for _, previous := range []uint64{0, 1, math.MaxUint64 - 1, math.MaxUint64} {
		for selector := range uint8(5) {
			f.Add(previous, selector)
		}
	}
	f.Fuzz(func(t *testing.T, previous uint64, selector uint8) {
		hash := core.SHA256Of([]byte("prior durable event"))
		if previous == 0 {
			hash = GenesisHash()
		}
		after := Head{Ledger: ledger, Sequence: Position(previous), Hash: hash}
		intent := AppendIntent[ledgerTestPayload]{Ledger: ledger, ExpectedHead: after, Actor: actor, Request: nonce, Payload: ledgerTestPayload{Value: 1}}
		if err := ValidateAppendHead(intent, after); err != nil {
			t.Fatalf("ValidateAppendHead(exact) error = %v, want nil", err)
		}
		event, err := NewEnvelope(Issue[ledgerTestPayload]{Intent: intent, Event: identity, RecordedAt: instant})
		if previous == math.MaxUint64 {
			if event != (Envelope[ledgerTestPayload]{}) || !errors.Is(err, core.ErrProofLedgerSequenceConflict) || !errors.Is(err, core.ErrProofLedgerContract) {
				t.Fatalf("NewEnvelope(exhausted) = (%v, %v), want zero typed refusal", event.Head(), err)
			}
			return
		}
		if err != nil || uint64(event.Sequence) != previous+1 || event.PreviousHash != hash {
			t.Fatalf("NewEnvelope() = (%v, %v), want exact successor of %d", event.Head(), err, previous)
		}
		data, err := event.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() error = %v, want nil", err)
		}
		received, err := DecodeEnvelope[ledgerTestPayload, *ledgerTestPayload](data)
		if err != nil || received != event {
			t.Fatalf("DecodeEnvelope() = (%v, %v), want exact issued event", received.Head(), err)
		}
		verifier, err := NewVerifier[ledgerTestPayload](after)
		if err != nil {
			t.Fatalf("NewVerifier() error = %v, want nil", err)
		}
		candidate := received
		wantErr := error(nil)
		switch selector % 5 {
		case 1:
			candidate.Payload.Value = 2
			wantErr = core.ErrProofLedgerTampering
		case 2:
			candidate.Hash = core.SHA256Of([]byte("forged event"))
			wantErr = core.ErrProofLedgerTampering
		case 3:
			candidate = Envelope[ledgerTestPayload]{}
			wantErr = core.ErrProofLedgerContract
		case 4:
			if err := verifier.Observe(received); err != nil {
				t.Fatalf("Observe(first) error = %v, want nil", err)
			}
			wantErr = core.ErrProofLedgerSequenceConflict
		}
		before := verifier.Head()
		err = verifier.Observe(candidate)
		if !errors.Is(err, wantErr) || err != nil && (!errors.Is(err, core.ErrProofLedgerContract) || verifier.Head() != before) {
			t.Fatalf("Observe() = (%v, %v), want %v with preserved refusal cursor", verifier.Head(), err, wantErr)
		}
		if err == nil && verifier.Head() != event.Head() {
			t.Fatalf("Head() = %v, want exact %v", verifier.Head(), event.Head())
		}
		if err := verifier.Finish(verifier.Head()); err != nil {
			t.Fatalf("Finish(admitted head) error = %v, want nil", err)
		}
		if err := ValidateAppendHead(intent, event.Head()); !errors.Is(err, core.ErrProofLedgerSequenceConflict) {
			t.Fatalf("ValidateAppendHead(stale) error = %v, want conflict", err)
		}
	})
}

func FuzzProofLedgerSigningDomainText(f *testing.F) {
	seed, err := AppendReceiptSigningDomainV1.MarshalText()
	if err != nil {
		f.Fatalf("MarshalText() error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add(append(bytes.Clone(seed), 'x'))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := AppendReceiptSigningDomainUnknown.ParseCanonicalText(data)
		if bytes.Equal(data, seed) {
			if err != nil || got != AppendReceiptSigningDomainV1 || !got.IsValid() {
				t.Fatalf("ParseCanonicalText() = (%v, %v), want V1 and nil", got, err)
			}
			encoded, err := got.MarshalText()
			if err != nil || !bytes.Equal(encoded, data) {
				t.Fatalf("MarshalText() = (%q, %v), want exact canonical input", encoded, err)
			}
		} else if !errors.Is(err, core.ErrProofLedgerContract) || got != AppendReceiptSigningDomainUnknown {
			t.Fatalf("ParseCanonicalText() = (%v, %v), want unknown and contract refusal", got, err)
		}
	})
}
