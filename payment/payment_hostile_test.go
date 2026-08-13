package payment

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

type paymentFixtureRequest struct {
	Scope       receipt.Scope
	Marker      byte
	Millisecond int64
	MinorUnits  int64
}

type paymentFixture struct {
	private  ed25519.PrivateKey
	trusted  attest.TrustedKeys
	scope    receipt.Scope
	identity PaymentID
	document Document
}

type paymentCatalogFixtureRequest struct {
	Continuation Continuation
	Entries      uint16
	Marker       byte
}

type paymentCatalogFixture struct {
	private  ed25519.PrivateKey
	payload  CatalogPayload
	document CatalogDocument
	trusted  attest.TrustedKeys
	scope    receipt.Scope
	request  QueryPayload
}

type paymentJSONCase struct {
	name string
	data []byte
}

func TestPaymentPayloadSchemaLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newPaymentFixture(t, paymentFixtureRequest{
		Marker: 0x41, Millisecond: 1, MinorUnits: 1,
	})
	base := fixture.document.Payload

	t.Run("positive exact amount and service-period boundaries validate", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name       string
			minorUnits int64
			start      int64
			end        int64
		}{
			{name: "minimum positive amount and one-nanosecond service", minorUnits: 1, start: 1, end: 2},
			{name: "two minor units", minorUnits: 2, start: 1, end: 2},
			{name: "one below hundred minor units", minorUnits: 99, start: 1, end: 2},
			{name: "at hundred minor units", minorUnits: 100, start: 1, end: 2},
			{name: "one above hundred minor units", minorUnits: 101, start: 1, end: 2},
			{name: "maximum positive amount", minorUnits: math.MaxInt64, start: 1, end: 2},
			{name: "service crosses zero", minorUnits: 1, start: -1, end: 1},
			{name: "service begins at zero", minorUnits: 1, start: 0, end: 1},
			{name: "service ends at maximum instant fixture", minorUnits: 1, start: math.MaxInt64 - 1, end: math.MaxInt64},
			{name: "wide service interval", minorUnits: 1, start: -1_000_000, end: 1_000_000},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := base
				got.Amount = mustPaymentAmount(t, tc.minorUnits)
				got.Service = ServicePeriod{
					Start: temporal.InstantFromNanoseconds(tc.start),
					End:   temporal.InstantFromNanoseconds(tc.end),
				}
				if gotErr := got.Validate(); gotErr != nil {
					t.Fatalf("Payload.Validate(%d, [%d,%d)) error = %v, want nil", tc.minorUnits, tc.start, tc.end, gotErr)
				}
			})
		}
	})

	t.Run("negative missing nonpositive and unordered facts reject", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			mutate func(*Payload)
			name   string
		}{
			{name: "zero payload", mutate: func(value *Payload) { *value = Payload{} }},
			{name: "payment identity absent", mutate: func(value *Payload) { value.Identity = PaymentID{} }},
			{name: "tenant scope absent", mutate: func(value *Payload) { value.Scope = receipt.Scope{} }},
			{name: "amount absent", mutate: func(value *Payload) { value.Amount = currency.Amount{} }},
			{name: "settlement instant absent", mutate: func(value *Payload) { value.PaidAt = temporal.Instant{} }},
			{name: "service period absent", mutate: func(value *Payload) { value.Service = ServicePeriod{} }},
			{name: "zero amount", mutate: func(value *Payload) { value.Amount = mustPaymentAmount(t, 0) }},
			{name: "negative one amount", mutate: func(value *Payload) { value.Amount = mustPaymentAmount(t, -1) }},
			{name: "minimum negative amount", mutate: func(value *Payload) { value.Amount = mustPaymentAmount(t, math.MinInt64) }},
			{name: "service period has zero duration", mutate: func(value *Payload) { value.Service.End = value.Service.Start }},
			{name: "service period is reversed by one nanosecond", mutate: func(value *Payload) {
				value.Service = ServicePeriod{Start: temporal.InstantFromNanoseconds(2), End: temporal.InstantFromNanoseconds(1)}
			}},
			{name: "service period is widely reversed", mutate: func(value *Payload) {
				value.Service = ServicePeriod{Start: temporal.InstantFromNanoseconds(1_000_000), End: temporal.InstantFromNanoseconds(-1_000_000)}
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := base
				tc.mutate(&got)
				gotErr := got.Validate()
				if !errors.Is(gotErr, core.ErrPaymentContract) {
					t.Fatalf("Payload.Validate() error = %v, want errors.Is %v", gotErr, core.ErrPaymentContract)
				}
				encoded, marshalErr := got.MarshalJSON()
				if !errors.Is(marshalErr, core.ErrJSONContract) || encoded != nil {
					t.Fatalf("Payload.MarshalJSON() = (%q, %v), want nil and errors.Is %v", encoded, marshalErr, core.ErrJSONContract)
				}
			})
		}
	})

	t.Run("neutral zero payload emits no receipt document", func(t *testing.T) {
		t.Parallel()

		got, gotErr := Issue(Issuance{})
		if !errors.Is(gotErr, core.ErrPaymentContract) || got != (Document{}) {
			t.Fatalf("Issue(zero) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrPaymentContract)
		}
	})
}

func TestPaymentReceiptVerificationLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact authority identity and scope authenticate receipts", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			request paymentFixtureRequest
		}{
			{name: "minimum positive amount receipt", request: paymentFixtureRequest{Marker: 0x41, Millisecond: 1, MinorUnits: 1}},
			{name: "two minor unit receipt", request: paymentFixtureRequest{Marker: 0x42, Millisecond: 2, MinorUnits: 2}},
			{name: "one below hundred minor units", request: paymentFixtureRequest{Marker: 0x43, Millisecond: 3, MinorUnits: 99}},
			{name: "at hundred minor units", request: paymentFixtureRequest{Marker: 0x44, Millisecond: 4, MinorUnits: 100}},
			{name: "one above hundred minor units", request: paymentFixtureRequest{Marker: 0x45, Millisecond: 5, MinorUnits: 101}},
			{name: "one thousand minor units", request: paymentFixtureRequest{Marker: 0x46, Millisecond: 6, MinorUnits: 1_000}},
			{name: "ten thousand minor units", request: paymentFixtureRequest{Marker: 0x47, Millisecond: 7, MinorUnits: 10_000}},
			{name: "hundred thousand minor units", request: paymentFixtureRequest{Marker: 0x48, Millisecond: 8, MinorUnits: 100_000}},
			{name: "million minor units", request: paymentFixtureRequest{Marker: 0x49, Millisecond: 9, MinorUnits: 1_000_000}},
			{name: "maximum signed thirty-two-bit amount", request: paymentFixtureRequest{Marker: 0x4a, Millisecond: 10, MinorUnits: math.MaxInt32}},
		}
		for _, tc := range cases {
			fixture := newPaymentFixture(t, tc.request)
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, gotErr := Verify(Verification{
					Document:    fixture.document,
					Expected:    Expectation{Identity: fixture.identity, Scope: fixture.scope},
					TrustedKeys: fixture.trusted,
				})
				gotDocument, documentErr := got.Document()
				if gotErr != nil || documentErr != nil || gotDocument != fixture.document {
					t.Fatalf("Verify()/Verified.Document() = (%v, %v, %v), want exact document and nil", got, gotErr, documentErr)
				}
			})
		}
	})

	t.Run("negative expectation authority and every signed payment fact reject", func(t *testing.T) {
		t.Parallel()

		fixture := newPaymentFixture(t, paymentFixtureRequest{Marker: 0x52, Millisecond: 20, MinorUnits: 1250})
		other := newPaymentFixture(t, paymentFixtureRequest{Scope: fixture.scope, Marker: 0x62, Millisecond: 21, MinorUnits: 1251})
		cases := []struct {
			wantErr error
			mutate  func(*Verification)
			name    string
		}{
			{name: "zero verification", mutate: func(value *Verification) { *value = Verification{} }, wantErr: core.ErrPaymentContract},
			{name: "different expected payment identity", mutate: func(value *Verification) { value.Expected.Identity = other.identity }, wantErr: core.ErrPaymentVerification},
			{name: "different expected tenant scope", mutate: func(value *Verification) { value.Expected.Scope = paymentScopeFixture(t, 0x72) }, wantErr: core.ErrPaymentVerification},
			{name: "different authority trust set", mutate: func(value *Verification) { value.TrustedKeys = other.trusted }, wantErr: core.ErrPaymentVerification},
			{name: "signed payment identity substituted", mutate: func(value *Verification) { value.Document.Payload.Identity = other.identity }, wantErr: core.ErrPaymentVerification},
			{name: "signed tenant scope substituted", mutate: func(value *Verification) { value.Document.Payload.Scope = paymentScopeFixture(t, 0x73) }, wantErr: core.ErrPaymentVerification},
			{name: "signed amount substituted", mutate: func(value *Verification) { value.Document.Payload.Amount = mustPaymentAmount(t, 1251) }, wantErr: core.ErrPaymentVerification},
			{name: "signed settlement instant substituted", mutate: func(value *Verification) { value.Document.Payload.PaidAt = temporal.InstantFromNanoseconds(21) }, wantErr: core.ErrPaymentVerification},
			{name: "signed service start substituted", mutate: func(value *Verification) { value.Document.Payload.Service.Start = temporal.InstantFromNanoseconds(20) }, wantErr: core.ErrPaymentVerification},
			{name: "signed service end substituted", mutate: func(value *Verification) { value.Document.Payload.Service.End = temporal.InstantFromNanoseconds(23) }, wantErr: core.ErrPaymentVerification},
			{name: "receipt domain substituted with catalog domain", mutate: func(value *Verification) { value.Document.Attestation.Domain = SigningDomainCatalogV1 }, wantErr: core.ErrPaymentVerification},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				input := Verification{
					Document:    fixture.document,
					Expected:    Expectation{Identity: fixture.identity, Scope: fixture.scope},
					TrustedKeys: fixture.trusted,
				}
				tc.mutate(&input)
				got, gotErr := Verify(input)
				if !errors.Is(gotErr, tc.wantErr) || got != (Verified{}) {
					t.Fatalf("Verify() = (%v, %v), want zero and errors.Is %v", got, gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero verified receipt discloses no document", func(t *testing.T) {
		t.Parallel()

		got, gotErr := (Verified{}).Document()
		if !errors.Is(gotErr, core.ErrPaymentVerification) || got != (Document{}) {
			t.Fatalf("zero Verified.Document() = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrPaymentVerification)
		}
	})
}

func TestPaymentReceiptDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newPaymentFixture(t, paymentFixtureRequest{Marker: 0x5a, Millisecond: 30, MinorUnits: 125})
	canonical, gotErr := fixture.document.MarshalJSON()
	if gotErr != nil {
		t.Fatalf("Document.MarshalJSON() setup error = %v, want nil", gotErr)
	}

	t.Run("positive canonical reordered and exact extent documents preserve every fact", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			data []byte
		}{
			{name: "canonical receipt document", data: canonical},
			{name: "leading whitespace", data: append([]byte(" \n\t"), canonical...)},
			{name: "trailing whitespace", data: append(append([]byte(nil), canonical...), ' ', '\n', '\t')},
			{name: "both-side whitespace", data: append(append([]byte(" \n"), canonical...), '\n', ' ')},
			{name: "top-level members reordered", data: marshalReorderedPaymentReceipt(t, fixture.document)},
			{name: "one below document ceiling", data: paymentPadJSON(canonical, ReceiptDocumentJSONMaximumBytes-1)},
			{name: "at document ceiling", data: paymentPadJSON(canonical, ReceiptDocumentJSONMaximumBytes)},
			{name: "one trailing carriage return", data: append(append([]byte(nil), canonical...), '\r')},
			{name: "four leading whitespace forms", data: append([]byte("\t\r\n "), canonical...)},
			{name: "four trailing whitespace forms", data: append(append([]byte(nil), canonical...), " \n\r\t"...)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var got Document
				decodeErr := got.UnmarshalJSON(tc.data)
				if decodeErr != nil || got != fixture.document {
					t.Fatalf("Document.UnmarshalJSON() = (%v, %v), want exact receipt and nil", got, decodeErr)
				}
			})
		}
	})

	t.Run("negative malformed missing duplicate type-wrong and oversized documents reject", func(t *testing.T) {
		t.Parallel()

		cases := paymentDocumentHostileJSONCases(canonical, ReceiptDocumentJSONMaximumBytes)
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := fixture.document
				decodeErr := got.UnmarshalJSON(tc.data)
				if !errors.Is(decodeErr, core.ErrJSONContract) || got != fixture.document {
					t.Fatalf("Document.UnmarshalJSON() = (%v, %v), want preserved receiver and errors.Is %v", got, decodeErr, core.ErrJSONContract)
				}
			})
		}
	})

	t.Run("neutral rejected input discloses no receipt from zero receiver", func(t *testing.T) {
		t.Parallel()

		var got Document
		decodeErr := got.UnmarshalJSON(nil)
		if !errors.Is(decodeErr, core.ErrJSONContract) || got != (Document{}) {
			t.Fatalf("zero Document.UnmarshalJSON(nil) = (%v, %v), want zero and errors.Is %v", got, decodeErr, core.ErrJSONContract)
		}
	})
}

func TestPaymentCatalogIssuanceLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact empty through maximum pages issue signed documents", func(t *testing.T) {
		t.Parallel()

		cursor := mustPaymentCursor(t, 0x43)
		more, gotErr := More(cursor)
		if gotErr != nil {
			t.Fatalf("More() setup error = %v, want nil", gotErr)
		}
		cases := []paymentCatalogFixtureRequest{
			{Marker: 0x31, Entries: 0},
			{Marker: 0x32, Entries: 1},
			{Marker: 0x33, Entries: 2},
			{Marker: 0x34, Entries: 3},
			{Marker: 0x35, Entries: core.CatalogPageMaximumEntries/2 - 1},
			{Marker: 0x36, Entries: core.CatalogPageMaximumEntries / 2},
			{Marker: 0x37, Entries: core.CatalogPageMaximumEntries/2 + 1},
			{Marker: 0x38, Entries: core.CatalogPageMaximumEntries - 1},
			{Marker: 0x39, Entries: core.CatalogPageMaximumEntries},
			{Marker: 0x3a, Entries: 1, Continuation: more},
		}
		for _, tc := range cases {
			fixture := newPaymentCatalogFixture(t, tc)
			if fixture.document.Validate() != nil || fixture.document.Payload.Validate() != nil {
				t.Fatalf("IssueCatalog(%d entries) produced invalid document", tc.Entries)
			}
		}
	})

	t.Run("negative nil oversize unordered cross-scope and invalid continuation pages reject", func(t *testing.T) {
		t.Parallel()

		fixture := newPaymentCatalogFixture(t, paymentCatalogFixtureRequest{Marker: 0x44, Entries: 2})
		other := newPaymentCatalogFixture(t, paymentCatalogFixtureRequest{Marker: 0x54, Entries: 2})
		cursor := mustPaymentCursor(t, 0x64)
		more, gotErr := More(cursor)
		if gotErr != nil {
			t.Fatalf("More() setup error = %v, want nil", gotErr)
		}
		cases := []struct {
			wantErr error
			mutate  func(*CatalogPayload)
			name    string
		}{
			{name: "zero payload", mutate: func(value *CatalogPayload) { *value = CatalogPayload{} }, wantErr: core.ErrPaymentContract},
			{name: "nil entries", mutate: func(value *CatalogPayload) { value.Entries = nil }, wantErr: core.ErrPaymentContract},
			{name: "one above maximum entries", mutate: func(value *CatalogPayload) { value.Entries = make([]Document, core.CatalogPageMaximumEntries+1) }, wantErr: core.ErrPaymentContract},
			{name: "newest-first order reversed", mutate: func(value *CatalogPayload) { value.Entries[0], value.Entries[1] = value.Entries[1], value.Entries[0] }, wantErr: core.ErrPaymentVerification},
			{name: "duplicate payment identity", mutate: func(value *CatalogPayload) { value.Entries[1] = value.Entries[0] }, wantErr: core.ErrPaymentVerification},
			{name: "entry belongs to another scope", mutate: func(value *CatalogPayload) { value.Entries = []Document{other.payload.Entries[0]} }, wantErr: core.ErrPaymentVerification},
			{name: "watermark belongs to another scope", mutate: func(value *CatalogPayload) { value.Watermark = other.payload.Watermark }, wantErr: core.ErrPaymentVerification},
			{name: "tenant scope absent", mutate: func(value *CatalogPayload) { value.Scope = receipt.Scope{} }, wantErr: core.ErrPaymentContract},
			{name: "watermark absent", mutate: func(value *CatalogPayload) { value.Watermark = receipt.Watermark{} }, wantErr: core.ErrPaymentContract},
			{name: "observation instant absent", mutate: func(value *CatalogPayload) { value.ObservedAt = temporal.Instant{} }, wantErr: core.ErrPaymentContract},
			{name: "continuation absent", mutate: func(value *CatalogPayload) { value.Continuation = Continuation{} }, wantErr: core.ErrPaymentContract},
			{name: "end continuation carries cursor", mutate: func(value *CatalogPayload) {
				value.Continuation = Continuation{State: core.CatalogContinuationEnd, Cursor: cursor}
			}, wantErr: core.ErrPaymentContract},
			{name: "more continuation omits cursor", mutate: func(value *CatalogPayload) { value.Continuation = Continuation{State: core.CatalogContinuationMore} }, wantErr: core.ErrPaymentContract},
			{name: "empty page claims continuation", mutate: func(value *CatalogPayload) { value.Entries = []Document{}; value.Continuation = more }, wantErr: core.ErrPaymentVerification},
			{name: "future continuation state", mutate: func(value *CatalogPayload) {
				value.Continuation = Continuation{State: core.CatalogContinuationState(255)}
			}, wantErr: core.ErrPaymentContract},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				input := fixture.payload
				input.Entries = append([]Document(nil), fixture.payload.Entries...)
				tc.mutate(&input)
				got, issueErr := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: input})
				if !errors.Is(issueErr, tc.wantErr) || !samePaymentCatalogDocument(got, CatalogDocument{}) {
					t.Fatalf("IssueCatalog() = (%v, %v), want zero and errors.Is %v", got, issueErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero issuance emits no catalog document", func(t *testing.T) {
		t.Parallel()

		got, gotErr := IssueCatalog(CatalogIssuance{})
		if !errors.Is(gotErr, core.ErrPaymentContract) || !samePaymentCatalogDocument(got, CatalogDocument{}) {
			t.Fatalf("IssueCatalog(zero) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrPaymentContract)
		}
	})
}

func TestPaymentCatalogVerificationLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact empty through maximum pages authenticate unchanged", func(t *testing.T) {
		t.Parallel()

		counts := []uint16{0, 1, 2, 3, 4, core.CatalogPageMaximumEntries/2 - 1,
			core.CatalogPageMaximumEntries / 2, core.CatalogPageMaximumEntries/2 + 1,
			core.CatalogPageMaximumEntries - 1, core.CatalogPageMaximumEntries}
		for index, count := range counts {
			fixture := newPaymentCatalogFixture(t, paymentCatalogFixtureRequest{Marker: byte(0x61 + index), Entries: count})
			got, gotErr := VerifyCatalog(CatalogVerification{
				Document: fixture.document, Request: fixture.request, TrustedKeys: fixture.trusted,
			})
			if gotErr != nil || !samePaymentCatalog(got, fixture.payload) {
				t.Fatalf("VerifyCatalog(%d entries) = (%v, %v), want exact authenticated payload", count, got, gotErr)
			}
		}
	})

	t.Run("negative expectation authority envelope and signed payload substitutions reject", func(t *testing.T) {
		t.Parallel()

		fixture := newPaymentCatalogFixture(t, paymentCatalogFixtureRequest{Marker: 0x71, Entries: 2})
		other := newPaymentCatalogFixture(t, paymentCatalogFixtureRequest{Marker: 0x81, Entries: 2})
		more, gotErr := More(mustPaymentCursor(t, 0x72))
		if gotErr != nil {
			t.Fatalf("More() setup error = %v, want nil", gotErr)
		}
		after, gotErr := After(mustPaymentCursor(t, 0x73))
		if gotErr != nil {
			t.Fatalf("After() setup error = %v, want nil", gotErr)
		}
		minimumLimit, gotErr := core.NewCatalogPageLimit(1)
		if gotErr != nil {
			t.Fatalf("core.NewCatalogPageLimit(1) error = %v, want nil", gotErr)
		}
		cases := []struct {
			wantErr error
			mutate  func(*CatalogVerification)
			name    string
		}{
			{name: "zero verification", mutate: func(value *CatalogVerification) { *value = CatalogVerification{} }, wantErr: core.ErrPaymentContract},
			{name: "document absent", mutate: func(value *CatalogVerification) { value.Document = CatalogDocument{} }, wantErr: core.ErrPaymentContract},
			{name: "request absent", mutate: func(value *CatalogVerification) { value.Request = QueryPayload{} }, wantErr: core.ErrPaymentContract},
			{name: "trusted authority absent", mutate: func(value *CatalogVerification) { value.TrustedKeys = attest.TrustedKeys{} }, wantErr: core.ErrPaymentContract},
			{name: "different exact request", mutate: func(value *CatalogVerification) { value.Request = other.request }, wantErr: core.ErrPaymentVerification},
			{name: "different requested nonce", mutate: func(value *CatalogVerification) { value.Request.Nonce = other.request.Nonce }, wantErr: core.ErrPaymentVerification},
			{name: "different requested selection", mutate: func(value *CatalogVerification) { value.Request.Query.Selection = signedQuerySpecific(t, 0x74) }, wantErr: core.ErrPaymentVerification},
			{name: "different requested position", mutate: func(value *CatalogVerification) { value.Request.Query.Position = after }, wantErr: core.ErrPaymentVerification},
			{name: "different requested limit", mutate: func(value *CatalogVerification) { value.Request.Query.Limit = minimumLimit }, wantErr: core.ErrPaymentVerification},
			{name: "different requested build", mutate: func(value *CatalogVerification) { value.Request.Build = signedQueryBuild(t, core.OfferingBug) }, wantErr: core.ErrPaymentVerification},
			{name: "different authority trust set", mutate: func(value *CatalogVerification) { value.TrustedKeys = other.trusted }, wantErr: core.ErrPaymentVerification},
			{name: "signed observation instant substituted", mutate: func(value *CatalogVerification) {
				value.Document.Payload.ObservedAt = temporal.InstantFromNanoseconds(1_000_001)
			}, wantErr: core.ErrPaymentVerification},
			{name: "signed continuation substituted", mutate: func(value *CatalogVerification) { value.Document.Payload.Continuation = more }, wantErr: core.ErrPaymentVerification},
			{name: "signed query commitment substituted", mutate: func(value *CatalogVerification) { value.Document.Payload.Request = other.payload.Request }, wantErr: core.ErrPaymentVerification},
			{name: "signed entries shortened", mutate: func(value *CatalogVerification) { value.Document.Payload.Entries = value.Document.Payload.Entries[:1] }, wantErr: core.ErrPaymentVerification},
			{name: "signed entry substituted", mutate: func(value *CatalogVerification) {
				value.Document.Payload.Entries = []Document{newPaymentFixture(t, paymentFixtureRequest{Scope: fixture.scope, Marker: 0x91, Millisecond: 2_000, MinorUnits: 2}).document}
			}, wantErr: core.ErrPaymentVerification},
			{name: "catalog domain substituted with receipt domain", mutate: func(value *CatalogVerification) { value.Document.Attestation.Domain = SigningDomainReceiptV1 }, wantErr: core.ErrPaymentVerification},
			{name: "catalog signer substituted", mutate: func(value *CatalogVerification) {
				value.Document.Attestation.Signer = other.document.Attestation.Signer
			}, wantErr: core.ErrPaymentVerification},
			{name: "catalog signature substituted", mutate: func(value *CatalogVerification) {
				value.Document.Attestation.Signature = other.document.Attestation.Signature
			}, wantErr: core.ErrPaymentVerification},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				input := CatalogVerification{Document: fixture.document, Request: fixture.request, TrustedKeys: fixture.trusted}
				input.Document.Payload.Entries = append([]Document(nil), fixture.payload.Entries...)
				tc.mutate(&input)
				got, verifyErr := VerifyCatalog(input)
				if !errors.Is(verifyErr, tc.wantErr) || !isZeroPaymentCatalog(got) {
					t.Fatalf("VerifyCatalog() = (%v, %v), want zero and errors.Is %v", got, verifyErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero verification authenticates no catalog", func(t *testing.T) {
		t.Parallel()

		got, gotErr := VerifyCatalog(CatalogVerification{})
		if !errors.Is(gotErr, core.ErrPaymentContract) || !isZeroPaymentCatalog(got) {
			t.Fatalf("VerifyCatalog(zero) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrPaymentContract)
		}
	})
}

func TestPaymentCatalogVerificationClosesSpecificSelectionToZeroOrOneExactPayment(t *testing.T) {
	t.Parallel()

	fixture := newPaymentCatalogFixture(t, paymentCatalogFixtureRequest{Marker: 0x91, Entries: 1})
	selected := fixture.payload.Entries[0].Payload.Identity
	selection, err := Specific(selected)
	if err != nil {
		t.Fatalf("Specific(selected) error = %v, want nil", err)
	}
	request := fixture.request
	request.Query.Selection = selection
	commitment, err := CommitQuery(request)
	if err != nil {
		t.Fatalf("CommitQuery(specific) error = %v, want nil", err)
	}
	payload := fixture.payload
	payload.Request = commitment
	payload.Continuation = End()
	document, err := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: payload})
	if err != nil {
		t.Fatalf("IssueCatalog(specific exact) error = %v, want nil", err)
	}
	verified, err := VerifyCatalog(CatalogVerification{
		Document: document, Request: request, TrustedKeys: fixture.trusted,
	})
	if err != nil || !samePaymentCatalog(verified, payload) {
		t.Fatalf("VerifyCatalog(specific exact) = (%v, %v), want exact payload and nil", verified, err)
	}

	emptyPayload := payload
	emptyPayload.Entries = []Document{}
	emptyDocument, err := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: emptyPayload})
	if err != nil {
		t.Fatalf("IssueCatalog(specific empty) error = %v, want nil", err)
	}
	verified, err = VerifyCatalog(CatalogVerification{
		Document: emptyDocument, Request: request, TrustedKeys: fixture.trusted,
	})
	if err != nil || !samePaymentCatalog(verified, emptyPayload) {
		t.Fatalf("VerifyCatalog(specific empty) = (%v, %v), want exact empty payload and nil", verified, err)
	}

	other := newPaymentFixture(t, paymentFixtureRequest{
		Scope: fixture.scope, Marker: 0xa1, Millisecond: 20_000, MinorUnits: 2,
	})
	otherSelection, err := Specific(other.identity)
	if err != nil {
		t.Fatalf("Specific(other) error = %v, want nil", err)
	}
	wrongRequest := request
	wrongRequest.Query.Selection = otherSelection
	wrongCommitment, err := CommitQuery(wrongRequest)
	if err != nil {
		t.Fatalf("CommitQuery(other specific) error = %v, want nil", err)
	}
	wrongPayload := payload
	wrongPayload.Request = wrongCommitment
	wrongDocument, err := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: wrongPayload})
	if err != nil {
		t.Fatalf("IssueCatalog(wrong specific entry) error = %v, want nil", err)
	}
	if got, gotErr := VerifyCatalog(CatalogVerification{
		Document: wrongDocument, Request: wrongRequest, TrustedKeys: fixture.trusted,
	}); !errors.Is(gotErr, core.ErrPaymentVerification) || !isZeroPaymentCatalog(got) {
		t.Fatalf("VerifyCatalog(wrong specific entry) = (%v, %v), want zero and errors.Is %v",
			got, gotErr, core.ErrPaymentVerification)
	}

	continuedPayload := payload
	continuedPayload.Continuation, err = More(mustPaymentCursor(t, 0xa2))
	if err != nil {
		t.Fatalf("More(specific) error = %v, want nil", err)
	}
	continuedDocument, err := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: continuedPayload})
	if err != nil {
		t.Fatalf("IssueCatalog(specific continuation) error = %v, want nil", err)
	}
	if got, gotErr := VerifyCatalog(CatalogVerification{
		Document: continuedDocument, Request: request, TrustedKeys: fixture.trusted,
	}); !errors.Is(gotErr, core.ErrPaymentVerification) || !isZeroPaymentCatalog(got) {
		t.Fatalf("VerifyCatalog(specific continuation) = (%v, %v), want zero and errors.Is %v",
			got, gotErr, core.ErrPaymentVerification)
	}

	multiplePayload := payload
	multiplePayload.Entries = []Document{payload.Entries[0], other.document}
	if multiplePayload.Entries[0].Payload.Identity.String() < multiplePayload.Entries[1].Payload.Identity.String() {
		multiplePayload.Entries[0], multiplePayload.Entries[1] = multiplePayload.Entries[1], multiplePayload.Entries[0]
	}
	multipleDocument, err := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: multiplePayload})
	if err != nil {
		t.Fatalf("IssueCatalog(multiple specific entries) error = %v, want nil", err)
	}
	if got, gotErr := VerifyCatalog(CatalogVerification{
		Document: multipleDocument, Request: request, TrustedKeys: fixture.trusted,
	}); !errors.Is(gotErr, core.ErrPaymentVerification) || !isZeroPaymentCatalog(got) {
		t.Fatalf("VerifyCatalog(multiple specific entries) = (%v, %v), want zero and errors.Is %v",
			got, gotErr, core.ErrPaymentVerification)
	}
}

func TestPaymentCatalogDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newPaymentCatalogFixture(t, paymentCatalogFixtureRequest{Marker: 0x73, Entries: 2})
	canonical, gotErr := fixture.document.MarshalJSON()
	if gotErr != nil {
		t.Fatalf("CatalogDocument.MarshalJSON() setup error = %v, want nil", gotErr)
	}

	t.Run("positive canonical reordered and exact extent documents preserve every fact", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			data []byte
		}{
			{name: "canonical catalog document", data: canonical},
			{name: "leading whitespace", data: append([]byte(" \n\t"), canonical...)},
			{name: "trailing whitespace", data: append(append([]byte(nil), canonical...), ' ', '\n', '\t')},
			{name: "both-side whitespace", data: append(append([]byte(" \n"), canonical...), '\n', ' ')},
			{name: "top-level members reordered", data: marshalReorderedPaymentCatalog(t, fixture.document)},
			{name: "one below document ceiling", data: paymentPadJSON(canonical, core.JSONDocumentMaximumBytes-1)},
			{name: "at document ceiling", data: paymentPadJSON(canonical, core.JSONDocumentMaximumBytes)},
			{name: "one trailing carriage return", data: append(append([]byte(nil), canonical...), '\r')},
			{name: "four leading whitespace forms", data: append([]byte("\t\r\n "), canonical...)},
			{name: "four trailing whitespace forms", data: append(append([]byte(nil), canonical...), " \n\r\t"...)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var got CatalogDocument
				decodeErr := got.UnmarshalJSON(tc.data)
				if decodeErr != nil || !samePaymentCatalogDocument(got, fixture.document) {
					t.Fatalf("CatalogDocument.UnmarshalJSON() = (%v, %v), want exact catalog and nil", got, decodeErr)
				}
			})
		}
	})

	t.Run("negative malformed missing duplicate type-wrong and oversized documents reject", func(t *testing.T) {
		t.Parallel()

		cases := paymentDocumentHostileJSONCases(canonical, core.JSONDocumentMaximumBytes)
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := fixture.document
				decodeErr := got.UnmarshalJSON(tc.data)
				if !errors.Is(decodeErr, core.ErrJSONContract) || !samePaymentCatalogDocument(got, fixture.document) {
					t.Fatalf("CatalogDocument.UnmarshalJSON() = (%v, %v), want preserved receiver and errors.Is %v", got, decodeErr, core.ErrJSONContract)
				}
			})
		}
	})

	t.Run("neutral rejected input discloses no catalog from zero receiver", func(t *testing.T) {
		t.Parallel()

		var got CatalogDocument
		decodeErr := got.UnmarshalJSON(nil)
		if !errors.Is(decodeErr, core.ErrJSONContract) || !samePaymentCatalogDocument(got, CatalogDocument{}) {
			t.Fatalf("zero CatalogDocument.UnmarshalJSON(nil) = (%v, %v), want zero and errors.Is %v", got, decodeErr, core.ErrJSONContract)
		}
	})
}

