package proofledger

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestPageLimitExhaustsItsCompleteUint16Domain(t *testing.T) {
	t.Parallel()
	for raw := 0; raw <= math.MaxUint16; raw++ {
		got, gotErr := NewPageLimit(uint16(raw))
		wantValid := raw >= 1 && raw <= PageEventMaximum
		if (gotErr == nil) != wantValid || (got.Validate() == nil) != wantValid {
			t.Fatalf("NewPageLimit(%d) = (%+v, %v), want valid=%t", raw, got, gotErr, wantValid)
		}
	}
}

func TestSequencePressuresZeroOneMaximumAndCanonicalRepresentation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		wantErr  error
		name     string
		value    uint64
		wantNext uint64
	}{
		{name: "zero is outside the sequence domain", value: 0, wantErr: core.ErrProofLedgerContract},
		{name: "first sequence advances to second", value: 1, wantNext: 2},
		{name: "ordinary sequence advances exactly once", value: 41, wantNext: 42},
		{name: "one below maximum advances to maximum", value: math.MaxUint64 - 1, wantNext: math.MaxUint64},
		{name: "maximum refuses overflow", value: math.MaxUint64, wantErr: core.ErrProofLedgerSequenceConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sequence, gotErr := NewSequence(tc.value)
			if tc.value == 0 {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("NewSequence(%d) error = %v, want %v", tc.value, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("NewSequence(%d) error = %v, want nil", tc.value, gotErr)
			}
			next, gotErr := sequence.Next()
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("Sequence(%d).Next() error = %v, want %v", tc.value, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || uint64(next) != tc.wantNext {
				t.Fatalf("Sequence(%d).Next() = (%d, %v), want (%d, nil)", tc.value, next, gotErr, tc.wantNext)
			}
		})
	}
}

func TestEnvelopeHashCommitsEveryLoadBearingFactWithOneFactMutationPairs(t *testing.T) {
	t.Parallel()
	ledger := fixtureLedger(t)
	head := Head{Ledger: ledger, Sequence: 7, Hash: core.SHA256Of([]byte("head-a"))}
	baselineIssue := Issue[ledgerTestPayload]{
		Intent: AppendIntent[ledgerTestPayload]{Request: fixtureNonce(t, 1), Ledger: ledger, ExpectedHead: head, Actor: fixtureKey(t, 1), Payload: ledgerTestPayload{Value: 1}},
		Event:  fixtureEventIdentity(t, 0), RecordedAt: fixtureInstant(t, 1),
	}
	baseline, err := NewEnvelope(baselineIssue)
	if err != nil {
		t.Fatalf("NewEnvelope(baseline) error = %v, want nil", err)
	}
	otherLedger, err := NewLedgerIdentity(fixtureUUID(t, "01890f42-6a00-7000-8000-000000000009"))
	if err != nil {
		t.Fatalf("NewLedgerIdentity(other) error = %v, want nil", err)
	}
	cases := []struct {
		mutate func(*Issue[ledgerTestPayload])
		name   string
	}{
		{name: "ledger identity changes commitment", mutate: func(i *Issue[ledgerTestPayload]) {
			i.Intent.Ledger = otherLedger
			i.Intent.ExpectedHead.Ledger = otherLedger
		}},
		{name: "event identity changes commitment", mutate: func(i *Issue[ledgerTestPayload]) { i.Event = fixtureEventIdentity(t, 1) }},
		{name: "request identity changes commitment", mutate: func(i *Issue[ledgerTestPayload]) { i.Intent.Request = fixtureNonce(t, 2) }},
		{name: "sequence changes commitment", mutate: func(i *Issue[ledgerTestPayload]) { i.Intent.ExpectedHead.Sequence++ }},
		{name: "previous hash changes commitment", mutate: func(i *Issue[ledgerTestPayload]) { i.Intent.ExpectedHead.Hash = core.SHA256Of([]byte("head-b")) }},
		{name: "actor changes commitment", mutate: func(i *Issue[ledgerTestPayload]) { i.Intent.Actor = fixtureKey(t, 2) }},
		{name: "recorded instant changes commitment", mutate: func(i *Issue[ledgerTestPayload]) { i.RecordedAt = fixtureInstant(t, 2) }},
		{name: "payload changes commitment", mutate: func(i *Issue[ledgerTestPayload]) { i.Intent.Payload.Value = 2 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			changedIssue := baselineIssue
			tc.mutate(&changedIssue)
			changed, gotErr := NewEnvelope(changedIssue)
			if gotErr != nil {
				t.Fatalf("NewEnvelope(one-fact mutation) error = %v, want nil", gotErr)
			}
			if changed.Hash == baseline.Hash {
				t.Fatalf("NewEnvelope(one-fact mutation) hash = %v, want different from baseline %v", changed.Hash, baseline.Hash)
			}
		})
	}
}

