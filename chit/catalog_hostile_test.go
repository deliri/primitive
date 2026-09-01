package chit

import (
	"bytes"
	"crypto"
	"crypto/sha256"
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

type catalogFixture struct {
	private  crypto.Signer
	request  QueryPayload
	payload  CatalogPayload
	document CatalogDocument
	trusted  attest.TrustedKeys
}

func TestCustodyStateExhaustsItsByteDomainAndCanonicalJSON(t *testing.T) {
	t.Parallel()

	admitted := 0
	for value := 0; value <= 255; value++ {
		state := CustodyState(value)
		encoded, marshalErr := state.MarshalJSON()
		if state.IsValid() {
			admitted++
			if marshalErr != nil || state.String() == "" {
				t.Fatalf("CustodyState(%d) = (%q, %v), want named canonical member", value, state, marshalErr)
			}
			receiver := CustodyStateUnknown
			if err := receiver.UnmarshalJSON(encoded); err != nil || receiver != state {
				t.Fatalf("CustodyState(%d) JSON round trip = (%v, %v), want exact %v and nil", value, receiver, err, state)
			}
			continue
		}
		if !errors.Is(marshalErr, core.ErrChitContract) || !errors.Is(marshalErr, core.ErrJSONContract) ||
			encoded != nil || state.String() != "" {
			t.Fatalf("CustodyState(%d) = (%q, %v, %v), want unnamed nil output and errors.Is %v and %v",
				value, state, encoded, marshalErr, core.ErrChitContract, core.ErrJSONContract)
		}
	}
	if admitted != int(custodyStateLimit-CustodyStateUnknown-1) {
		t.Fatalf("admitted custody states = %d, want %d", admitted, custodyStateLimit-CustodyStateUnknown-1)
	}

	receiver := CustodyStateStored
	if err := receiver.UnmarshalJSON([]byte{}); !errors.Is(err, core.ErrJSONContract) || receiver != CustodyStateStored {
		t.Fatalf("CustodyState.UnmarshalJSON(empty) = (%v, %v), want preserved and errors.Is %v",
			receiver, err, core.ErrJSONContract)
	}
	var nilReceiver *CustodyState
	if err := nilReceiver.UnmarshalJSON([]byte{}); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil CustodyState.UnmarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
}

func TestCatalogLayerTriadAuthenticatesTenIndependentPages(t *testing.T) {
	t.Parallel()

	for index := range 10 {
		fixture := newCatalogFixture(t, byte(0x21+index), uint64(index+1))
		verified, err := VerifyCatalog(CatalogVerification{
			Document: fixture.document, Request: fixture.request, TrustedKeys: fixture.trusted,
		})
		if err != nil || !verifiedCatalogPayloadsEqual(verified, fixture.payload) {
			t.Fatalf("VerifyCatalog(configuration %d) = (%v, %v), want exact signed payload and nil",
				index, verified, err)
		}
		payloadJSON, err := fixture.payload.MarshalJSON()
		if err != nil {
			t.Fatalf("CatalogPayload.MarshalJSON(configuration %d) error = %v, want nil", index, err)
		}
		var payloadRoundTrip CatalogPayload
		if err := payloadRoundTrip.UnmarshalJSON(payloadJSON); err != nil ||
			!catalogPayloadsEqual(payloadRoundTrip, fixture.payload) {
			t.Fatalf("CatalogPayload round trip(configuration %d) = (%v, %v), want exact payload and nil",
				index, payloadRoundTrip, err)
		}
	}
}

func TestVerifiedCatalogOwnsAuthenticatedEntriesAcrossInputAndAccessorMutation(t *testing.T) {
	t.Parallel()

	fixture := newCatalogFixture(t, 0x34, 2)
	want := cloneCatalogPayload(fixture.payload)
	document := fixture.document
	verified, err := VerifyCatalog(CatalogVerification{
		Document: document, Request: fixture.request, TrustedKeys: fixture.trusted,
	})
	if err != nil {
		t.Fatalf("VerifyCatalog() error = %v, want nil", err)
	}
	document.Payload.Entries[0].State = CustodyStateDeleted
	first, err := verified.Payload()
	if err != nil {
		t.Fatalf("VerifiedCatalog.Payload(first) error = %v, want nil", err)
	}
	first.Entries[0].State = CustodyStateDeleted
	second, err := verified.Payload()
	if err != nil || !catalogPayloadsEqual(second, want) {
		t.Fatalf("VerifiedCatalog.Payload(after mutation) = (%v, %v), want original authenticated page", second, err)
	}
}

func TestCatalogPartitionLayerTriadExhaustsMatchingForeignAndEmptyRelations(t *testing.T) {
	t.Parallel()

	fixture := newCatalogFixture(t, 0x35, 1)
	type partitionRelation uint8
	const (
		partitionRelationUnknown partitionRelation = iota
		partitionRelationEmpty
		partitionRelationMatching
		partitionRelationForeign
	)

	type partitionRelationCase struct {
		name         string
		entryCount   int
		foreignIndex int
		relation     partitionRelation
		wantErr      error
	}
	cases := []partitionRelationCase{{
		name: "neutral empty partition emits no invented custody evidence", relation: partitionRelationEmpty,
	}}
	for _, count := range []int{1, 2, 3, 4, 5, 8, 16, 32, core.CatalogPageMaximumEntries - 1, core.CatalogPageMaximumEntries} {
		cases = append(cases, partitionRelationCase{
			name:       "positive matching partition page extent " + strconv.Itoa(count),
			entryCount: count, relation: partitionRelationMatching,
		})
	}
	for position := range 20 {
		cases = append(cases, partitionRelationCase{
			name:       "negative foreign partition at page position " + strconv.Itoa(position),
			entryCount: core.CatalogPageMaximumEntries, foreignIndex: position, relation: partitionRelationForeign,
			wantErr: core.ErrChitConflict,
		})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			payload := fixture.payload
			switch tc.relation {
			case partitionRelationEmpty:
				payload.Entries = []CatalogEntry{}
			case partitionRelationMatching:
				payload.Entries = catalogHistoryEntries(t, fixture, tc.entryCount)
			case partitionRelationForeign:
				payload.Entries = catalogHistoryEntries(t, fixture, tc.entryCount)
				foreign := payload.Entries[tc.foreignIndex]
				foreignPayload := foreign.Chit.Payload
				foreignPayload.Partition = mustPartition(t, 0xe5)
				foreignDocument, issueErr := Issue(Issuance{
					Signer: fixture.private, TrustedKeys: fixture.trusted, Payload: foreignPayload,
				})
				if issueErr != nil {
					t.Fatalf("Issue(foreign partition chit) error = %v, want nil", issueErr)
				}
				payload.Entries[tc.foreignIndex] = CatalogEntry{Chit: foreignDocument, State: foreign.State}
			case partitionRelationUnknown:
				t.Fatalf("partition relation = %d, want a compiler-owned test relation", tc.relation)
			default:
				t.Fatalf("partition relation = %d, want a published test relation", tc.relation)
			}
			payload.Continuation = End()
			document, issueErr := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: payload})
			if issueErr != nil {
				t.Fatalf("IssueCatalog() error = %v, want nil", issueErr)
			}
			got, gotErr := VerifyCatalog(CatalogVerification{
				Document: document, Request: fixture.request, TrustedKeys: fixture.trusted,
			})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (VerifiedCatalog{}) {
					t.Fatalf("VerifyCatalog() = (%v, %v), want zero and errors.Is %v", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || !verifiedCatalogPayloadsEqual(got, payload) {
				t.Fatalf("VerifyCatalog() = (%v, %v), want exact payload and nil", got, gotErr)
			}
		})
	}
}

