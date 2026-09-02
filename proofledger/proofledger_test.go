package proofledger

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/temporal"
)

type ledgerTestPayload struct {
	Value uint8 `json:"value"`
}
type ledgerTestPayloadWire ledgerTestPayload

func (p ledgerTestPayload) Validate() error {
	if p.Value < 1 || p.Value > 3 {
		return core.ErrProofLedgerContract
	}
	return nil
}
func (p ledgerTestPayload) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(ledgerTestPayloadWire(p))
}
func (p *ledgerTestPayload) UnmarshalJSON(data []byte) error {
	if p == nil {
		return core.ErrJSONContract
	}
	w, e := core.DecodeStrictJSONStructure[ledgerTestPayloadWire](data, core.DefaultStrictJSONLimits())
	c := ledgerTestPayload(w)
	if e != nil {
		return e
	}
	if e = c.Validate(); e != nil {
		return e
	}
	*p = c
	return nil
}

func fixtureUUID(t testing.TB, value string) primitiveid.UUIDv7 {
	t.Helper()
	got, err := primitiveid.ParseUUIDv7(value)
	if err != nil {
		t.Fatalf("ParseUUIDv7(%q) error = %v, want nil", value, err)
	}
	return got
}
func fixtureLedger(t testing.TB) LedgerIdentity {
	t.Helper()
	got, err := NewLedgerIdentity(fixtureUUID(t, "01890f42-6a00-7000-8000-000000000001"))
	if err != nil {
		t.Fatalf("NewLedgerIdentity() error = %v, want nil", err)
	}
	return got
}
func fixtureEventIdentity(t testing.TB, index int) EventIdentity {
	t.Helper()
	values := []string{"01890f42-6a00-7000-8000-000000000002", "01890f42-6a00-7000-8000-000000000003", "01890f42-6a00-7000-8000-000000000004"}
	got, err := NewEventIdentity(fixtureUUID(t, values[index]))
	if err != nil {
		t.Fatalf("NewEventIdentity() error = %v, want nil", err)
	}
	return got
}
func fixtureNonce(t testing.TB, value byte) controlwire.RequestNonce {
	t.Helper()
	var raw [core.SHA256DigestBytes]byte
	raw[0] = value
	got, err := controlwire.NewRequestNonce(raw)
	if err != nil {
		t.Fatalf("NewRequestNonce() error = %v, want nil", err)
	}
	return got
}
func fixtureKey(t testing.TB, seed byte) core.Ed25519PublicKey {
	t.Helper()
	_, public := fixtureSigner(t, seed)
	return public
}

func fixtureSigner(t testing.TB, seed byte) (ed25519.PrivateKey, core.Ed25519PublicKey) {
	t.Helper()
	raw := make([]byte, ed25519.SeedSize)
	raw[0] = seed
	private := ed25519.NewKeyFromSeed(raw)
	got, err := core.NewEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("NewEd25519PublicKey() error = %v, want nil", err)
	}
	return private, got
}
func fixtureInstant(t testing.TB, second int) temporal.Instant {
	t.Helper()
	value := "2026-09-02T00:00:0" + string(rune('0'+second)) + "Z"
	got, err := temporal.ParseRFC3339(value)
	if err != nil {
		t.Fatalf("ParseRFC3339(%q) error = %v, want nil", value, err)
	}
	return got
}

func fixtureIntent(t testing.TB, head Head, nonce byte, payload uint8) AppendIntent[ledgerTestPayload] {
	t.Helper()
	return AppendIntent[ledgerTestPayload]{Request: fixtureNonce(t, nonce), Ledger: head.Ledger, ExpectedHead: head, Actor: fixtureKey(t, 1), Payload: ledgerTestPayload{Value: payload}}
}

