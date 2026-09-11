package timeproof

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

type timeproofJSONDoor uint8

const (
	timeproofJSONDoorUnknown timeproofJSONDoor = iota
	timeproofJSONDoorAuthority
	timeproofJSONDoorTimestampPolicy
	timeproofJSONDoorRequest
	timeproofJSONDoorNonce
	timeproofJSONDoorAuthorityEvidence
	timeproofJSONDoorSerialNumber
	timeproofJSONDoorAuthoritativeTimestamp
	timeproofJSONDoorLimit
)

func (d timeproofJSONDoor) receiverName() string {
	switch d {
	case timeproofJSONDoorAuthority:
		return "Authority"
	case timeproofJSONDoorTimestampPolicy:
		return "TimestampPolicy"
	case timeproofJSONDoorRequest:
		return "Request"
	case timeproofJSONDoorNonce:
		return "Nonce"
	case timeproofJSONDoorAuthorityEvidence:
		return "AuthorityEvidence"
	case timeproofJSONDoorSerialNumber:
		return "SerialNumber"
	case timeproofJSONDoorAuthoritativeTimestamp:
		return "AuthoritativeTimestamp"
	case timeproofJSONDoorUnknown, timeproofJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type timeproofFuzzFixtures struct {
	response  []byte
	evidence  AuthorityEvidence
	request   Request
	timestamp AuthoritativeTimestamp
	serial    SerialNumber
	nonce     Nonce
	authority Authority
	policy    TimestampPolicy
}

type timeproofJSONSeed struct {
	document []byte
	door     timeproofJSONDoor
}

func FuzzTimeproofExternalJSONDoorInventory(f *testing.F) {
	fixtures := timeproofFixturesForFuzz(f)
	f.Add(uint8(timeproofJSONDoorAuthorityEvidence-1), timeproofRefusalJSONFixture(f, fixtures))
	for _, seed := range timeproofJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door-1), seed.document)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`),
		[]byte(`0`), []byte(`true`), []byte(`{`),
		bytes.Repeat([]byte(`[`), core.JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(timeproofJSONDoorAuthoritativeTimestamp-1), hostile)
	}
	f.Add(uint8(timeproofJSONDoorAuthority-1), []byte(`""`))

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		door := timeproofJSONDoor(rawDoor%uint8(timeproofJSONDoorLimit-1)) + 1
		switch door {
		case timeproofJSONDoorAuthority:
			fuzzTimeproofJSONValue(t, data, fixtures.authority)
		case timeproofJSONDoorTimestampPolicy:
			fuzzTimeproofJSONValue(t, data, fixtures.policy)
		case timeproofJSONDoorRequest:
			fuzzTimeproofJSONValue(t, data, fixtures.request)
		case timeproofJSONDoorNonce:
			fuzzTimeproofJSONValue(t, data, fixtures.nonce)
		case timeproofJSONDoorAuthorityEvidence:
			fuzzTimeproofAuthorityEvidence(t, data, fixtures)
		case timeproofJSONDoorSerialNumber:
			fuzzTimeproofJSONValue(t, data, fixtures.serial)
		case timeproofJSONDoorAuthoritativeTimestamp:
			fuzzAuthoritativeTimestamp(t, data, fixtures)
		case timeproofJSONDoorUnknown, timeproofJSONDoorLimit:
			return
		default:
			return
		}
	})
}

type timeproofJSONValue interface {
	Validate() error
	MarshalJSON() ([]byte, error)
}

func fuzzTimeproofJSONValue[T timeproofJSONValue, P interface {
	*T
	json.Unmarshaler
}](t *testing.T, data []byte, seed T) {
	t.Helper()
	before, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("timeproof seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := seed
	decoder := P(&candidate)
	decodeErr := decoder.UnmarshalJSON(data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, core.ErrTimeProofContract) ||
			!errors.Is(decodeErr, core.ErrJSONContract) {
			t.Fatalf("timeproof JSON door error = %v, want typed JSON/timeproof refusal", decodeErr)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected timeproof JSON door changed its receiver: marshal error %v", marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted timeproof JSON validation error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || len(canonical) > core.JSONDocumentMaximumBytes || !bytes.Equal(canonical, data) {
		t.Fatalf("timeproof canonical JSON = (%d bytes, %v), want exact admitted source, bounded and nil", len(canonical), err)
	}
	var roundTrip T
	roundTripDecoder := P(&roundTrip)
	if err := roundTripDecoder.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("timeproof canonical JSON decode error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("timeproof JSON door lacks a canonical fixed point: marshal error %v", err)
	}
}

func fuzzTimeproofAuthorityEvidence(t *testing.T, data []byte, fixtures timeproofFuzzFixtures) {
	t.Helper()
	fuzzTimeproofJSONValue(t, data, fixtures.evidence)
	candidate := fixtures.evidence
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	if candidate.Validate() != nil {
		t.Fatalf("accepted evidence = %+v, want valid structural metadata", candidate)
	}
	// Evidence is metadata, not authentication. The separate Restore oracle
	// binds metadata to caller-owned bytes through the real verifier.

}

func fuzzAuthoritativeTimestamp(t *testing.T, data []byte, fixtures timeproofFuzzFixtures) {
	t.Helper()
	before, err := fixtures.timestamp.MarshalJSON()
	if err != nil {
		t.Fatalf("AuthoritativeTimestamp.MarshalJSON(seed) error = %v, want nil", err)
	}
	candidate := fixtures.timestamp
	decodeErr := candidate.Restore(RestoreRequest{Document: data, Response: fixtures.response, ExpectedDigest: fixtures.request.Digest()})
	if decodeErr != nil {
		stable := errors.Is(decodeErr, core.ErrJSONContract) ||
			errors.Is(decodeErr, core.ErrTimeProofInvalid)
		after, marshalErr := candidate.MarshalJSON()
		if !stable || marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("AuthoritativeTimestamp refusal = (%v, %v), want typed and preserved", candidate, decodeErr)
		}
		return
	}
	if candidate.Validate() != nil {
		t.Fatalf("accepted AuthoritativeTimestamp failed Validate")
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || !bytes.Equal(canonical, before) {
		t.Fatalf("authenticated timestamp differs from authentic canonical seed")
	}
	proof, err := Verify(VerifyRequest{
		Response: fixtures.response, Request: candidate.Evidence().Request(),
		ExpectedDigest: candidate.Evidence().Digest(),
	})
	if err != nil || proof.Validate() != nil {
		t.Fatalf("independent Verify(accepted timestamp) = (%v, %v), want valid and nil", proof, err)
	}
	second, err := proof.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("independent timestamp verification lacks canonical fixed point")
	}
}

func sameTimeproofEvidence(left, right AuthorityEvidence) bool {
	return left.Request().Digest() == right.Request().Digest() &&
		left.Request().Nonce() == right.Request().Nonce() &&
		left.Authority() == right.Authority() &&
		left.ResponseDigest() == right.ResponseDigest() && left.ResponseSize() == right.ResponseSize()
}

func timeproofFixturesForFuzz(t testing.TB) timeproofFuzzFixtures {
	t.Helper()
	authentic := loadAuthenticFixture(t)
	timestamp, err := Verify(VerifyRequest{
		Response: authentic.response, Request: authentic.request,
		ExpectedDigest: authentic.digest,
	})
	if err != nil {
		t.Fatalf("Verify(authentic fixture) error = %v, want nil", err)
	}
	return timeproofFuzzFixtures{
		response:  authentic.response,
		authority: authentic.request.Authority(), policy: timestamp.Policy(),
		request: authentic.request, nonce: authentic.request.Nonce(),
		evidence: timestamp.Evidence(), serial: timestamp.Serial(), timestamp: timestamp,
	}
}

func timeproofJSONSeedsForFuzz(t testing.TB, fixtures timeproofFuzzFixtures) []timeproofJSONSeed {
	t.Helper()
	return []timeproofJSONSeed{
		timeproofJSONSeedForFuzz(t, timeproofJSONDoorAuthority, fixtures.authority),
		timeproofJSONSeedForFuzz(t, timeproofJSONDoorTimestampPolicy, fixtures.policy),
		timeproofJSONSeedForFuzz(t, timeproofJSONDoorRequest, fixtures.request),
		timeproofJSONSeedForFuzz(t, timeproofJSONDoorNonce, fixtures.nonce),
		timeproofJSONSeedForFuzz(t, timeproofJSONDoorAuthorityEvidence, fixtures.evidence),
		timeproofJSONSeedForFuzz(t, timeproofJSONDoorSerialNumber, fixtures.serial),
		timeproofJSONSeedForFuzz(t, timeproofJSONDoorAuthoritativeTimestamp, fixtures.timestamp),
	}
}

func timeproofJSONSeedForFuzz(t testing.TB, door timeproofJSONDoor, value timeproofJSONValue) timeproofJSONSeed {
	t.Helper()
	document, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("timeproof fuzz seed MarshalJSON(%d) error = %v, want nil", door, err)
	}
	return timeproofJSONSeed{door: door, document: document}
}

func TestTimeproofExternalIngressFuzzInventoryMatchesProduction(t *testing.T) {
	t.Parallel()
	gotJSON, err := timeproofExportedJSONReceiverNames()
	if err != nil {
		t.Fatalf("timeproofExportedJSONReceiverNames() error = %v, want nil", err)
	}
	var wantJSON []string
	for door := range timeproofJSONDoorLimit {
		if door < timeproofJSONDoorUnknown+1 {
			continue
		}
		wantJSON = append(wantJSON, door.receiverName())
	}
	slices.Sort(wantJSON)
	if !slices.Equal(gotJSON, wantJSON) {
		t.Fatalf("public JSON receivers = %v, fuzz inventory = %v", gotJSON, wantJSON)
	}
	_ = FuzzVerifyFreeTSAResponse
	_ = FuzzVerifyDigiCertResponse
	_ = FuzzVerifyAuthorityBindingMutations
}

func timeproofExportedJSONReceiverNames() ([]string, error) {
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
			if !ok || (function.Name.Name != "UnmarshalJSON" && function.Name.Name != "Restore") || function.Recv == nil || len(function.Recv.List) != 1 {
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