func TestCatalogPaginationLayerTriadClosesTailOrderAndRequestedLimit(t *testing.T) {
	t.Parallel()

	fixture := newCatalogFixture(t, 0x21, 1)
	entries := catalogHistoryEntries(t, fixture, 10)

	t.Run("positive page tails derive exact distinct cursors through the full pressure set", func(t *testing.T) {
		t.Parallel()

		prior := Cursor{}
		for count := 1; count <= len(entries); count++ {
			pageEntries := append([]CatalogEntry(nil), entries[:count]...)
			cursor, err := CursorFor(pageEntries[len(pageEntries)-1].Chit.Payload.Identity)
			if err != nil {
				t.Fatalf("CursorFor(page tail %d) error = %v, want nil", count, err)
			}
			framed := CatalogCursorCommitmentDomain + string([]byte{CatalogCursorFrameSeparator}) +
				pageEntries[len(pageEntries)-1].Chit.Payload.Identity.String()
			wantDigest := core.NewSHA256Digest(sha256.Sum256([]byte(framed)))
			if cursor.value != wantDigest || (count > 1 && cursor == prior) {
				t.Fatalf("CursorFor(page tail %d) = (%v, distinct %t), want (%v, true)",
					count, cursor, cursor != prior, wantDigest)
			}
			prior = cursor

			continuation, err := More(cursor)
			if err != nil {
				t.Fatalf("More(page tail %d) error = %v, want nil", count, err)
			}
			request := fixture.request
			request.Query.Limit = catalogPageLimitFixture(t, uint16(count))
			commitment, err := CommitQuery(request)
			if err != nil {
				t.Fatalf("CommitQuery(page %d) error = %v, want nil", count, err)
			}
			payload := fixture.payload
			payload.Entries, payload.Request, payload.Continuation = pageEntries, commitment, continuation
			document, err := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: payload})
			if err != nil {
				t.Fatalf("IssueCatalog(page %d) error = %v, want nil", count, err)
			}
			got, gotErr := VerifyCatalog(CatalogVerification{
				Document: document, Request: request, TrustedKeys: fixture.trusted,
			})
			if gotErr != nil || !verifiedCatalogPayloadsEqual(got, payload) {
				t.Fatalf("VerifyCatalog(page %d) = (%v, %v), want exact payload and nil", count, got, gotErr)
			}
		}
	})

	t.Run("negative arbitrary tail cursors and every newest-first inversion conflict", func(t *testing.T) {
		t.Parallel()

		for index := range entries {
			pageEntries := append([]CatalogEntry(nil), entries...)
			if index < len(pageEntries)-1 {
				pageEntries[index], pageEntries[index+1] = pageEntries[index+1], pageEntries[index]
			} else {
				pageEntries[index] = pageEntries[index-1]
			}
			payload := fixture.payload
			payload.Entries = pageEntries
			if gotErr := payload.Validate(); !errors.Is(gotErr, core.ErrChitConflict) {
				t.Fatalf("CatalogPayload.Validate(order mutation %d) error = %v, want errors.Is %v", index, gotErr, core.ErrChitConflict)
			}

			payload.Entries = append([]CatalogEntry(nil), entries[:index+1]...)
			wrongIdentity := entries[(index+1)%len(entries)].Chit.Payload.Identity
			wrong, err := CursorFor(wrongIdentity)
			if err != nil {
				t.Fatalf("CursorFor(wrong tail %d) error = %v, want nil", index, err)
			}
			payload.Continuation, err = More(wrong)
			if err != nil {
				t.Fatalf("More(wrong tail %d) error = %v, want nil", index, err)
			}
			if gotErr := payload.Validate(); !errors.Is(gotErr, core.ErrChitConflict) {
				t.Fatalf("CatalogPayload.Validate(wrong tail %d) error = %v, want errors.Is %v", index, gotErr, core.ErrChitConflict)
			}
		}
	})

	t.Run("neutral end page carries no cursor and oversized response cannot exceed its request", func(t *testing.T) {
		t.Parallel()

		payload := fixture.payload
		payload.Entries = append([]CatalogEntry(nil), entries[:2]...)
		payload.Continuation = End()
		request := fixture.request
		request.Query.Limit = catalogPageLimitFixture(t, 1)
		commitment, err := CommitQuery(request)
		if err != nil {
			t.Fatalf("CommitQuery(one-entry limit) error = %v, want nil", err)
		}
		payload.Request = commitment
		document, err := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: payload})
		if err != nil {
			t.Fatalf("IssueCatalog(two-entry response) error = %v, want nil before request verification", err)
		}
		got, gotErr := VerifyCatalog(CatalogVerification{
			Document: document, Request: request, TrustedKeys: fixture.trusted,
		})
		if !errors.Is(gotErr, core.ErrChitConflict) || got != (VerifiedCatalog{}) {
			t.Fatalf("VerifyCatalog(two entries for limit one) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrChitConflict)
		}
	})
}