func fixtureEvent(t testing.TB, head Head, index int, payload uint8) Envelope[ledgerTestPayload] {
	t.Helper()
	got, err := NewEnvelope(Issue[ledgerTestPayload]{Intent: fixtureIntent(t, head, byte(index+1), payload), Event: fixtureEventIdentity(t, index), RecordedAt: fixtureInstant(t, index+1)})
	if err != nil {
		t.Fatalf("NewEnvelope() error = %v, want nil", err)
	}
	return got
}

func fixtureReceiptDocument(t testing.TB, event Envelope[ledgerTestPayload]) AppendReceiptDocument {
	t.Helper()
	signer, producer := fixtureSigner(t, 2)
	document, err := IssueAppendReceipt(AppendReceiptIssuance[ledgerTestPayload]{Event: event, Producer: producer, Signer: signer})
	if err != nil {
		t.Fatalf("IssueAppendReceipt() error = %v, want nil", err)
	}
	return document
}

func fixtureTrustedKeys(t testing.TB, keys ...core.Ed25519PublicKey) attest.TrustedKeys {
	t.Helper()
	got, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: keys})
	if err != nil {
		t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
	}
	return got
}

func TestProofLedgerLayerTriad(t *testing.T) {
	t.Parallel()
	ledger := fixtureLedger(t)
	genesis, err := NewGenesisHead(ledger)
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	t.Run("positive chain advances one exact event", func(t *testing.T) {
		t.Parallel()
		event := fixtureEvent(t, genesis, 0, 1)
		verifier, gotErr := NewVerifier[ledgerTestPayload](genesis)
		if gotErr == nil {
			gotErr = verifier.Observe(event)
		}
		if gotErr != nil || verifier.Head() != event.Head() {
			t.Fatalf("Observe() = (%+v, %v), want (%+v, nil)", verifier.Head(), gotErr, event.Head())
		}
	})
	t.Run("negative broken predecessor is rejected", func(t *testing.T) {
		t.Parallel()
		event := fixtureEvent(t, genesis, 0, 1)
		event.PreviousHash = core.SHA256Of([]byte("wrong"))
		if gotErr := event.Validate(); !errors.Is(gotErr, core.ErrProofLedgerPreviousHashMismatch) {
			t.Fatalf("Envelope.Validate() error = %v, want %v", gotErr, core.ErrProofLedgerPreviousHashMismatch)
		}
	})
	t.Run("neutral empty page preserves cursor", func(t *testing.T) {
		t.Parallel()
		limit, limitErr := NewPageLimit(1)
		if limitErr != nil {
			t.Fatalf("NewPageLimit(1) error = %v, want nil", limitErr)
		}
		page := Page[ledgerTestPayload]{After: genesis, Limit: limit, Next: genesis}
		if gotErr := page.Validate(); gotErr != nil {
			t.Fatalf("Page.Validate(empty) error = %v, want nil", gotErr)
		}
	})
}

func TestProofLedgerChainTamperingAndTruncation(t *testing.T) {
	t.Parallel()
	genesis, _ := NewGenesisHead(fixtureLedger(t))
	first := fixtureEvent(t, genesis, 0, 1)
	second := fixtureEvent(t, first.Head(), 1, 2)
	verifier, _ := NewVerifier[ledgerTestPayload](genesis)
	if err := verifier.Observe(first); err != nil {
		t.Fatalf("Observe(first) error = %v, want nil", err)
	}
	if err := verifier.Finish(second.Head()); !errors.Is(err, core.ErrProofLedgerTruncated) {
		t.Fatalf("Finish(short prefix) error = %v, want %v", err, core.ErrProofLedgerTruncated)
	}
	second.PreviousHash = GenesisHash()
	if err := verifier.Observe(second); !errors.Is(err, core.ErrProofLedgerPreviousHashMismatch) {
		t.Fatalf("Observe(broken link) error = %v, want %v", err, core.ErrProofLedgerPreviousHashMismatch)
	}
}

