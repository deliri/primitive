package proofledger

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestProofLedgerReplayLayerTriad(t *testing.T) {
	t.Parallel()
	genesis, err := NewGenesisHead(fixtureLedger(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	first := fixtureEvent(t, genesis, 0, 1)
	second := fixtureEvent(t, first.Head(), 1, 2)
	third := fixtureEvent(t, second.Head(), 2, 3)
	sibling := fixtureEvent(t, genesis, 0, 3)
	foreignLink := fixtureEvent(t, sibling.Head(), 1, 2)
	tampered := second
	tampered.Payload.Value = 3
	for _, tc := range []struct {
		wantErr error
		name    string
		events  []Envelope[ledgerTestPayload]
		after   Head
		want    Head
	}{
		{name: "genesis_replays_three_distinct_links", after: genesis, events: []Envelope[ledgerTestPayload]{first, second, third}, want: third.Head(), wantErr: nil},
		{name: "resumed_cursor_replays_exact_suffix", after: first.Head(), events: []Envelope[ledgerTestPayload]{second, third}, want: third.Head(), wantErr: nil},
		{name: "empty_genesis_retains_empty_chain", after: genesis, events: nil, want: genesis, wantErr: nil},
		{name: "empty_suffix_retains_existing_head", after: second.Head(), events: nil, want: second.Head(), wantErr: nil},
		{name: "duplicate_event_cannot_advance_twice", after: genesis, events: []Envelope[ledgerTestPayload]{first, first}, want: first.Head(), wantErr: core.ErrProofLedgerSequenceConflict},
		{name: "gap_cannot_skip_first_event", after: genesis, events: []Envelope[ledgerTestPayload]{second}, want: genesis, wantErr: core.ErrProofLedgerSequenceConflict},
		{name: "reverse_order_cannot_rewind_cursor", after: second.Head(), events: []Envelope[ledgerTestPayload]{first}, want: second.Head(), wantErr: core.ErrProofLedgerSequenceConflict},
		{name: "authentic_sibling_chain_cannot_replace_link", after: first.Head(), events: []Envelope[ledgerTestPayload]{foreignLink}, want: first.Head(), wantErr: core.ErrProofLedgerPreviousHashMismatch},
		{name: "payload_tampering_preserves_admitted_prefix", after: genesis, events: []Envelope[ledgerTestPayload]{first, tampered}, want: first.Head(), wantErr: core.ErrProofLedgerTampering},
		{name: "absent_event_cannot_create_evidence", after: genesis, events: []Envelope[ledgerTestPayload]{{}}, want: genesis, wantErr: core.ErrProofLedgerContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			verifier, err := NewVerifier[ledgerTestPayload](tc.after)
			if err != nil {
				t.Fatalf("NewVerifier() error = %v, want nil", err)
			}
			for _, event := range tc.events {
				data, encodeErr := event.MarshalJSON()
				if encodeErr == nil {
					decoded, decodeErr := DecodeEnvelope[ledgerTestPayload, *ledgerTestPayload](data)
					if decodeErr != nil || decoded != event {
						t.Fatalf("DecodeEnvelope() = (%v, %v), want exact issued event", decoded.Head(), decodeErr)
					}
					event = decoded
				}
				err = verifier.Observe(event)
				if err != nil {
					break
				}
			}
			if !errors.Is(err, tc.wantErr) || verifier.Head() != tc.want || err != nil && !errors.Is(err, core.ErrProofLedgerContract) {
				t.Fatalf("Observe() = (%v, %v), want (%v, %v)", verifier.Head(), err, tc.want, tc.wantErr)
			}
			if err := verifier.Finish(tc.want); err != nil {
				t.Fatalf("Finish(admitted prefix) error = %v, want nil", err)
			}
			if tc.want != third.Head() {
				if err := verifier.Finish(third.Head()); !errors.Is(err, core.ErrProofLedgerTruncated) || verifier.Head() != tc.want {
					t.Fatalf("Finish(unseen suffix) error = %v, want truncation with preserved head", err)
				}
			}
		})
	}
	var absent *Verifier[ledgerTestPayload]
	if err := absent.Observe(first); !errors.Is(err, core.ErrProofLedgerContract) {
		t.Fatalf("Observe(nil) error = %v, want contract refusal", err)
	}
}

func TestProofLedgerSequenceExhaustionLayerTriad(t *testing.T) {
	t.Parallel()
	after := Head{Ledger: fixtureLedger(t), Sequence: math.MaxUint64 - 1, Hash: core.SHA256Of([]byte("last predecessor"))}
	last := fixtureEvent(t, after, 0, 1)
	limit, err := NewPageLimit(1)
	if err != nil {
		t.Fatalf("NewPageLimit(1) error = %v, want nil", err)
	}
	for _, tc := range []struct {
		wantErr error
		name    string
		more    bool
	}{
		{name: "last_native_sequence_is_a_final_page", more: false, wantErr: nil},
		{name: "last_native_sequence_cannot_promise_a_successor", more: true, wantErr: core.ErrProofLedgerSequenceConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			page := Page[ledgerTestPayload]{After: after, Next: last.Head(), Events: []Envelope[ledgerTestPayload]{last}, Limit: limit, More: tc.more}
			if err := page.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Page.Validate() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
	got, err := NewEnvelope(Issue[ledgerTestPayload]{Intent: fixtureIntent(t, last.Head(), 2, 2), Event: fixtureEventIdentity(t, 1), RecordedAt: fixtureInstant(t, 2)})
	if got != (Envelope[ledgerTestPayload]{}) || !errors.Is(err, core.ErrProofLedgerSequenceConflict) || !errors.Is(err, core.ErrProofLedgerContract) {
		t.Fatalf("NewEnvelope(exhausted) = (%v, %v), want zero conflict/contract refusal", got.Head(), err)
	}
	verifier, err := NewVerifier[ledgerTestPayload](last.Head())
	if err != nil {
		t.Fatalf("NewVerifier(last) error = %v, want nil", err)
	}
	if err := verifier.Observe(last); !errors.Is(err, core.ErrProofLedgerSequenceConflict) || verifier.Head() != last.Head() {
		t.Fatalf("Observe(exhausted) error = %v, want conflict with unchanged head", err)
	}
	if err := verifier.Finish(last.Head()); err != nil {
		t.Fatalf("Finish(exhausted) error = %v, want nil", err)
	}
}
