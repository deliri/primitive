package receipt

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestEvidenceSchemaJSONLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 100)
	document := issueFixture(t, fixture)
	cases := []struct {
		value core.ValidatedJSONMarshaler
		new   func() core.Validatable
		name  string
	}{
		{name: "body", value: fixture.body, new: func() core.Validatable { return &EvidenceBody{} }},
		{name: "header", value: document.Payload.Header, new: func() core.Validatable { return &Header{} }},
		{name: "payload", value: document.Payload, new: func() core.Validatable { return &EvidencePayload{} }},
		{name: "document", value: document, new: func() core.Validatable { return &EvidenceDocument{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			canonical, gotErr := tc.value.MarshalJSON()
			if gotErr != nil {
				t.Fatalf("MarshalJSON() error = %v, want nil", gotErr)
			}
			receiver := tc.new()
			if gotErr := json.Unmarshal(canonical, receiver); gotErr != nil ||
				receiver.Validate() != nil {
				t.Fatalf("json.Unmarshal(canonical) = (%v, %v), want valid and nil", receiver, gotErr)
			}
		})
	}
}

func TestEvidenceDocumentStrictJSONHostileMatrix(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 110)
	document := issueFixture(t, fixture)
	canonical, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal(document) error = %v, want nil", err)
	}
	reordered, err := json.Marshal(struct {
		Attestation attest.Envelope[Domain] `json:"attestation"`
		Payload     EvidencePayload         `json:"payload"`
	}{Attestation: document.Attestation, Payload: document.Payload})
	if err != nil {
		t.Fatalf("json.Marshal(reordered fixture) error = %v, want nil", err)
	}
	validCases := []struct {
		name string
		data []byte
	}{
		{name: "canonical document is admitted", data: canonical},
		{name: "bounded surrounding whitespace is admitted", data: append(append([]byte(" \n\t"), canonical...), '\r')},
		{name: "member reordering is admitted and normalized", data: reordered},
	}
	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got EvidenceDocument
			if gotErr := got.UnmarshalJSON(tc.data); gotErr != nil || got != document {
				t.Fatalf("EvidenceDocument.UnmarshalJSON() = (%v, %v), want exact document and nil", got, gotErr)
			}
		})
	}

	duplicate := append([]byte(`{"payload":`), canonical[len(`{"payload":`):]...)
	duplicate = bytes.Replace(duplicate, []byte(`,"attestation":`), []byte(`,"payload":{},"attestation":`), 1)
	hostileCases := []struct {
		name string
		data []byte
	}{
		{name: "empty input is rejected", data: nil},
		{name: "null is rejected", data: []byte("null")},
		{name: "truncation is rejected", data: canonical[:len(canonical)-1]},
		{name: "trailing document is rejected", data: append(append([]byte{}, canonical...), []byte("{}")...)},
		{name: "duplicate member is rejected", data: duplicate},
		{name: "unknown member is rejected", data: bytes.Replace(canonical, []byte(`"payload":`), []byte(`"unknown":0,"payload":`), 1)},
		{name: "wrong top-level type is rejected", data: []byte("[]")},
		{name: "invalid UTF-8 is rejected", data: []byte{'"', 0xff, '"'}},
		{name: "one above byte bound is rejected", data: bytes.Repeat([]byte{' '}, EvidenceDocumentJSONMaximumBytes+1)},
		{name: "nesting bomb is rejected", data: []byte(`{"payload":{"header":{"receipt_identity":{"nested":{}}}}}`)},
	}
	for _, tc := range hostileCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver := document
			gotErr := receiver.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || receiver != document {
				t.Fatalf("EvidenceDocument.UnmarshalJSON(%q) = (%v, %v), want preserved receiver and JSON rejection",
					tc.data, receiver, gotErr)
			}
		})
	}
}

func TestEvidenceCanonicalMaximaAreAttainable(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 120)
	var maximumID [ReceiptIDBytes]byte
	for index := range maximumID {
		maximumID[index] = math.MaxUint8
	}
	identity, err := NewReceiptID(maximumID)
	if err != nil {
		t.Fatalf("NewReceiptID(maximum) error = %v, want nil", err)
	}
	maximumBody := fixture.body
	maximumBody.Extent = mustByteLength(t, math.MaxInt64)
	payload := EvidencePayload{
		Header: Header{
			Identity: identity, Account: fixture.account, Offering: fixture.offering,
			Revision: RevisionV1, OccurredAt: temporal.InstantFromNanoseconds(math.MinInt64),
		},
		Body: maximumBody,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(maximum payload) error = %v, want nil", err)
	}
	if len(payloadJSON) != EvidencePayloadCanonicalJSONMaximumBytes {
		t.Fatalf("maximum payload extent = %d, want %d", len(payloadJSON), EvidencePayloadCanonicalJSONMaximumBytes)
	}
	document, err := IssueEvidence(IssueEvidenceRequest{
		Identity: identity, Account: fixture.account, Offering: fixture.offering,
		OccurredAt: payload.Header.OccurredAt, Body: maximumBody, Key: fixture.private,
	})
	if err != nil {
		t.Fatalf("IssueEvidence(maximum) error = %v, want nil", err)
	}
	documentJSON, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal(maximum document) error = %v, want nil", err)
	}
	if len(documentJSON) != EvidenceDocumentCanonicalJSONMaximumBytes {
		t.Fatalf("maximum document extent = %d, want %d", len(documentJSON), EvidenceDocumentCanonicalJSONMaximumBytes)
	}
}