func TestProofLedgerCanonicalJSONHostile(t *testing.T) {
	t.Parallel()
	genesis, _ := NewGenesisHead(fixtureLedger(t))
	event := fixtureEvent(t, genesis, 0, 1)
	encoded, err := event.MarshalJSON()
	if err != nil {
		t.Fatalf("Envelope.MarshalJSON() error = %v, want nil", err)
	}
	got, err := DecodeEnvelope[ledgerTestPayload, *ledgerTestPayload](encoded)
	if err != nil || got.Hash != event.Hash {
		t.Fatalf("DecodeEnvelope(canonical) = (%+v, %v), want hash %v and nil", got, err, event.Hash)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{name: "unknown member is refused", data: append(append([]byte(nil), encoded[:len(encoded)-1]...), []byte(`,"unknown":1}`)...)},
		{name: "duplicate member is refused", data: append(append([]byte(nil), encoded[:len(encoded)-1]...), []byte(`,"hash":"`+event.HashHex(t)+`"}`)...)},
		{name: "trailing data is refused", data: append(append([]byte(nil), encoded...), byte('x'))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, gotErr := DecodeEnvelope[ledgerTestPayload, *ledgerTestPayload](tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) {
				t.Fatalf("DecodeEnvelope(hostile) error = %v, want %v", gotErr, core.ErrJSONContract)
			}
		})
	}
}

func (e Envelope[P]) HashHex(t testing.TB) string {
	t.Helper()
	value, err := e.Hash.Hex()
	if err != nil {
		t.Fatalf("SHA256Digest.Hex() error = %v, want nil", err)
	}
	return value
}

type memoryEntry struct {
	digest  core.SHA256Digest
	receipt AppendReceiptDocument
}
type memoryAppender struct {
	replays  map[string]memoryEntry
	events   []Envelope[ledgerTestPayload]
	signer   ed25519.PrivateKey
	ids      []EventIdentity
	instants []temporal.Instant
	producer core.Ed25519PublicKey
}

func newMemoryAppender(t testing.TB) *memoryAppender {
	t.Helper()
	signer, producer := fixtureSigner(t, 2)
	return &memoryAppender{replays: make(map[string]memoryEntry), signer: signer, producer: producer, ids: []EventIdentity{fixtureEventIdentity(t, 0), fixtureEventIdentity(t, 1)}, instants: []temporal.Instant{fixtureInstant(t, 1), fixtureInstant(t, 2)}}
}
func (m *memoryAppender) Append(ctx context.Context, intent AppendIntent[ledgerTestPayload]) (AppendReceiptDocument, error) {
	if err := ctx.Err(); err != nil {
		return AppendReceiptDocument{}, err
	}
	digest, err := intent.Digest()
	if err != nil {
		return AppendReceiptDocument{}, err
	}
	key := intent.Request.String()
	if prior, ok := m.replays[key]; ok {
		if prior.digest != digest {
			return AppendReceiptDocument{}, core.ErrProofLedgerIdempotencyConflict
		}
		return prior.receipt, nil
	}
	current, err := NewGenesisHead(intent.Ledger)
	if err != nil {
		return AppendReceiptDocument{}, err
	}
	if len(m.events) > 0 {
		current = m.events[len(m.events)-1].Head()
	}
	if err := ValidateAppendHead(intent, current); err != nil {
		return AppendReceiptDocument{}, err
	}
	event, err := NewEnvelope(Issue[ledgerTestPayload]{Intent: intent, Event: m.ids[len(m.events)], RecordedAt: m.instants[len(m.events)]})
	if err != nil {
		return AppendReceiptDocument{}, err
	}
	receipt, err := IssueAppendReceipt(AppendReceiptIssuance[ledgerTestPayload]{Event: event, Producer: m.producer, Signer: m.signer})
	if err != nil {
		return AppendReceiptDocument{}, err
	}
	m.events = append(m.events, event)
	m.replays[key] = memoryEntry{digest: digest, receipt: receipt}
	return receipt, nil
}
func (m *memoryAppender) Resolve(ctx context.Context, request ResolveRequest) (AppendReceiptDocument, error) {
	if err := errors.Join(ctx.Err(), request.Validate()); err != nil {
		return AppendReceiptDocument{}, err
	}
	entry, ok := m.replays[request.Request.String()]
	if !ok {
		return AppendReceiptDocument{}, core.ErrProofLedgerAppendIndeterminate
	}
	if entry.receipt.Receipt.Ledger != request.Ledger {
		return AppendReceiptDocument{}, core.ErrProofLedgerAppendIndeterminate
	}
	return entry.receipt, nil
}

