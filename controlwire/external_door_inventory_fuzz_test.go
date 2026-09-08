package controlwire

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"errors"
	"go/ast"
	"go/token"
	"math"
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
	token          RegistrationToken
	replayIdentity ReplayIdentity
	policyCursor   PolicyCursor
	requestNonce   RequestNonce
	authorityNonce AuthorityNonce
	verifier       RegistrationTokenVerifier
	commitment     RequestCommitment
	policyID       PolicyRevisionID
	revision       Revision
	routeFamily    RouteFamily
}

type controlwireJSONSeed struct {
	document []byte
	door     controlwireJSONDoor
}

func FuzzControlwireExternalJSONDoorInventory(f *testing.F) {
	fixtures := controlwireFixturesForFuzz(f)
	defer func() { _ = fixtures.token.Destroy() }()
	for _, seed := range controlwireJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door), seed.document)
		f.Add(uint8(seed.door), append([]byte{' '}, seed.document...))
	}
	wide := fixtures.policyCursor
	wide.Activation = PolicyActivation(math.MaxUint64)
	wideSeed := controlwireJSONSeedForFuzz(f, controlwireJSONDoorPolicyCursor, wide)
	f.Add(uint8(wideSeed.door), wideSeed.document)

	for _, hostile := range [][]byte{nil, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`), []byte(`0`), []byte(`true`), []byte(`{`)} {
		f.Add(uint8(controlwireJSONDoorPolicyCursor), hostile)
	}
	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		door := controlwireJSONDoor(rawDoor)
		if door <= controlwireJSONDoorUnknown || door >= controlwireJSONDoorLimit {
			door = controlwireJSONDoor(rawDoor%uint8(controlwireJSONDoorLimit-1)) + 1
		}
		got, wantIdentity := fixtures.jsonReceiver(door)
		before, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("seed encoding error=%v, want nil", err)
		}
		wantAccept := controlwireJSONReferenceAccepts(door, data)
		gotErr := got.UnmarshalJSON(data)
		if (gotErr == nil) != wantAccept {
			t.Fatalf("door=%v decode error=%v, want acceptance=%v", door, gotErr, wantAccept)
		}
		if gotErr != nil {
			after, err := got.MarshalJSON()
			if !errors.Is(gotErr, wantIdentity) || !errors.Is(gotErr, core.ErrJSONContract) || err != nil || !bytes.Equal(after, before) {
				t.Fatalf("refusal=%v receiver=%q/%v, want %v and preserved %q", gotErr, after, err, wantIdentity, before)
			}
			return
		}
		if token, ok := got.(*RegistrationToken); ok {
			defer func() { _ = token.Destroy() }()
		}
		canonical, err := got.MarshalJSON()
		if err != nil || got.Validate() != nil {
			t.Fatalf("accepted value encoding=%v validation=%v, want nil", err, got.Validate())
		}
		// Preserve exact integers: RFC 8785's default float64 canonicalization
		// would erase differences above 2^53 in PolicyActivation.
		wantFacts := jsontext.Value(bytes.Clone(data))
		gotFacts := jsontext.Value(bytes.Clone(canonical))
		wantErr := wantFacts.Canonicalize(jsontext.CanonicalizeRawInts(false))
		factErr := gotFacts.Canonicalize(jsontext.CanonicalizeRawInts(false))
		if wantErr != nil || factErr != nil || !bytes.Equal(gotFacts, wantFacts) {
			t.Fatalf("accepted JSON facts=%q/%v, want %q/%v", gotFacts, factErr, wantFacts, wantErr)
		}
		round, _ := fixtures.jsonReceiver(door)
		if err := round.UnmarshalJSON(canonical); err != nil {
			t.Fatalf("canonical decode error=%v, want nil", err)
		}
		if token, ok := round.(*RegistrationToken); ok {
			defer func() { _ = token.Destroy() }()
		}
		second, err := round.MarshalJSON()
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatalf("fixed point=%q/%v, want %q/nil", second, err, canonical)
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
		door := controlwireTextDoor(rawDoor)
		if door <= controlwireTextDoorUnknown || door >= controlwireTextDoorLimit {
			door = controlwireTextDoor(rawDoor%uint8(controlwireTextDoorLimit-1)) + 1
		}
		wantAccept := controlwireTextReferenceAccepts(door, value)
		switch door {
		case controlwireTextDoorRequestNonce:
			got, err := ParseRequestNonce(value)
			outcome = controlwireTextOutcome{projection: got.String(), err: err, want: core.ErrControlWireNonce, validate: got.Validate}
		case controlwireTextDoorAuthorityNonce:
			got, err := ParseAuthorityNonce(value)
			outcome = controlwireTextOutcome{projection: got.String(), err: err, want: core.ErrControlWireNonce, validate: got.Validate}
		case controlwireTextDoorRevision:
			got, err := ParseRevision(value)
			outcome = controlwireTextOutcome{projection: got.String(), err: err, want: core.ErrControlWireRevision, validate: got.Validate}
		case controlwireTextDoorPolicyRevisionID:
			got, err := ParsePolicyRevisionID(value)
			outcome = controlwireTextOutcome{
				projection: got.String(), err: err,
				want: core.ErrControlWirePolicyCursor, validate: got.Validate,
				refusalProjection: (PolicyRevisionID{}).String(),
			}
		case controlwireTextDoorRegistrationToken:
			got, err := ParseRegistrationToken([]byte(value))
			if (err == nil) != wantAccept {
				t.Fatalf("token error=%v, want acceptance=%v", err, wantAccept)
			}
			if err != nil {
				if !errors.Is(err, core.ErrControlWireToken) || got.Validate() == nil {
					t.Fatalf("token refusal=%v/%v, want typed invalid zero", got, err)
				}
				return
			}
			defer func() { _ = got.Destroy() }()
			raw, err := hex.DecodeString(value)
			if err != nil {
				t.Fatalf("reference hex error=%v, want nil", err)
			}
			wantDigest := sha256.Sum256(raw)
			verifier, err := got.Verifier()
			if err != nil || verifier.String() != hex.EncodeToString(wantDigest[:]) {
				t.Fatalf("token verifier=%v/%v, want SHA256 %x", verifier, err, wantDigest)
			}
			encoded, err := got.MarshalJSON()
			text, decodeErr := core.DecodeJSONStringToken(encoded)
			if err != nil || decodeErr != nil || text != value {
				t.Fatalf("token projection=%q/%v/%v, want %q/nil", text, err, decodeErr, value)
			}
			return
		case controlwireTextDoorRegistrationTokenVerifier:
			got, err := ParseRegistrationTokenVerifier(value)
			outcome = controlwireTextOutcome{projection: got.String(), err: err, want: core.ErrControlWireToken, validate: got.Validate}
		case controlwireTextDoorRouteFamily:
			got, err := ParseRouteFamily(value)
			projection := ""
			if got.IsValid() {
				projection = routeFamilyTokens()[got]
			}
			outcome = controlwireTextOutcome{projection: projection, err: err, want: core.ErrControlWireRoute, validate: got.Validate}
		case controlwireTextDoorUnknown, controlwireTextDoorLimit:
			return
		default:
			return
		}
		if (outcome.err == nil) != wantAccept {
			t.Fatalf("text error=%v, want acceptance=%v", outcome.err, wantAccept)
		}
		if outcome.err != nil {
			if !errors.Is(outcome.err, outcome.want) || outcome.projection != outcome.refusalProjection {
				t.Fatalf("text refusal=%q/%v, want %q/%v", outcome.projection, outcome.err, outcome.refusalProjection, outcome.want)
			}
			return
		}
		if outcome.validate() != nil || outcome.projection != value {
			t.Fatalf("text facts=%q/%v, want %q/nil", outcome.projection, outcome.validate(), value)
		}
	})
}

type controlwireJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

type controlwireTextOutcome struct {
	err               error
	want              error
	validate          func() error
	projection        string
	refusalProjection string
}

func controlwireFixturesForFuzz(t testing.TB) controlwireFuzzFixtures {
	t.Helper()
	var requestBytes, authorityBytes, tokenBytes [core.SHA256DigestBytes]byte
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
		commitment: commitment, nonce: requestNonce, offering: controlwireOfferingFixture(t, 4),
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
	for door := range controlwireJSONDoorLimit {
		if door < controlwireJSONDoorUnknown+1 {
			continue
		}
		wantJSON = append(wantJSON, door.receiverName())
	}
	slices.Sort(wantJSON)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", gotJSON, wantJSON)
	}
}

func controlwireExportedJSONReceiverNames() ([]string, error) {
	files, err := controlwireGoSources.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	fileSet := token.NewFileSet()
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		parsed, parseErr := controlwireParseSource(fileSet, file.Name())
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