type shortReceiptWriter struct{}

func (shortReceiptWriter) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	return len(data) - 1, nil
}

type failedReceiptWriter struct {
	err error
}

func (w failedReceiptWriter) Write([]byte) (int, error) { return 0, w.err }

func TestEvidenceCanonicalWriterPreservesNativeFailures(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 130)
	payload := issueFixture(t, fixture).Payload
	native := errors.New("hostile destination failure")
	cases := []struct {
		writer  io.Writer
		wantErr error
		name    string
	}{
		{name: "nil destination is rejected", wantErr: core.ErrReceiptContract},
		{name: "short write remains reachable", writer: shortReceiptWriter{}, wantErr: io.ErrShortWrite},
		{name: "native write failure remains reachable", writer: failedReceiptWriter{err: native}, wantErr: native},
		{name: "complete standard writer succeeds", writer: &bytes.Buffer{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := payload.WriteCanonical(tc.writer)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("EvidencePayload.WriteCanonical() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestSealedReceiptResultsRejectInternalContradictions(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 140)
	document := issueFixture(t, fixture)
	verified, err := VerifyEvidence(VerifyEvidenceRequest{
		Document: document, TrustedKeys: fixture.trusted, Expected: fixture.expectation,
	})
	if err != nil {
		t.Fatalf("VerifyEvidence() error = %v, want nil", err)
	}
	scope := Scope{Account: fixture.account, Offering: fixture.offering}
	watermark := watermarkFixture(t, scope, 2, "sealed")
	// wantErr is the oracle. A test that reads its own case name to decide the
	// expected outcome silently changes behavior on any rename.
	cases := []struct {
		value   core.Validatable
		wantErr error
		name    string
	}{
		{name: "authentic sealed evidence is admitted", value: verified},
		{name: "zero verified evidence is rejected", value: VerifiedEvidence{}, wantErr: core.ErrReceiptContract},
		{name: "sealed evidence with zero document is rejected", value: VerifiedEvidence{verified: true}, wantErr: core.ErrReceiptContract},
		{name: "sealed evidence with an unsigned document is rejected", value: VerifiedEvidence{verified: true, document: EvidenceDocument{Payload: document.Payload}}, wantErr: core.ErrReceiptContract},
		{name: "unsealed evidence over an authentic document is rejected", value: VerifiedEvidence{document: document}, wantErr: core.ErrReceiptContract},
		{name: "zero advance result is rejected", value: AdvanceResult{}, wantErr: core.ErrReceiptContract},
		{name: "state without watermark is rejected", value: AdvanceResult{state: AdvanceAccepted}, wantErr: core.ErrReceiptContract},
		{name: "watermark without state is rejected", value: AdvanceResult{watermark: watermark}, wantErr: core.ErrReceiptContract},
		{name: "future state over a real watermark is rejected", value: AdvanceResult{state: AdvanceState(math.MaxUint8), watermark: watermark}, wantErr: core.ErrReceiptContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if gotErr := tc.value.Validate(); !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

// TestSealedReceiptAccessorsRefuseRatherThanReturnSilentZeros ratchets the
// contract that an unset capability answers with an error.
//
// A silent zero is indistinguishable from a real answer at the call site: a
// caller that reads Header() off an unverified proof receives a structurally
// valid-looking zero header and no signal that nothing was authenticated. Every
// accessor must refuse instead.
func TestSealedReceiptAccessorsRefuseRatherThanReturnSilentZeros(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 145)
	document := issueFixture(t, fixture)
	watermark := watermarkFixture(
		t, Scope{Account: fixture.account, Offering: fixture.offering}, 2, "accessor",
	)
	forgedEvidence := VerifiedEvidence{document: document}
	forgedAdvance := AdvanceResult{watermark: watermark}
	cases := []struct {
		call func() error
		name string
	}{
		{name: "zero evidence document", call: func() error { _, err := (VerifiedEvidence{}).Document(); return err }},
		{name: "zero evidence header", call: func() error { _, err := (VerifiedEvidence{}).Header(); return err }},
		{name: "zero evidence body", call: func() error { _, err := (VerifiedEvidence{}).Body(); return err }},
		{name: "unsealed evidence document", call: func() error { _, err := forgedEvidence.Document(); return err }},
		{name: "unsealed evidence header", call: func() error { _, err := forgedEvidence.Header(); return err }},
		{name: "unsealed evidence body", call: func() error { _, err := forgedEvidence.Body(); return err }},
		{name: "zero advance state", call: func() error { _, err := (AdvanceResult{}).State(); return err }},
		{name: "zero advance watermark", call: func() error { _, err := (AdvanceResult{}).Watermark(); return err }},
		{name: "stateless advance state", call: func() error { _, err := forgedAdvance.State(); return err }},
		{name: "stateless advance watermark", call: func() error { _, err := forgedAdvance.Watermark(); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if gotErr := tc.call(); !errors.Is(gotErr, core.ErrReceiptContract) {
				t.Fatalf("accessor error = %v, want %v", gotErr, core.ErrReceiptContract)
			}
		})
	}
}

func TestWatermarkPersistenceJSONLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 180)
	scope := Scope{Account: fixture.account, Offering: fixture.offering}
	maximum := watermarkFixture(t, scope, math.MaxUint64, "maximum")
	canonical, err := json.Marshal(maximum)
	if err != nil {
		t.Fatalf("json.Marshal(maximum watermark) error = %v, want nil", err)
	}
	if len(canonical) != WatermarkCanonicalJSONMaximumBytes {
		t.Fatalf("maximum watermark extent = %d, want %d", len(canonical), WatermarkCanonicalJSONMaximumBytes)
	}
	var decoded Watermark
	if err := json.Unmarshal(canonical, &decoded); err != nil || decoded != maximum {
		t.Fatalf("json.Unmarshal(canonical watermark) = (%v, %v), want exact watermark and nil", decoded, err)
	}
	padded := append(append([]byte(" \n"), canonical...), '\t')
	if err := json.Unmarshal(padded, &decoded); err != nil || decoded != maximum {
		t.Fatalf("json.Unmarshal(padded watermark) = (%v, %v), want exact watermark and nil", decoded, err)
	}
	hostileCases := []struct {
		name string
		data []byte
	}{
		{name: "empty input is rejected", data: nil},
		{name: "null is rejected", data: []byte("null")},
		{name: "truncation is rejected", data: canonical[:len(canonical)-1]},
		{name: "missing generation is rejected", data: bytes.Replace(canonical, []byte(`"generation":18446744073709551615,`), nil, 1)},
		{name: "unknown member is rejected", data: bytes.Replace(canonical, []byte(`"revision":`), []byte(`"unknown":0,"revision":`), 1)},
		{name: "duplicate revision is rejected", data: bytes.Replace(canonical, []byte(`"revision":`), []byte(`"revision":"v1","revision":`), 1)},
		{name: "wrong top-level type is rejected", data: []byte("[]")},
		{name: "one above byte bound is rejected", data: bytes.Repeat([]byte{' '}, WatermarkJSONMaximumBytes+1)},
	}
	for _, tc := range hostileCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver := maximum
			gotErr := receiver.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || receiver != maximum {
				t.Fatalf("Watermark.UnmarshalJSON(%q) = (%v, %v), want preserved receiver and JSON rejection",
					tc.data, receiver, gotErr)
			}
		})
	}
	if gotErr := (*Watermark)(nil).UnmarshalJSON(canonical); !errors.Is(gotErr, core.ErrJSONContract) {
		t.Fatalf("nil Watermark.UnmarshalJSON() error = %v, want %v", gotErr, core.ErrJSONContract)
	}
}