func TestProofLedgerIdempotentAppendContract(t *testing.T) {
	t.Parallel()
	genesis, _ := NewGenesisHead(fixtureLedger(t))
	provider := newMemoryAppender(t)
	intent := fixtureIntent(t, genesis, 1, 1)
	first, err := provider.Append(context.Background(), intent)
	if err != nil {
		t.Fatalf("Append(first) error = %v, want nil", err)
	}
	secondIntent := fixtureIntent(t, provider.events[0].Head(), 2, 2)
	second, err := provider.Append(context.Background(), secondIntent)
	if err != nil || second.Receipt.Sequence != 2 || len(provider.events) != 2 {
		t.Fatalf("Append(next head) = (%+v, %v, events=%d), want (sequence=2, nil, events=2)", second, err, len(provider.events))
	}
	replayed, err := provider.Append(context.Background(), intent)
	if err != nil || replayed != first || len(provider.events) != 2 {
		t.Fatalf("Append(replay after head advanced) = (%+v, %v, events=%d), want (%+v, nil, events=2)", replayed, err, len(provider.events), first)
	}
	intent.Payload.Value = 2
	if _, err := provider.Append(context.Background(), intent); !errors.Is(err, core.ErrProofLedgerIdempotencyConflict) {
		t.Fatalf("Append(conflicting replay) error = %v, want %v", err, core.ErrProofLedgerIdempotencyConflict)
	}
	stale := fixtureIntent(t, genesis, 3, 3)
	if _, err := provider.Append(context.Background(), stale); !errors.Is(err, core.ErrProofLedgerSequenceConflict) || len(provider.events) != 2 {
		t.Fatalf("Append(stale genesis) = (events=%d, %v), want (2, %v)", len(provider.events), err, core.ErrProofLedgerSequenceConflict)
	}
	resolved, err := provider.Resolve(context.Background(), ResolveRequest{Ledger: genesis.Ledger, Request: intent.Request})
	if err != nil || resolved != first {
		t.Fatalf("Resolve(committed request) = (%+v, %v), want original receipt %+v and nil", resolved, err, first)
	}
	otherLedger, err := NewLedgerIdentity(fixtureUUID(t, "01890f42-6a00-7000-8000-000000000009"))
	if err != nil {
		t.Fatalf("NewLedgerIdentity(other) error = %v, want nil", err)
	}
	if _, err := provider.Resolve(context.Background(), ResolveRequest{Ledger: otherLedger, Request: intent.Request}); !errors.Is(err, core.ErrProofLedgerAppendIndeterminate) {
		t.Fatalf("Resolve(request under another ledger) error = %v, want %v", err, core.ErrProofLedgerAppendIndeterminate)
	}
}

type memoryReader struct {
	events []Envelope[ledgerTestPayload]
}

type memoryIterator struct {
	events []Envelope[ledgerTestPayload]
	index  int
	closed bool
}

func (r memoryReader) ReadPage(ctx context.Context, request PageRequest) (Page[ledgerTestPayload], error) {
	if err := errors.Join(ctx.Err(), request.Validate()); err != nil {
		return Page[ledgerTestPayload]{}, err
	}
	start, err := r.start(request.After)
	if err != nil {
		return Page[ledgerTestPayload]{}, err
	}
	limit, err := request.Limit.Uint16()
	if err != nil {
		return Page[ledgerTestPayload]{}, err
	}
	end := min(start+int(limit), len(r.events))
	events := slices.Clone(r.events[start:end])
	next := request.After
	if len(events) > 0 {
		next = events[len(events)-1].Head()
	}
	page := Page[ledgerTestPayload]{After: request.After, Limit: request.Limit, Events: events, Next: next, More: end < len(r.events)}
	return page, page.Validate()
}

