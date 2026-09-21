package proofledger

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// These are historical regression coordinates, not admission limits.
const formerEventExtent = 96 << 10
const formerPageExtent = 800 << 10
const formerReceiptExtent = 4 << 10
const formerReceiptDocumentExtent = 1 << 16

type extentPayload struct {
	Text string `json:"text"`
}
type extentPayloadWire extentPayload

func (p extentPayload) Validate() error {
	if p.Text == "" {
		return core.ErrProofLedgerContract
	}
	return nil
}
func (p extentPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONDocument(extentPayloadWire(p))
}
func (p *extentPayload) UnmarshalJSON(data []byte) error {
	wire, err := core.DecodeStrictJSONStructure[extentPayloadWire](data, core.ExtensibleJSONLimits())
	if err != nil {
		return err
	}
	candidate := extentPayload(wire)
	if err := candidate.Validate(); err != nil {
		return err
	}
	*p = candidate
	return nil
}

func extentIssue(t testing.TB, size int) Issue[extentPayload] {
	t.Helper()
	head, err := NewGenesisHead(fixtureLedger(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	return Issue[extentPayload]{Intent: AppendIntent[extentPayload]{
		Ledger: head.Ledger, ExpectedHead: head, Request: fixtureNonce(t, 1),
		Actor: fixtureKey(t, 1), Payload: extentPayload{Text: strings.Repeat("x", size)},
	}, Event: fixtureEventIdentity(t, 0), RecordedAt: fixtureInstant(t, 1)}
}

func TestProofLedgerEventExtentLayerTriad(t *testing.T) {
	t.Parallel()
	seed := extentIssue(t, 1)
	first, err := NewEnvelope(seed)
	if err != nil {
		t.Fatalf("NewEnvelope(seed) error = %v, want nil", err)
	}
	encoded, err := first.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
	}
	overhead := len(encoded) - 1
	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "one_below_former_event_quota", size: formerEventExtent - 1},
		{name: "exact_former_event_quota", size: formerEventExtent},
		{name: "one_above_former_event_quota", size: formerEventExtent + 1},
		{name: "many_windows_with_partial_tail", size: 1<<20 + 17},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			issue := extentIssue(t, tc.size-overhead)
			got, err := NewEnvelope(issue)
			if err != nil || got.Payload != issue.Intent.Payload {
				t.Fatalf("NewEnvelope(%d bytes) = (%v, %v), want exact payload and nil", tc.size, got.Head(), err)
			}
			encoded, err := got.MarshalJSON()
			if err != nil || len(encoded) != tc.size {
				t.Fatalf("MarshalJSON() = (%d bytes, %v), want (%d bytes, nil)", len(encoded), err, tc.size)
			}
			received, err := DecodeEnvelope[extentPayload, *extentPayload](encoded)
			if err != nil || received != got {
				t.Fatalf("DecodeEnvelope() = (%v, %v), want exact event %v", received.Head(), err, got.Head())
			}
			verifier, err := NewVerifier[extentPayload](issue.Intent.ExpectedHead)
			if err != nil {
				t.Fatalf("NewVerifier() error = %v, want nil", err)
			}
			if err := verifier.Observe(received); err != nil || verifier.Head() != got.Head() {
				t.Fatalf("Observe() = (%v, %v), want exact %v", verifier.Head(), err, got.Head())
			}
			if err := verifier.Finish(got.Head()); err != nil {
				t.Fatalf("Finish() error = %v, want nil", err)
			}
			refused, err := DecodeEnvelope[extentPayload, *extentPayload](encoded[:len(encoded)-1])
			if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrProofLedgerContract) || refused != (Envelope[extentPayload]{}) {
				t.Fatalf("DecodeEnvelope(truncated) = (%v, %v), want zero and typed refusal", refused.Head(), err)
			}
		})
	}
	t.Run("absent_payload_creates_no_event", func(t *testing.T) {
		t.Parallel()
		seed.Intent.Payload = extentPayload{}
		got, err := NewEnvelope(seed)
		if !errors.Is(err, core.ErrProofLedgerContract) || got != (Envelope[extentPayload]{}) {
			t.Fatalf("NewEnvelope(absent) = (%v, %v), want zero and contract refusal", got.Head(), err)
		}
	})
}