func isZeroPaymentCatalog(payload CatalogPayload) bool {
	return payload.Entries == nil && payload.Scope == (receipt.Scope{}) &&
		payload.Watermark == (receipt.Watermark{}) &&
		payload.ObservedAt == (temporal.Instant{}) &&
		payload.Request == (QueryCommitment{}) &&
		payload.Continuation == (Continuation{})
}

func samePaymentCatalog(got, want CatalogPayload) bool {
	if got.Scope != want.Scope || got.Watermark != want.Watermark ||
		got.ObservedAt != want.ObservedAt || got.Request != want.Request ||
		got.Continuation != want.Continuation ||
		len(got.Entries) != len(want.Entries) || (got.Entries == nil) != (want.Entries == nil) {
		return false
	}
	for index := range got.Entries {
		if got.Entries[index] != want.Entries[index] {
			return false
		}
	}
	return true
}

func samePaymentCatalogDocument(got, want CatalogDocument) bool {
	return got.Attestation == want.Attestation && samePaymentCatalog(got.Payload, want.Payload)
}

func newPaymentCatalogFixture(t testing.TB, request paymentCatalogFixtureRequest) paymentCatalogFixture {
	t.Helper()

	if request.Marker == 0 {
		request.Marker = 1
	}
	if request.Continuation == (Continuation{}) {
		request.Continuation = End()
	}
	scope := paymentScopeFixture(t, request.Marker)
	query := paymentCatalogQueryPayload(t, scope, request.Marker)
	commitment, err := CommitQuery(query)
	if err != nil {
		t.Fatalf("CommitQuery() error = %v, want nil", err)
	}
	private, trusted := paymentSigningFixture(t, request.Marker+2)
	entries := make([]Document, 0, request.Entries)
	for remaining := request.Entries; remaining > 0; remaining-- {
		entry := newPaymentFixture(t, paymentFixtureRequest{
			Scope: scope, Marker: request.Marker + 3,
			Millisecond: int64(10_000 + remaining), MinorUnits: int64(remaining),
		})
		entries = append(entries, entry.document)
	}
	payload := CatalogPayload{
		Entries: entries, Watermark: paymentWatermarkFixture(t, scope),
		ObservedAt: temporal.InstantFromNanoseconds(1_000_000), Scope: scope, Request: commitment,
		Continuation: request.Continuation,
	}
	document, gotErr := IssueCatalog(CatalogIssuance{Signer: private, Payload: payload})
	if gotErr != nil {
		t.Fatalf("IssueCatalog(%d-entry fixture) error = %v, want nil", request.Entries, gotErr)
	}
	return paymentCatalogFixture{
		private: private, trusted: trusted, scope: scope, request: query, payload: payload, document: document,
	}
}