func TestVerifyCatalogRejectsEveryIndependentAgreementSubstitution(t *testing.T) {
	t.Parallel()

	fixture := newCatalogFixture(t, 0x40, 1)
	other := newCatalogFixture(t, 0x71, 2)
	alternatePayload := fixture.payload
	alternatePayload.Entries = append([]CatalogEntry(nil), fixture.payload.Entries...)
	alternatePayload.Entries[0].Chit.Payload.Version = mustVersion(t, 2)
	alternateChit, err := Issue(Issuance{
		Signer: fixture.private, TrustedKeys: fixture.trusted,
		Payload: alternatePayload.Entries[0].Chit.Payload,
	})
	if err != nil {
		t.Fatalf("Issue(alternate catalog chit) error = %v, want nil", err)
	}
	alternateWatermark := catalogWatermarkFixture(t, fixture.payload.Scope, 0x62)
	moreCursor, err := CursorFor(fixture.payload.Entries[0].Chit.Payload.Identity)
	if err != nil {
		t.Fatalf("CursorFor(catalog tail) error = %v, want nil", err)
	}
	more, err := More(moreCursor)
	if err != nil {
		t.Fatalf("More() error = %v, want nil", err)
	}

	cases := []struct {
		mutate  func(*CatalogVerification)
		wantErr error
		name    string
	}{
		{name: "zero verification", wantErr: core.ErrChitContract, mutate: func(value *CatalogVerification) { *value = CatalogVerification{} }},
		{name: "foreign authority", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) { value.TrustedKeys = other.trusted }},
		{name: "foreign exact query", wantErr: core.ErrChitConflict, mutate: func(value *CatalogVerification) { value.Request = other.request }},
		{name: "foreign request nonce", wantErr: core.ErrChitConflict, mutate: func(value *CatalogVerification) { value.Request.Nonce = other.request.Nonce }},
		{name: "foreign requested selection", wantErr: core.ErrChitConflict, mutate: func(value *CatalogVerification) { value.Request.Query.Selection = signedQuerySpecific(t, 0x72) }},
		{name: "foreign requested position", wantErr: core.ErrChitConflict, mutate: func(value *CatalogVerification) { value.Request.Query.Position = signedQueryAfter(t, 0x73) }},
		{name: "signed observation substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Payload.ObservedAt = temporal.InstantFromNanoseconds(10_000)
		}},
		{name: "signed watermark substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Payload.Watermark = alternateWatermark
		}},
		{name: "signed query commitment substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Payload.Request = other.payload.Request
		}},
		{name: "signed chit substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Payload.Entries[0].Chit = alternateChit
		}},
		{name: "signed custody state substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			if value.Document.Payload.Entries[0].State == CustodyStateStored {
				value.Document.Payload.Entries[0].State = CustodyStateDeleted
			} else {
				value.Document.Payload.Entries[0].State = CustodyStateStored
			}
		}},
		{name: "signed continuation substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Payload.Continuation = more
		}},
		{name: "signing domain substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Attestation.Domain = SigningDomainQueryV1
		}},
		{name: "signer substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Attestation.Signer = other.document.Attestation.Signer
		}},
		{name: "signature substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Attestation.Signature = other.document.Attestation.Signature
		}},
		{name: "body digest substituted", wantErr: core.ErrChitVerification, mutate: func(value *CatalogVerification) {
			value.Document.Attestation.BodySHA256 = other.document.Attestation.BodySHA256
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := CatalogVerification{
				Document: cloneCatalogDocument(fixture.document), Request: fixture.request, TrustedKeys: fixture.trusted,
			}
			tc.mutate(&input)
			got, gotErr := VerifyCatalog(input)
			if !errors.Is(gotErr, tc.wantErr) || got != (VerifiedCatalog{}) {
				t.Fatalf("VerifyCatalog(%s) = (%v, %v), want zero and errors.Is %v", tc.name, got, gotErr, tc.wantErr)
			}
		})
	}
}