func (r memoryReader) Open(ctx context.Context, request PageRequest) (Iterator[ledgerTestPayload], error) {
	page, err := r.ReadPage(ctx, request)
	if err != nil {
		return nil, err
	}
	return &memoryIterator{events: page.Events}, nil
}

func (r memoryReader) start(after Head) (int, error) {
	genesis, err := NewGenesisHead(after.Ledger)
	if err != nil {
		return 0, err
	}
	if after == genesis {
		return 0, nil
	}
	for index := range r.events {
		if r.events[index].Head() == after {
			return index + 1, nil
		}
	}
	return 0, core.ErrProofLedgerSequenceConflict
}

func (i *memoryIterator) Next(ctx context.Context) (Envelope[ledgerTestPayload], error) {
	if err := ctx.Err(); err != nil {
		return Envelope[ledgerTestPayload]{}, err
	}
	if i == nil || i.closed || i.index >= len(i.events) {
		return Envelope[ledgerTestPayload]{}, io.EOF
	}
	event := i.events[i.index]
	i.index++
	return event, nil
}

func (i *memoryIterator) Close() error {
	if i == nil {
		return core.ErrProofLedgerContract
	}
	i.closed = true
	return nil
}

func TestProofLedgerReaderBoundsPagesAndPreservesExactSequence(t *testing.T) {
	t.Parallel()
	genesis, err := NewGenesisHead(fixtureLedger(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	first := fixtureEvent(t, genesis, 0, 1)
	second := fixtureEvent(t, first.Head(), 1, 2)
	reader := memoryReader{events: []Envelope[ledgerTestPayload]{first, second}}
	limit, err := NewPageLimit(1)
	if err != nil {
		t.Fatalf("NewPageLimit(1) error = %v, want nil", err)
	}
	firstPage, err := reader.ReadPage(context.Background(), PageRequest{Ledger: genesis.Ledger, After: genesis, Limit: limit})
	if err != nil || len(firstPage.Events) != 1 || firstPage.Events[0] != first || firstPage.Next != first.Head() || !firstPage.More {
		t.Fatalf("ReadPage(first) = (%+v, %v), want one first event, exact next, and more", firstPage, err)
	}
	secondPage, err := reader.ReadPage(context.Background(), PageRequest{Ledger: genesis.Ledger, After: firstPage.Next, Limit: limit})
	if err != nil || len(secondPage.Events) != 1 || secondPage.Events[0] != second || secondPage.Next != second.Head() || secondPage.More {
		t.Fatalf("ReadPage(second) = (%+v, %v), want one second event, exact next, and no more", secondPage, err)
	}
	iterator, err := reader.Open(context.Background(), PageRequest{Ledger: genesis.Ledger, After: genesis, Limit: limit})
	if err != nil {
		t.Fatalf("Open(first page) error = %v, want nil", err)
	}
	got, gotErr := iterator.Next(context.Background())
	if gotErr != nil || got != first {
		t.Fatalf("Iterator.Next(first) = (%+v, %v), want (%+v, nil)", got, gotErr, first)
	}
	if got, gotErr = iterator.Next(context.Background()); !errors.Is(gotErr, io.EOF) || got != (Envelope[ledgerTestPayload]{}) {
		t.Fatalf("Iterator.Next(past bounded page) = (%+v, %v), want zero and %v", got, gotErr, io.EOF)
	}
	if err := iterator.Close(); err != nil {
		t.Fatalf("Iterator.Close() error = %v, want nil", err)
	}
}

func TestProofLedgerReceiptAttestationLayerTriad(t *testing.T) {
	t.Parallel()
	genesis, err := NewGenesisHead(fixtureLedger(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	event := fixtureEvent(t, genesis, 0, 1)
	document := fixtureReceiptDocument(t, event)
	trusted := fixtureTrustedKeys(t, document.Receipt.Producer)

	t.Run("positive exact event and producer signature verify", func(t *testing.T) {
		t.Parallel()
		verified, gotErr := VerifyAppendReceiptDocument(AppendReceiptVerification[ledgerTestPayload]{Event: event, Document: document, TrustedKeys: trusted})
		if gotErr != nil {
			t.Fatalf("VerifyAppendReceiptDocument() error = %v, want nil", gotErr)
		}
		got, gotErr := verified.Document()
		if gotErr != nil || got != document {
			t.Fatalf("VerifiedAppendReceipt.Document() = (%+v, %v), want (%+v, nil)", got, gotErr, document)
		}
	})
	t.Run("negative event mutation cannot reuse signed receipt", func(t *testing.T) {
		t.Parallel()
		changed := fixtureEvent(t, genesis, 0, 2)
		_, gotErr := VerifyAppendReceiptDocument(AppendReceiptVerification[ledgerTestPayload]{Event: changed, Document: document, TrustedKeys: trusted})
		if !errors.Is(gotErr, core.ErrProofLedgerAppendReceiptMismatch) {
			t.Fatalf("VerifyAppendReceiptDocument(changed event) error = %v, want %v", gotErr, core.ErrProofLedgerAppendReceiptMismatch)
		}
	})
	t.Run("negative claimed producer must match the signing authority", func(t *testing.T) {
		t.Parallel()
		signer, _ := fixtureSigner(t, 2)
		got, gotErr := IssueAppendReceipt(AppendReceiptIssuance[ledgerTestPayload]{Event: event, Producer: fixtureKey(t, 3), Signer: signer})
		if !errors.Is(gotErr, core.ErrProofLedgerAppendReceiptMismatch) || got != (AppendReceiptDocument{}) {
			t.Fatalf("IssueAppendReceipt(mismatched producer) = (%+v, %v), want zero and %v", got, gotErr, core.ErrProofLedgerAppendReceiptMismatch)
		}
	})
	t.Run("neutral untrusted producer never becomes verified", func(t *testing.T) {
		t.Parallel()
		trustedOther := fixtureTrustedKeys(t, fixtureKey(t, 3))
		got, gotErr := VerifyAppendReceiptDocument(AppendReceiptVerification[ledgerTestPayload]{Event: event, Document: document, TrustedKeys: trustedOther})
		if !errors.Is(gotErr, core.ErrProofLedgerAppendReceiptMismatch) || got != (VerifiedAppendReceipt{}) {
			t.Fatalf("VerifyAppendReceiptDocument(untrusted) = (%+v, %v), want zero and %v", got, gotErr, core.ErrProofLedgerAppendReceiptMismatch)
		}
	})
}

func TestReceiptDocumentEncodedExtentBoundary(t *testing.T) {
	t.Parallel()
	genesis, err := NewGenesisHead(fixtureLedger(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	document := fixtureReceiptDocument(t, fixtureEvent(t, genesis, 0, 1))
	canonical, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("AppendReceiptDocument.MarshalJSON() error = %v, want nil", err)
	}
	spacesAtMaximum := bytes.Repeat([]byte{' '}, AppendReceiptDocumentJSONMaximumBytes-len(canonical))
	atMaximum := append(spacesAtMaximum, canonical...)
	oneAbove := append([]byte{' '}, atMaximum...)

	got := AppendReceiptDocument{}
	if gotErr := got.UnmarshalJSON(atMaximum); gotErr != nil || got != document {
		t.Fatalf("AppendReceiptDocument.UnmarshalJSON(at maximum) = (%+v, %v), want (%+v, nil)", got, gotErr, document)
	}
	got = document
	if gotErr := got.UnmarshalJSON(oneAbove); !errors.Is(gotErr, core.ErrJSONContract) || got != document {
		t.Fatalf("AppendReceiptDocument.UnmarshalJSON(one above maximum) = (%+v, %v), want preserved and %v", got, gotErr, core.ErrJSONContract)
	}
}

func TestProofLedgerCancellationDoesNotAppendOrResolveSuccess(t *testing.T) {
	t.Parallel()
	genesis, err := NewGenesisHead(fixtureLedger(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	provider := newMemoryAppender(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	intent := fixtureIntent(t, genesis, 1, 1)
	if _, gotErr := provider.Append(ctx, intent); !errors.Is(gotErr, context.Canceled) || len(provider.events) != 0 {
		t.Fatalf("Append(cancelled) = (events=%d, %v), want (0, %v)", len(provider.events), gotErr, context.Canceled)
	}
	request := ResolveRequest{Ledger: genesis.Ledger, Request: intent.Request}
	if _, gotErr := provider.Resolve(ctx, request); !errors.Is(gotErr, context.Canceled) {
		t.Fatalf("Resolve(cancelled) error = %v, want %v", gotErr, context.Canceled)
	}
}

func BenchmarkProofLedgerEventHash(b *testing.B) {
	genesis, _ := NewGenesisHead(fixtureLedger(b))
	intent := fixtureIntent(b, genesis, 1, 1)
	issue := Issue[ledgerTestPayload]{Intent: intent, Event: fixtureEventIdentity(b, 0), RecordedAt: fixtureInstant(b, 1)}
	b.ReportAllocs()
	b.ResetTimer()
	var sink Envelope[ledgerTestPayload]
	var err error
	for range b.N {
		sink, err = NewEnvelope(issue)
		if err != nil {
			b.Fatalf("NewEnvelope() error = %v, want nil", err)
		}
	}
	if sink.Hash == (core.SHA256Digest{}) {
		b.Fatalf("NewEnvelope() hash = %v, want a nonzero observed result", sink.Hash)
	}
}

func BenchmarkProofLedgerReceiptVerification(b *testing.B) {
	genesis, _ := NewGenesisHead(fixtureLedger(b))
	event := fixtureEvent(b, genesis, 0, 1)
	document := fixtureReceiptDocument(b, event)
	trusted := fixtureTrustedKeys(b, document.Receipt.Producer)
	verification := AppendReceiptVerification[ledgerTestPayload]{Event: event, Document: document, TrustedKeys: trusted}
	b.ReportAllocs()
	b.ResetTimer()
	var sink VerifiedAppendReceipt
	var err error
	for range b.N {
		sink, err = VerifyAppendReceiptDocument(verification)
		if err != nil {
			b.Fatalf("VerifyAppendReceiptDocument() error = %v, want nil", err)
		}
	}
	if err := sink.Validate(); err != nil {
		b.Fatalf("VerifiedAppendReceipt.Validate() error = %v, want nil", err)
	}
}

func BenchmarkProofLedgerStreamingChainReplayPerEvent(b *testing.B) {
	genesis, _ := NewGenesisHead(fixtureLedger(b))
	first := fixtureEvent(b, genesis, 0, 1)
	second := fixtureEvent(b, first.Head(), 1, 2)
	third := fixtureEvent(b, second.Head(), 2, 3)
	events := [...]Envelope[ledgerTestPayload]{first, second, third}
	b.ReportAllocs()
	b.ReportMetric(float64(len(events)), "events/op")
	b.ResetTimer()
	var sink Head
	for range b.N {
		verifier, err := NewVerifier[ledgerTestPayload](genesis)
		if err != nil {
			b.Fatalf("NewVerifier() error = %v, want nil", err)
		}
		for index := range events {
			if err := verifier.Observe(events[index]); err != nil {
				b.Fatalf("Verifier.Observe(event %d) error = %v, want nil", index, err)
			}
		}
		sink = verifier.Head()
	}
	if sink != third.Head() {
		b.Fatalf("Verifier.Head() = %+v, want %+v", sink, third.Head())
	}
}