func paymentCatalogQueryPayload(t testing.TB, scope receipt.Scope, marker byte) QueryPayload {
	t.Helper()
	query, err := NewQuery(QueryRequest{
		Scope: scope, Selection: All(), Position: Start(), PageSize: core.CatalogPageMaximumEntries,
	})
	if err != nil {
		t.Fatalf("NewQuery() error = %v, want nil", err)
	}
	payload := QueryPayload{
		Query: query, Build: signedQueryBuild(t, core.OfferingWitness),
		Nonce: signedQueryNonce(t, marker), Revision: controlwire.Revision2026V1,
	}
	if err := payload.Validate(); err != nil {
		t.Fatalf("QueryPayload.Validate() error = %v, want nil", err)
	}
	return payload
}

func mustPaymentCursor(t *testing.T, marker byte) Cursor {
	t.Helper()

	got, gotErr := NewCursor(core.SHA256Of([]byte{marker}))
	if gotErr != nil {
		t.Fatalf("NewCursor() error = %v, want nil", gotErr)
	}
	return got
}

func marshalReorderedPaymentReceipt(t *testing.T, document Document) []byte {
	t.Helper()

	encoded, gotErr := core.MarshalCanonicalJSONDocument(struct {
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
		Payload     Payload                        `json:"payload"`
	}{Attestation: document.Attestation, Payload: document.Payload})
	if gotErr != nil {
		t.Fatalf("core.MarshalCanonicalJSONDocument(reordered receipt) error = %v, want nil", gotErr)
	}
	return encoded
}

func marshalReorderedPaymentCatalog(t *testing.T, document CatalogDocument) []byte {
	t.Helper()

	encoded, gotErr := core.MarshalCanonicalJSONDocument(struct {
		Payload     CatalogPayload                 `json:"payload"`
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
	}{Attestation: document.Attestation, Payload: document.Payload})
	if gotErr != nil {
		t.Fatalf("core.MarshalCanonicalJSONDocument(reordered catalog) error = %v, want nil", gotErr)
	}
	return encoded
}