func TestVerifyCatalogClosesSpecificSelectionToZeroOrOneExactChit(t *testing.T) {
	t.Parallel()

	fixture := newCatalogFixture(t, 0x81, 1)
	selected := fixture.payload.Entries[0].Chit.Payload.Identity
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
	if err != nil || !verifiedCatalogPayloadsEqual(verified, payload) {
		t.Fatalf("VerifyCatalog(specific exact) = (%v, %v), want exact payload and nil", verified, err)
	}

	otherID := mustChitID(t, 0xf1, 900)
	otherSelection, err := Specific(otherID)
	if err != nil {
		t.Fatalf("Specific(other) error = %v, want nil", err)
	}
	otherRequest := request
	otherRequest.Query.Selection = otherSelection
	otherCommitment, err := CommitQuery(otherRequest)
	if err != nil {
		t.Fatalf("CommitQuery(other specific) error = %v, want nil", err)
	}
	wrongPayload := payload
	wrongPayload.Request = otherCommitment
	wrongDocument, err := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: wrongPayload})
	if err != nil {
		t.Fatalf("IssueCatalog(wrong specific entry) error = %v, want nil", err)
	}
	if got, gotErr := VerifyCatalog(CatalogVerification{
		Document: wrongDocument, Request: otherRequest, TrustedKeys: fixture.trusted,
	}); !errors.Is(gotErr, core.ErrChitConflict) || got != (VerifiedCatalog{}) {
		t.Fatalf("VerifyCatalog(wrong specific entry) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrChitConflict)
	}

	continuedPayload := payload
	continuedCursor, err := CursorFor(continuedPayload.Entries[0].Chit.Payload.Identity)
	if err != nil {
		t.Fatalf("CursorFor(specific entry) error = %v, want nil", err)
	}
	continuedPayload.Continuation, err = More(continuedCursor)
	if err != nil {
		t.Fatalf("More(specific) error = %v, want nil", err)
	}
	continuedDocument, err := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: continuedPayload})
	if err != nil {
		t.Fatalf("IssueCatalog(specific continuation) error = %v, want nil", err)
	}
	if got, gotErr := VerifyCatalog(CatalogVerification{
		Document: continuedDocument, Request: request, TrustedKeys: fixture.trusted,
	}); !errors.Is(gotErr, core.ErrChitConflict) || got != (VerifiedCatalog{}) {
		t.Fatalf("VerifyCatalog(specific continuation) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrChitConflict)
	}

	secondPayload := payload.Entries[0].Chit.Payload
	secondPayload.Identity = otherID
	secondChit, err := Issue(Issuance{
		Signer: fixture.private, TrustedKeys: fixture.trusted, Payload: secondPayload,
	})
	if err != nil {
		t.Fatalf("Issue(second chit) error = %v, want nil", err)
	}
	multiplePayload := payload
	multiplePayload.Entries = []CatalogEntry{
		{Chit: payload.Entries[0].Chit, State: CustodyStateStored},
		{Chit: secondChit, State: CustodyStateStored},
	}
	if multiplePayload.Entries[0].Chit.Payload.Identity.String() < multiplePayload.Entries[1].Chit.Payload.Identity.String() {
		multiplePayload.Entries[0], multiplePayload.Entries[1] = multiplePayload.Entries[1], multiplePayload.Entries[0]
	}
	multipleDocument, err := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: multiplePayload})
	if err != nil {
		t.Fatalf("IssueCatalog(multiple specific entries) error = %v, want nil", err)
	}
	if got, gotErr := VerifyCatalog(CatalogVerification{
		Document: multipleDocument, Request: request, TrustedKeys: fixture.trusted,
	}); !errors.Is(gotErr, core.ErrChitConflict) || got != (VerifiedCatalog{}) {
		t.Fatalf("VerifyCatalog(multiple specific entries) = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrChitConflict)
	}
}