func TestWatermarkNominalDigestJSONBoundaries(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 190)
	watermark := watermarkFixture(
		t,
		Scope{Account: fixture.account, Offering: fixture.offering},
		1,
		"digest-json",
	)
	cursorJSON, err := json.Marshal(watermark.CursorDigest)
	if err != nil {
		t.Fatalf("json.Marshal(CursorDigest) error = %v, want nil", err)
	}
	var cursor CursorDigest
	if err := json.Unmarshal(cursorJSON, &cursor); err != nil ||
		cursor != watermark.CursorDigest {
		t.Fatalf("json.Unmarshal(CursorDigest) = (%v, %v), want (%v, nil)",
			cursor, err, watermark.CursorDigest)
	}
	chainJSON, err := json.Marshal(watermark.ChainHash)
	if err != nil {
		t.Fatalf("json.Marshal(ChainHash) error = %v, want nil", err)
	}
	var chain ChainHash
	if err := json.Unmarshal(chainJSON, &chain); err != nil ||
		chain != watermark.ChainHash {
		t.Fatalf("json.Unmarshal(ChainHash) = (%v, %v), want (%v, nil)",
			chain, err, watermark.ChainHash)
	}
	for _, data := range [][]byte{nil, []byte("null"), []byte(`""`), []byte(`"00"`), []byte("{}")} {
		cursorReceiver := watermark.CursorDigest
		if gotErr := cursorReceiver.UnmarshalJSON(data); !errors.Is(gotErr, core.ErrJSONContract) ||
			cursorReceiver != watermark.CursorDigest {
			t.Fatalf("CursorDigest.UnmarshalJSON(%q) = (%v, %v), want preserved receiver and JSON rejection",
				data, cursorReceiver, gotErr)
		}
		chainReceiver := watermark.ChainHash
		if gotErr := chainReceiver.UnmarshalJSON(data); !errors.Is(gotErr, core.ErrJSONContract) ||
			chainReceiver != watermark.ChainHash {
			t.Fatalf("ChainHash.UnmarshalJSON(%q) = (%v, %v), want preserved receiver and JSON rejection",
				data, chainReceiver, gotErr)
		}
	}
}
