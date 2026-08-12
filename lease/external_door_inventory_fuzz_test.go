package lease_test

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
	"github.com/deliri/primitive/v2026/lease"
)

type leaseJSONDoor uint8

const (
	leaseJSONDoorUnknown leaseJSONDoor = iota
	leaseJSONDoorProduct
	leaseJSONDoorEntitlementID
	leaseJSONDoorDeviceID
	leaseJSONDoorGeneration
	leaseJSONDoorDocument
	leaseJSONDoorSubject
	leaseJSONDoorGrant
	leaseJSONDoorRefusal
	leaseJSONDoorRevocation
	leaseJSONDoorDecision
	leaseJSONDoorRevision
	leaseJSONDoorOutcome
	leaseJSONDoorRevocationReason
	leaseJSONDoorLimit
)

func (d leaseJSONDoor) receiverName() string {
	switch d {
	case leaseJSONDoorProduct:
		return "Product"
	case leaseJSONDoorEntitlementID:
		return "EntitlementID"
	case leaseJSONDoorDeviceID:
		return "DeviceID"
	case leaseJSONDoorGeneration:
		return "Generation"
	case leaseJSONDoorDocument:
		return "Document"
	case leaseJSONDoorSubject:
		return "Subject"
	case leaseJSONDoorGrant:
		return "Grant"
	case leaseJSONDoorRefusal:
		return "Refusal"
	case leaseJSONDoorRevocation:
		return "Revocation"
	case leaseJSONDoorDecision:
		return "Decision"
	case leaseJSONDoorRevision:
		return "Revision"
	case leaseJSONDoorOutcome:
		return "Outcome"
	case leaseJSONDoorRevocationReason:
		return "RevocationReason"
	case leaseJSONDoorUnknown, leaseJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type leaseJSONFixtures struct {
	product       lease.Product
	entitlement   lease.EntitlementID
	device        lease.DeviceID
	generation    lease.Generation
	document      lease.Document
	subject       lease.Subject
	grant         lease.Grant
	refusal       lease.Refusal
	revocation    lease.Revocation
	decision      lease.Decision
	revision      lease.Revision
	outcome       lease.Outcome
	reason        lease.RevocationReason
	authority     authorityFixture
	signedSubject lease.Subject
}

type leaseJSONSeed struct {
	door     leaseJSONDoor
	document []byte
}

func FuzzLeaseExternalJSONDoorInventory(f *testing.F) {
	fixtures := leaseFixturesForFuzz(f)
	for _, seed := range leaseJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door), seed.document)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`),
		[]byte(`0`), []byte(`true`), []byte(`{`),
		bytes.Repeat([]byte(`[`), core.JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(leaseJSONDoorDocument), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		switch leaseJSONDoor(rawDoor) {
		case leaseJSONDoorProduct:
			fuzzLeaseJSONValue(t, data, fixtures.product)
		case leaseJSONDoorEntitlementID:
			fuzzLeaseJSONValue(t, data, fixtures.entitlement)
		case leaseJSONDoorDeviceID:
			fuzzLeaseJSONValue(t, data, fixtures.device)
		case leaseJSONDoorGeneration:
			fuzzLeaseJSONValue(t, data, fixtures.generation)
		case leaseJSONDoorDocument:
			fuzzLeaseDocument(t, data, fixtures)
		case leaseJSONDoorSubject:
			fuzzLeaseJSONValue(t, data, fixtures.subject)
		case leaseJSONDoorGrant:
			fuzzLeaseJSONValue(t, data, fixtures.grant)
		case leaseJSONDoorRefusal:
			fuzzLeaseJSONValue(t, data, fixtures.refusal)
		case leaseJSONDoorRevocation:
			fuzzLeaseJSONValue(t, data, fixtures.revocation)
		case leaseJSONDoorDecision:
			fuzzLeaseJSONValue(t, data, fixtures.decision)
		case leaseJSONDoorRevision:
			fuzzLeaseJSONValue(t, data, fixtures.revision)
		case leaseJSONDoorOutcome:
			fuzzLeaseJSONValue(t, data, fixtures.outcome)
		case leaseJSONDoorRevocationReason:
			fuzzLeaseJSONValue(t, data, fixtures.reason)
		case leaseJSONDoorUnknown, leaseJSONDoorLimit:
			return
		default:
			return
		}
	})
}

type leaseTextDoor uint8

const (
	leaseTextDoorUnknown leaseTextDoor = iota
	leaseTextDoorProduct
	leaseTextDoorEntitlementID
	leaseTextDoorDeviceID
	leaseTextDoorGeneration
	leaseTextDoorRevision
	leaseTextDoorOutcome
	leaseTextDoorRevocationReason
	leaseTextDoorLimit
)

func (d leaseTextDoor) functionName() string {
	switch d {
	case leaseTextDoorProduct:
		return "ParseProduct"
	case leaseTextDoorEntitlementID:
		return "ParseEntitlementID"
	case leaseTextDoorDeviceID:
		return "ParseDeviceID"
	case leaseTextDoorGeneration:
		return "ParseGeneration"
	case leaseTextDoorRevision:
		return "ParseRevision"
	case leaseTextDoorOutcome:
		return "ParseOutcome"
	case leaseTextDoorRevocationReason:
		return "ParseRevocationReason"
	case leaseTextDoorUnknown, leaseTextDoorLimit:
		return ""
	default:
		return ""
	}
}

func FuzzLeaseExternalTextDoorInventory(f *testing.F) {
	fixtures := leaseFixturesForFuzz(f)
	f.Add(uint8(leaseTextDoorProduct), fixtures.product.String())
	f.Add(uint8(leaseTextDoorEntitlementID), fixtures.entitlement.String())
	f.Add(uint8(leaseTextDoorDeviceID), fixtures.device.String())
	f.Add(uint8(leaseTextDoorGeneration), fixtures.generation.String())
	f.Add(uint8(leaseTextDoorRevision), fixtures.revision.String())
	f.Add(uint8(leaseTextDoorOutcome), fixtures.outcome.String())
	f.Add(uint8(leaseTextDoorRevocationReason), fixtures.reason.String())
	for _, hostile := range []string{"", " ", "0", "01", "A", "\x00", "\xff"} {
		f.Add(uint8(leaseTextDoorGeneration), hostile)
		f.Add(uint8(leaseTextDoorRevision), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, value string) {
		var outcome leaseTextOutcome
		switch leaseTextDoor(rawDoor) {
		case leaseTextDoorProduct:
			got, err := lease.ParseProduct(value)
			outcome = leaseTextOutcome{input: value, projection: got.String(), err: err, validate: got.Validate}
		case leaseTextDoorEntitlementID:
			got, err := lease.ParseEntitlementID(value)
			outcome = leaseTextOutcome{input: value, projection: got.String(), err: err, validate: got.Validate}
		case leaseTextDoorDeviceID:
			got, err := lease.ParseDeviceID(value)
			outcome = leaseTextOutcome{input: value, projection: got.String(), err: err, validate: got.Validate}
		case leaseTextDoorGeneration:
			got, err := lease.ParseGeneration(value)
			outcome = leaseTextOutcome{input: value, projection: got.String(), err: err, validate: got.Validate}
		case leaseTextDoorRevision:
			got, err := lease.ParseRevision(value)
			outcome = leaseTextOutcome{
				input: value, projection: got.String(), refusalProjection: core.UnknownEnumDiagnostic,
				err: err, validate: got.Validate,
			}
		case leaseTextDoorOutcome:
			got, err := lease.ParseOutcome(value)
			outcome = leaseTextOutcome{
				input: value, projection: got.String(), refusalProjection: core.UnknownEnumDiagnostic,
				err: err, validate: got.Validate,
			}
		case leaseTextDoorRevocationReason:
			got, err := lease.ParseRevocationReason(value)
			outcome = leaseTextOutcome{
				input: value, projection: got.String(), refusalProjection: core.UnknownEnumDiagnostic,
				err: err, validate: got.Validate,
			}
		case leaseTextDoorUnknown, leaseTextDoorLimit:
			return
		default:
			return
		}
		fuzzLeaseTextOutcome(t, outcome)
	})
}

type leaseJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

func fuzzLeaseJSONValue[T leaseJSONValue](t *testing.T, data []byte, seed T) {
	t.Helper()

	before, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("lease seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := seed
	decoder, ok := any(&candidate).(json.Unmarshaler)
	if !ok {
		t.Fatalf("lease JSON receiver %T lacks json.Unmarshaler", &candidate)
	}
	decodeErr := decoder.UnmarshalJSON(data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, core.ErrLeaseContract) ||
			!errors.Is(decodeErr, core.ErrJSONContract) {
			t.Fatalf("lease JSON door error = %v, want %v and %v",
				decodeErr, core.ErrLeaseContract, core.ErrJSONContract)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected lease JSON door changed its receiver: marshal error %v", marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted lease JSON validation error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || len(canonical) > core.JSONDocumentMaximumBytes {
		t.Fatalf("lease canonical JSON = (%d bytes, %v), want bounded and nil", len(canonical), err)
	}
	var roundTrip T
	roundTripDecoder, ok := any(&roundTrip).(json.Unmarshaler)
	if !ok {
		t.Fatalf("lease round-trip receiver %T lacks json.Unmarshaler", &roundTrip)
	}
	if err := roundTripDecoder.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("lease canonical JSON decode error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("lease JSON door lacks a canonical fixed point: marshal error %v", err)
	}
}

func fuzzLeaseDocument(t *testing.T, data []byte, fixtures leaseJSONFixtures) {
	t.Helper()
	fuzzLeaseJSONValue(t, data, fixtures.document)

	candidate := fixtures.document
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	proof, err := lease.Verify(lease.VerifyRequest{
		Document: candidate, TrustedKeys: fixtures.authority.trusted,
		ExpectedSubject: fixtures.signedSubject,
	})
	if err != nil {
		if !errors.Is(err, core.ErrLeaseVerification) || proof != (lease.Verified{}) {
			t.Fatalf("lease.Verify(fuzz document) = (%v, %v), want typed refusal and zero proof", proof, err)
		}
		return
	}
	canonical, marshalErr := candidate.MarshalJSON()
	signed, signedErr := fixtures.document.MarshalJSON()
	if proof.Validate() != nil || marshalErr != nil || signedErr != nil || !bytes.Equal(canonical, signed) {
		t.Fatalf("lease.Verify(fuzz document) authenticated facts outside the signed seed")
	}
}

func leaseFixturesForFuzz(t testing.TB) leaseJSONFixtures {
	t.Helper()
	subject := fixtureSubject(t, 191)
	decision := fixtureGrantDecision(t, subject, 1, 1_000, fixtureGrant())
	authority := fixtureAuthority(t, 192)
	document, _ := fixtureVerified(t, authority, decision, subject)
	generation, err := lease.NewGeneration(1)
	if err != nil {
		t.Fatalf("lease.NewGeneration() error = %v, want nil", err)
	}
	return leaseJSONFixtures{
		product: subject.Product, entitlement: subject.EntitlementID, device: subject.DeviceID,
		generation: generation, document: document, subject: subject, grant: fixtureGrant(),
		refusal:    lease.Refusal{ContactAfter: fixtureInstant(6_000)},
		revocation: lease.Revocation{Reason: lease.RevocationReasonLicenceBreach},
		decision:   decision, revision: lease.RevisionV1, outcome: lease.OutcomeGrant,
		reason: lease.RevocationReasonLicenceBreach, authority: authority, signedSubject: subject,
	}
}

func leaseJSONSeedsForFuzz(t testing.TB, fixtures leaseJSONFixtures) []leaseJSONSeed {
	t.Helper()
	return []leaseJSONSeed{
		leaseJSONSeedForFuzz(t, leaseJSONDoorProduct, fixtures.product),
		leaseJSONSeedForFuzz(t, leaseJSONDoorEntitlementID, fixtures.entitlement),
		leaseJSONSeedForFuzz(t, leaseJSONDoorDeviceID, fixtures.device),
		leaseJSONSeedForFuzz(t, leaseJSONDoorGeneration, fixtures.generation),
		leaseJSONSeedForFuzz(t, leaseJSONDoorDocument, fixtures.document),
		leaseJSONSeedForFuzz(t, leaseJSONDoorSubject, fixtures.subject),
		leaseJSONSeedForFuzz(t, leaseJSONDoorGrant, fixtures.grant),
		leaseJSONSeedForFuzz(t, leaseJSONDoorRefusal, fixtures.refusal),
		leaseJSONSeedForFuzz(t, leaseJSONDoorRevocation, fixtures.revocation),
		leaseJSONSeedForFuzz(t, leaseJSONDoorDecision, fixtures.decision),
		leaseJSONSeedForFuzz(t, leaseJSONDoorRevision, fixtures.revision),
		leaseJSONSeedForFuzz(t, leaseJSONDoorOutcome, fixtures.outcome),
		leaseJSONSeedForFuzz(t, leaseJSONDoorRevocationReason, fixtures.reason),
	}
}

func leaseJSONSeedForFuzz(t testing.TB, door leaseJSONDoor, value leaseJSONValue) leaseJSONSeed {
	t.Helper()
	document, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("lease fuzz seed MarshalJSON(%d) error = %v, want nil", door, err)
	}
	return leaseJSONSeed{door: door, document: document}
}

type leaseTextOutcome struct {
	input             string
	projection        string
	refusalProjection string
	err               error
	validate          func() error
}

func fuzzLeaseTextOutcome(t *testing.T, outcome leaseTextOutcome) {
	t.Helper()
	if outcome.err != nil {
		if !errors.Is(outcome.err, core.ErrLeaseContract) ||
			outcome.projection != outcome.refusalProjection {
			t.Fatalf("lease text refusal = (%q, %v), want %q and %v",
				outcome.projection, outcome.err, outcome.refusalProjection, core.ErrLeaseContract)
		}
		return
	}
	if outcome.validate() != nil || outcome.projection != outcome.input {
		t.Fatalf("lease text acceptance = (%q, %v), want exact %q and nil",
			outcome.projection, outcome.validate(), outcome.input)
	}
}

func TestLeaseExternalIngressFuzzInventoryMatchesProduction(t *testing.T) {
	t.Parallel()
	gotJSON, err := leaseExportedJSONReceiverNames()
	if err != nil {
		t.Fatalf("leaseExportedJSONReceiverNames() error = %v, want nil", err)
	}
	var wantJSON []string
	for door := leaseJSONDoorUnknown + 1; door < leaseJSONDoorLimit; door++ {
		wantJSON = append(wantJSON, door.receiverName())
	}
	slices.Sort(wantJSON)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", gotJSON, wantJSON)
	}
	gotText := []string{
		"ParseDeviceID", "ParseEntitlementID", "ParseGeneration", "ParseOutcome",
		"ParseProduct", "ParseRevision", "ParseRevocationReason",
	}
	var wantText []string
	for door := leaseTextDoorUnknown + 1; door < leaseTextDoorLimit; door++ {
		wantText = append(wantText, door.functionName())
	}
	slices.Sort(wantText)
	if !slices.Equal(gotText, wantText) {
		t.Fatalf("public text parsers = %v, fuzz inventory = %v", gotText, wantText)
	}
}

func leaseExportedJSONReceiverNames() ([]string, error) {
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