func TestCatalogJSONPressuresValidRejectedAndExactExtentBoundaries(t *testing.T) {
	t.Parallel()

	fixture := newCatalogFixture(t, 0x51, 1)
	canonical, err := fixture.document.MarshalJSON()
	if err != nil {
		t.Fatalf("CatalogDocument.MarshalJSON() error = %v, want nil", err)
	}
	for index := range 10 {
		candidate := newCatalogFixture(t, byte(0x52+index), uint64(index+1))
		encoded, err := candidate.document.MarshalJSON()
		if err != nil {
			t.Fatalf("CatalogDocument.MarshalJSON(valid %d) error = %v, want nil", index, err)
		}
		var got CatalogDocument
		if err := got.UnmarshalJSON(encoded); err != nil || !catalogDocumentsEqual(got, candidate.document) {
			t.Fatalf("CatalogDocument.UnmarshalJSON(valid %d) = (%v, %v), want exact document and nil", index, got, err)
		}
	}

	below := padCatalogJSON(canonical, core.JSONDocumentMaximumBytes-1)
	at := padCatalogJSON(canonical, core.JSONDocumentMaximumBytes)
	above := padCatalogJSON(canonical, core.JSONDocumentMaximumBytes+1)
	for _, accepted := range [][]byte{below, at} {
		var got CatalogDocument
		if err := got.UnmarshalJSON(accepted); err != nil || !catalogDocumentsEqual(got, fixture.document) {
			t.Fatalf("CatalogDocument.UnmarshalJSON(%d-byte boundary) = (%v, %v), want exact document and nil",
				len(accepted), got, err)
		}
	}

	rejected := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: []byte{}},
		{name: "whitespace", data: []byte{' '}},
		{name: "null", data: []byte("null")},
		{name: "empty object", data: []byte("{}")},
		{name: "array", data: []byte("[]")},
		{name: "truncated", data: canonical[:len(canonical)-1]},
		{name: "trailing zero", data: append(bytes.Clone(canonical), 0)},
		{name: "trailing document", data: append(bytes.Clone(canonical), canonical...)},
		{name: "leading invalid token", data: append([]byte{'x'}, canonical...)},
		{name: "one above maximum", data: above},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := cloneCatalogDocument(fixture.document)
			before := cloneCatalogDocument(receiver)
			gotErr := receiver.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || !catalogDocumentsEqual(receiver, before) {
				t.Fatalf("CatalogDocument.UnmarshalJSON(%s) = (%v, %v), want preserved and errors.Is %v",
					tc.name, receiver, gotErr, core.ErrJSONContract)
			}
		})
	}
	var nilReceiver *CatalogDocument
	if err := nilReceiver.UnmarshalJSON(canonical); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil CatalogDocument.UnmarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
}

