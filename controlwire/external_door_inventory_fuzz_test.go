package controlwire

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

type controlwireJSONDoor uint8

const (
	controlwireJSONDoorUnknown controlwireJSONDoor = iota
	controlwireJSONDoorRequestNonce
	controlwireJSONDoorAuthorityNonce
	controlwireJSONDoorRevision
	controlwireJSONDoorPolicyRevisionID
	controlwireJSONDoorPolicyCursor
	controlwireJSONDoorRegistrationToken
	controlwireJSONDoorRegistrationTokenVerifier
	controlwireJSONDoorRouteFamily
	controlwireJSONDoorRequestCommitment
	controlwireJSONDoorReplayIdentity
	controlwireJSONDoorLimit
)

func (d controlwireJSONDoor) receiverName() string {
	switch d {
	case controlwireJSONDoorRequestNonce:
		return "RequestNonce"
	case controlwireJSONDoorAuthorityNonce:
		return "AuthorityNonce"
	case controlwireJSONDoorRevision:
		return "Revision"
	case controlwireJSONDoorPolicyRevisionID:
		return "PolicyRevisionID"
	case controlwireJSONDoorPolicyCursor:
		return "PolicyCursor"
	case controlwireJSONDoorRegistrationToken:
		return "RegistrationToken"
	case controlwireJSONDoorRegistrationTokenVerifier:
		return "RegistrationTokenVerifier"
	case controlwireJSONDoorRouteFamily:
		return "RouteFamily"
	case controlwireJSONDoorRequestCommitment:
		return "RequestCommitment"
	case controlwireJSONDoorReplayIdentity:
		return "ReplayIdentity"
	case controlwireJSONDoorUnknown, controlwireJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type controlwireFuzzFixtures struct {
	requestNonce   RequestNonce
	authorityNonce AuthorityNonce
	revision       Revision
	policyID       PolicyRevisionID
	policyCursor   PolicyCursor
	token          RegistrationToken
	verifier       RegistrationTokenVerifier
	routeFamily    RouteFamily
	commitment     RequestCommitment
	replayIdentity ReplayIdentity
}

type controlwireJSONSeed struct {
	door     controlwireJSONDoor
	document []byte
}

func FuzzControlwireExternalJSONDoorInventory(f *testing.F) {
	fixtures := controlwireFixturesForFuzz(f)
	defer func() { _ = fixtures.token.Destroy() }()
	for _, seed := range controlwireJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door), seed.document)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`),
		[]byte(`0`), []byte(`true`), []byte(`{`),
		bytes.Repeat([]byte(`[`), core.JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(controlwireJSONDoorPolicyCursor), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		switch controlwireJSONDoor(rawDoor) {
		case controlwireJSONDoorRequestNonce:
			fuzzControlwireJSONValue(t, controlwireJSONRequest[RequestNonce]{
				data: data, seed: fixtures.requestNonce, want: core.ErrControlWireNonce,
			})
		case controlwireJSONDoorAuthorityNonce:
			fuzzControlwireJSONValue(t, controlwireJSONRequest[AuthorityNonce]{
				data: data, seed: fixtures.authorityNonce, want: core.ErrControlWireNonce,
			})
		case controlwireJSONDoorRevision:
			fuzzControlwireJSONValue(t, controlwireJSONRequest[Revision]{
				data: data, seed: fixtures.revision, want: core.ErrControlWireRevision,
			})
		case controlwireJSONDoorPolicyRevisionID:
			fuzzControlwireJSONValue(t, controlwireJSONRequest[PolicyRevisionID]{
				data: data, seed: fixtures.policyID, want: core.ErrControlWirePolicyCursor,
			})
		case controlwireJSONDoorPolicyCursor:
			fuzzControlwireJSONValue(t, controlwireJSONRequest[PolicyCursor]{
				data: data, seed: fixtures.policyCursor, want: core.ErrControlWirePolicyCursor,
			})
		case controlwireJSONDoorRegistrationToken:
			fuzzRegistrationTokenJSON(t, data, fixtures.token)
		case controlwireJSONDoorRegistrationTokenVerifier:
			fuzzControlwireJSONValue(t, controlwireJSONRequest[RegistrationTokenVerifier]{
				data: data, seed: fixtures.verifier, want: core.ErrControlWireToken,
			})
		case controlwireJSONDoorRouteFamily:
			fuzzControlwireJSONValue(t, controlwireJSONRequest[RouteFamily]{
				data: data, seed: fixtures.routeFamily, want: core.ErrControlWireRoute,
			})
		case controlwireJSONDoorRequestCommitment:
			fuzzControlwireJSONValue(t, controlwireJSONRequest[RequestCommitment]{
				data: data, seed: fixtures.commitment, want: core.ErrControlWireContract,
			})
		case controlwireJSONDoorReplayIdentity:
			fuzzControlwireJSONValue(t, controlwireJSONRequest[ReplayIdentity]{
				data: data, seed: fixtures.replayIdentity, want: core.ErrControlWireContract,
			})
		case controlwireJSONDoorUnknown, controlwireJSONDoorLimit:
			return
		default:
			return
		}
	})
}

type controlwireTextDoor uint8

const (
	controlwireTextDoorUnknown controlwireTextDoor = iota
	controlwireTextDoorRequestNonce
	controlwireTextDoorAuthorityNonce
	controlwireTextDoorRevision
	controlwireTextDoorPolicyRevisionID
	controlwireTextDoorRegistrationToken
	controlwireTextDoorRegistrationTokenVerifier
	controlwireTextDoorRouteFamily
	controlwireTextDoorLimit
)

func FuzzControlwireExternalTextDoorInventory(f *testing.F) {
	fixtures := controlwireFixturesForFuzz(f)
	defer func() { _ = fixtures.token.Destroy() }()
	tokenText, err := fixtures.tokenText()
	if err != nil {
		f.Fatalf("tokenText() error = %v, want nil", err)
	}
	f.Add(uint8(controlwireTextDoorRequestNonce), fixtures.requestNonce.String())
	f.Add(uint8(controlwireTextDoorAuthorityNonce), fixtures.authorityNonce.String())
	f.Add(uint8(controlwireTextDoorRevision), fixtures.revision.String())
	f.Add(uint8(controlwireTextDoorPolicyRevisionID), fixtures.policyID.String())
	f.Add(uint8(controlwireTextDoorPolicyRevisionID), (PolicyRevisionID{}).String())
	f.Add(uint8(controlwireTextDoorRegistrationToken), tokenText)
	f.Add(uint8(controlwireTextDoorRegistrationTokenVerifier), fixtures.verifier.String())
	f.Add(uint8(controlwireTextDoorRouteFamily), routeFamilyTokens()[fixtures.routeFamily])
	for _, hostile := range []string{"", " ", "0", "unknown", "\x00", "\xff"} {
		f.Add(uint8(controlwireTextDoorAuthorityNonce), hostile)
		f.Add(uint8(controlwireTextDoorRegistrationToken), hostile)
	}
	f.Fuzz(func(t *testing.T, rawDoor uint8, value string) {
		var outcome controlwireTextOutcome
		switch controlwireTextDoor(rawDoor) {
		case controlwireTextDoorRequestNonce:
			got, err := ParseRequestNonce(value)
			outcome = controlwireTextOutcome{input: value, projection: got.String(), err: err, want: core.ErrControlWireNonce, validate: got.Validate}
		case controlwireTextDoorAuthorityNonce:
			got, err := ParseAuthorityNonce(value)
			outcome = controlwireTextOutcome{input: value, projection: got.String(), err: err, want: core.ErrControlWireNonce, validate: got.Validate}
		case controlwireTextDoorRevision:
			got, err := ParseRevision(value)
			outcome = controlwireTextOutcome{input: value, projection: got.String(), err: err, want: core.ErrControlWireRevision, validate: got.Validate}
		case controlwireTextDoorPolicyRevisionID:
			got, err := ParsePolicyRevisionID(value)
			outcome = controlwireTextOutcome{
				input: value, projection: got.String(), err: err,
				want: core.ErrControlWirePolicyCursor, validate: got.Validate,
				refusalProjection: (PolicyRevisionID{}).String(),
			}
		case controlwireTextDoorRegistrationToken:
			fuzzRegistrationTokenText(t, []byte(value))
			return
		case controlwireTextDoorRegistrationTokenVerifier:
			got, err := ParseRegistrationTokenVerifier(value)
			outcome = controlwireTextOutcome{input: value, projection: got.String(), err: err, want: core.ErrControlWireToken, validate: got.Validate}
		case controlwireTextDoorRouteFamily:
			got, err := ParseRouteFamily(value)
			projection := ""
			if got.IsValid() {
				projection = routeFamilyTokens()[got]
			}
			outcome = controlwireTextOutcome{input: value, projection: projection, err: err, want: core.ErrControlWireRoute, validate: got.Validate}
		case controlwireTextDoorUnknown, controlwireTextDoorLimit:
			return
		default:
			return
		}
		fuzzControlwireTextOutcome(t, outcome)
	})
}

type controlwireJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

type controlwireJSONRequest[T controlwireJSONValue] struct {
	data []byte
	seed T
	want error
}

func fuzzControlwireJSONValue[T controlwireJSONValue](t *testing.T, request controlwireJSONRequest[T]) {
	t.Helper()
	before, err := request.seed.MarshalJSON()
	if err != nil {
		t.Fatalf("controlwire seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := request.seed
	decoder := any(&candidate).(json.Unmarshaler)
	decodeErr := decoder.UnmarshalJSON(request.data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, request.want) || !errors.Is(decodeErr, core.ErrJSONContract) {
			t.Fatalf("controlwire JSON error = %v, want %v and %v", decodeErr, request.want, core.ErrJSONContract)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected controlwire JSON changed receiver: marshal error %v", marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted controlwire JSON Validate() error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || len(canonical) > core.JSONDocumentMaximumBytes {
		t.Fatalf("controlwire canonical JSON = (%d bytes, %v), want bounded and nil", len(canonical), err)
	}
	var roundTrip T
	if err := any(&roundTrip).(json.Unmarshaler).UnmarshalJSON(canonical); err != nil {
		t.Fatalf("controlwire canonical JSON decode error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("controlwire JSON lacks canonical fixed point: %v", err)
	}
}

func fuzzRegistrationTokenJSON(t *testing.T, data []byte, seed RegistrationToken) {
	t.Helper()
	beforeVerifier, err := seed.Verifier()
	if err != nil {
		t.Fatalf("RegistrationToken.Verifier(seed) error = %v, want nil", err)
	}
	candidate := seed
	decodeErr := candidate.UnmarshalJSON(data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, core.ErrControlWireToken) || !errors.Is(decodeErr, core.ErrJSONContract) {
			t.Fatalf("RegistrationToken.UnmarshalJSON() error = %v, want typed JSON/token refusal", decodeErr)
		}
		afterVerifier, verifierErr := candidate.Verifier()
		if verifierErr != nil || !afterVerifier.Equal(beforeVerifier) {
			t.Fatalf("rejected RegistrationToken JSON changed its receiver")
		}
		return
	}
	defer func() { _ = candidate.Destroy() }()
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted RegistrationToken.Validate() error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || !bytes.Equal(canonical, data) {
		t.Fatalf("accepted RegistrationToken JSON is not canonical: %v", err)
	}
	var roundTrip RegistrationToken
	if err := roundTrip.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("RegistrationToken canonical decode error = %v, want nil", err)
	}
	defer func() { _ = roundTrip.Destroy() }()
	verifier, err := candidate.Verifier()
	roundVerifier, roundErr := roundTrip.Verifier()
	if err != nil || roundErr != nil || !roundVerifier.Equal(verifier) {
		t.Fatalf("RegistrationToken canonical round trip changed verifier")
	}
}

func fuzzRegistrationTokenText(t *testing.T, data []byte) {
	t.Helper()
	token, err := ParseRegistrationToken(data)
	if err != nil {
		if !errors.Is(err, core.ErrControlWireToken) || token.Validate() == nil {
			t.Fatalf("ParseRegistrationToken() = (%v, %v), want zero typed refusal", token, err)
		}
		return
	}
	defer func() { _ = token.Destroy() }()
	verifier, err := token.Verifier()
	if err != nil || verifier.Validate() != nil || verifier.String() == string(data) {
		t.Fatalf("accepted RegistrationToken verifier = (%v, %v), want valid one-way projection", verifier, err)
	}
	encoded, err := token.MarshalJSON()
	if err != nil {
		t.Fatalf("RegistrationToken.MarshalJSON() error = %v, want nil", err)
	}
	decoded, err := core.DecodeJSONStringToken(encoded)
	if err != nil || decoded != string(data) {
		t.Fatalf("RegistrationToken canonical projection = (%q, %v), want exact input", decoded, err)
	}
}

type controlwireTextOutcome struct {
	input             string
	projection        string
	refusalProjection string
	err               error
	want              error
	validate          func() error
}

func fuzzControlwireTextOutcome(t *testing.T, outcome controlwireTextOutcome) {
	t.Helper()
	if outcome.err != nil {
		if !errors.Is(outcome.err, outcome.want) || outcome.projection != outcome.refusalProjection {
			t.Fatalf("controlwire text refusal = (%q, %v), want %q and %v",
				outcome.projection, outcome.err, outcome.refusalProjection, outcome.want)
		}
		return
	}
	if outcome.validate() != nil || outcome.projection != outcome.input {
		t.Fatalf("controlwire text acceptance = (%q, %v), want exact %q", outcome.projection, outcome.validate(), outcome.input)
	}
}

func controlwireFixturesForFuzz(t testing.TB) controlwireFuzzFixtures {
	t.Helper()
	var requestBytes, authorityBytes, tokenBytes [NonceBytes]byte
	for index := range requestBytes {
		requestBytes[index] = byte(index + 1)
		authorityBytes[index] = byte(index + 33)
		tokenBytes[index] = byte(index + 65)
	}
	requestNonce, err := NewRequestNonce(requestBytes)
	if err != nil {
		t.Fatalf("NewRequestNonce() error = %v, want nil", err)
	}
	authorityNonce, err := NewAuthorityNonce(authorityBytes)
	if err != nil {
		t.Fatalf("NewAuthorityNonce() error = %v, want nil", err)
	}
	policyID := PolicyRevisionID{15: 1}
	activation, err := NewPolicyActivation(1)
	if err != nil {
		t.Fatalf("NewPolicyActivation() error = %v, want nil", err)
	}
	token, err := NewRegistrationToken(tokenBytes)
	if err != nil {
		t.Fatalf("NewRegistrationToken() error = %v, want nil", err)
	}
	verifier, err := token.Verifier()
	if err != nil {
		t.Fatalf("RegistrationToken.Verifier() error = %v, want nil", err)
	}
	canonicalRequest, err := core.MarshalCanonicalJSONDocument(struct {
		Inventory string `json:"inventory"`
	}{Inventory: "controlwire"})
	if err != nil {
		t.Fatalf("MarshalCanonicalJSONDocument(replay seed) error = %v, want nil", err)
	}
	commitment, err := commitCanonicalRequest(canonicalRequest)
	if err != nil {
		t.Fatalf("commitCanonicalRequest() error = %v, want nil", err)
	}
	replayIdentity := ReplayIdentity{
		commitment: commitment, nonce: requestNonce, offering: core.OfferingWitness,
		family: RouteFamilyRegistrations, revision: Revision2026V1,
	}
	if err := replayIdentity.Validate(); err != nil {
		t.Fatalf("ReplayIdentity fuzz seed Validate() error = %v, want nil", err)
	}
	return controlwireFuzzFixtures{
		requestNonce: requestNonce, authorityNonce: authorityNonce, revision: Revision2026V1,
		policyID: policyID, policyCursor: PolicyCursor{Revision: policyID, Activation: activation},
		token: token, verifier: verifier, routeFamily: RouteFamilyRegistrations,
		commitment: commitment, replayIdentity: replayIdentity,
	}
}

func (f controlwireFuzzFixtures) tokenText() (string, error) {
	encoded, err := f.token.MarshalJSON()
	if err != nil {
		return "", err
	}
	return core.DecodeJSONStringToken(encoded)
}

func controlwireJSONSeedsForFuzz(t testing.TB, fixtures controlwireFuzzFixtures) []controlwireJSONSeed {
	t.Helper()
	return []controlwireJSONSeed{
		controlwireJSONSeedForFuzz(t, controlwireJSONDoorRequestNonce, fixtures.requestNonce),
		controlwireJSONSeedForFuzz(t, controlwireJSONDoorAuthorityNonce, fixtures.authorityNonce),
		controlwireJSONSeedForFuzz(t, controlwireJSONDoorRevision, fixtures.revision),
		controlwireJSONSeedForFuzz(t, controlwireJSONDoorPolicyRevisionID, fixtures.policyID),
		controlwireJSONSeedForFuzz(t, controlwireJSONDoorPolicyCursor, fixtures.policyCursor),
		controlwireJSONSeedForFuzz(t, controlwireJSONDoorRegistrationToken, fixtures.token),
		controlwireJSONSeedForFuzz(t, controlwireJSONDoorRegistrationTokenVerifier, fixtures.verifier),
		controlwireJSONSeedForFuzz(t, controlwireJSONDoorRouteFamily, fixtures.routeFamily),
		controlwireJSONSeedForFuzz(t, controlwireJSONDoorRequestCommitment, fixtures.commitment),
		controlwireJSONSeedForFuzz(t, controlwireJSONDoorReplayIdentity, fixtures.replayIdentity),
	}
}

func controlwireJSONSeedForFuzz(t testing.TB, door controlwireJSONDoor, value controlwireJSONValue) controlwireJSONSeed {
	t.Helper()
	document, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("controlwire fuzz seed MarshalJSON(%d) error = %v, want nil", door, err)
	}
	return controlwireJSONSeed{door: door, document: document}
}

func TestControlwireExternalIngressFuzzInventoryMatchesProduction(t *testing.T) {
	t.Parallel()
	gotJSON, err := controlwireExportedJSONReceiverNames()
	if err != nil {
		t.Fatalf("controlwireExportedJSONReceiverNames() error = %v, want nil", err)
	}
	var wantJSON []string
	for door := controlwireJSONDoorUnknown + 1; door < controlwireJSONDoorLimit; door++ {
		wantJSON = append(wantJSON, door.receiverName())
	}
	slices.Sort(wantJSON)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", gotJSON, wantJSON)
	}
}

func controlwireExportedJSONReceiverNames() ([]string, error) {
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