func paymentPadJSON(document []byte, wantBytes int) []byte {
	if len(document) >= wantBytes {
		return append([]byte(nil), document...)
	}
	return append(append([]byte(nil), document...), bytes.Repeat([]byte{' '}, wantBytes-len(document))...)
}

func paymentDocumentHostileJSONCases(canonical []byte, maximumBytes int) []paymentJSONCase {
	return []paymentJSONCase{
		{name: "empty document", data: nil},
		{name: "whitespace-only document", data: []byte(" \n\t")},
		{name: "null document", data: []byte("null")},
		{name: "array instead of structure", data: []byte("[]")},
		{name: "string instead of structure", data: []byte(`"payment"`)},
		{name: "number instead of structure", data: []byte("1")},
		{name: "boolean instead of structure", data: []byte("true")},
		{name: "truncated opening brace", data: []byte("{")},
		{name: "truncated inside payload", data: canonical[:len(canonical)/2]},
		{name: "truncated before final brace", data: canonical[:len(canonical)-1]},
		{name: "trailing object", data: append(append([]byte(nil), canonical...), '{', '}')},
		{name: "two concatenated documents", data: append(append([]byte(nil), canonical...), canonical...)},
		{name: "unknown top-level member", data: bytes.Replace(canonical, []byte(`{"payload"`), []byte(`{"unknown":1,"payload"`), 1)},
		{name: "duplicate payload member", data: bytes.Replace(canonical, []byte(`{"payload":`), []byte(`{"payload":null,"payload":`), 1)},
		{name: "duplicate attestation member", data: bytes.Replace(canonical, []byte(`,"attestation":`), []byte(`,"attestation":null,"attestation":`), 1)},
		{name: "missing both required members", data: []byte("{}")},
		{name: "missing payload member", data: []byte(`{"attestation":null}`)},
		{name: "missing attestation member", data: []byte(`{"payload":null}`)},
		{name: "payload has wrong scalar type", data: []byte(`{"payload":1,"attestation":null}`)},
		{name: "attestation has wrong scalar type", data: []byte(`{"payload":null,"attestation":1}`)},
		{name: "one above document ceiling", data: paymentPadJSON(canonical, maximumBytes+1)},
	}
}

func TestPaymentQueryPlannerLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := newPaymentFixture(t, paymentFixtureRequest{
		Marker: 0x48, Millisecond: 20, MinorUnits: 1,
	})
	specific, err := Specific(fixture.identity)
	if err != nil {
		t.Fatalf("Specific() error = %v, want nil", err)
	}
	cursor, err := NewCursor(core.SHA256Of([]byte{0x49}))
	if err != nil {
		t.Fatalf("NewCursor() error = %v, want nil", err)
	}
	after, err := After(cursor)
	if err != nil {
		t.Fatalf("After() error = %v, want nil", err)
	}

	t.Run("positive all and specific plans close exact page boundaries", func(t *testing.T) {
		t.Parallel()

		midpoint := uint16(core.CatalogPageMaximumEntries / 2)
		cases := []struct {
			name      string
			selection Selection
			position  Position
			pageSize  uint16
		}{
			{name: "all start minimum page", selection: All(), position: Start(), pageSize: 1},
			{name: "all start two entries", selection: All(), position: Start(), pageSize: 2},
			{name: "all start one below midpoint", selection: All(), position: Start(), pageSize: midpoint - 1},
			{name: "all start at midpoint", selection: All(), position: Start(), pageSize: midpoint},
			{name: "all start one above midpoint", selection: All(), position: Start(), pageSize: midpoint + 1},
			{name: "all start one below maximum", selection: All(), position: Start(), pageSize: core.CatalogPageMaximumEntries - 1},
			{name: "all start at maximum", selection: All(), position: Start(), pageSize: core.CatalogPageMaximumEntries},
			{name: "all after minimum page", selection: All(), position: after, pageSize: 1},
			{name: "all after maximum page", selection: All(), position: after, pageSize: core.CatalogPageMaximumEntries},
			{name: "specific start minimum page", selection: specific, position: Start(), pageSize: 1},
			{name: "specific start maximum page", selection: specific, position: Start(), pageSize: core.CatalogPageMaximumEntries},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, gotErr := NewQuery(QueryRequest{
					Scope: fixture.scope, Selection: tc.selection, Position: tc.position, PageSize: tc.pageSize,
				})
				if gotErr != nil || got.Validate() != nil || got.Limit.Uint16() != tc.pageSize {
					t.Fatalf("NewQuery(page %d) = (%v, %v), want exact valid plan", tc.pageSize, got, gotErr)
				}
			})
		}
	})

	t.Run("negative zero contradictory future and over-bound plans reject", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			wantErr error
			mutate  func(*QueryRequest)
			name    string
		}{
			{name: "zero request", mutate: func(value *QueryRequest) { *value = QueryRequest{} }, wantErr: core.ErrPaymentContract},
			{name: "tenant scope absent", mutate: func(value *QueryRequest) { value.Scope = receipt.Scope{} }, wantErr: core.ErrPaymentContract},
			{name: "selection absent", mutate: func(value *QueryRequest) { value.Selection = Selection{} }, wantErr: core.ErrPaymentContract},
			{name: "position absent", mutate: func(value *QueryRequest) { value.Position = Position{} }, wantErr: core.ErrPaymentContract},
			{name: "zero page size", mutate: func(value *QueryRequest) { value.PageSize = 0 }, wantErr: core.ErrPaymentContract},
			{name: "page size one above maximum", mutate: func(value *QueryRequest) { value.PageSize = core.CatalogPageMaximumEntries + 1 }, wantErr: core.ErrPaymentContract},
			{name: "specific selection cannot continue", mutate: func(value *QueryRequest) { value.Selection = specific; value.Position = after }, wantErr: core.ErrPaymentVerification},
			{name: "all selection carries payment identity", mutate: func(value *QueryRequest) {
				value.Selection = Selection{Kind: core.CatalogSelectionAll, Payment: fixture.identity}
			}, wantErr: core.ErrPaymentContract},
			{name: "specific selection omits payment identity", mutate: func(value *QueryRequest) { value.Selection = Selection{Kind: core.CatalogSelectionSpecific} }, wantErr: core.ErrPaymentContract},
			{name: "future selection kind", mutate: func(value *QueryRequest) { value.Selection = Selection{Kind: core.CatalogSelectionKind(255)} }, wantErr: core.ErrPaymentContract},
			{name: "start position carries cursor", mutate: func(value *QueryRequest) { value.Position = Position{Kind: core.CatalogPositionStart, Cursor: cursor} }, wantErr: core.ErrPaymentContract},
			{name: "after position omits cursor", mutate: func(value *QueryRequest) { value.Position = Position{Kind: core.CatalogPositionAfter} }, wantErr: core.ErrPaymentContract},
			{name: "future position kind", mutate: func(value *QueryRequest) { value.Position = Position{Kind: core.CatalogPositionKind(255)} }, wantErr: core.ErrPaymentContract},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				input := QueryRequest{Scope: fixture.scope, Selection: All(), Position: Start(), PageSize: 1}
				tc.mutate(&input)
				got, gotErr := NewQuery(input)
				if !errors.Is(gotErr, tc.wantErr) || got != (Query{}) {
					t.Fatalf("NewQuery() = (%v, %v), want zero and errors.Is %v", got, gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral zero request creates no plausible query", func(t *testing.T) {
		t.Parallel()

		got, gotErr := NewQuery(QueryRequest{})
		if !errors.Is(gotErr, core.ErrPaymentContract) || got != (Query{}) {
			t.Fatalf("NewQuery(zero) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrPaymentContract)
		}
	})
}

func newPaymentFixture(t testing.TB, request paymentFixtureRequest) paymentFixture {
	t.Helper()

	if request.Marker == 0 {
		request.Marker = 1
	}
	if request.Millisecond == 0 {
		request.Millisecond = int64(request.Marker)
	}
	if request.MinorUnits == 0 {
		request.MinorUnits = 1
	}
	if request.Scope == (receipt.Scope{}) {
		request.Scope = paymentScopeFixture(t, request.Marker+1)
	}
	private, trusted := paymentSigningFixture(t, request.Marker)
	identity := mustPaymentID(t, request.Marker+2, request.Millisecond)
	payload := Payload{
		Identity: identity, Scope: request.Scope,
		Amount: mustPaymentAmount(t, request.MinorUnits),
		PaidAt: temporal.InstantFromNanoseconds(request.Millisecond),
		Service: ServicePeriod{
			Start: temporal.InstantFromNanoseconds(request.Millisecond + 1),
			End:   temporal.InstantFromNanoseconds(request.Millisecond + 2),
		},
	}
	document, err := Issue(Issuance{Signer: private, Payload: payload})
	if err != nil {
		t.Fatalf("payment.Issue() error = %v, want nil", err)
	}
	return paymentFixture{
		private: private, trusted: trusted, scope: request.Scope,
		identity: identity, document: document,
	}
}

func paymentSigningFixture(t testing.TB, marker byte) (ed25519.PrivateKey, attest.TrustedKeys) {
	t.Helper()

	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	public, err := core.NewEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{public}})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	return private, trusted
}