func TestProofLedgerPageExtentLayerTriad(t *testing.T) {
	t.Parallel()
	issue := extentIssue(t, formerPageExtent+1)
	event, err := NewEnvelope(issue)
	if err != nil {
		t.Fatalf("NewEnvelope(large payload) error = %v, want nil", err)
	}
	limit, err := NewPageLimit(1)
	if err != nil {
		t.Fatalf("NewPageLimit(1) error = %v, want nil", err)
	}
	for _, tc := range []struct {
		wantErr error
		name    string
		page    Page[extentPayload]
	}{
		{name: "large_event_preserves_page_continuation", page: Page[extentPayload]{Events: []Envelope[extentPayload]{event}, After: issue.Intent.ExpectedHead, Next: event.Head(), Limit: limit, More: true}, wantErr: nil},
		{name: "large_event_cannot_claim_wrong_next", page: Page[extentPayload]{Events: []Envelope[extentPayload]{event}, After: issue.Intent.ExpectedHead, Next: issue.Intent.ExpectedHead, Limit: limit}, wantErr: core.ErrProofLedgerSequenceConflict},
		{name: "empty_page_preserves_exact_cursor", page: Page[extentPayload]{After: event.Head(), Next: event.Head(), Limit: limit}, wantErr: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.page.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Page.Validate() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestProofLedgerReceiptExtentLayerTriad(t *testing.T) {
	t.Parallel()
	head, err := NewGenesisHead(fixtureLedger(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	document := fixtureReceiptDocument(t, fixtureEvent(t, head, 0, 1))
	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "below_former_receipt_quota", size: formerReceiptExtent - 1},
		{name: "exact_former_receipt_quota", size: formerReceiptExtent},
		{name: "above_former_receipt_quota", size: formerReceiptExtent + 1},
		{name: "below_former_document_quota", size: formerReceiptDocumentExtent - 1},
		{name: "exact_former_document_quota", size: formerReceiptDocumentExtent},
		{name: "above_former_document_quota", size: formerReceiptDocumentExtent + 1},
		{name: "many_windows_and_partial_tail", size: 1<<20 + 17},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, door := range []struct {
				marshal func() ([]byte, error)
				decode  func([]byte) (bool, error)
				name    string
			}{
				{name: "receipt", marshal: document.Receipt.MarshalJSON, decode: func(data []byte) (bool, error) {
					got := document.Receipt
					err := got.UnmarshalJSON(data)
					return got == document.Receipt, err
				}},
				{name: "document", marshal: document.MarshalJSON, decode: func(data []byte) (bool, error) {
					got := document
					err := got.UnmarshalJSON(data)
					return got == document, err
				}},
			} {
				t.Run(door.name, func(t *testing.T) {
					t.Parallel()
					canonical, err := door.marshal()
					if err != nil || len(canonical) >= tc.size {
						t.Fatalf("MarshalJSON() = (%d bytes, %v), want smaller than %d", len(canonical), err, tc.size)
					}
					data := append(bytes.Repeat([]byte{' '}, tc.size-len(canonical)), canonical...)
					if exact, err := door.decode(data); err != nil || !exact {
						t.Fatalf("UnmarshalJSON(%d bytes) exact=%t error=%v, want exact and nil", len(data), exact, err)
					}
					if exact, err := door.decode(data[:len(data)-1]); !exact || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrProofLedgerContract) {
						t.Fatalf("UnmarshalJSON(truncated) preserved=%t error=%v, want preserved and typed refusal", exact, err)
					}
				})
			}
		})
	}
}

func TestProofLedgerRefusalIdentityLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("absent_ledger_creates_no_genesis_head", func(t *testing.T) {
		t.Parallel()
		got, err := NewGenesisHead(LedgerIdentity{})
		if got != (Head{}) || !errors.Is(err, core.ErrProofLedgerContract) {
			t.Fatalf("NewGenesisHead(absent) = (%v, %v), want zero and contract refusal", got, err)
		}
	})
	for _, tc := range []struct {
		wantErr  error
		name     string
		sequence Sequence
		want     Sequence
	}{
		{name: "maximum_minus_one_advances_exactly", sequence: math.MaxUint64 - 1, want: math.MaxUint64, wantErr: nil},
		{name: "maximum_preserves_contract_and_conflict", sequence: math.MaxUint64, want: 0, wantErr: core.ErrProofLedgerSequenceConflict},
		{name: "zero_cannot_issue_a_successor", sequence: 0, want: 0, wantErr: core.ErrProofLedgerSequenceConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.sequence.Next()
			if got != tc.want || !errors.Is(err, tc.wantErr) || err != nil && !errors.Is(err, core.ErrProofLedgerContract) {
				t.Fatalf("Next() = (%d, %v), want (%d, %v) with contract identity on refusal", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

type receiptWriter struct {
	cause error
	bytes.Buffer
	limit int
}

func (w *receiptWriter) Write(data []byte) (int, error) {
	n, err := w.Buffer.Write(data[:min(len(data), w.limit)])
	return n, errors.Join(err, w.cause)
}

func TestProofLedgerCanonicalWriterLayerTriad(t *testing.T) {
	t.Parallel()
	head, err := NewGenesisHead(fixtureLedger(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	receipt := fixtureReceiptDocument(t, fixtureEvent(t, head, 0, 1)).Receipt
	canonical, err := receipt.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v, want nil", err)
	}
	for _, tc := range []struct {
		cause, wantErr error
		name           string
		value          AppendReceipt
		limit          int
		want           int
	}{
		{name: "exact_write_preserves_every_byte", value: receipt, limit: len(canonical), cause: nil, wantErr: nil, want: len(canonical)},
		{name: "partial_write_retains_native_failure", value: receipt, limit: 17, cause: io.ErrClosedPipe, wantErr: io.ErrClosedPipe, want: 17},
		{name: "full_progress_still_preserves_failure", value: receipt, limit: len(canonical), cause: io.ErrClosedPipe, wantErr: io.ErrClosedPipe, want: len(canonical)},
		{name: "short_write_without_error_is_refused", value: receipt, limit: len(canonical) - 1, cause: nil, wantErr: io.ErrShortWrite, want: len(canonical) - 1},
		{name: "absent_receipt_writes_nothing", value: AppendReceipt{}, limit: len(canonical), cause: nil, wantErr: core.ErrProofLedgerContract, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			writer := &receiptWriter{limit: tc.limit, cause: tc.cause}
			err := tc.value.WriteCanonical(writer)
			if !errors.Is(err, tc.wantErr) || writer.Len() != tc.want || !bytes.Equal(writer.Bytes(), canonical[:tc.want]) || err != nil && !errors.Is(err, core.ErrProofLedgerContract) {
				t.Fatalf("WriteCanonical() = (%d bytes, %v), want exact %d-byte prefix and %v", writer.Len(), err, tc.want, tc.wantErr)
			}
		})
	}
	if err := receipt.WriteCanonical(nil); !errors.Is(err, core.ErrProofLedgerContract) {
		t.Fatalf("WriteCanonical(nil) error = %v, want contract refusal", err)
	}
}
