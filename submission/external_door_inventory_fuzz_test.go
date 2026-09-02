package submission

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
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

type submissionJSONDoor uint8

const (
	submissionJSONDoorUnknown submissionJSONDoor = iota
	submissionJSONDoorRequestPayload
	submissionJSONDoorRequestDocument
	submissionJSONDoorRequestCommitment
	submissionJSONDoorSigningDomain
	submissionJSONDoorDecisionKind
	submissionJSONDoorDecisionDocument
	submissionJSONDoorUploadID
	submissionJSONDoorGrantPayload
	submissionJSONDoorGrantDocument
	submissionJSONDoorCompletionPayload
	submissionJSONDoorCompletionDocument
	submissionJSONDoorLimit
)

func (d submissionJSONDoor) receiverName() string {
	switch d {
	case submissionJSONDoorRequestPayload:
		return "RequestPayload"
	case submissionJSONDoorRequestDocument:
		return "RequestDocument"
	case submissionJSONDoorRequestCommitment:
		return "RequestCommitment"
	case submissionJSONDoorSigningDomain:
		return "SigningDomain"
	case submissionJSONDoorDecisionKind:
		return "DecisionKind"
	case submissionJSONDoorDecisionDocument:
		return "DecisionDocument"
	case submissionJSONDoorUploadID:
		return "UploadID"
	case submissionJSONDoorGrantPayload:
		return "GrantPayload"
	case submissionJSONDoorGrantDocument:
		return "GrantDocument"
	case submissionJSONDoorCompletionPayload:
		return "CompletionPayload"
	case submissionJSONDoorCompletionDocument:
		return "CompletionDocument"
	case submissionJSONDoorUnknown, submissionJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type submissionFuzzFixtures struct {
	decisionDocument   DecisionDocument
	decisionWire       []byte
	grantWire          []byte
	requestPayload     RequestPayload
	completionPayload  CompletionPayload
	requestDocument    RequestDocument
	grantDocument      GrantDocument
	completionDocument CompletionDocument
	reuse              reuseEvidenceFixture
	grant              grantFixture
	completion         completionFixture
	grantPayload       GrantPayload
	requestCommitment  RequestCommitment
	uploadID           UploadID
	decisionKind       DecisionKind
	signingDomain      SigningDomain
}

type submissionJSONSeed struct {
	document []byte
	door     submissionJSONDoor
}

func FuzzSubmissionExternalJSONDoorInventory(f *testing.F) {
	fixtures := submissionFixturesForFuzz(f)
	for _, seed := range submissionJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door), seed.document)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`),
		[]byte(`0`), []byte(`true`), []byte(`{`),
		bytes.Repeat([]byte(`[`), core.JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(submissionJSONDoorCompletionDocument), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		switch submissionJSONDoor(rawDoor) {
		case submissionJSONDoorRequestPayload:
			fuzzSubmissionJSONValue(t, data, fixtures.requestPayload)
		case submissionJSONDoorRequestDocument:
			fuzzSubmissionRequestDocument(t, data, fixtures)
		case submissionJSONDoorRequestCommitment:
			fuzzSubmissionJSONValue(t, data, fixtures.requestCommitment)
		case submissionJSONDoorSigningDomain:
			fuzzSubmissionJSONValue(t, data, fixtures.signingDomain)
		case submissionJSONDoorDecisionKind:
			fuzzSubmissionJSONValue(t, data, fixtures.decisionKind)
		case submissionJSONDoorDecisionDocument:
			fuzzSubmissionDecisionDocument(t, data, fixtures)
		case submissionJSONDoorUploadID:
			fuzzSubmissionJSONValue(t, data, fixtures.uploadID)
		case submissionJSONDoorGrantPayload:
			fuzzSubmissionJSONValue(t, data, fixtures.grantPayload)
		case submissionJSONDoorGrantDocument:
			fuzzSubmissionGrantDocument(t, data, fixtures)
		case submissionJSONDoorCompletionPayload:
			fuzzSubmissionJSONValue(t, data, fixtures.completionPayload)
		case submissionJSONDoorCompletionDocument:
			fuzzSubmissionCompletionDocument(t, data, fixtures)
		case submissionJSONDoorUnknown, submissionJSONDoorLimit:
			return
		default:
			return
		}
	})
}

type submissionTextDoor uint8

const (
	submissionTextDoorUnknown submissionTextDoor = iota
	submissionTextDoorUploadID
	submissionTextDoorSigningDomain
	submissionTextDoorCanonicalSigningDomain
	submissionTextDoorLimit
)

func FuzzSubmissionExternalTextDoorInventory(f *testing.F) {
	fixtures := submissionFixturesForFuzz(f)
	f.Add(uint8(submissionTextDoorUploadID), fixtures.uploadID.String())
	f.Add(uint8(submissionTextDoorSigningDomain), fixtures.signingDomain.String())
	f.Add(uint8(submissionTextDoorCanonicalSigningDomain), fixtures.signingDomain.String())
	for _, hostile := range []string{"", " ", "unknown", "A", "\x00", "\xff"} {
		f.Add(uint8(submissionTextDoorSigningDomain), hostile)
		f.Add(uint8(submissionTextDoorUploadID), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, value string) {
		switch submissionTextDoor(rawDoor) {
		case submissionTextDoorUploadID:
			got, err := ParseUploadID(value)
			fuzzSubmissionTextOutcome(t, submissionTextOutcome{
				input: value, projection: got.String(), err: err, validate: got.Validate,
			})
		case submissionTextDoorSigningDomain:
			got, err := ParseSigningDomain(value)
			fuzzSubmissionTextOutcome(t, submissionTextOutcome{
				input: value, projection: got.String(), err: err, validate: got.Validate,
			})
		case submissionTextDoorCanonicalSigningDomain:
			got, err := SigningDomainUnknown.ParseCanonicalText([]byte(value))
			fuzzSubmissionTextOutcome(t, submissionTextOutcome{
				input: value, projection: got.String(), err: err, validate: got.Validate,
			})
		case submissionTextDoorUnknown, submissionTextDoorLimit:
			return
		default:
			return
		}
	})
}

type submissionJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

func fuzzSubmissionJSONValue[T submissionJSONValue](t *testing.T, data []byte, seed T) {
	t.Helper()
	before, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("submission seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := seed
	decoder, ok := any(&candidate).(json.Unmarshaler)
	if !ok {
		t.Fatalf("submission JSON receiver %T lacks json.Unmarshaler", &candidate)
	}
	decodeErr := decoder.UnmarshalJSON(data)
	if decodeErr != nil {
		if !submissionJSONRefusal(decodeErr) {
			t.Fatalf("submission JSON door error = %v, want typed JSON/control-plane refusal", decodeErr)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected submission JSON door changed its receiver: marshal error %v", marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted submission JSON validation error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || len(canonical) > core.JSONDocumentMaximumBytes {
		t.Fatalf("submission canonical JSON = (%d bytes, %v), want bounded and nil", len(canonical), err)
	}
	var roundTrip T
	roundTripDecoder, ok := any(&roundTrip).(json.Unmarshaler)
	if !ok {
		t.Fatalf("submission round-trip receiver %T lacks json.Unmarshaler", &roundTrip)
	}
	if err := roundTripDecoder.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("submission canonical JSON decode error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("submission JSON door lacks a canonical fixed point: marshal error %v", err)
	}
}

func fuzzSubmissionRequestDocument(t *testing.T, data []byte, fixtures submissionFuzzFixtures) {
	t.Helper()
	fuzzSubmissionJSONValue(t, data, fixtures.requestDocument)
	candidate := fixtures.requestDocument
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	proof, err := VerifyRequest(RequestVerification{
		Document: candidate, TrustedKeys: fixtures.completion.deviceKeys,
	})
	if err != nil {
		if !errors.Is(err, core.ErrControlPlaneContract) ||
			!errors.Is(err, core.ErrAttestVerification) || proof != (VerifiedRequest{}) {
			t.Fatalf("VerifyRequest(fuzz document) = (%v, %v), want typed refusal and zero proof", proof, err)
		}
		return
	}
	if proof.Validate() != nil || candidate != fixtures.requestDocument {
		t.Fatalf("VerifyRequest(fuzz document) authenticated facts outside the signed seed")
	}
}

func fuzzSubmissionGrantDocument(t *testing.T, data []byte, fixtures submissionFuzzFixtures) {
	t.Helper()
	candidate := fixtures.grantDocument
	if err := candidate.UnmarshalJSON(data); err != nil {
		if !submissionJSONRefusal(err) || !sameGrantDocument(candidate, fixtures.grantDocument) {
			t.Fatalf("GrantDocument refusal changed receiver or lost typed identity: %v", err)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted GrantDocument.Validate() error = %v, want nil", err)
	}
	proof, err := VerifyGrant(GrantExpectation{
		Document: candidate, Request: fixtures.grant.request,
		ObservedAt:  temporal.InstantFromNanoseconds(testGrantIssuedAt),
		TrustedKeys: fixtures.grant.trusted,
	})
	if err != nil {
		if !errors.Is(err, core.ErrControlPlaneContract) || !verifiedGrantIsZero(proof) {
			t.Fatalf("VerifyGrant(fuzz document) = (%v, %v), want typed refusal and zero proof", proof, err)
		}
		return
	}
	if proof.Validate() != nil || !sameGrantDocument(candidate, fixtures.grantDocument) {
		t.Fatalf("VerifyGrant(fuzz document) authenticated facts outside the signed seed")
	}
}

func verifiedGrantIsZero(proof VerifiedGrant) bool {
	payload, payloadErr := proof.Payload()
	capability, capabilityErr := proof.Capability()
	return payload == (GrantPayload{}) && payloadErr != nil && capabilityErr != nil &&
		capability.Validate() != nil
}

func fuzzSubmissionDecisionDocument(t *testing.T, data []byte, fixtures submissionFuzzFixtures) {
	t.Helper()
	candidate := fixtures.decisionDocument
	if err := candidate.UnmarshalJSON(data); err != nil {
		if !submissionJSONRefusal(err) || !sameReuseDecision(candidate, fixtures.decisionDocument) {
			t.Fatalf("DecisionDocument refusal changed receiver or lost typed identity: %v", err)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted DecisionDocument.Validate() error = %v, want nil", err)
	}
	proof, err := VerifyDecision(DecisionExpectation{
		Decision: candidate, Request: fixtures.grant.request,
		Scope:       receipt.Scope{Principal: fixtures.reuse.account, Offering: fixtures.reuse.offering},
		ObservedAt:  temporal.InstantFromNanoseconds(testGrantIssuedAt),
		TrustedKeys: fixtures.grant.trusted,
	})
	if err != nil {
		if !errors.Is(err, core.ErrControlPlaneContract) || proof != (VerifiedDecision{}) {
			t.Fatalf("VerifyDecision(fuzz document) = (%v, %v), want typed refusal and zero proof", proof, err)
		}
		return
	}
	if proof.Validate() != nil || !sameReuseDecision(candidate, fixtures.decisionDocument) {
		t.Fatalf("VerifyDecision(fuzz document) authenticated facts outside the signed seed")
	}
}

func fuzzSubmissionCompletionDocument(t *testing.T, data []byte, fixtures submissionFuzzFixtures) {
	t.Helper()
	fuzzSubmissionJSONValue(t, data, fixtures.completionDocument)
	candidate := fixtures.completionDocument
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	proof, err := VerifyCompletion(CompletionExpectation{
		Document: candidate, Request: fixtures.completion.request,
		Grant: fixtures.completion.grantDocument, GrantKeys: fixtures.completion.grantKeys,
		CompletionKeys: fixtures.completion.deviceKeys, Nonce: fixtures.completion.nonce,
	})
	if err != nil {
		if !errors.Is(err, core.ErrControlPlaneContract) || proof != (VerifiedCompletion{}) {
			t.Fatalf("VerifyCompletion(fuzz document) = (%v, %v), want typed refusal and zero proof", proof, err)
		}
		return
	}
	if proof.Validate() != nil || candidate != fixtures.completionDocument {
		t.Fatalf("VerifyCompletion(fuzz document) authenticated facts outside the signed seed")
	}
}

func submissionJSONRefusal(err error) bool {
	return errors.Is(err, core.ErrJSONContract) && errors.Is(err, core.ErrControlPlaneContract)
}

func sameGrantDocument(left, right GrantDocument) bool {
	leftCommitment, leftErr := left.Capability.Commitment()
	rightCommitment, rightErr := right.Capability.Commitment()
	return leftErr == nil && rightErr == nil && leftCommitment == rightCommitment &&
		left.Payload == right.Payload && left.Attestation == right.Attestation
}

func submissionFixturesForFuzz(t testing.TB) submissionFuzzFixtures {
	t.Helper()
	completion := newCompletionFixture(t, submissionOffering(t, 2), []byte("submission door fuzz"), 0x10)
	completionDocument := receiveIssuedCompletion(t, completion)
	requestDocument, err := IssueRequest(RequestIssuance{
		Signer: completion.deviceSigner, Payload: completion.request,
	})
	if err != nil {
		t.Fatalf("IssueRequest() error = %v, want nil", err)
	}
	commitment, err := CommitRequest(completion.request)
	if err != nil {
		t.Fatalf("CommitRequest() error = %v, want nil", err)
	}
	grant := newGrantFixture(t, grantFixtureRequest{})
	reuse := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{
		Request: grant.request, KeyByte: 0x41, ScopeByte: 0x61,
	})
	decisionProjection, err := ReuseDecision(reuseDecisionRequest(reuse))
	if err != nil {
		t.Fatalf("ReuseDecision() error = %v, want nil", err)
	}
	decisionWire, err := decisionProjection.MarshalJSON()
	if err != nil {
		t.Fatalf("DecisionProjection.MarshalJSON() error = %v, want nil", err)
	}
	var decision DecisionDocument
	if err := decision.UnmarshalJSON(decisionWire); err != nil {
		t.Fatalf("DecisionDocument.UnmarshalJSON() error = %v, want nil", err)
	}
	grantWire, err := grant.projection.MarshalJSON()
	if err != nil {
		t.Fatalf("GrantProjection.MarshalJSON() error = %v, want nil", err)
	}
	return submissionFuzzFixtures{
		requestPayload: completion.request, requestDocument: requestDocument,
		requestCommitment: commitment, signingDomain: SigningDomainRequestV1,
		decisionKind: DecisionReuse, decisionDocument: decision, decisionWire: decisionWire,
		uploadID: completion.request.Manifest.Upload, grantPayload: grant.payload,
		grantDocument: grant.document, grantWire: grantWire,
		completionPayload: completionDocument.Payload, completionDocument: completionDocument,
		completion: completion, grant: grant, reuse: reuse,
	}
}

func submissionJSONSeedsForFuzz(t testing.TB, fixtures submissionFuzzFixtures) []submissionJSONSeed {
	t.Helper()
	return []submissionJSONSeed{
		submissionJSONSeedForFuzz(t, submissionJSONDoorRequestPayload, fixtures.requestPayload),
		submissionJSONSeedForFuzz(t, submissionJSONDoorRequestDocument, fixtures.requestDocument),
		submissionJSONSeedForFuzz(t, submissionJSONDoorRequestCommitment, fixtures.requestCommitment),
		submissionJSONSeedForFuzz(t, submissionJSONDoorSigningDomain, fixtures.signingDomain),
		submissionJSONSeedForFuzz(t, submissionJSONDoorDecisionKind, fixtures.decisionKind),
		{door: submissionJSONDoorDecisionDocument, document: fixtures.decisionWire},
		submissionJSONSeedForFuzz(t, submissionJSONDoorUploadID, fixtures.uploadID),
		submissionJSONSeedForFuzz(t, submissionJSONDoorGrantPayload, fixtures.grantPayload),
		{door: submissionJSONDoorGrantDocument, document: fixtures.grantWire},
		submissionJSONSeedForFuzz(t, submissionJSONDoorCompletionPayload, fixtures.completionPayload),
		submissionJSONSeedForFuzz(t, submissionJSONDoorCompletionDocument, fixtures.completionDocument),
	}
}

func submissionJSONSeedForFuzz(
	t testing.TB,
	door submissionJSONDoor,
	value submissionJSONValue,
) submissionJSONSeed {
	t.Helper()
	document, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("submission fuzz seed MarshalJSON(%d) error = %v, want nil", door, err)
	}
	return submissionJSONSeed{door: door, document: document}
}

type submissionTextOutcome struct {
	err        error
	validate   func() error
	input      string
	projection string
}

func fuzzSubmissionTextOutcome(t *testing.T, outcome submissionTextOutcome) {
	t.Helper()
	if outcome.err != nil {
		if !errors.Is(outcome.err, core.ErrControlPlaneContract) || outcome.projection != "" {
			t.Fatalf("submission text refusal = (%q, %v), want empty typed refusal", outcome.projection, outcome.err)
		}
		return
	}
	if outcome.validate() != nil || outcome.projection != outcome.input {
		t.Fatalf("submission text acceptance = (%q, %v), want exact %q and nil",
			outcome.projection, outcome.validate(), outcome.input)
	}
}

func TestSubmissionExternalIngressFuzzInventoryMatchesProduction(t *testing.T) {
	t.Parallel()
	gotJSON, err := submissionExportedJSONReceiverNames()
	if err != nil {
		t.Fatalf("submissionExportedJSONReceiverNames() error = %v, want nil", err)
	}
	var wantJSON []string
	for door := submissionJSONDoorUnknown + 1; door < submissionJSONDoorLimit; door++ {
		wantJSON = append(wantJSON, door.receiverName())
	}
	slices.Sort(wantJSON)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", gotJSON, wantJSON)
	}
}

func submissionExportedJSONReceiverNames() ([]string, error) {
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