func newCatalogFixture(t testing.TB, marker byte, version uint64) catalogFixture {
	t.Helper()

	chit := newChitFixture(t, marker, version)
	state := CustodyStateStored
	if marker%3 == 1 {
		state = CustodyStateRetrievalUnavailable
	}
	if marker%3 == 2 {
		state = CustodyStateDeleted
	}
	continuation := End()
	if marker%2 == 1 {
		var err error
		cursor, cursorErr := CursorFor(chit.identity)
		if cursorErr != nil {
			t.Fatalf("CursorFor(catalog entry) error = %v, want nil", cursorErr)
		}
		continuation, err = More(cursor)
		if err != nil {
			t.Fatalf("More() error = %v, want nil", err)
		}
	}
	request := catalogQueryPayload(t, chit.scope, chit.document.Payload.Partition, marker)
	commitment, err := CommitQuery(request)
	if err != nil {
		t.Fatalf("CommitQuery() error = %v, want nil", err)
	}
	payload := CatalogPayload{
		Entries:    []CatalogEntry{{Chit: chit.document, State: state}},
		Watermark:  catalogWatermarkFixture(t, chit.scope, marker+2),
		ObservedAt: chit.document.Payload.RetainUntil,
		Scope:      chit.scope, Request: commitment, Continuation: continuation,
	}
	document, err := IssueCatalog(CatalogIssuance{Signer: chit.private, Payload: payload})
	if err != nil {
		t.Fatalf("IssueCatalog() error = %v, want nil", err)
	}
	return catalogFixture{
		private: chit.private, trusted: chit.trusted, document: document, payload: payload, request: request,
	}
}

