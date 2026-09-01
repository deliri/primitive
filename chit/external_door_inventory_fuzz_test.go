package chit

import (
	"bytes"
	json "encoding/json/v2"
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

type chitJSONDoor uint8

const (
	chitJSONDoorUnknown chitJSONDoor = iota
	chitJSONDoorEntryName
	chitJSONDoorChitID
	chitJSONDoorCollectionID
	chitJSONDoorPartition
	chitJSONDoorVersion
	chitJSONDoorPayload
	chitJSONDoorDocument
	chitJSONDoorQueryPayload
	chitJSONDoorQueryCommitment
	chitJSONDoorQueryDocument
	chitJSONDoorCustodyState
	chitJSONDoorCursor
	chitJSONDoorCatalogPayload
	chitJSONDoorCatalogDocument
	chitJSONDoorSigningDomain
	chitJSONDoorObjectCount
	chitJSONDoorEntrySequence
	chitJSONDoorManifestDigest
	chitJSONDoorLimit
)

func (d chitJSONDoor) receiverName() string {
	switch d {
	case chitJSONDoorEntryName:
		return "EntryName"
	case chitJSONDoorChitID:
		return "ChitID"
	case chitJSONDoorCollectionID:
		return "CollectionID"
	case chitJSONDoorPartition:
		return "Partition"
	case chitJSONDoorVersion:
		return "Version"
	case chitJSONDoorPayload:
		return "Payload"
	case chitJSONDoorDocument:
		return "Document"
	case chitJSONDoorQueryPayload:
		return "QueryPayload"
	case chitJSONDoorQueryCommitment:
		return "QueryCommitment"
	case chitJSONDoorQueryDocument:
		return "QueryDocument"
	case chitJSONDoorCustodyState:
		return "CustodyState"
	case chitJSONDoorCursor:
		return "Cursor"
	case chitJSONDoorCatalogPayload:
		return "CatalogPayload"
	case chitJSONDoorCatalogDocument:
		return "CatalogDocument"
	case chitJSONDoorSigningDomain:
		return "SigningDomain"
	case chitJSONDoorObjectCount:
		return "ObjectCount"
	case chitJSONDoorEntrySequence:
		return "EntrySequence"
	case chitJSONDoorManifestDigest:
		return "ManifestDigest"
	case chitJSONDoorUnknown, chitJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type chitFuzzFixtures struct {
	entryName       EntryName
	queryPayload    QueryPayload
	payload         Payload
	catalogPayload  CatalogPayload
	queryDocument   QueryDocument
	document        Document
	catalogDocument CatalogDocument
	query           signedQueryFixture
	catalog         catalogFixture
	chit            chitFixture
	objectCount     ObjectCount
	entrySequence   EntrySequence
	version         Version
	queryCommitment QueryCommitment
	cursor          Cursor
	manifestDigest  ManifestDigest
	collectionID    CollectionID
	partition       Partition
	chitID          ChitID
	custodyState    CustodyState
	signingDomain   SigningDomain
}

type chitJSONSeed struct {
	document []byte
	door     chitJSONDoor
}

func FuzzChitExternalJSONDoorInventory(f *testing.F) {
	fixtures := chitFixturesForFuzz(f)
	for _, seed := range chitJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door), seed.document)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`),
		[]byte(`0`), []byte(`true`), []byte(`{`),
		bytes.Repeat([]byte(`[`), core.JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(chitJSONDoorDocument), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		switch chitJSONDoor(rawDoor) {
		case chitJSONDoorEntryName:
			fuzzChitJSONValue(t, data, fixtures.entryName)
		case chitJSONDoorChitID:
			fuzzChitJSONValue(t, data, fixtures.chitID)
		case chitJSONDoorCollectionID:
			fuzzChitJSONValue(t, data, fixtures.collectionID)
		case chitJSONDoorPartition:
			fuzzChitJSONValue(t, data, fixtures.partition)
		case chitJSONDoorVersion:
			fuzzChitJSONValue(t, data, fixtures.version)
		case chitJSONDoorPayload:
			fuzzChitJSONValue(t, data, fixtures.payload)
		case chitJSONDoorDocument:
			fuzzChitDocument(t, data, fixtures)
		case chitJSONDoorQueryPayload:
			fuzzChitJSONValue(t, data, fixtures.queryPayload)
		case chitJSONDoorQueryCommitment:
			fuzzChitJSONValue(t, data, fixtures.queryCommitment)
		case chitJSONDoorQueryDocument:
			fuzzChitQueryDocument(t, data, fixtures)
		case chitJSONDoorCustodyState:
			fuzzChitJSONValue(t, data, fixtures.custodyState)
		case chitJSONDoorCursor:
			fuzzChitJSONValue(t, data, fixtures.cursor)
		case chitJSONDoorCatalogPayload:
			fuzzChitJSONValue(t, data, fixtures.catalogPayload)
		case chitJSONDoorCatalogDocument:
			fuzzChitCatalogDocument(t, data, fixtures)
		case chitJSONDoorSigningDomain:
			fuzzChitJSONValue(t, data, fixtures.signingDomain)
		case chitJSONDoorObjectCount:
			fuzzChitJSONValue(t, data, fixtures.objectCount)
		case chitJSONDoorEntrySequence:
			fuzzChitJSONValue(t, data, fixtures.entrySequence)
		case chitJSONDoorManifestDigest:
			fuzzChitJSONValue(t, data, fixtures.manifestDigest)
		case chitJSONDoorUnknown, chitJSONDoorLimit:
			return
		default:
			return
		}
	})
}

type chitTextDoor uint8

const (
	chitTextDoorUnknown chitTextDoor = iota
	chitTextDoorEntryName
	chitTextDoorChitID
	chitTextDoorCollectionID
	chitTextDoorSigningDomain
	chitTextDoorLimit
)

func FuzzChitExternalTextDoorInventory(f *testing.F) {
	fixtures := chitFixturesForFuzz(f)
	f.Add(uint8(chitTextDoorEntryName), fixtures.entryName.String())
	f.Add(uint8(chitTextDoorChitID), fixtures.chitID.String())
	f.Add(uint8(chitTextDoorCollectionID), fixtures.collectionID.String())
	f.Add(uint8(chitTextDoorSigningDomain), fixtures.signingDomain.String())
	for _, hostile := range []string{"", " ", ".", "..", "unknown", "\x00", "\xff"} {
		f.Add(uint8(chitTextDoorEntryName), hostile)
		f.Add(uint8(chitTextDoorChitID), hostile)
	}
	f.Fuzz(func(t *testing.T, rawDoor uint8, value string) {
		var outcome chitTextOutcome
		switch chitTextDoor(rawDoor) {
		case chitTextDoorEntryName:
			got, err := ParseEntryName(value)
			outcome = chitTextOutcome{input: value, projection: got.String(), err: err, validate: got.Validate}
		case chitTextDoorChitID:
			got, err := ParseChitID(value)
			outcome = chitTextOutcome{input: value, projection: got.String(), err: err, validate: got.Validate}
		case chitTextDoorCollectionID:
			got, err := ParseCollectionID(value)
			outcome = chitTextOutcome{input: value, projection: got.String(), err: err, validate: got.Validate}
		case chitTextDoorSigningDomain:
			got, err := SigningDomainUnknown.ParseCanonicalText([]byte(value))
			outcome = chitTextOutcome{input: value, projection: got.String(), err: err, validate: got.Validate}
		case chitTextDoorUnknown, chitTextDoorLimit:
			return
		default:
			return
		}
		fuzzChitTextOutcome(t, outcome)
	})
}

type chitJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

func fuzzChitJSONValue[T chitJSONValue](t *testing.T, data []byte, seed T) {
	t.Helper()
	before, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("chit seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := seed
	decoder := any(&candidate).(json.Unmarshaler)
	decodeErr := decoder.UnmarshalJSON(data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, core.ErrChitContract) || !errors.Is(decodeErr, core.ErrJSONContract) {
			t.Fatalf("chit JSON error = %v, want typed JSON/chit refusal", decodeErr)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected chit JSON changed receiver: marshal error %v", marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted chit JSON Validate() error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || len(canonical) > core.JSONDocumentMaximumBytes {
		t.Fatalf("chit canonical JSON = (%d bytes, %v), want bounded and nil", len(canonical), err)
	}
	var roundTrip T
	if err := any(&roundTrip).(json.Unmarshaler).UnmarshalJSON(canonical); err != nil {
		t.Fatalf("chit canonical JSON decode error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("chit JSON lacks canonical fixed point: %v", err)
	}
}

func fuzzChitDocument(t *testing.T, data []byte, fixtures chitFuzzFixtures) {
	t.Helper()
	fuzzChitJSONValue(t, data, fixtures.document)
	candidate := fixtures.document
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	proof, err := Verify(Verification{
		Document:    candidate,
		Expected:    Expectation{Identity: fixtures.chit.identity, Scope: fixtures.chit.scope},
		TrustedKeys: fixtures.chit.trusted,
	})
	if err != nil {
		if !errors.Is(err, core.ErrChitVerification) || proof != (Verified{}) {
			t.Fatalf("chit.Verify(fuzz document) = (%v, %v), want typed refusal and zero proof", proof, err)
		}
		return
	}
	document, documentErr := proof.Document()
	if proof.Validate() != nil || documentErr != nil || document != fixtures.document {
		t.Fatalf("chit.Verify(fuzz document) authenticated facts outside the signed seed")
	}
}

func fuzzChitQueryDocument(t *testing.T, data []byte, fixtures chitFuzzFixtures) {
	t.Helper()
	fuzzChitJSONValue(t, data, fixtures.queryDocument)
	candidate := fixtures.queryDocument
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	proof, err := VerifyQuery(QueryVerification{Document: candidate, TrustedKeys: fixtures.query.trusted})
	if err != nil {
		if !errors.Is(err, core.ErrChitVerification) || proof != (VerifiedQuery{}) {
			t.Fatalf("VerifyQuery(fuzz document) = (%v, %v), want typed refusal and zero proof", proof, err)
		}
		return
	}
	if proof.Validate() != nil || candidate != fixtures.queryDocument {
		t.Fatalf("VerifyQuery(fuzz document) authenticated facts outside the signed seed")
	}
}

func fuzzChitCatalogDocument(t *testing.T, data []byte, fixtures chitFuzzFixtures) {
	t.Helper()
	fuzzChitJSONValue(t, data, fixtures.catalogDocument)
	candidate := fixtures.catalogDocument
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	proof, err := VerifyCatalog(CatalogVerification{
		Document: candidate, Request: fixtures.catalog.request, TrustedKeys: fixtures.catalog.trusted,
	})
	if err != nil {
		if !errors.Is(err, core.ErrChitVerification) || proof.Validate() == nil {
			t.Fatalf("VerifyCatalog(fuzz document) = (%v, %v), want typed refusal and zero proof", proof, err)
		}
		return
	}
	payload, payloadErr := proof.Payload()
	got, gotErr := payload.MarshalJSON()
	want, wantErr := fixtures.catalog.payload.MarshalJSON()
	if payloadErr != nil || gotErr != nil || wantErr != nil || !bytes.Equal(got, want) {
		t.Fatalf("VerifyCatalog(fuzz document) authenticated facts outside the signed seed")
	}
}

type chitTextOutcome struct {
	err        error
	validate   func() error
	input      string
	projection string
}

func fuzzChitTextOutcome(t *testing.T, outcome chitTextOutcome) {
	t.Helper()
	if outcome.err != nil {
		if !errors.Is(outcome.err, core.ErrChitContract) || outcome.projection != "" {
			t.Fatalf("chit text refusal = (%q, %v), want empty typed refusal", outcome.projection, outcome.err)
		}
		return
	}
	if outcome.validate() != nil || outcome.projection != outcome.input {
		t.Fatalf("chit text acceptance = (%q, %v), want exact %q", outcome.projection, outcome.validate(), outcome.input)
	}
}

func chitFixturesForFuzz(t testing.TB) chitFuzzFixtures {
	t.Helper()
	chit := newChitFixture(t, 0xd1, 1)
	catalog := newCatalogFixture(t, 0xd2, 2)
	query := newSignedQueryFixture(t, signedQueryFixtureRequest{marker: 0xd3, pageSize: 2})
	queryCommitment, err := CommitQuery(query.payload)
	if err != nil {
		t.Fatalf("CommitQuery() error = %v, want nil", err)
	}
	return chitFuzzFixtures{
		chit: chit, catalog: catalog, query: query,
		entryName: chit.addition.Entry.Name, chitID: chit.identity,
		collectionID: chit.document.Payload.Collection, version: chit.document.Payload.Version,
		partition: mustPartition(t, 0xd5),
		payload:   chit.document.Payload, document: chit.document,
		queryPayload: query.payload, queryCommitment: queryCommitment, queryDocument: query.document,
		custodyState: CustodyStateStored, cursor: catalogCursorFixture(t, 0xd4),
		catalogPayload: catalog.payload, catalogDocument: catalog.document,
		signingDomain: SigningDomainChitV1, objectCount: chit.summary.Objects,
		entrySequence: chit.addition.Entry.Sequence, manifestDigest: chit.summary.Digest,
	}
}

func chitJSONSeedsForFuzz(t testing.TB, fixtures chitFuzzFixtures) []chitJSONSeed {
	t.Helper()
	return []chitJSONSeed{
		chitJSONSeedForFuzz(t, chitJSONDoorEntryName, fixtures.entryName),
		chitJSONSeedForFuzz(t, chitJSONDoorChitID, fixtures.chitID),
		chitJSONSeedForFuzz(t, chitJSONDoorCollectionID, fixtures.collectionID),
		chitJSONSeedForFuzz(t, chitJSONDoorPartition, fixtures.partition),
		chitJSONSeedForFuzz(t, chitJSONDoorVersion, fixtures.version),
		chitJSONSeedForFuzz(t, chitJSONDoorPayload, fixtures.payload),
		chitJSONSeedForFuzz(t, chitJSONDoorDocument, fixtures.document),
		chitJSONSeedForFuzz(t, chitJSONDoorQueryPayload, fixtures.queryPayload),
		chitJSONSeedForFuzz(t, chitJSONDoorQueryCommitment, fixtures.queryCommitment),
		chitJSONSeedForFuzz(t, chitJSONDoorQueryDocument, fixtures.queryDocument),
		chitJSONSeedForFuzz(t, chitJSONDoorCustodyState, fixtures.custodyState),
		chitJSONSeedForFuzz(t, chitJSONDoorCursor, fixtures.cursor),
		chitJSONSeedForFuzz(t, chitJSONDoorCatalogPayload, fixtures.catalogPayload),
		chitJSONSeedForFuzz(t, chitJSONDoorCatalogDocument, fixtures.catalogDocument),
		chitJSONSeedForFuzz(t, chitJSONDoorSigningDomain, fixtures.signingDomain),
		chitJSONSeedForFuzz(t, chitJSONDoorObjectCount, fixtures.objectCount),
		chitJSONSeedForFuzz(t, chitJSONDoorEntrySequence, fixtures.entrySequence),
		chitJSONSeedForFuzz(t, chitJSONDoorManifestDigest, fixtures.manifestDigest),
	}
}

func chitJSONSeedForFuzz(t testing.TB, door chitJSONDoor, value chitJSONValue) chitJSONSeed {
	t.Helper()
	document, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("chit fuzz seed MarshalJSON(%d) error = %v, want nil", door, err)
	}
	return chitJSONSeed{door: door, document: document}
}

func TestChitExternalIngressFuzzInventoryMatchesProduction(t *testing.T) {
	t.Parallel()
	gotJSON, err := chitExportedJSONReceiverNames()
	if err != nil {
		t.Fatalf("chitExportedJSONReceiverNames() error = %v, want nil", err)
	}
	var wantJSON []string
	for door := chitJSONDoorUnknown + 1; door < chitJSONDoorLimit; door++ {
		wantJSON = append(wantJSON, door.receiverName())
	}
	slices.Sort(wantJSON)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", gotJSON, wantJSON)
	}
}

func chitExportedJSONReceiverNames() ([]string, error) {
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