func paymentScopeFixture(t testing.TB, marker byte) receipt.Scope {
	t.Helper()
	return receipt.Scope{
		Account:  mustPaymentLifecycleIdentity(t, marker, receipt.NewAccountIdentity),
		Offering: mustPaymentOfferingIdentity(t, marker+1),
	}
}

func mustPaymentOfferingIdentity(t testing.TB, marker byte) receipt.OfferingIdentity {
	t.Helper()
	offerings := [...]core.Offering{core.OfferingBug, core.OfferingWitness, core.OfferingPeachfuzz}
	offering := offerings[int(marker)%len(offerings)]
	identity, err := receipt.OfferingIdentityFor(offering)
	if err != nil {
		t.Fatalf("receipt.OfferingIdentityFor(%v) error = %v, want nil", offering, err)
	}
	return identity
}

func mustPaymentLifecycleIdentity[T core.Validatable](
	t testing.TB,
	marker byte,
	constructor func([receipt.LifecycleIdentityBytes]byte) (T, error),
) T {
	t.Helper()
	value := [receipt.LifecycleIdentityBytes]byte{}
	value[0] = marker
	identity, err := constructor(value)
	if err != nil {
		t.Fatalf("payment lifecycle constructor error = %v, want nil", err)
	}
	return identity
}

func mustPaymentID(t testing.TB, marker byte, milliseconds int64) PaymentID {
	t.Helper()

	material, err := core.NewSecretMaterial(bytes.Repeat([]byte{marker}, core.SecretMaterialMinimumBytes))
	if err != nil {
		t.Fatalf("core.NewSecretMaterial() error = %v, want nil", err)
	}
	observation, err := temporal.NewObservation(time.UnixMilli(milliseconds))
	if err != nil {
		t.Fatalf("temporal.NewObservation() error = %v, want nil", err)
	}
	uuid, err := id.NewUUIDv7(id.Request{Entropy: material, Observation: observation})
	destroyErr := material.Destroy()
	if err != nil || destroyErr != nil {
		t.Fatalf("id.NewUUIDv7()/SecretMaterial.Destroy() errors = (%v, %v), want nil", err, destroyErr)
	}
	identity, err := NewPaymentID(uuid)
	if err != nil {
		t.Fatalf("NewPaymentID() error = %v, want nil", err)
	}
	return identity
}

func mustPaymentAmount(t testing.TB, minorUnits int64) currency.Amount {
	t.Helper()
	amount, err := currency.New(currency.CodeUSD, minorUnits)
	if err != nil {
		t.Fatalf("currency.New(%d) error = %v, want nil", minorUnits, err)
	}
	return amount
}

func paymentWatermarkFixture(t testing.TB, scope receipt.Scope) receipt.Watermark {
	t.Helper()

	generation, err := receipt.NewGeneration(1)
	if err != nil {
		t.Fatalf("receipt.NewGeneration() error = %v, want nil", err)
	}
	cursor, err := receipt.NewCursorDigest(core.SHA256Of([]byte{1}))
	if err != nil {
		t.Fatalf("receipt.NewCursorDigest() error = %v, want nil", err)
	}
	chain, err := receipt.NewChainHash(core.SHA256Of([]byte{2}))
	if err != nil {
		t.Fatalf("receipt.NewChainHash() error = %v, want nil", err)
	}
	watermark, err := receipt.NewWatermark(receipt.WatermarkRequest{
		Generation: generation, Scope: scope, CursorDigest: cursor, ChainHash: chain,
	})
	if err != nil {
		t.Fatalf("receipt.NewWatermark() error = %v, want nil", err)
	}
	return watermark
}