func catalogQueryPayload(t testing.TB, scope receipt.Scope, partition Partition, marker byte) QueryPayload {
	t.Helper()
	query, err := NewQuery(QueryRequest{
		Scope: scope, Partition: partition, Selection: All(), Position: Start(), PageSize: core.CatalogPageMaximumEntries,
	})
	if err != nil {
		t.Fatalf("NewQuery() error = %v, want nil", err)
	}
	payload := QueryPayload{
		Query: query, Build: signedQueryBuild(t, chitOffering(t, 2)),
		Nonce: signedQueryNonce(t, marker), Revision: controlwire.Revision2026V1,
	}
	if err := payload.Validate(); err != nil {
		t.Fatalf("QueryPayload.Validate() error = %v, want nil", err)
	}
	return payload
}

func catalogCursorFixture(t testing.TB, marker byte) Cursor {
	t.Helper()
	cursor, err := CursorFor(mustChitID(t, marker, int64(marker)+1))
	if err != nil {
		t.Fatalf("CursorFor() error = %v, want nil", err)
	}
	return cursor
}

func catalogPageLimitFixture(t testing.TB, value uint16) core.CatalogPageLimit {
	t.Helper()

	limit, err := core.NewCatalogPageLimit(value)
	if err != nil {
		t.Fatalf("core.NewCatalogPageLimit(%d) error = %v, want nil", value, err)
	}
	return limit
}

func catalogHistoryEntries(t testing.TB, fixture catalogFixture, count int) []CatalogEntry {
	t.Helper()

	entries := make([]CatalogEntry, 0, count)
	for index := count - 1; index >= 0; index-- {
		payload := fixture.payload.Entries[0].Chit.Payload
		payload.Identity = mustChitID(t, byte(index%251)+1, int64(1_000+index))
		payload.Version = mustVersion(t, uint64(index+1))
		document, err := Issue(Issuance{
			Signer: fixture.private, TrustedKeys: fixture.trusted, Payload: payload,
		})
		if err != nil {
			t.Fatalf("Issue(history entry %d) error = %v, want nil", index, err)
		}
		entries = append(entries, CatalogEntry{Chit: document, State: CustodyStateStored})
	}
	return entries
}

func catalogWatermarkFixture(t testing.TB, scope receipt.Scope, marker byte) receipt.Watermark {
	t.Helper()

	generation, err := receipt.NewGeneration(uint64(marker) + 1)
	if err != nil {
		t.Fatalf("receipt.NewGeneration() error = %v, want nil", err)
	}
	cursor, err := receipt.NewCursorDigest(core.SHA256Of([]byte{marker, 1}))
	if err != nil {
		t.Fatalf("receipt.NewCursorDigest() error = %v, want nil", err)
	}
	chain, err := receipt.NewChainHash(core.SHA256Of([]byte{marker, 2}))
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

func cloneCatalogDocument(document CatalogDocument) CatalogDocument {
	clone := document
	clone.Payload.Entries = append([]CatalogEntry(nil), document.Payload.Entries...)
	return clone
}

func catalogDocumentsEqual(left, right CatalogDocument) bool {
	return left.Attestation == right.Attestation && catalogPayloadsEqual(left.Payload, right.Payload)
}

func catalogPayloadsEqual(left, right CatalogPayload) bool {
	if left.Watermark != right.Watermark || left.ObservedAt != right.ObservedAt ||
		left.Scope != right.Scope || left.Request != right.Request || left.Continuation != right.Continuation ||
		(left.Entries == nil) != (right.Entries == nil) || len(left.Entries) != len(right.Entries) {
		return false
	}
	for index := range left.Entries {
		if left.Entries[index] != right.Entries[index] {
			return false
		}
	}
	return true
}

func verifiedCatalogPayloadsEqual(verified VerifiedCatalog, want CatalogPayload) bool {
	got, err := verified.Payload()
	return err == nil && catalogPayloadsEqual(got, want)
}

func padCatalogJSON(canonical []byte, target int) []byte {
	padding := bytes.Repeat([]byte{' '}, target-len(canonical))
	return append(padding, canonical...)
}
