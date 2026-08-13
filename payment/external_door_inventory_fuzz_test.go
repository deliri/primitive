package payment

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type paymentJSONDoor uint8

const (
	paymentJSONDoorUnknown paymentJSONDoor = iota
	paymentJSONDoorPaymentID
	paymentJSONDoorSigningDomain
	paymentJSONDoorPayload
	paymentJSONDoorDocument
	paymentJSONDoorQueryPayload
	paymentJSONDoorQueryDocument
	paymentJSONDoorQueryCommitment
	paymentJSONDoorCursor
	paymentJSONDoorCatalogPayload
	paymentJSONDoorCatalogDocument
	paymentJSONDoorLimit
)

func (d paymentJSONDoor) receiverName() string {
	switch d {
	case paymentJSONDoorPaymentID:
		return "PaymentID"
	case paymentJSONDoorSigningDomain:
		return "SigningDomain"
	case paymentJSONDoorPayload:
		return "Payload"
	case paymentJSONDoorDocument:
		return "Document"
	case paymentJSONDoorQueryPayload:
		return "QueryPayload"
	case paymentJSONDoorQueryDocument:
		return "QueryDocument"
	case paymentJSONDoorQueryCommitment:
		return "QueryCommitment"
	case paymentJSONDoorCursor:
		return "Cursor"
	case paymentJSONDoorCatalogPayload:
		return "CatalogPayload"
	case paymentJSONDoorCatalogDocument:
		return "CatalogDocument"
	case paymentJSONDoorUnknown, paymentJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type paymentFuzzFixtures struct {
	payment         paymentFixture
	catalog         paymentCatalogFixture
	query           signedQueryFixture
	paymentID       PaymentID
	signingDomain   SigningDomain
	payload         Payload
	document        Document
	queryPayload    QueryPayload
	queryDocument   QueryDocument
	queryCommitment QueryCommitment
	cursor          Cursor
	catalogPayload  CatalogPayload
	catalogDocument CatalogDocument
}

type paymentJSONSeed struct {
	door     paymentJSONDoor
	document []byte
}

func FuzzPaymentExternalJSONDoorInventory(f *testing.F) {
	fixtures := paymentFixturesForFuzz(f)
	for _, seed := range paymentJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door), seed.document)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`),
		[]byte(`0`), []byte(`true`), []byte(`{`),
		bytes.Repeat([]byte(`[`), core.JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(paymentJSONDoorCatalogDocument), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		switch paymentJSONDoor(rawDoor) {
		case paymentJSONDoorPaymentID:
			fuzzPaymentJSONValue(t, data, fixtures.paymentID)
		case paymentJSONDoorSigningDomain:
			fuzzPaymentJSONValue(t, data, fixtures.signingDomain)
		case paymentJSONDoorPayload:
			fuzzPaymentJSONValue(t, data, fixtures.payload)
		case paymentJSONDoorDocument:
			fuzzPaymentDocument(t, data, fixtures)
		case paymentJSONDoorQueryPayload:
			fuzzPaymentJSONValue(t, data, fixtures.queryPayload)
		case paymentJSONDoorQueryDocument:
			fuzzPaymentQueryDocument(t, data, fixtures)
		case paymentJSONDoorQueryCommitment:
			fuzzPaymentJSONValue(t, data, fixtures.queryCommitment)
		case paymentJSONDoorCursor:
			fuzzPaymentJSONValue(t, data, fixtures.cursor)
		case paymentJSONDoorCatalogPayload:
			fuzzPaymentJSONValue(t, data, fixtures.catalogPayload)
		case paymentJSONDoorCatalogDocument:
			fuzzPaymentCatalogDocument(t, data, fixtures)
		case paymentJSONDoorUnknown, paymentJSONDoorLimit:
			return
		default:
			return
		}
	})
}

type paymentTextDoor uint8

const (
	paymentTextDoorUnknown paymentTextDoor = iota
	paymentTextDoorPaymentID
	paymentTextDoorSigningDomain
	paymentTextDoorLimit
)

func FuzzPaymentExternalTextDoorInventory(f *testing.F) {
	fixtures := paymentFixturesForFuzz(f)
	f.Add(uint8(paymentTextDoorPaymentID), fixtures.paymentID.String())
	f.Add(uint8(paymentTextDoorSigningDomain), fixtures.signingDomain.String())
	for _, hostile := range []string{"", " ", "unknown", "A", "\x00", "\xff"} {
		f.Add(uint8(paymentTextDoorPaymentID), hostile)
		f.Add(uint8(paymentTextDoorSigningDomain), hostile)
	}
	f.Fuzz(func(t *testing.T, rawDoor uint8, value string) {
		var outcome paymentTextOutcome
		switch paymentTextDoor(rawDoor) {
		case paymentTextDoorPaymentID:
			got, err := ParsePaymentID(value)
			outcome = paymentTextOutcome{input: value, projection: got.String(), err: err, validate: got.Validate}
		case paymentTextDoorSigningDomain:
			got, err := SigningDomainUnknown.ParseCanonicalText([]byte(value))
			outcome = paymentTextOutcome{input: value, projection: got.String(), err: err, validate: got.Validate}
		case paymentTextDoorUnknown, paymentTextDoorLimit:
			return
		default:
			return
		}
		fuzzPaymentTextOutcome(t, outcome)
	})
}

type paymentJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

func fuzzPaymentJSONValue[T paymentJSONValue](t *testing.T, data []byte, seed T) {
	t.Helper()
	before, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("payment seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := seed
	decoder, ok := any(&candidate).(json.Unmarshaler)
	if !ok {
		t.Fatalf("payment JSON receiver %T lacks json.Unmarshaler", &candidate)
	}
	decodeErr := decoder.UnmarshalJSON(data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, core.ErrPaymentContract) ||
			!errors.Is(decodeErr, core.ErrJSONContract) {
			t.Fatalf("payment JSON door error = %v, want typed JSON/payment refusal", decodeErr)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected payment JSON door changed its receiver: marshal error %v", marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted payment JSON validation error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || len(canonical) > core.JSONDocumentMaximumBytes {
		t.Fatalf("payment canonical JSON = (%d bytes, %v), want bounded and nil", len(canonical), err)
	}
	var roundTrip T
	roundTripDecoder, ok := any(&roundTrip).(json.Unmarshaler)
	if !ok {
		t.Fatalf("payment round-trip receiver %T lacks json.Unmarshaler", &roundTrip)
	}
	if err := roundTripDecoder.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("payment canonical JSON decode error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("payment JSON door lacks a canonical fixed point: marshal error %v", err)
	}
}

func fuzzPaymentDocument(t *testing.T, data []byte, fixtures paymentFuzzFixtures) {
	t.Helper()
	fuzzPaymentJSONValue(t, data, fixtures.document)
	candidate := fixtures.document
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	proof, err := Verify(Verification{
		Document:    candidate,
		Expected:    Expectation{Identity: fixtures.payment.identity, Scope: fixtures.payment.scope},
		TrustedKeys: fixtures.payment.trusted,
	})
	if err != nil {
		if !errors.Is(err, core.ErrPaymentVerification) || proof != (Verified{}) {
			t.Fatalf("payment.Verify(fuzz document) = (%v, %v), want typed refusal and zero proof", proof, err)
		}
		return
	}
	got, documentErr := proof.Document()
	if proof.Validate() != nil || documentErr != nil || got != fixtures.document {
		t.Fatalf("payment.Verify(fuzz document) authenticated facts outside the signed seed")
	}
}

func fuzzPaymentQueryDocument(t *testing.T, data []byte, fixtures paymentFuzzFixtures) {
	t.Helper()
	fuzzPaymentJSONValue(t, data, fixtures.queryDocument)
	candidate := fixtures.queryDocument
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	proof, err := VerifyQuery(QueryVerification{Document: candidate, TrustedKeys: fixtures.query.trusted})
	if err != nil {
		if !errors.Is(err, core.ErrPaymentVerification) || proof != (VerifiedQuery{}) {
			t.Fatalf("VerifyQuery(fuzz document) = (%v, %v), want typed refusal and zero proof", proof, err)
		}
		return
	}
	if proof.Validate() != nil || candidate != fixtures.queryDocument {
		t.Fatalf("VerifyQuery(fuzz document) authenticated facts outside the signed seed")
	}
}

func fuzzPaymentCatalogDocument(t *testing.T, data []byte, fixtures paymentFuzzFixtures) {
	t.Helper()
	fuzzPaymentJSONValue(t, data, fixtures.catalogDocument)
	candidate := fixtures.catalogDocument
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	proof, err := VerifyCatalog(CatalogVerification{
		Document: candidate, Request: fixtures.catalog.request, TrustedKeys: fixtures.catalog.trusted,
	})
	if err != nil {
		if !errors.Is(err, core.ErrPaymentVerification) || !isZeroPaymentCatalog(proof) {
			t.Fatalf("VerifyCatalog(fuzz document) = (%v, %v), want typed refusal and zero proof", proof, err)
		}
		return
	}
	if !samePaymentCatalog(proof, fixtures.catalog.payload) {
		t.Fatalf("VerifyCatalog(fuzz document) authenticated facts outside the signed seed")
	}
}

func paymentFixturesForFuzz(t testing.TB) paymentFuzzFixtures {
	t.Helper()
	payment := newPaymentFixture(t, paymentFixtureRequest{Marker: 0xa1, Millisecond: 11, MinorUnits: 101})
	catalog := newPaymentCatalogFixture(t, paymentCatalogFixtureRequest{Marker: 0xb1, Entries: 2})
	query := newSignedQueryFixture(t, signedQueryFixtureRequest{marker: 0xc1, pageSize: 2})
	cursor, err := NewCursor(core.SHA256Of([]byte("payment fuzz cursor")))
	if err != nil {
		t.Fatalf("NewCursor() error = %v, want nil", err)
	}
	queryCommitment, err := CommitQuery(query.payload)
	if err != nil {
		t.Fatalf("CommitQuery() error = %v, want nil", err)
	}
	return paymentFuzzFixtures{
		payment: payment, catalog: catalog, query: query,
		paymentID: payment.identity, signingDomain: SigningDomainReceiptV1,
		payload: payment.document.Payload, document: payment.document,
		queryPayload: query.payload, queryDocument: query.document,
		queryCommitment: queryCommitment, cursor: cursor,
		catalogPayload: catalog.payload, catalogDocument: catalog.document,
	}
}

func paymentJSONSeedsForFuzz(t testing.TB, fixtures paymentFuzzFixtures) []paymentJSONSeed {
	t.Helper()
	return []paymentJSONSeed{
		paymentJSONSeedForFuzz(t, paymentJSONDoorPaymentID, fixtures.paymentID),
		paymentJSONSeedForFuzz(t, paymentJSONDoorSigningDomain, fixtures.signingDomain),
		paymentJSONSeedForFuzz(t, paymentJSONDoorPayload, fixtures.payload),
		paymentJSONSeedForFuzz(t, paymentJSONDoorDocument, fixtures.document),
		paymentJSONSeedForFuzz(t, paymentJSONDoorQueryPayload, fixtures.queryPayload),
		paymentJSONSeedForFuzz(t, paymentJSONDoorQueryDocument, fixtures.queryDocument),
		paymentJSONSeedForFuzz(t, paymentJSONDoorQueryCommitment, fixtures.queryCommitment),
		paymentJSONSeedForFuzz(t, paymentJSONDoorCursor, fixtures.cursor),
		paymentJSONSeedForFuzz(t, paymentJSONDoorCatalogPayload, fixtures.catalogPayload),
		paymentJSONSeedForFuzz(t, paymentJSONDoorCatalogDocument, fixtures.catalogDocument),
	}
}

func paymentJSONSeedForFuzz(t testing.TB, door paymentJSONDoor, value paymentJSONValue) paymentJSONSeed {
	t.Helper()
	document, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("payment fuzz seed MarshalJSON(%d) error = %v, want nil", door, err)
	}
	return paymentJSONSeed{door: door, document: document}
}

type paymentTextOutcome struct {
	input      string
	projection string
	err        error
	validate   func() error
}

func fuzzPaymentTextOutcome(t *testing.T, outcome paymentTextOutcome) {
	t.Helper()
	if outcome.err != nil {
		if !errors.Is(outcome.err, core.ErrPaymentContract) || outcome.projection != "" {
			t.Fatalf("payment text refusal = (%q, %v), want empty typed refusal", outcome.projection, outcome.err)
		}
		return
	}
	if outcome.validate() != nil || outcome.projection != outcome.input {
		t.Fatalf("payment text acceptance = (%q, %v), want exact %q and nil",
			outcome.projection, outcome.validate(), outcome.input)
	}
}

func TestPaymentExternalIngressFuzzInventoryMatchesProduction(t *testing.T) {
	t.Parallel()
	gotJSON, err := paymentExportedJSONReceiverNames()
	if err != nil {
		t.Fatalf("paymentExportedJSONReceiverNames() error = %v, want nil", err)
	}
	var wantJSON []string
	for door := paymentJSONDoorUnknown + 1; door < paymentJSONDoorLimit; door++ {
		wantJSON = append(wantJSON, door.receiverName())
	}
	slices.Sort(wantJSON)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", gotJSON, wantJSON)
	}
}

func paymentExportedJSONReceiverNames() ([]string, error) {
	files, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	fileSet := token.NewFileSet()
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(fileSet, file.Name(), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != "UnmarshalJSON" || function.Recv == nil || len(function.Recv.List) != 1 {
				continue
			}
			pointer, ok := function.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			receiver, ok := pointer.X.(*ast.Ident)
			if ok && receiver.IsExported() {
				names = append(names, receiver.Name)
			}
		}
	}
	slices.Sort(names)
	return names, nil
}