func TestEnvelopeValidateRejectsEveryMissingOrContradictoryFact(t *testing.T) {
	t.Parallel()
	genesis, err := NewGenesisHead(fixtureLedger(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	baseline := fixtureEvent(t, genesis, 0, 1)
	cases := []struct {
		wantErr error
		mutate  func(*Envelope[ledgerTestPayload])
		name    string
	}{
		{name: "missing ledger identity", mutate: func(e *Envelope[ledgerTestPayload]) { e.Ledger = LedgerIdentity{} }, wantErr: core.ErrProofLedgerContract},
		{name: "missing event identity", mutate: func(e *Envelope[ledgerTestPayload]) { e.Event = EventIdentity{} }, wantErr: core.ErrProofLedgerContract},
		{name: "missing request identity", mutate: func(e *Envelope[ledgerTestPayload]) { e.Request = controlwire.RequestNonce{} }, wantErr: core.ErrProofLedgerContract},
		{name: "zero sequence", mutate: func(e *Envelope[ledgerTestPayload]) { e.Sequence = 0 }, wantErr: core.ErrProofLedgerContract},
		{name: "missing previous hash", mutate: func(e *Envelope[ledgerTestPayload]) { e.PreviousHash = core.SHA256Digest{} }, wantErr: core.ErrProofLedgerContract},
		{name: "missing event hash", mutate: func(e *Envelope[ledgerTestPayload]) { e.Hash = core.SHA256Digest{} }, wantErr: core.ErrProofLedgerContract},
		{name: "missing actor", mutate: func(e *Envelope[ledgerTestPayload]) { e.Actor = core.Ed25519PublicKey{} }, wantErr: core.ErrProofLedgerContract},
		{name: "missing recorded instant", mutate: func(e *Envelope[ledgerTestPayload]) { e.RecordedAt = temporal.Instant{} }, wantErr: core.ErrProofLedgerContract},
		{name: "invalid payload", mutate: func(e *Envelope[ledgerTestPayload]) { e.Payload.Value = 0 }, wantErr: core.ErrProofLedgerContract},
		{name: "changed hash bytes", mutate: func(e *Envelope[ledgerTestPayload]) { e.Hash = core.SHA256Of([]byte("forged")) }, wantErr: core.ErrProofLedgerTampering},
		{name: "first event names non-genesis predecessor", mutate: func(e *Envelope[ledgerTestPayload]) { e.PreviousHash = core.SHA256Of([]byte("prior")) }, wantErr: core.ErrProofLedgerPreviousHashMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := baseline
			tc.mutate(&got)
			if gotErr := got.Validate(); !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Envelope.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestReceiptRejectsLaterSequenceWithGenesisPredecessor(t *testing.T) {
	t.Parallel()
	genesis, err := NewGenesisHead(fixtureLedger(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	first := fixtureEvent(t, genesis, 0, 1)
	second := fixtureEvent(t, first.Head(), 1, 2)
	receipt, err := NewAppendReceipt(second, fixtureKey(t, 2))
	if err != nil {
		t.Fatalf("NewAppendReceipt(second) error = %v, want nil", err)
	}
	mutated := receipt
	mutated.PreviousHash = GenesisHash()
	if gotErr := mutated.Validate(); !errors.Is(gotErr, core.ErrProofLedgerPreviousHashMismatch) {
		t.Fatalf("AppendReceipt.Validate(sequence two with genesis predecessor) error = %v, want %v", gotErr, core.ErrProofLedgerPreviousHashMismatch)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(appendReceiptWire(mutated))
	if err != nil {
		t.Fatalf("MarshalCanonicalJSONDocument(mutated receipt) error = %v, want nil", err)
	}
	preserved := receipt
	if gotErr := preserved.UnmarshalJSON(encoded); !errors.Is(gotErr, core.ErrProofLedgerPreviousHashMismatch) || !errors.Is(gotErr, core.ErrJSONContract) || preserved != receipt {
		t.Fatalf("AppendReceipt.UnmarshalJSON(sequence two with genesis predecessor) = (%+v, %v), want preserved and both %v and %v", preserved, gotErr, core.ErrProofLedgerPreviousHashMismatch, core.ErrJSONContract)
	}
}

func TestPageValidationPreservesExactSequenceWithoutDuplicates(t *testing.T) {
	t.Parallel()
	genesis, err := NewGenesisHead(fixtureLedger(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	first := fixtureEvent(t, genesis, 0, 1)
	second := fixtureEvent(t, first.Head(), 1, 2)
	limit, err := NewPageLimit(2)
	if err != nil {
		t.Fatalf("NewPageLimit(2) error = %v, want nil", err)
	}
	page := Page[ledgerTestPayload]{After: genesis, Limit: limit, Events: []Envelope[ledgerTestPayload]{first, second}, Next: second.Head()}
	if gotErr := page.Validate(); gotErr != nil {
		t.Fatalf("Page.Validate(exact sequence) error = %v, want nil", gotErr)
	}
	cases := []struct {
		name   string
		events []Envelope[ledgerTestPayload]
		next   Head
	}{
		{name: "duplicate first event is refused", events: []Envelope[ledgerTestPayload]{first, first}, next: first.Head()},
		{name: "reverse order is refused", events: []Envelope[ledgerTestPayload]{second, first}, next: first.Head()},
		{name: "missing second event cannot claim second head", events: []Envelope[ledgerTestPayload]{first}, next: second.Head()},
		{name: "event after cursor gap is refused", events: []Envelope[ledgerTestPayload]{second}, next: second.Head()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Page[ledgerTestPayload]{After: genesis, Limit: limit, Events: tc.events, Next: tc.next}
			if gotErr := got.Validate(); !errors.Is(gotErr, core.ErrProofLedgerSequenceConflict) {
				t.Fatalf("Page.Validate(hostile order) error = %v, want %v", gotErr, core.ErrProofLedgerSequenceConflict)
			}
		})
	}
	limitOne, err := NewPageLimit(1)
	if err != nil {
		t.Fatalf("NewPageLimit(1) error = %v, want nil", err)
	}
	overLimit := Page[ledgerTestPayload]{After: genesis, Limit: limitOne, Events: []Envelope[ledgerTestPayload]{first, second}, Next: second.Head()}
	if gotErr := overLimit.Validate(); !errors.Is(gotErr, core.ErrProofLedgerSequenceConflict) {
		t.Fatalf("Page.Validate(over limit) error = %v, want %v", gotErr, core.ErrProofLedgerSequenceConflict)
	}
	underfilledMore := Page[ledgerTestPayload]{After: genesis, Limit: limit, Events: []Envelope[ledgerTestPayload]{first}, Next: first.Head(), More: true}
	if gotErr := underfilledMore.Validate(); !errors.Is(gotErr, core.ErrProofLedgerSequenceConflict) {
		t.Fatalf("Page.Validate(underfilled with more) error = %v, want %v", gotErr, core.ErrProofLedgerSequenceConflict)
	}
	encoded, err := page.Events[0].MarshalJSON()
	if err != nil || len(encoded) == 0 || bytes.Equal(encoded, page.Events[1].mustMarshalForTest(t)) {
		t.Fatalf("page event canonical evidence = (bytes=%d, error=%v, distinct=%t), want nonempty, nil, true", len(encoded), err, !bytes.Equal(encoded, page.Events[1].mustMarshalForTest(t)))
	}
}

func (e Envelope[P]) mustMarshalForTest(t testing.TB) []byte {
	t.Helper()
	got, err := e.MarshalJSON()
	if err != nil {
		t.Fatalf("Envelope.MarshalJSON() error = %v, want nil", err)
	}
	return got
}
