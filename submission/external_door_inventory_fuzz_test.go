package submission

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
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
	decisionDocument       DecisionDocument
	decisionWire           []byte
	uploadDecisionWire     []byte
	uploadDecisionDocument DecisionDocument
	grantWire              []byte
	requestPayload         RequestPayload
	completionPayload      CompletionPayload
	requestDocument        RequestDocument
	grantDocument          GrantDocument
	completionDocument     CompletionDocument
	reuse                  reuseEvidenceFixture
	grant                  grantFixture
	completion             completionFixture
	grantPayload           GrantPayload
	requestCommitment      RequestCommitment
	uploadID               UploadID
	decisionKind           DecisionKind
	signingDomain          SigningDomain
}

type submissionJSONSeed struct {
	document []byte
	door     submissionJSONDoor
}

func FuzzSubmissionExternalJSONDoorInventory(f *testing.F) {
	fixtures := submissionFixturesForFuzz(f)
	for _, seed := range submissionJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door-1), seed.document)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`),
		[]byte(`0`), []byte(`true`), []byte(`{`),
		bytes.Repeat([]byte(`[`), core.JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(submissionJSONDoorCompletionDocument-1), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		switch submissionJSONDoor(rawDoor%uint8(submissionJSONDoorLimit-1) + 1) {
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
	identity := testManifestIntent(f).Upload
	f.Add(uint8(submissionTextDoorUploadID-1), identity.String())
	for _, domain := range []SigningDomain{SigningDomainRequestV1, SigningDomainGrantV1, SigningDomainCompletionV1} {
		f.Add(uint8(submissionTextDoorSigningDomain-1), domain.String())
		f.Add(uint8(submissionTextDoorCanonicalSigningDomain-1), domain.String())
	}
	for _, hostile := range []string{"", " ", "unknown", "A", "\x00", "\xff"} {
		for door := submissionTextDoorUploadID; door < submissionTextDoorLimit; door++ {
			f.Add(uint8(door-1), hostile)
		}
	}
	f.Fuzz(func(t *testing.T, rawDoor uint8, value string) {
		door := submissionTextDoor(rawDoor%uint8(submissionTextDoorLimit-1) + 1)
		if door == submissionTextDoorUploadID {
			got, err := ParseUploadID(value)
			_, ownerErr := id.ParseUUIDv7(value)
			if (err == nil) != (ownerErr == nil) {
				t.Fatalf("upload identity differs from UUIDv7 owner: %v / %v", err, ownerErr)
			}
			fuzzSubmissionTextOutcome(t, submissionTextOutcome{input: value, projection: got.String(), err: err, validate: got.Validate})
			return
		}
		want := SigningDomainUnknown
		switch value {
		case SigningDomainRequestV1Token:
			want = SigningDomainRequestV1
		case SigningDomainGrantV1Token:
			want = SigningDomainGrantV1
		case SigningDomainCompletionV1Token:
			want = SigningDomainCompletionV1
		}
		var got SigningDomain
		var err error
		if door == submissionTextDoorSigningDomain {
			got, err = ParseSigningDomain(value)
		} else {
			got, err = SigningDomainUnknown.ParseCanonicalText([]byte(value))
		}
		if got != want || (err == nil) != (want != SigningDomainUnknown) {
			t.Fatalf("domain=%v error=%v, want %v", got, err, want)
		}
		fuzzSubmissionTextOutcome(t, submissionTextOutcome{input: value, projection: got.String(), err: err, validate: got.Validate})
	})
}

type submissionJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

func fuzzSubmissionJSONValue[T interface {
	comparable
	submissionJSONValue
}, P submissionJSONReceiver[T]](t *testing.T, data []byte, seed T) {
	t.Helper()
	before, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("submission seed MarshalJSON() error = %v, want nil", err)
	}
	probe := seed
	padded := append(bytes.Repeat([]byte(" "), len(data)%4096), before...)
	if err := P(&probe).UnmarshalJSON(padded); err != nil || probe != seed {
		t.Fatalf("generated valid representation changed facts or was refused: %v", err)
	}
	candidate := seed
	decodeErr := P(&candidate).UnmarshalJSON(data)
	if decodeErr != nil {
		if bytes.Equal(data, before) {
			t.Fatalf("valid seed refused: %v", decodeErr)
		}
		if !submissionJSONRefusal(decodeErr) {
			t.Fatalf("submission JSON door error = %v, want typed JSON/control-plane refusal", decodeErr)
		}
		after, marshalErr := candidate.MarshalJSON()
		if candidate != seed || marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected submission JSON door changed its receiver: marshal error %v", marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted submission JSON validation error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil {
		t.Fatalf("submission canonical JSON = (%d bytes, %v), want canonical bytes and nil", len(canonical), err)
	}
	var roundTrip T
	if err := P(&roundTrip).UnmarshalJSON(canonical); err != nil || roundTrip != candidate {
		t.Fatalf("canonical JSON changed typed facts: %v", err)
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
			!errors.Is(err, core.ErrAttestVerification) || proof != (VerifiedRequest{}) || candidate == fixtures.requestDocument {
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
	probe := fixtures.grantDocument
	padded := append(bytes.Repeat([]byte(" "), len(data)%4096), fixtures.grantWire...)
	if err := probe.UnmarshalJSON(padded); err != nil || !sameGrantDocument(probe, fixtures.grantDocument) {
		t.Fatalf("valid grant probe refused or changed: %v", err)
	}
	candidate := fixtures.grantDocument
	if err := candidate.UnmarshalJSON(data); err != nil {
		if bytes.Equal(data, fixtures.grantWire) || !submissionJSONRefusal(err) || !sameGrantDocument(candidate, fixtures.grantDocument) {
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
		if !errors.Is(err, core.ErrControlPlaneContract) || !verifiedGrantIsZero(proof) || sameGrantDocument(candidate, fixtures.grantDocument) {
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
	probe := fixtures.decisionDocument
	padded := append(bytes.Repeat([]byte(" "), len(data)%4096), fixtures.decisionWire...)
	if err := probe.UnmarshalJSON(padded); err != nil || !sameReuseDecision(probe, fixtures.decisionDocument) {
		t.Fatalf("valid decision probe refused or changed: %v", err)
	}
	candidate := fixtures.decisionDocument
	if err := candidate.UnmarshalJSON(data); err != nil {
		if bytes.Equal(data, fixtures.decisionWire) || bytes.Equal(data, fixtures.uploadDecisionWire) || !submissionJSONRefusal(err) || !sameReuseDecision(candidate, fixtures.decisionDocument) {
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
		if !errors.Is(err, core.ErrControlPlaneContract) || proof != (VerifiedDecision{}) || sameReuseDecision(candidate, fixtures.decisionDocument) || sameUploadDecision(candidate, fixtures.uploadDecisionDocument) {
			t.Fatalf("VerifyDecision(fuzz document) = (%v, %v), want typed refusal and zero proof", proof, err)
		}
		return
	}
	if proof.Validate() != nil || !(sameReuseDecision(candidate, fixtures.decisionDocument) || sameUploadDecision(candidate, fixtures.uploadDecisionDocument)) {
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
		if !errors.Is(err, core.ErrControlPlaneContract) || proof != (VerifiedCompletion{}) || candidate == fixtures.completionDocument {
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
	uploadProjection, err := UploadDecision(grant.projection)
	if err != nil {
		t.Fatal(err)
	}
	uploadWire, err := uploadProjection.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var uploadDocument DecisionDocument
	if err := uploadDocument.UnmarshalJSON(uploadWire); err != nil {
		t.Fatal(err)
	}
	return submissionFuzzFixtures{
		requestPayload: completion.request, requestDocument: requestDocument,
		requestCommitment: commitment, signingDomain: SigningDomainRequestV1,
		decisionKind: DecisionReuse, decisionDocument: decision, decisionWire: decisionWire,
		uploadDecisionWire: uploadWire, uploadDecisionDocument: uploadDocument,
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
		{door: submissionJSONDoorDecisionDocument, document: fixtures.uploadDecisionWire},
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
	for door := range submissionJSONDoorLimit {
		if door < submissionJSONDoorUnknown+1 {
			continue
		}
		wantJSON = append(wantJSON, door.receiverName())
	}
	slices.Sort(wantJSON)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", gotJSON, wantJSON)
	}
}

func submissionExportedJSONReceiverNames() ([]string, error) {
	files, err := submissionContractSources.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	fileSet := token.NewFileSet()
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		source, readErr := submissionContractSources.ReadFile(file.Name())
		if readErr != nil {
			return nil, readErr
		}
		parsed, parseErr := parser.ParseFile(fileSet, file.Name(), source, parser.SkipObjectResolution)
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

func sameUploadDecision(left, right DecisionDocument) bool {
	return left.Kind == DecisionUpload && right.Kind == DecisionUpload && left.Evidence == nil && right.Evidence == nil && left.Grant != nil && right.Grant != nil && sameGrantDocument(*left.Grant, *right.Grant)
}
